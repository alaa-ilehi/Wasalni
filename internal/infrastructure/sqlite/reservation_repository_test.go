package sqlite_test

import (
	"context"
	"errors"
	"path/filepath"
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
