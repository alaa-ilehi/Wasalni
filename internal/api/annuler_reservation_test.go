package api_test

import (
	"context"
	"database/sql"
	"errors"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"github.com/alaa-ilehi/wasalni/internal/api"
	"github.com/alaa-ilehi/wasalni/internal/application"
	"github.com/alaa-ilehi/wasalni/internal/domain"
	"github.com/alaa-ilehi/wasalni/internal/infrastructure/sqlite"
)

type horlogeAnnulationFigee struct{ instant time.Time }

func (h horlogeAnnulationFigee) Maintenant() time.Time { return h.instant }

type notificateurAnnulationHTTP struct{ erreur error }

func (n notificateurAnnulationHTTP) NotifierAnnulation(context.Context, domain.Reservation) error {
	return n.erreur
}

func bancAnnulation(t *testing.T, maintenant time.Time) (*httptest.Server, *sql.DB) {
	t.Helper()
	db, err := sqlite.Ouvrir(context.Background(), filepath.Join(t.TempDir(), "annulations.db"))
	if err != nil {
		t.Fatalf("ouverture de la base : %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	depot := sqlite.NouveauReservationRepository(db)
	uc := application.NouveauAnnulerReservation(depot, horlogeAnnulationFigee{maintenant}, notificateurAnnulationHTTP{
		erreur: errors.New("service de notification indisponible"),
	})
	serveur := httptest.NewServer(api.Routes(api.Deps{Authentification: identitePartagee.auth, Annulations: uc}))
	t.Cleanup(serveur.Close)
	return serveur, db
}

func creerReservationAnnulable(t *testing.T, db *sql.DB, places int, delai time.Duration) (string, string) {
	t.Helper()
	trajets := sqlite.NouveauTrajetRepository(db)
	trajet, err := trajets.Publier(context.Background(), domain.Trajet{
		Conducteur: "bob", Origine: "Lyon", Destination: "Grenoble",
		Depart:      time.Date(2026, 12, 1, 8, 0, 0, 0, time.UTC),
		PlacesTotal: places, Places: places, DelaiAnnulation: delai, Statut: domain.TrajetPublie,
	})
	if err != nil {
		t.Fatalf("publication du trajet : %v", err)
	}
	reservation, err := sqlite.NouveauReservationRepository(db).Reserver(context.Background(), domain.Reservation{
		Trajet: trajet.Identifiant, Passager: "alice", Statut: domain.ReservationActive,
	})
	if err != nil {
		t.Fatalf("réservation : %v", err)
	}
	return trajet.Identifiant, reservation.Identifiant
}

func supprimerReservation(t *testing.T, serveur *httptest.Server, id, acteur string) int {
	t.Helper()
	requete, err := http.NewRequest(http.MethodDelete, serveur.URL+"/bookings/"+id, nil)
	if err != nil {
		t.Fatalf("requête : %v", err)
	}
	if acteur != "" {
		requete.Header.Set(api.EnTeteAutorisation, autorisationDe(t, acteur))
	}
	reponse, err := http.DefaultClient.Do(requete)
	if err != nil {
		t.Fatalf("appel : %v", err)
	}
	defer reponse.Body.Close()
	return reponse.StatusCode
}

func TestAnnulerReservationHTTPMetAJourEtatEtRendPlace(t *testing.T) {
	maintenant := time.Date(2026, 12, 1, 5, 0, 0, 0, time.UTC)
	serveur, db := bancAnnulation(t, maintenant)
	trajet, reservation := creerReservationAnnulable(t, db, 2, 2*time.Hour)

	if statut := supprimerReservation(t, serveur, reservation, "alice"); statut != http.StatusNoContent {
		t.Fatalf("statut = %d, attendu 204", statut)
	}
	var statut string
	if err := db.QueryRow(`SELECT status FROM bookings WHERE id = ?`, reservation).Scan(&statut); err != nil {
		t.Fatalf("lecture du statut : %v", err)
	}
	if statut != string(domain.ReservationAnnulee) {
		t.Errorf("statut réservation = %q, attendu CANCELLED", statut)
	}
	var places int
	if err := db.QueryRow(`SELECT seats_available FROM rides WHERE id = ?`, trajet).Scan(&places); err != nil {
		t.Fatalf("lecture des places : %v", err)
	}
	if places != 2 {
		t.Errorf("places disponibles = %d, attendu 2", places)
	}
	if statut := supprimerReservation(t, serveur, reservation, "alice"); statut != http.StatusConflict {
		t.Errorf("seconde annulation : statut = %d, attendu 409", statut)
	}
	if err := db.QueryRow(`SELECT seats_available FROM rides WHERE id = ?`, trajet).Scan(&places); err != nil {
		t.Fatalf("relecture des places : %v", err)
	}
	if places != 2 {
		t.Errorf("la répétition a changé le compteur : %d, attendu 2", places)
	}
}

func TestAnnulerReservationHTTPRefuseIdentiteProprietaireEtEcheance(t *testing.T) {
	maintenant := time.Date(2026, 12, 1, 6, 0, 0, 0, time.UTC)
	serveur, db := bancAnnulation(t, maintenant)
	_, reservation := creerReservationAnnulable(t, db, 2, 2*time.Hour)

	if statut := supprimerReservation(t, serveur, reservation, ""); statut != http.StatusUnauthorized {
		t.Errorf("sans identité : statut = %d, attendu 401", statut)
	}
	if statut := supprimerReservation(t, serveur, reservation, "bob"); statut != http.StatusNotFound {
		t.Errorf("autre passager : statut = %d, attendu 404", statut)
	}
	if statut := supprimerReservation(t, serveur, reservation, "alice"); statut != http.StatusConflict {
		t.Errorf("échéance atteinte : statut = %d, attendu 409", statut)
	}
}

func TestAnnulerReservationHTTPInconnue(t *testing.T) {
	serveur, _ := bancAnnulation(t, time.Date(2026, 12, 1, 5, 0, 0, 0, time.UTC))
	if statut := supprimerReservation(t, serveur, "absente", "alice"); statut != http.StatusNotFound {
		t.Errorf("réservation inconnue : statut = %d, attendu 404", statut)
	}
}
