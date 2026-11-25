package api_test

import (
	"context"
	"encoding/json"
	"io"
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

// bancMes : le routeur UC10 sur une base temporaire ; les données sont écrites par les dépôts réels.
type bancMes struct {
	serveur      *httptest.Server
	identite     *identiteDeTest
	trajets      *sqlite.TrajetRepository
	reservations *sqlite.ReservationRepository
}

func nouveauBancMes(t *testing.T) *bancMes {
	t.Helper()
	ctx := context.Background()
	db, err := sqlite.Ouvrir(ctx, filepath.Join(t.TempDir(), "mes.db"))
	if err != nil {
		t.Fatalf("ouverture de la base : %v", err)
	}
	t.Cleanup(func() { db.Close() })

	identite := nouvelleIdentite()
	trajets := sqlite.NouveauTrajetRepository(db)
	reservations := sqlite.NouveauReservationRepository(db)
	serveur := httptest.NewServer(api.Routes(api.Deps{
		MesTrajets:       application.NouveauMesTrajets(trajets),
		MesReservations:  application.NouveauMesReservations(reservations, trajets),
		Authentification: identite.auth,
	}))
	t.Cleanup(serveur.Close)
	return &bancMes{serveur: serveur, identite: identite, trajets: trajets, reservations: reservations}
}

func (b *bancMes) publier(t *testing.T, conducteur string, depart time.Time) domain.Trajet {
	t.Helper()
	cree, err := b.trajets.Publier(context.Background(), domain.Trajet{
		Conducteur:      conducteur,
		Origine:         "Lyon",
		Destination:     "Grenoble",
		Depart:          depart,
		PlacesTotal:     3,
		Places:          3,
		Prix:            12.5,
		DelaiAnnulation: 2 * time.Hour,
		Statut:          domain.TrajetPublie,
	})
	if err != nil {
		t.Fatalf("publication : %v", err)
	}
	return cree
}

func (b *bancMes) reserver(t *testing.T, trajet, passager string) {
	t.Helper()
	if _, err := b.reservations.Reserver(context.Background(), domain.Reservation{
		Trajet: trajet, Passager: passager, Statut: domain.ReservationActive,
	}); err != nil {
		t.Fatalf("réservation : %v", err)
	}
}

// obtenir appelle une route de liste et rend le statut et le tableau décodé.
func (b *bancMes) obtenir(t *testing.T, chemin, autorisation string) (int, []map[string]any) {
	t.Helper()
	requete, err := http.NewRequest(http.MethodGet, b.serveur.URL+chemin, nil)
	if err != nil {
		t.Fatalf("requête : %v", err)
	}
	if autorisation != "" {
		requete.Header.Set(api.EnTeteAutorisation, autorisation)
	}
	reponse, err := http.DefaultClient.Do(requete)
	if err != nil {
		t.Fatalf("appel %s : %v", chemin, err)
	}
	defer reponse.Body.Close()
	brut, err := io.ReadAll(reponse.Body)
	if err != nil {
		t.Fatalf("lecture : %v", err)
	}
	if reponse.StatusCode != http.StatusOK {
		return reponse.StatusCode, nil
	}
	var liste []map[string]any
	if err := json.Unmarshal(brut, &liste); err != nil {
		t.Fatalf("décodage de %q : %v", brut, err)
	}
	return reponse.StatusCode, liste
}

func TestMesTrajetsListeCeuxDuConducteur(t *testing.T) {
	banc := nouveauBancMes(t)
	premier := banc.publier(t, "amine", time.Date(2026, 12, 1, 8, 30, 0, 0, time.UTC))
	second := banc.publier(t, "amine", time.Date(2026, 12, 3, 9, 0, 0, 0, time.UTC))
	banc.publier(t, "bob", time.Date(2026, 12, 2, 7, 0, 0, 0, time.UTC))

	statut, liste := banc.obtenir(t, "/me/rides", banc.identite.autorisation(t, "amine"))
	if statut != http.StatusOK {
		t.Fatalf("statut = %d, attendu 200", statut)
	}
	if len(liste) != 2 || liste[0]["id"] != premier.Identifiant || liste[1]["id"] != second.Identifiant {
		t.Fatalf("trajets = %v, attendu les deux d'amine triés par départ", liste)
	}
}

func TestMesTrajetsVidePourUnAutreConducteur(t *testing.T) {
	banc := nouveauBancMes(t)
	banc.publier(t, "amine", time.Date(2026, 12, 1, 8, 30, 0, 0, time.UTC))

	statut, liste := banc.obtenir(t, "/me/rides", banc.identite.autorisation(t, "bob"))
	if statut != http.StatusOK || len(liste) != 0 {
		t.Fatalf("statut = %d, liste = %v, attendu 200 et [] (isolation)", statut, liste)
	}
}

func TestMesReservationsListeCellesDuPassager(t *testing.T) {
	banc := nouveauBancMes(t)
	trajet := banc.publier(t, "bob", time.Date(2026, 12, 1, 8, 30, 0, 0, time.UTC))
	banc.reserver(t, trajet.Identifiant, "amine")

	statut, liste := banc.obtenir(t, "/me/bookings", banc.identite.autorisation(t, "amine"))
	if statut != http.StatusOK || len(liste) != 1 {
		t.Fatalf("statut = %d, réservations = %v, attendu une réservation", statut, liste)
	}
	if liste[0]["status"] != "ACTIVE" || liste[0]["ride_id"] != trajet.Identifiant {
		t.Fatalf("réservation = %v", liste[0])
	}
	trajetJoint, ok := liste[0]["ride"].(map[string]any)
	if !ok || trajetJoint["id"] != trajet.Identifiant {
		t.Fatalf("trajet joint = %v, attendu le trajet réservé", liste[0]["ride"])
	}

	// Isolation : le conducteur ne voit pas la réservation de son passager dans ses propres listes.
	if statut, liste := banc.obtenir(t, "/me/bookings", banc.identite.autorisation(t, "bob")); statut != http.StatusOK || len(liste) != 0 {
		t.Fatalf("réservations de bob = %v (statut %d), attendu []", liste, statut)
	}
}

// Un trajet supprimé disparaît de /me/rides, mais la réservation annulée par cette suppression reste listée.
func TestMesReservationsApresSuppressionDuTrajet(t *testing.T) {
	banc := nouveauBancMes(t)
	trajet := banc.publier(t, "bob", time.Date(2026, 12, 1, 8, 30, 0, 0, time.UTC))
	banc.reserver(t, trajet.Identifiant, "amine")
	if err := banc.trajets.Supprimer(context.Background(), trajet.Identifiant, "bob"); err != nil {
		t.Fatalf("suppression : %v", err)
	}

	if statut, liste := banc.obtenir(t, "/me/rides", banc.identite.autorisation(t, "bob")); statut != http.StatusOK || len(liste) != 0 {
		t.Fatalf("trajets de bob = %v (statut %d), attendu [] après suppression", liste, statut)
	}
	statut, liste := banc.obtenir(t, "/me/bookings", banc.identite.autorisation(t, "amine"))
	if statut != http.StatusOK || len(liste) != 1 || liste[0]["status"] != "CANCELLED" {
		t.Fatalf("réservations = %v (statut %d), attendu la réservation annulée", liste, statut)
	}
	if liste[0]["ride"] != nil {
		t.Fatalf("trajet joint = %v, attendu null pour un trajet supprimé", liste[0]["ride"])
	}
}

func TestMesListesRefusentSansJeton(t *testing.T) {
	banc := nouveauBancMes(t)
	for _, chemin := range []string{"/me/rides", "/me/bookings"} {
		if statut, _ := banc.obtenir(t, chemin, ""); statut != http.StatusUnauthorized {
			t.Fatalf("%s sans jeton : statut = %d, attendu 401", chemin, statut)
		}
	}
}
