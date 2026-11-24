package sqlite

import (
	"context"
	"database/sql"
	"fmt"
)

// Schema des profils (UC8). Idempotent, appliqué à chaque démarrage.
// Le sexe est facultatif (NULL) et la photo est un WebP sans perte encodé en base64, ou NULL.
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

// Schema des réservations (UC3). bookings.ride_id référence rides, comme le décrément atomique
// des places l'exige (ADR 0004) : une réservation ne peut pas viser un trajet inexistant.
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

// Schema des évaluations (UC6). Une seule note par couple (trajet, évaluateur, cible) ;
// la moyenne affichée par UC7 est calculée à la lecture, sans colonne redondante sur users.
const schemaEvaluation = `
CREATE TABLE IF NOT EXISTS ratings (
    id         TEXT PRIMARY KEY,
    ride_id    TEXT NOT NULL REFERENCES rides(id),
    evaluator  TEXT NOT NULL,
    target     TEXT NOT NULL,
    score      INTEGER NOT NULL CHECK (score BETWEEN 1 AND 5),
    comment    TEXT,
    created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now'))
);

CREATE UNIQUE INDEX IF NOT EXISTS ratings_unique
    ON ratings (ride_id, evaluator, target);
`

// AppliquerSchema crée les tables manquantes. Rejouable sur une base existante.
func AppliquerSchema(ctx context.Context, db *sql.DB) error {
	for _, table := range []struct{ nom, ddl string }{
		{"users", schemaProfil},
		{"rides", schemaTrajet},
		{"bookings", schemaReservation},
		{"ratings", schemaEvaluation},
	} {
		if _, err := db.ExecContext(ctx, table.ddl); err != nil {
			return fmt.Errorf("application du schéma %s : %w", table.nom, err)
		}
	}

	// SQLite ne sait pas ajouter une contrainte à une table existante : une base créée avant la clé
	// étrangère est reconstruite. La copie est stricte — une réservation orpheline fait échouer le
	// démarrage plutôt que d'être perdue en silence.
	aCleEtrangere, err := possedeCleEtrangere(ctx, db, "bookings")
	if err != nil {
		return err
	}
	if !aCleEtrangere {
		return reconstruireReservations(ctx, db)
	}
	return nil
}

// possedeCleEtrangere : la table déclare-t-elle au moins une clé étrangère ?
func possedeCleEtrangere(ctx context.Context, db *sql.DB, table string) (bool, error) {
	lignes, err := db.QueryContext(ctx, "PRAGMA foreign_key_list("+table+")")
	if err != nil {
		return false, fmt.Errorf("inspection des clés étrangères de %s : %w", table, err)
	}
	defer func() { _ = lignes.Close() }()
	aCle := lignes.Next()
	return aCle, lignes.Err()
}

// reconstruireReservations : bookings sans clé étrangère → bookings avec, réservations conservées.
func reconstruireReservations(ctx context.Context, db *sql.DB) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("début de la migration des réservations : %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	for _, etape := range []string{
		`ALTER TABLE bookings RENAME TO bookings_ancienne`,
		`DROP INDEX IF EXISTS bookings_active_unique`,
		schemaReservation,
		`INSERT INTO bookings (id, ride_id, passenger, status, created_at)
		 SELECT id, ride_id, passenger, status, created_at FROM bookings_ancienne`,
		`DROP TABLE bookings_ancienne`,
	} {
		if _, err := tx.ExecContext(ctx, etape); err != nil {
			return fmt.Errorf("migration des réservations : %w", err)
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("validation de la migration des réservations : %w", err)
	}
	return nil
}
