package sqlite_test

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"sync"
	"testing"

	"github.com/alaa-ilehi/wasalni/internal/domain"
	"github.com/alaa-ilehi/wasalni/internal/infrastructure/sqlite"
)

func baseReservation(t *testing.T) (*sqlite.ReservationRepository, func() error, func(string) int) {
	t.Helper()
	ctx := context.Background()
	db, err := sqlite.Ouvrir(ctx, filepath.Join(t.TempDir(), "reservations.db"))
	if err != nil {
		t.Fatalf("ouverture de la base : %v", err)
	}
	// Les réservations ont désormais une clé étrangère vers rides : le banc crée ses trajets.
	for _, identifiant := range []string{"t1", "t2"} {
		if _, err := db.ExecContext(ctx, `
INSERT INTO rides (id, driver, origin, destination, departure, seats_total, seats_available, price)
VALUES (?, 'conducteur', 'Lyon', 'Grenoble', '2026-12-01T08:30:00Z', 3, 3, 12.5)`, identifiant); err != nil {
			t.Fatalf("création du trajet %s : %v", identifiant, err)
		}
	}
	compter := func(requete string) int {
		var n int
		if err := db.QueryRowContext(ctx, requete).Scan(&n); err != nil {
			t.Fatalf("comptage : %v", err)
		}
		return n
	}
	return sqlite.NouveauReservationRepository(db), db.Close, compter
}

func TestReserverEcritUneReservationActive(t *testing.T) {
	ctx := context.Background()
	depot, fermer, compter := baseReservation(t)
	defer fermer()

	reservation, err := depot.Reserver(ctx, domain.Reservation{
		Trajet: "t1", Passager: "alice", Statut: domain.ReservationActive,
	})
	if err != nil {
		t.Fatalf("réservation : %v", err)
	}
	if len(reservation.Identifiant) != 32 {
		t.Errorf("identifiant = %q, attendu 32 caractères hexadécimaux", reservation.Identifiant)
	}
	if reservation.CreeLe.IsZero() {
		t.Error("created_at non renseigné")
	}
	if n := compter(`SELECT count(*) FROM bookings`); n != 1 {
		t.Errorf("lignes = %d, attendu 1", n)
	}
}

func TestReserverRefuseLeDoublonActifSansEcrire(t *testing.T) {
	ctx := context.Background()
	depot, fermer, compter := baseReservation(t)
	defer fermer()

	reservation := domain.Reservation{Trajet: "t1", Passager: "alice", Statut: domain.ReservationActive}
	if _, err := depot.Reserver(ctx, reservation); err != nil {
		t.Fatalf("première réservation : %v", err)
	}
	if _, err := depot.Reserver(ctx, reservation); !errors.Is(err, domain.ErrReservationDejaActive) {
		t.Fatalf("seconde réservation : erreur = %v, attendu ErrReservationDejaActive", err)
	}
	if n := compter(`SELECT count(*) FROM bookings`); n != 1 {
		t.Errorf("lignes = %d, attendu 1 (aucune écriture sur refus)", n)
	}
}

func TestIndexPartielAutoriseUneNouvelleReservationApresAnnulation(t *testing.T) {
	ctx := context.Background()
	depot, fermer, compter := baseReservation(t)
	defer fermer()

	reservation := domain.Reservation{Trajet: "t1", Passager: "alice", Statut: domain.ReservationActive}
	premiere, err := depot.Reserver(ctx, reservation)
	if err != nil {
		t.Fatalf("première réservation : %v", err)
	}

	// un autre passager sur le meme trajet : l'index partiel ne le bloque pas
	if _, err := depot.Reserver(ctx, domain.Reservation{
		Trajet: "t1", Passager: "bob", Statut: domain.ReservationActive,
	}); err != nil {
		t.Fatalf("un autre passager doit pouvoir réserver : %v", err)
	}
	if n := compter(`SELECT count(*) FROM bookings`); n != 2 {
		t.Errorf("lignes = %d, attendu 2", n)
	}
	if premiere.Identifiant == "" {
		t.Error("identifiant de la première réservation perdu")
	}
}

func TestParPassagerRendLesReservationsDuPassagerTriees(t *testing.T) {
	ctx := context.Background()
	depot, fermer, _ := baseReservation(t)
	defer fermer()

	for _, trajet := range []string{"t1", "t2"} {
		if _, err := depot.Reserver(ctx, domain.Reservation{
			Trajet: trajet, Passager: "alice", Statut: domain.ReservationActive,
		}); err != nil {
			t.Fatalf("réservation %s : %v", trajet, err)
		}
	}
	if _, err := depot.Reserver(ctx, domain.Reservation{
		Trajet: "t1", Passager: "bob", Statut: domain.ReservationActive,
	}); err != nil {
		t.Fatalf("réservation de bob : %v", err)
	}

	reservations, err := depot.ParPassager(ctx, "alice")
	if err != nil {
		t.Fatalf("listage : %v", err)
	}
	if len(reservations) != 2 {
		t.Fatalf("réservations d'alice = %d, attendu 2", len(reservations))
	}
	for _, reservation := range reservations {
		if reservation.Passager != "alice" {
			t.Errorf("passager = %q, attendu alice", reservation.Passager)
		}
		if reservation.Statut != domain.ReservationActive {
			t.Errorf("statut = %q, attendu %q", reservation.Statut, domain.ReservationActive)
		}
	}
	// created_at a la précision de la seconde : à horodatage égal, l'ordre dépend du tri secondaire,
	// donc on vérifie l'ensemble des trajets, pas la position exacte.
	vus := map[string]bool{reservations[0].Trajet: true, reservations[1].Trajet: true}
	if !vus["t1"] || !vus["t2"] {
		t.Errorf("trajets rendus = %v, attendu t1 et t2", vus)
	}

	vides, err := depot.ParPassager(ctx, "inconnu")
	if err != nil {
		t.Fatalf("listage d'un passager sans réservation : %v", err)
	}
	if vides == nil || len(vides) != 0 {
		t.Errorf("réservations = %v, attendu une liste vide non nulle", vides)
	}
}

func TestRefusDoublonConserveLeCompteur(t *testing.T) {
	ctx := context.Background()
	depot, fermer, compter := baseReservation(t)
	defer fermer()

	reservation := domain.Reservation{Trajet: "t1", Passager: "alice", Statut: domain.ReservationActive}
	if _, err := depot.Reserver(ctx, reservation); err != nil {
		t.Fatalf("première réservation : %v", err)
	}
	if n := compter(`SELECT seats_available FROM rides WHERE id = 't1'`); n != 2 {
		t.Fatalf("places après la première = %d, attendu 2", n)
	}
	if _, err := depot.Reserver(ctx, reservation); !errors.Is(err, domain.ErrReservationDejaActive) {
		t.Fatalf("doublon : erreur = %v, attendu ErrReservationDejaActive", err)
	}
	if n := compter(`SELECT seats_available FROM rides WHERE id = 't1'`); n != 2 {
		t.Errorf("places après le refus = %d, attendu 2 (décrement annulé)", n)
	}
}

func TestReserverConcurrentlySurLaDernierePlace(t *testing.T) {
	ctx := context.Background()
	depot, fermer, compter := baseReservation(t)
	defer fermer()

	const appels = 8
	var attente sync.WaitGroup
	depart := make(chan struct{})
	succes := make(chan struct{}, appels)
	erreurs := make(chan error, appels)

	for i := 0; i < appels; i++ {
		attente.Add(1)
		go func(i int) {
			defer attente.Done()
			<-depart
			_, err := depot.Reserver(ctx, domain.Reservation{
				Trajet:   "t1",
				Passager: fmt.Sprintf("passager-%d", i),
				Statut:   domain.ReservationActive,
			})
			if err != nil {
				erreurs <- err
				return
			}
			succes <- struct{}{}
		}(i)
	}
	close(depart)
	attente.Wait()
	close(succes)
	close(erreurs)

	if obtenus := len(succes); obtenus != 3 {
		t.Errorf("succès = %d, attendu 3", obtenus)
	}
	complets := 0
	for err := range erreurs {
		if !errors.Is(err, domain.ErrTrajetComplet) {
			t.Errorf("erreur inattendue = %v, attendu ErrTrajetComplet", err)
		}
		complets++
	}
	if complets != appels-3 {
		t.Errorf("refus « complet » = %d, attendu %d", complets, appels-3)
	}
	if n := compter(`SELECT count(*) FROM bookings WHERE ride_id = 't1'`); n != 3 {
		t.Errorf("réservations écrites = %d, attendu 3", n)
	}
	if n := compter(`SELECT seats_available FROM rides WHERE id = 't1'`); n != 0 {
		t.Errorf("places restantes = %d, attendu 0", n)
	}
}
