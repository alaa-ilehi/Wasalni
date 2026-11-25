package sqlite_test

import (
	"context"
	"errors"
	"path/filepath"
	"sync"
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

func TestTerminerTrajetVerifieProprietaireEtTransition(t *testing.T) {
	ctx := context.Background()
	depot, _, fermer := baseTrajet(t)
	defer fermer()
	cree, err := depot.Publier(ctx, trajetAPublier())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := depot.Terminer(ctx, cree.Identifiant, "alice"); !errors.Is(err, domain.ErrTrajetNonProprietaire) {
		t.Fatalf("erreur mauvais conducteur = %v", err)
	}
	if _, err := depot.Terminer(ctx, "inconnu", "bob"); !errors.Is(err, domain.ErrTrajetIntrouvable) {
		t.Fatalf("erreur trajet inconnu = %v", err)
	}

	termine, err := depot.Terminer(ctx, cree.Identifiant, "bob")
	if err != nil {
		t.Fatalf("terminaison : %v", err)
	}
	if termine.Statut != domain.TrajetTermine {
		t.Fatalf("statut retourné = %q", termine.Statut)
	}
	relu, err := depot.ParIdentifiant(ctx, cree.Identifiant)
	if err != nil || relu.Statut != domain.TrajetTermine {
		t.Fatalf("trajet relu = %+v, erreur = %v", relu, err)
	}
	if _, err := depot.Terminer(ctx, cree.Identifiant, "bob"); !errors.Is(err, domain.ErrTrajetDejaTermine) {
		t.Fatalf("erreur seconde terminaison = %v", err)
	}
}

func TestTerminerTrajetConcurrentUneSeuleTransition(t *testing.T) {
	depot, _, fermer := baseTrajet(t)
	defer fermer()
	cree, err := depot.Publier(context.Background(), trajetAPublier())
	if err != nil {
		t.Fatal(err)
	}

	var attente sync.WaitGroup
	erreurs := make(chan error, 2)
	for range 2 {
		attente.Add(1)
		go func() {
			defer attente.Done()
			_, err := depot.Terminer(context.Background(), cree.Identifiant, "bob")
			erreurs <- err
		}()
	}
	attente.Wait()
	close(erreurs)
	succes, dejaTermine := 0, 0
	for err := range erreurs {
		switch {
		case err == nil:
			succes++
		case errors.Is(err, domain.ErrTrajetDejaTermine):
			dejaTermine++
		default:
			t.Errorf("erreur inattendue : %v", err)
		}
	}
	if succes != 1 || dejaTermine != 1 {
		t.Fatalf("succès = %d, déjà terminé = %d; attendu 1 de chaque", succes, dejaTermine)
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

func TestUC9ModifierAjustePlacesEtSupprimerAnnuleReservations(t *testing.T) {
	ctx := context.Background()
	depot, reservations, fermer := baseTrajet(t)
	defer fermer()
	cree, err := depot.Publier(ctx, trajetAPublier())
	if err != nil {
		t.Fatal(err)
	}
	for _, passager := range []string{"alice", "carol"} {
		if _, err := reservations.Reserver(ctx, domain.Reservation{Trajet: cree.Identifiant, Passager: passager, Statut: domain.ReservationActive}); err != nil {
			t.Fatal(err)
		}
	}
	modifie := trajetAPublier()
	modifie.Identifiant = cree.Identifiant
	modifie.Origine = "Paris"
	modifie.PlacesTotal = 2
	misAJour, err := depot.Modifier(ctx, modifie, "bob")
	if err != nil {
		t.Fatal(err)
	}
	if misAJour.Origine != "Paris" || misAJour.Places != 0 {
		t.Fatalf("trajet modifié = %+v", misAJour)
	}
	modifie.PlacesTotal = 1
	if _, err := depot.Modifier(ctx, modifie, "bob"); !errors.Is(err, domain.ErrPlacesInsuffisantes) {
		t.Fatalf("places insuffisantes: %v", err)
	}
	if err := depot.Supprimer(ctx, cree.Identifiant, "bob"); err != nil {
		t.Fatal(err)
	}
	if _, err := depot.ParIdentifiant(ctx, cree.Identifiant); !errors.Is(err, domain.ErrTrajetIntrouvable) {
		t.Fatalf("trajet supprimé visible: %v", err)
	}
	listed, err := reservations.ParPassager(ctx, "alice")
	if err != nil || len(listed) != 1 || listed[0].Statut != domain.ReservationAnnulee {
		t.Fatalf("réservation après suppression = %+v, %v", listed, err)
	}
	if _, err := reservations.Reserver(ctx, domain.Reservation{Trajet: cree.Identifiant, Passager: "dave", Statut: domain.ReservationActive}); !errors.Is(err, domain.ErrTrajetComplet) {
		t.Fatalf("nouvelle réservation après suppression: %v", err)
	}
}

// UC10 : les trajets d'un conducteur, supprimés exclus, du départ le plus proche au plus lointain.
func TestParConducteurListeLesTrajetsNonSupprimes(t *testing.T) {
	ctx := context.Background()
	depot, _, fermer := baseTrajet(t)
	defer fermer()

	publier := func(conducteur, origine string, depart time.Time) domain.Trajet {
		t.Helper()
		trajet := trajetAPublier()
		trajet.Conducteur = conducteur
		trajet.Origine = origine
		trajet.Depart = depart
		cree, err := depot.Publier(ctx, trajet)
		if err != nil {
			t.Fatal(err)
		}
		return cree
	}
	premier := publier("bob", "Lyon", time.Date(2026, 12, 1, 8, 30, 0, 0, time.UTC))
	second := publier("bob", "Paris", time.Date(2026, 12, 3, 9, 0, 0, 0, time.UTC))
	publier("alice", "Nice", time.Date(2026, 12, 2, 7, 0, 0, 0, time.UTC))

	liste, err := depot.ParConducteur(ctx, "bob")
	if err != nil {
		t.Fatalf("lecture : %v", err)
	}
	if len(liste) != 2 || liste[0].Identifiant != premier.Identifiant || liste[1].Identifiant != second.Identifiant {
		t.Fatalf("trajets de bob = %+v, attendu les deux siens triés par départ", liste)
	}

	// Un trajet supprimé (UC9) disparaît de la liste, et un conducteur inconnu rend une liste vide.
	if err := depot.Supprimer(ctx, premier.Identifiant, "bob"); err != nil {
		t.Fatal(err)
	}
	liste, err = depot.ParConducteur(ctx, "bob")
	if err != nil || len(liste) != 1 || liste[0].Identifiant != second.Identifiant {
		t.Fatalf("trajets après suppression = %+v, %v", liste, err)
	}
	if liste, err := depot.ParConducteur(ctx, "inconnu"); err != nil || len(liste) != 0 {
		t.Fatalf("conducteur inconnu = %+v, %v, attendu vide", liste, err)
	}
}

// La suppression annule les réservations actives : leurs places doivent revenir au trajet, comme dans UC5.
func TestSupprimerRendLesPlacesDesReservationsAnnulees(t *testing.T) {
	ctx := context.Background()
	db, err := sqlite.Ouvrir(ctx, filepath.Join(t.TempDir(), "suppression.db"))
	if err != nil {
		t.Fatalf("ouverture de la base : %v", err)
	}
	defer db.Close()
	depot := sqlite.NouveauTrajetRepository(db)
	reservations := sqlite.NouveauReservationRepository(db)

	placesDuTrajet := func(identifiant string) (int, int) {
		t.Helper()
		var places, total int
		if err := db.QueryRowContext(ctx, "SELECT seats_available, seats_total FROM rides WHERE id=?", identifiant).Scan(&places, &total); err != nil {
			t.Fatalf("lecture directe du trajet : %v", err)
		}
		return places, total
	}

	cree, err := depot.Publier(ctx, trajetAPublier())
	if err != nil {
		t.Fatal(err)
	}
	for _, passager := range []string{"alice", "carol"} {
		if _, err := reservations.Reserver(ctx, domain.Reservation{Trajet: cree.Identifiant, Passager: passager, Statut: domain.ReservationActive}); err != nil {
			t.Fatal(err)
		}
	}
	if places, total := placesDuTrajet(cree.Identifiant); places != 1 || total != 3 {
		t.Fatalf("avant suppression : places = %d/%d, attendu 1/3", places, total)
	}

	if err := depot.Supprimer(ctx, cree.Identifiant, "bob"); err != nil {
		t.Fatal(err)
	}
	// La lecture publique masque un trajet supprimé : le compteur se vérifie en SQL, sur la ligne conservée.
	if places, total := placesDuTrajet(cree.Identifiant); places != total {
		t.Fatalf("après suppression : places = %d/%d, attendu places = total", places, total)
	}
	if err := depot.Supprimer(ctx, cree.Identifiant, "bob"); !errors.Is(err, domain.ErrTrajetIntrouvable) {
		t.Fatalf("seconde suppression : %v, attendu ErrTrajetIntrouvable", err)
	}

	// Un trajet sans réservation active ne bouge pas.
	sansReservation, err := depot.Publier(ctx, trajetAPublier())
	if err != nil {
		t.Fatal(err)
	}
	if err := depot.Supprimer(ctx, sansReservation.Identifiant, "bob"); err != nil {
		t.Fatal(err)
	}
	if places, total := placesDuTrajet(sansReservation.Identifiant); places != total {
		t.Fatalf("suppression sans réservation : places = %d/%d, attendu places = total", places, total)
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
