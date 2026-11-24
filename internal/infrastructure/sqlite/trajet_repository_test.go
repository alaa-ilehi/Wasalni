package sqlite_test

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/alaa-ilehi/wasalni/internal/domain"
	"github.com/alaa-ilehi/wasalni/internal/infrastructure/sqlite"
)

func baseTrajet(t *testing.T) (*sqlite.TrajetRepository, *sqlite.ReservationRepository, func()) {
	t.Helper()
	db, err := sqlite.Ouvrir(context.Background(), filepath.Join(t.TempDir(), "trajets.db"))
	if err != nil {
		t.Fatalf("ouverture de la base : %v", err)
	}
	return sqlite.NouveauTrajetRepository(db), sqlite.NouveauReservationRepository(db), func() { db.Close() }
}

func trajetAPublier() domain.Trajet {
	return domain.Trajet{
		Conducteur:      "bob",
		Origine:         "Lyon",
		Destination:     "Grenoble",
		Depart:          time.Date(2026, 12, 1, 8, 30, 0, 0, time.UTC),
		PlacesTotal:     3,
		Places:          3,
		Prix:            12.5,
		DelaiAnnulation: 90 * time.Minute,
		Statut:          domain.TrajetPublie,
	}
}

func TestPublierPuisRelireUnTrajet(t *testing.T) {
	ctx := context.Background()
	depot, _, fermer := baseTrajet(t)
	defer fermer()

	cree, err := depot.Publier(ctx, trajetAPublier())
	if err != nil {
		t.Fatalf("publication : %v", err)
	}
	if len(cree.Identifiant) != 32 {
		t.Fatalf("identifiant = %q, attendu 32 caractères hexadécimaux", cree.Identifiant)
	}
	if cree.CreeLe.IsZero() {
		t.Error("created_at non renseigné")
	}

	relu, err := depot.ParIdentifiant(ctx, cree.Identifiant)
	if err != nil {
		t.Fatalf("relecture : %v", err)
	}
	if relu.Conducteur != "bob" || relu.Origine != "Lyon" || relu.Destination != "Grenoble" {
		t.Errorf("trajet relu = %+v", relu)
	}
	if !relu.Depart.Equal(trajetAPublier().Depart) {
		t.Errorf("départ = %v, attendu %v", relu.Depart, trajetAPublier().Depart)
	}
	if relu.PlacesTotal != 3 || relu.Places != 3 {
		t.Errorf("places = %d/%d, attendu 3/3", relu.Places, relu.PlacesTotal)
	}
	if relu.Prix != 12.5 {
		t.Errorf("prix = %v, attendu 12.5", relu.Prix)
	}
	if relu.DelaiAnnulation != 90*time.Minute {
		t.Errorf("délai = %v, attendu 1h30", relu.DelaiAnnulation)
	}
	if relu.Statut != domain.TrajetPublie {
		t.Errorf("statut = %q, attendu %q", relu.Statut, domain.TrajetPublie)
	}
}

func TestParIdentifiantRefuseUnTrajetInconnu(t *testing.T) {
	ctx := context.Background()
	depot, _, fermer := baseTrajet(t)
	defer fermer()

	if _, err := depot.ParIdentifiant(ctx, "inconnu"); !errors.Is(err, domain.ErrTrajetIntrouvable) {
		t.Fatalf("erreur = %v, attendu ErrTrajetIntrouvable", err)
	}
}

// Le compteur de places ne bouge pas encore : la réservation s'écrit, le trajet reste plein.
// Ce test fixe la limite actuelle (ADR 0004) et devra être inversé avec le décrément atomique.
func TestReserverNeDecrementePasEncoreLesPlaces(t *testing.T) {
	ctx := context.Background()
	trajets, reservations, fermer := baseTrajet(t)
	defer fermer()

	cree, err := trajets.Publier(ctx, trajetAPublier())
	if err != nil {
		t.Fatalf("publication : %v", err)
	}
	if _, err := reservations.Reserver(ctx, domain.Reservation{
		Trajet: cree.Identifiant, Passager: "alice", Statut: domain.ReservationActive,
	}); err != nil {
		t.Fatalf("réservation : %v", err)
	}

	relu, err := trajets.ParIdentifiant(ctx, cree.Identifiant)
	if err != nil {
		t.Fatalf("relecture : %v", err)
	}
	if relu.Places != relu.PlacesTotal {
		t.Errorf("places disponibles = %d, attendu %d tant que le décrément n'existe pas",
			relu.Places, relu.PlacesTotal)
	}
}
