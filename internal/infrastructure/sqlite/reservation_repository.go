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

// Reserver écrit la réservation dans une transaction ouverte en mode immédiat (ADR 0004).
// Un doublon est signalé par l'index unique, traduit en domain.ErrReservationDejaActive.
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

// nouvelIdentifiant : 128 bits aléatoires, pour ne pas dépendre d'un compteur partagé.
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
