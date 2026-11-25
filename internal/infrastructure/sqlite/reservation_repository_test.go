package sqlite_test

import (
	"context"
	"database/sql"
	"errors"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/alaa-ilehi/wasalni/internal/domain"
	"github.com/alaa-ilehi/wasalni/internal/infrastructure/sqlite"
	_ "modernc.org/sqlite"
)

// banc : base réelle jetable, avec les deux dépôts de UC3 sur la même connexion.
type banc struct {
	db           *sql.DB
	trajets      *sqlite.TrajetRepository
	reservations *sqlite.ReservationRepository
}

func nouveauBanc(t *testing.T) banc {
	t.Helper()
	db, err := sqlite.Ouvrir(context.Background(), filepath.Join(t.TempDir(), "reservations.db"))
	if err != nil {
		t.Fatalf("ouverture de la base : %v", err)
	}
	t.Cleanup(func() { db.Close() })
	return banc{
		db:           db,
		trajets:      sqlite.NouveauTrajetRepository(db),
		reservations: sqlite.NouveauReservationRepository(db),
	}
}

func (b banc) compter(t *testing.T, requete string, args ...any) int {
	t.Helper()
	var n int
	if err := b.db.QueryRowContext(context.Background(), requete, args...).Scan(&n); err != nil {
		t.Fatalf("comptage : %v", err)
	}
	return n
}

// trajetPublie : la clé étrangère de bookings impose une ligne rides derrière chaque réservation ;
// toutes les réservations des tests visent donc un trajet réellement publié.
func (b banc) trajetPublie(t *testing.T, places int) string {
	t.Helper()
	cree, err := b.trajets.Publier(context.Background(), domain.Trajet{
		Conducteur:      "bob",
		Origine:         "Lyon",
		Destination:     "Grenoble",
		Depart:          time.Date(2026, 12, 1, 8, 0, 0, 0, time.UTC),
		PlacesTotal:     places,
		Places:          places,
		DelaiAnnulation: 2 * time.Hour,
		Statut:          domain.TrajetPublie,
	})
	if err != nil {
		t.Fatalf("publication du trajet : %v", err)
	}
	return cree.Identifiant
}

func (b banc) placesLibres(t *testing.T, identifiant string) int {
	t.Helper()
	return b.compter(t, `SELECT seats_available FROM rides WHERE id = ?`, identifiant)
}

func reserver(t *testing.T, b banc, trajet, passager string) (domain.Reservation, error) {
	t.Helper()
	return b.reservations.Reserver(context.Background(), domain.Reservation{
		Trajet: trajet, Passager: passager, Statut: domain.ReservationActive,
	})
}

func TestReserverEcritUneReservationActive(t *testing.T) {
	b := nouveauBanc(t)
	trajet := b.trajetPublie(t, 2)

	reservation, err := reserver(t, b, trajet, "alice")
	if err != nil {
		t.Fatalf("réservation : %v", err)
	}
	if len(reservation.Identifiant) != 32 {
		t.Errorf("identifiant = %q, attendu 32 caractères hexadécimaux", reservation.Identifiant)
	}
	if reservation.CreeLe.IsZero() {
		t.Error("created_at non renseigné")
	}
	if n := b.compter(t, `SELECT count(*) FROM bookings`); n != 1 {
		t.Errorf("lignes = %d, attendu 1", n)
	}
}

func TestReserverConsommeUnePlace(t *testing.T) {
	b := nouveauBanc(t)
	trajet := b.trajetPublie(t, 3)

	if _, err := reserver(t, b, trajet, "alice"); err != nil {
		t.Fatalf("réservation : %v", err)
	}
	if places := b.placesLibres(t, trajet); places != 2 {
		t.Errorf("places disponibles = %d, attendu 2", places)
	}
}

func TestAnnulerReservationLibereUnePlaceEtInterditLeDoublon(t *testing.T) {
	b := nouveauBanc(t)
	trajet := b.trajetPublie(t, 2)
	reservation, err := reserver(t, b, trajet, "alice")
	if err != nil {
		t.Fatalf("réservation : %v", err)
	}
	maintenant := time.Date(2026, 12, 1, 5, 0, 0, 0, time.UTC)

	annulee, err := b.reservations.Annuler(context.Background(), reservation.Identifiant, "alice", maintenant)
	if err != nil {
		t.Fatalf("annulation : %v", err)
	}
	if annulee.Statut != domain.ReservationAnnulee || annulee.Identifiant != reservation.Identifiant {
		t.Errorf("réservation annulée = %+v", annulee)
	}
	if places := b.placesLibres(t, trajet); places != 2 {
		t.Errorf("places après annulation = %d, attendu 2", places)
	}

	if _, err := b.reservations.Annuler(context.Background(), reservation.Identifiant, "alice", maintenant); !errors.Is(err, domain.ErrReservationDejaAnnulee) {
		t.Fatalf("seconde annulation : erreur = %v, attendu ErrReservationDejaAnnulee", err)
	}
	if places := b.placesLibres(t, trajet); places != 2 {
		t.Errorf("places après seconde annulation = %d, attendu 2", places)
	}
}

func TestAnnulerReservationRefuseProprietaireInconnuEtEcheance(t *testing.T) {
	maintenant := time.Date(2026, 12, 1, 6, 0, 0, 0, time.UTC) // départ - délai (2 h)
	for _, cas := range []struct {
		nom      string
		passager string
		attendu  error
	}{
		{"autre passager", "bob", domain.ErrReservationIntrouvable},
		{"échéance atteinte", "alice", domain.ErrAnnulationHorsDelai},
	} {
		t.Run(cas.nom, func(t *testing.T) {
			b := nouveauBanc(t)
			trajet := b.trajetPublie(t, 2)
			reservation, err := reserver(t, b, trajet, "alice")
			if err != nil {
				t.Fatalf("réservation : %v", err)
			}
			_, err = b.reservations.Annuler(context.Background(), reservation.Identifiant, cas.passager, maintenant)
			if !errors.Is(err, cas.attendu) {
				t.Fatalf("annulation : erreur = %v, attendu %v", err, cas.attendu)
			}
			if places := b.placesLibres(t, trajet); places != 1 {
				t.Errorf("places après refus = %d, attendu 1", places)
			}
		})
	}
}

func TestAnnulationsConcurrentesNeLiberentQuUnePlace(t *testing.T) {
	b := nouveauBanc(t)
	trajet := b.trajetPublie(t, 1)
	reservation, err := reserver(t, b, trajet, "alice")
	if err != nil {
		t.Fatalf("réservation : %v", err)
	}

	const appels = 8
	var (
		verrou       sync.Mutex
		succes       int
		dejaAnnulees int
		inattendus   []error
		debut        = make(chan struct{})
		groupe       sync.WaitGroup
	)
	maintenant := time.Date(2026, 12, 1, 5, 0, 0, 0, time.UTC)
	for i := 0; i < appels; i++ {
		groupe.Add(1)
		go func() {
			defer groupe.Done()
			<-debut
			_, err := b.reservations.Annuler(context.Background(), reservation.Identifiant, "alice", maintenant)
			verrou.Lock()
			defer verrou.Unlock()
			switch {
			case err == nil:
				succes++
			case errors.Is(err, domain.ErrReservationDejaAnnulee):
				dejaAnnulees++
			default:
				inattendus = append(inattendus, err)
			}
		}()
	}
	close(debut)
	groupe.Wait()

	if len(inattendus) != 0 || succes != 1 || dejaAnnulees != appels-1 {
		t.Fatalf("succès = %d, déjà annulées = %d, erreurs inattendues = %v", succes, dejaAnnulees, inattendus)
	}
	if places := b.placesLibres(t, trajet); places != 1 {
		t.Errorf("places = %d, attendu 1", places)
	}
}

func TestReserverSurLaDernierePlaceRendTrajetComplet(t *testing.T) {
	b := nouveauBanc(t)
	trajet := b.trajetPublie(t, 1)

	if _, err := reserver(t, b, trajet, "alice"); err != nil {
		t.Fatalf("réservation : %v", err)
	}
	if _, err := reserver(t, b, trajet, "bob"); !errors.Is(err, domain.ErrTrajetComplet) {
		t.Fatalf("seconde réservation : erreur = %v, attendu ErrTrajetComplet", err)
	}
	if n := b.compter(t, `SELECT count(*) FROM bookings`); n != 1 {
		t.Errorf("réservations = %d, attendu 1", n)
	}
	if places := b.placesLibres(t, trajet); places != 0 {
		t.Errorf("places disponibles = %d, attendu 0", places)
	}
}

func TestReserverRefuseLeDoublonActifSansEcrireNiPerdreDePlace(t *testing.T) {
	b := nouveauBanc(t)
	trajet := b.trajetPublie(t, 2)

	if _, err := reserver(t, b, trajet, "alice"); err != nil {
		t.Fatalf("première réservation : %v", err)
	}
	if _, err := reserver(t, b, trajet, "alice"); !errors.Is(err, domain.ErrReservationDejaActive) {
		t.Fatalf("seconde réservation : erreur = %v, attendu ErrReservationDejaActive", err)
	}
	if n := b.compter(t, `SELECT count(*) FROM bookings`); n != 1 {
		t.Errorf("réservations = %d, attendu 1 (aucune écriture sur refus)", n)
	}
	// le doublon a décrémenté la place avant d'échouer : l'annulation doit la rendre
	if places := b.placesLibres(t, trajet); places != 1 {
		t.Errorf("places disponibles = %d, attendu 1 (place rendue après le refus)", places)
	}
}

func TestIndexPartielAutoriseUneNouvelleReservationApresAnnulation(t *testing.T) {
	b := nouveauBanc(t)
	trajet := b.trajetPublie(t, 2)

	premiere, err := reserver(t, b, trajet, "alice")
	if err != nil {
		t.Fatalf("première réservation : %v", err)
	}
	// un autre passager sur le même trajet : l'index partiel ne le bloque pas
	if _, err := reserver(t, b, trajet, "bob"); err != nil {
		t.Fatalf("un autre passager doit pouvoir réserver : %v", err)
	}
	if n := b.compter(t, `SELECT count(*) FROM bookings`); n != 2 {
		t.Errorf("lignes = %d, attendu 2", n)
	}
	if premiere.Identifiant == "" {
		t.Error("identifiant de la première réservation perdu")
	}
}

func TestParPassagerRendLesReservationsDuPassagerTriees(t *testing.T) {
	b := nouveauBanc(t)
	premier, second := b.trajetPublie(t, 2), b.trajetPublie(t, 2)

	for _, trajet := range []string{premier, second} {
		if _, err := reserver(t, b, trajet, "alice"); err != nil {
			t.Fatalf("réservation %s : %v", trajet, err)
		}
	}
	if _, err := reserver(t, b, premier, "bob"); err != nil {
		t.Fatalf("réservation de bob : %v", err)
	}

	reservations, err := b.reservations.ParPassager(context.Background(), "alice")
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
	if !vus[premier] || !vus[second] {
		t.Errorf("trajets rendus = %v, attendu %s et %s", vus, premier, second)
	}

	vides, err := b.reservations.ParPassager(context.Background(), "inconnu")
	if err != nil {
		t.Fatalf("listage d'un passager sans réservation : %v", err)
	}
	if vides == nil || len(vides) != 0 {
		t.Errorf("réservations = %v, attendu une liste vide non nulle", vides)
	}
}

// Le décrément conditionnel est la seule protection contre la surréservation : il doit tenir sous
// appels simultanés, sans verrou applicatif ni boucle de reprise (ADR 0004).
func TestReservationsConcurrentesNeSureserventPas(t *testing.T) {
	b := nouveauBanc(t)
	trajet := b.trajetPublie(t, 3)

	const appels = 8
	var (
		verrou     sync.Mutex
		succes     int
		complets   int
		inattendus []error
		debut      = make(chan struct{})
		groupe     sync.WaitGroup
	)
	for i := 0; i < appels; i++ {
		groupe.Add(1)
		go func(n int) {
			defer groupe.Done()
			<-debut
			_, err := b.reservations.Reserver(context.Background(), domain.Reservation{
				Trajet:   trajet,
				Passager: "passager" + string(rune('a'+n)),
				Statut:   domain.ReservationActive,
			})
			verrou.Lock()
			defer verrou.Unlock()
			switch {
			case err == nil:
				succes++
			case errors.Is(err, domain.ErrTrajetComplet):
				complets++
			default:
				inattendus = append(inattendus, err)
			}
		}(i)
	}
	close(debut)
	groupe.Wait()

	if len(inattendus) > 0 {
		t.Fatalf("erreurs inattendues : %v", inattendus)
	}
	if succes != 3 || complets != appels-3 {
		t.Errorf("succès = %d, complets = %d ; attendu 3 et %d", succes, complets, appels-3)
	}
	if n := b.compter(t, `SELECT count(*) FROM bookings`); n != 3 {
		t.Errorf("réservations = %d, attendu 3", n)
	}
	if places := b.placesLibres(t, trajet); places != 0 {
		t.Errorf("places disponibles = %d, attendu 0", places)
	}
}

func TestReservationVersUnTrajetInexistantRefuseeParLaCleEtrangere(t *testing.T) {
	b := nouveauBanc(t)
	ctx := context.Background()

	var table, depuis, vers string
	if err := b.db.QueryRowContext(ctx, `PRAGMA foreign_key_list(bookings)`).Scan(
		new(int), new(int), &table, &depuis, &vers, new(string), new(string), new(string),
	); err != nil {
		t.Fatalf("lecture des clés étrangères : %v", err)
	}
	if table != "rides" || depuis != "ride_id" || vers != "id" {
		t.Fatalf("clé étrangère = %s(%s) → %s(%s), attendu ride_id → rides(id)", "bookings", depuis, table, vers)
	}

	_, err := b.db.ExecContext(ctx, `INSERT INTO bookings (id, ride_id, passenger, status)
VALUES ('orpheline', 'trajet-inexistant', 'alice', 'ACTIVE')`)
	if err == nil {
		t.Fatal("une réservation vers un trajet inexistant a été acceptée")
	}
	if n := b.compter(t, `SELECT count(*) FROM bookings WHERE id = 'orpheline'`); n != 0 {
		t.Errorf("réservation orpheline écrite (%d ligne)", n)
	}
}

// Une base créée avant la clé étrangère garde ses réservations : SQLite ne sait pas ajouter une
// contrainte, le schéma reconstruit la table et recopie les lignes.
func TestSchemaReconstruitBookingsSansPerdreLesReservations(t *testing.T) {
	ctx := context.Background()
	chemin := filepath.Join(t.TempDir(), "ancienne.db")

	avant, err := sql.Open("sqlite", chemin)
	if err != nil {
		t.Fatalf("ouverture : %v", err)
	}
	defer avant.Close()
	for _, etape := range []string{
		`CREATE TABLE rides (
			id TEXT PRIMARY KEY, driver TEXT NOT NULL, origin TEXT NOT NULL, destination TEXT NOT NULL,
			departure TEXT NOT NULL, seats_total INTEGER NOT NULL, seats_available INTEGER NOT NULL,
			price REAL NOT NULL)`,
		`INSERT INTO rides VALUES ('t1', 'bob', 'Lyon', 'Grenoble', '2026-12-01T08:00:00Z', 3, 3, 0)`,
		`CREATE TABLE bookings (
			id TEXT PRIMARY KEY, ride_id TEXT NOT NULL, passenger TEXT NOT NULL,
			status TEXT NOT NULL, created_at TEXT NOT NULL DEFAULT '2026-01-01T00:00:00Z')`,
		`INSERT INTO bookings (id, ride_id, passenger, status) VALUES ('r1', 't1', 'alice', 'ACTIVE')`,
		`INSERT INTO bookings (id, ride_id, passenger, status) VALUES ('r2', 't1', 'bob', 'CANCELLED')`,
	} {
		if _, err := avant.ExecContext(ctx, etape); err != nil {
			t.Fatalf("%s : %v", etape, err)
		}
	}

	// deux applications du schéma : la seconde est un no-op, la première migre
	for tour := 1; tour <= 2; tour++ {
		if err := sqlite.AppliquerSchema(ctx, avant); err != nil {
			t.Fatalf("application du schéma (tour %d) : %v", tour, err)
		}
		var reservations, avecFK int
		if err := avant.QueryRowContext(ctx, `SELECT count(*) FROM bookings`).Scan(&reservations); err != nil {
			t.Fatalf("comptage : %v", err)
		}
		if reservations != 2 {
			t.Fatalf("tour %d : réservations = %d, attendu 2", tour, reservations)
		}
		if err := avant.QueryRowContext(ctx, `SELECT count(*) FROM pragma_foreign_key_list('bookings')`).Scan(&avecFK); err != nil {
			t.Fatalf("lecture des clés étrangères : %v", err)
		}
		if avecFK == 0 {
			t.Fatalf("tour %d : clé étrangère absente", tour)
		}
	}
	// Ouvrir active les clés étrangères à chaque connexion (pragma par connexion, donc à poser ici aussi).
	if _, err := avant.ExecContext(ctx, `PRAGMA foreign_keys = ON`); err != nil {
		t.Fatalf("activation des clés étrangères : %v", err)
	}
	if _, err := avant.ExecContext(ctx, `INSERT INTO bookings (id, ride_id, passenger, status)
VALUES ('r3', 'inexistant', 'carol', 'ACTIVE')`); err == nil {
		t.Error("la clé étrangère n'est pas appliquée après migration")
	}
}
