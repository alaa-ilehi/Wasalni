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

// Le compteur de places est consommé par la réservation : la lecture suivante voit 2 places sur 3.
func TestReserverDecrementeLesPlaces(t *testing.T) {
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
	if relu.Places != relu.PlacesTotal-1 {
		t.Errorf("places disponibles = %d, attendu %d", relu.Places, relu.PlacesTotal-1)
	}
}

func publierPourRecherche(t *testing.T, depot *sqlite.TrajetRepository, origine, destination string, depart time.Time, statut domain.StatutTrajet) {
	t.Helper()
	trajet := trajetAPublier()
	trajet.Origine = origine
	trajet.Destination = destination
	trajet.Depart = depart
	trajet.Statut = statut
	if _, err := depot.Publier(context.Background(), trajet); err != nil {
		t.Fatalf("publication de %s → %s : %v", origine, destination, err)
	}
}

func identifiants(trajets []domain.Trajet) []string {
	ids := make([]string, 0, len(trajets))
	for _, trajet := range trajets {
		ids = append(ids, trajet.Origine+"@"+trajet.Depart.Format("2006-01-02T15:04"))
	}
	return ids
}

func TestRechercherFiltreEtTrieLesTrajetsPublies(t *testing.T) {
	ctx := context.Background()
	depot, _, fermer := baseTrajet(t)
	defer fermer()

	publierPourRecherche(t, depot, "Lyon", "Grenoble", time.Date(2026, 12, 1, 10, 0, 0, 0, time.UTC), domain.TrajetPublie)
	publierPourRecherche(t, depot, "Paris", "Lyon", time.Date(2026, 12, 1, 8, 0, 0, 0, time.UTC), domain.TrajetPublie)
	publierPourRecherche(t, depot, "lyon", "grenoble", time.Date(2026, 11, 30, 9, 0, 0, 0, time.UTC), domain.TrajetPublie)
	publierPourRecherche(t, depot, "Lyon", "Grenoble", time.Date(2026, 12, 1, 7, 0, 0, 0, time.UTC), domain.TrajetTermine)

	cas := map[string]struct {
		criteres domain.CriteresRecherche
		attendu  []string
	}{
		"sans filtre, terminé exclu et tri par départ": {
			criteres: domain.CriteresRecherche{},
			attendu: []string{
				"lyon@2026-11-30T09:00",
				"Paris@2026-12-01T08:00",
				"Lyon@2026-12-01T10:00",
			},
		},
		"origine insensible à la casse": {
			criteres: domain.CriteresRecherche{Origine: "LYON"},
			attendu:  []string{"lyon@2026-11-30T09:00", "Lyon@2026-12-01T10:00"},
		},
		"destination insensible à la casse": {
			criteres: domain.CriteresRecherche{Destination: "grenoble"},
			attendu:  []string{"lyon@2026-11-30T09:00", "Lyon@2026-12-01T10:00"},
		},
		"date calendaire": {
			criteres: domain.CriteresRecherche{Date: "2026-12-01"},
			attendu:  []string{"Paris@2026-12-01T08:00", "Lyon@2026-12-01T10:00"},
		},
		"tous les filtres": {
			criteres: domain.CriteresRecherche{Origine: "Lyon", Destination: "Grenoble", Date: "2026-12-01"},
			attendu:  []string{"Lyon@2026-12-01T10:00"},
		},
	}
	for nom, cas := range cas {
		t.Run(nom, func(t *testing.T) {
			trajets, err := depot.Rechercher(ctx, cas.criteres)
			if err != nil {
				t.Fatalf("recherche : %v", err)
			}
			obtenu := identifiants(trajets)
			if len(obtenu) != len(cas.attendu) {
				t.Fatalf("résultats = %v, attendu %v", obtenu, cas.attendu)
			}
			for i := range cas.attendu {
				if obtenu[i] != cas.attendu[i] {
					t.Errorf("résultat %d = %q, attendu %q (liste %v)", i, obtenu[i], cas.attendu[i], obtenu)
				}
			}
		})
	}
}

func TestRechercherSansResultatRendUneListeVide(t *testing.T) {
	depot, _, fermer := baseTrajet(t)
	defer fermer()

	trajets, err := depot.Rechercher(context.Background(), domain.CriteresRecherche{Origine: "Nullepart"})
	if err != nil {
		t.Fatalf("recherche : %v", err)
	}
	if trajets == nil || len(trajets) != 0 {
		t.Errorf("trajets = %v, attendu une liste vide non nil", trajets)
	}
}
