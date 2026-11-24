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

// Schema des trajets (UC1). L'identifiant est un texte aléatoire, comme celui des réservations.
// cancel_delay_hours est en heures décimales ; status reste PUBLISHED jusqu'à UC11.
const schemaTrajet = `
CREATE TABLE IF NOT EXISTS rides (
    id                 TEXT PRIMARY KEY,
    driver             TEXT NOT NULL,
    origin             TEXT NOT NULL,
    destination        TEXT NOT NULL,
    departure          TEXT NOT NULL,
    seats_total        INTEGER NOT NULL CHECK (seats_total >= 1),
    seats_available    INTEGER NOT NULL CHECK (seats_available >= 0 AND seats_available <= seats_total),
    price              REAL    NOT NULL CHECK (price >= 0),
    cancel_delay_hours REAL    NOT NULL DEFAULT 2 CHECK (cancel_delay_hours >= 0),
    status             TEXT    NOT NULL DEFAULT 'PUBLISHED' CHECK (status IN ('PUBLISHED', 'COMPLETED')),
    created_at         TEXT    NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now'))
);
`

// Schema des réservations (UC3). bookings.ride_id référence rides(id) (ADR 0004) :
// la contrainte est posée à la création et rattrapée par migration sur une base existante.
// La garantie « une seule réservation active par couple (trajet, passager) » est portée par l'index partiel.
const schemaReservation = `
CREATE TABLE IF NOT EXISTS bookings (
    id         TEXT PRIMARY KEY,
    ride_id    TEXT NOT NULL REFERENCES rides(id),
    passenger  TEXT NOT NULL,
    status     TEXT NOT NULL CHECK (status IN ('ACTIVE', 'CANCELLED')),
    created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now'))
);

CREATE UNIQUE INDEX IF NOT EXISTS bookings_active_unique
    ON bookings (ride_id, passenger) WHERE status = 'ACTIVE';
`

// migruerCleEtrangereReservations ajoute la référence à rides sur une base créée avant ADR 0004.
// Le contrôle rend l'opération idempotente : une base neuve possède déjà la contrainte via schemaReservation.
func migruerCleEtrangereReservations(ctx context.Context, db *sql.DB) error {
	const presence = `SELECT count(*) FROM pragma_foreign_key_list('bookings') WHERE "table" = 'rides' AND "from" = 'ride_id'`
	var dejaPresente int
	if err := db.QueryRowContext(ctx, presence).Scan(&dejaPresente); err != nil {
		return fmt.Errorf("vérification de la clé étrangère des réservations : %w", err)
	}
	if dejaPresente > 0 {
		return nil
	}

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("début de la migration des réservations : %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	const reconstruction = `
CREATE TABLE bookings_migration (
    id         TEXT PRIMARY KEY,
    ride_id    TEXT NOT NULL REFERENCES rides(id),
    passenger  TEXT NOT NULL,
    status     TEXT NOT NULL CHECK (status IN ('ACTIVE', 'CANCELLED')),
    created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now'))
)`
	etapes := []string{
		reconstruction,
		`INSERT INTO bookings_migration (id, ride_id, passenger, status, created_at)
         SELECT id, ride_id, passenger, status, created_at FROM bookings`,
		`DROP TABLE bookings`,
		`ALTER TABLE bookings_migration RENAME TO bookings`,
		`CREATE UNIQUE INDEX bookings_active_unique
         ON bookings (ride_id, passenger) WHERE status = 'ACTIVE'`,
	}
	for _, etape := range etapes {
		if _, err := tx.ExecContext(ctx, etape); err != nil {
			return fmt.Errorf("migration des réservations : %w", err)
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("validation de la migration des réservations : %w", err)
	}
	return nil
}

// AppliquerSchema crée les tables manquantes et rattrape la clé étrangère de bookings.
// Rejouable sur une base existante.
func AppliquerSchema(ctx context.Context, db *sql.DB) error {
	for nom, ddl := range map[string]string{
		"users":    schemaProfil,
		"rides":    schemaTrajet,
		"bookings": schemaReservation,
	} {
		if _, err := db.ExecContext(ctx, ddl); err != nil {
			return fmt.Errorf("application du schéma %s : %w", nom, err)
		}
	}
	return migruerCleEtrangereReservations(ctx, db)
}
