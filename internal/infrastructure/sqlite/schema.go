package sqlite

import (
	"context"
	"database/sql"
	"fmt"
)

// Schema des profils (UC8). Idempotent, appliqué à chaque démarrage.
// Le sexe est facultatif (NULL) et la photo est un PNG encodé en base64, ou NULL.
const schemaProfil = `
CREATE TABLE IF NOT EXISTS users (
    id         TEXT PRIMARY KEY,
    name       TEXT NOT NULL,
    age        INTEGER NOT NULL CHECK (age BETWEEN 18 AND 99),
    sex        TEXT CHECK (sex IS NULL OR sex IN ('MALE', 'FEMALE', 'OTHER')),
    photo      TEXT,
    created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now')),
    updated_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now'))
);
`

// Schema des réservations (UC3, périmètre réduit : pas de table rides, donc pas de clé étrangère).
// La garantie « une seule réservation active par couple (trajet, passager) » est portée par l'index partiel.
const schemaReservation = `
CREATE TABLE IF NOT EXISTS bookings (
    id         TEXT PRIMARY KEY,
    ride_id    TEXT NOT NULL,
    passenger  TEXT NOT NULL,
    status     TEXT NOT NULL CHECK (status IN ('ACTIVE', 'CANCELLED')),
    created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now'))
);

CREATE UNIQUE INDEX IF NOT EXISTS bookings_active_unique
    ON bookings (ride_id, passenger) WHERE status = 'ACTIVE';
`

// AppliquerSchema crée les tables manquantes. Rejouable sur une base existante.
func AppliquerSchema(ctx context.Context, db *sql.DB) error {
	for nom, ddl := range map[string]string{
		"users":    schemaProfil,
		"bookings": schemaReservation,
	} {
		if _, err := db.ExecContext(ctx, ddl); err != nil {
			return fmt.Errorf("application du schéma %s : %w", nom, err)
		}
	}
	return nil
}
