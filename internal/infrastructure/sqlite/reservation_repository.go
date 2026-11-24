package sqlite

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	"github.com/alaa-ilehi/wasalni/internal/domain"
	moderncsqlite "modernc.org/sqlite"
)

// codeContrainteUnique : SQLITE_CONSTRAINT_UNIQUE, renvoyé quand l'index partiel refuse un second
// enregistrement ACTIVE pour le même couple (trajet, passager).
const codeContrainteUnique = 2067

// ReservationRepository : adaptateur SQLite du port domain.ReservationRepository.
type ReservationRepository struct {
	db *sql.DB
}

func NouveauReservationRepository(db *sql.DB) *ReservationRepository {
	return &ReservationRepository{db: db}
}

// Reserver consomme une place puis écrit la réservation, dans une seule transaction ouverte en mode
// immédiat (ADR 0004). Le décrément conditionnel porte la règle « pas de surréservation » : sans place
// libre, aucune ligne n'est modifiée et la transaction est annulée. Un doublon est signalé par l'index
// unique ; son annulation rend la place décrémentée.
func (r *ReservationRepository) Reserver(ctx context.Context, reservation domain.Reservation) (domain.Reservation, error) {
	identifiant, err := nouvelIdentifiant()
	if err != nil {
		return domain.Reservation{}, err
	}

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return domain.Reservation{}, fmt.Errorf("début de transaction : %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	const decrement = `
UPDATE rides SET seats_available = seats_available - 1
WHERE id = ? AND seats_available > 0`
	resultat, err := tx.ExecContext(ctx, decrement, reservation.Trajet)
	if err != nil {
		return domain.Reservation{}, fmt.Errorf("décrément des places : %w", err)
	}
	consommees, err := resultat.RowsAffected()
	if err != nil {
		return domain.Reservation{}, fmt.Errorf("décrément des places : %w", err)
	}
	if consommees == 0 {
		return domain.Reservation{}, domain.ErrTrajetComplet
	}

	const requete = `
INSERT INTO bookings (id, ride_id, passenger, status)
VALUES (?, ?, ?, ?)
RETURNING created_at`
	var creeLe string
	err = tx.QueryRowContext(ctx, requete,
		identifiant, reservation.Trajet, reservation.Passager, string(reservation.Statut)).Scan(&creeLe)
	if err != nil {
		if estViolationUnique(err) {
			return domain.Reservation{}, domain.ErrReservationDejaActive
		}
		return domain.Reservation{}, fmt.Errorf("écriture de la réservation : %w", err)
	}
	if err := tx.Commit(); err != nil {
		return domain.Reservation{}, fmt.Errorf("validation de la transaction : %w", err)
	}

	horodatage, err := time.Parse(time.RFC3339, creeLe)
	if err != nil {
		return domain.Reservation{}, fmt.Errorf("horodatage de création illisible %q : %w", creeLe, err)
	}
	reservation.Identifiant = identifiant
	reservation.CreeLe = horodatage
	return reservation, nil
}

// ParPassager liste les réservations d'un passager, de la plus récente à la plus ancienne (ADR 0003 :
// les deux états sont rendus, l'appelant décide de ce qu'il affiche).
func (r *ReservationRepository) ParPassager(ctx context.Context, passager string) ([]domain.Reservation, error) {
	const requete = `
SELECT id, ride_id, passenger, status, created_at
FROM bookings
WHERE passenger = ?
ORDER BY created_at DESC, id DESC`
	lignes, err := r.db.QueryContext(ctx, requete, passager)
	if err != nil {
		return nil, fmt.Errorf("lecture des réservations : %w", err)
	}
	defer func() { _ = lignes.Close() }()

	reservations := make([]domain.Reservation, 0, 4)
	for lignes.Next() {
		var (
			identifiant, trajet, passagerLigne, statut, creeLe string
		)
		if err := lignes.Scan(&identifiant, &trajet, &passagerLigne, &statut, &creeLe); err != nil {
			return nil, fmt.Errorf("lecture d'une réservation : %w", err)
		}
		horodatage, err := time.Parse(time.RFC3339, creeLe)
		if err != nil {
			return nil, fmt.Errorf("horodatage de création illisible %q : %w", creeLe, err)
		}
		reservations = append(reservations, domain.Reservation{
			Identifiant: identifiant,
			Trajet:      trajet,
			Passager:    passagerLigne,
			Statut:      domain.StatutReservation(statut),
			CreeLe:      horodatage,
		})
	}
	if err := lignes.Err(); err != nil {
		return nil, fmt.Errorf("parcours des réservations : %w", err)
	}
	return reservations, nil
}

// nouvelIdentifiant : 128 bits aléatoires, partagé par les réservations et les trajets, pour ne pas dépendre d'un compteur.
func nouvelIdentifiant() (string, error) {
	octets := make([]byte, 16)
	if _, err := rand.Read(octets); err != nil {
		return "", fmt.Errorf("identifiant de réservation : %w", err)
	}
	return hex.EncodeToString(octets), nil
}

// estViolationUnique reconnaît la contrainte d'unicité, sans dépendre du texte du message.
func estViolationUnique(err error) bool {
	var erreurSQLite *moderncsqlite.Error
	return errors.As(err, &erreurSQLite) && erreurSQLite.Code() == codeContrainteUnique
}
