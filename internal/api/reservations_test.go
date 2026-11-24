package api_test

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/alaa-ilehi/wasalni/internal/api"
	"github.com/alaa-ilehi/wasalni/internal/application"
	"github.com/alaa-ilehi/wasalni/internal/domain"
	"github.com/alaa-ilehi/wasalni/internal/infrastructure/sqlite"
)

// trajetsEnMemoire : le port TrajetRepository en mémoire, alimenté par le test.
type trajetsEnMemoire map[string]domain.Trajet

func (t trajetsEnMemoire) ParIdentifiant(_ context.Context, identifiant string) (domain.Trajet, error) {
	trajet, ok := t[identifiant]
	if !ok {
		return domain.Trajet{}, domain.ErrTrajetIntrouvable
	}
	return trajet, nil
}

func (t trajetsEnMemoire) Publier(_ context.Context, trajet domain.Trajet) (domain.Trajet, error) {
	if trajet.Identifiant == "" {
		trajet.Identifiant = "t-" + trajet.Origine
	}
	t[trajet.Identifiant] = trajet
	return trajet, nil
}

type notificateurMuet struct{}

func (notificateurMuet) NotifierReservation(context.Context, domain.Reservation) error { return nil }

func bancReservations(t *testing.T, trajets trajetsEnMemoire) *httptest.Server {
	t.Helper()
	db, err := sqlite.Ouvrir(context.Background(), filepath.Join(t.TempDir(), "reservations.db"))
	if err != nil {
		t.Fatalf("ouverture de la base : %v", err)
	}
	t.Cleanup(func() { db.Close() })

	reserver := application.NouveauReserverPlace(
		trajets,
		sqlite.NouveauReservationRepository(db),
		notificateurMuet{},
	)
	serveur := httptest.NewServer(api.Routes(api.Deps{Reservations: reserver}))
	t.Cleanup(serveur.Close)
	return serveur
}

func postReservation(t *testing.T, serveur *httptest.Server, trajet, acteur string) (int, map[string]any) {
	t.Helper()
	requete, err := http.NewRequest(http.MethodPost, serveur.URL+"/rides/"+trajet+"/bookings", nil)
	if err != nil {
		t.Fatalf("requête : %v", err)
	}
	if acteur != "" {
		requete.Header.Set(api.EnTeteIdentite, acteur)
	}
	reponse, err := http.DefaultClient.Do(requete)
	if err != nil {
		t.Fatalf("appel : %v", err)
	}
	defer reponse.Body.Close()
	brut, _ := io.ReadAll(reponse.Body)
	var charge map[string]any
	if len(brut) > 0 {
		if err := json.Unmarshal(brut, &charge); err != nil {
			t.Fatalf("réponse non JSON (%d) : %s", reponse.StatusCode, brut)
		}
	}
	return reponse.StatusCode, charge
}

func TestReservationIdentiteObligatoire(t *testing.T) {
	serveur := bancReservations(t, trajetsEnMemoire{"t1": {Identifiant: "t1", Conducteur: "bob", Places: 2}})
	statut, charge := postReservation(t, serveur, "t1", "")
	if statut != http.StatusUnauthorized {
		t.Errorf("statut = %d, attendu 401", statut)
	}
	if charge["error"] == nil {
		t.Error("message d'erreur absent")
	}
}

func TestReservationRefusMetier(t *testing.T) {
	trajets := trajetsEnMemoire{
		"t1":      {Identifiant: "t1", Conducteur: "bob", Places: 2},
		"complet": {Identifiant: "complet", Conducteur: "bob", Places: 0},
	}
	cas := []struct {
		libelle string
		trajet  string
		acteur  string
		attendu int
	}{
		{"trajet inconnu", "inconnu", "alice", http.StatusNotFound},
		{"conducteur", "t1", "bob", http.StatusBadRequest},
		{"trajet complet", "complet", "alice", http.StatusConflict},
	}
	for _, c := range cas {
		serveur := bancReservations(t, trajets)
		statut, charge := postReservation(t, serveur, c.trajet, c.acteur)
		if statut != c.attendu {
			t.Errorf("%s : statut = %d, attendu %d (corps %v)", c.libelle, statut, c.attendu, charge)
		}
	}
}

func TestReservationSuccesPuisDoublon(t *testing.T) {
	serveur := bancReservations(t, trajetsEnMemoire{
		"t1": {Identifiant: "t1", Conducteur: "bob", Places: 2},
	})

	statut, reservation := postReservation(t, serveur, "t1", "alice")
	if statut != http.StatusCreated {
		t.Fatalf("création : statut = %d, attendu 201 (corps %v)", statut, reservation)
	}
	if reservation["ride_id"] != "t1" || reservation["passenger"] != "alice" || reservation["status"] != "ACTIVE" {
		t.Errorf("réservation = %v", reservation)
	}
	if reservation["id"] == "" || reservation["created_at"] == "" {
		t.Errorf("identifiant ou horodatage absent : %v", reservation)
	}

	statut, charge := postReservation(t, serveur, "t1", "alice")
	if statut != http.StatusConflict {
		t.Errorf("doublon : statut = %d, attendu 409 (corps %v)", statut, charge)
	}
	if message, _ := charge["error"].(string); !strings.Contains(message, "déjà") {
		t.Errorf("message du doublon = %q", message)
	}

	// un autre passager obtient bien sa place
	if statut, _ := postReservation(t, serveur, "t1", "carol"); statut != http.StatusCreated {
		t.Errorf("second passager : statut = %d, attendu 201", statut)
	}
}
