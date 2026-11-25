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
    created_at         TEXT    NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now')),
    deleted_at         TEXT
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

// Schema de l'authentification (UC13). Le compte et le profil sont séparés : un identifiant peut
// s'inscrire avant d'avoir rempli son profil, et un profil créé par UC8 peut n'avoir aucun mot de
// passe. Aucune clé étrangère vers users, donc : elle imposerait un profil à l'inscription.
// Le jeton de session n'est jamais stocké en clair, seulement son hachage SHA-256.
const schemaIdentite = `
CREATE TABLE IF NOT EXISTS credentials (
    user_id    TEXT PRIMARY KEY,
    salt       BLOB NOT NULL,
    hash       BLOB NOT NULL,
    iterations INTEGER NOT NULL CHECK (iterations > 0),
    created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now'))
);

CREATE TABLE IF NOT EXISTS sessions (
    token_hash TEXT PRIMARY KEY,
    user_id    TEXT NOT NULL,
    created_at TEXT NOT NULL,
    expires_at TEXT NOT NULL
);

CREATE INDEX IF NOT EXISTS sessions_user ON sessions (user_id);
`

// AppliquerSchema crée les tables manquantes. Rejouable sur une base existante.
// schemaIndexTrajets : index partiels de la recherche UC2 et de UC10. Toutes les lectures publiques
// filtrent déjà deleted_at IS NULL, l'index ne contient donc que les trajets lisibles ; id termine la
// clé pour couvrir le tri et le curseur de pagination.
const schemaIndexTrajets = `
CREATE INDEX IF NOT EXISTS rides_search ON rides (status, departure, id) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS rides_driver ON rides (driver, departure, id) WHERE deleted_at IS NULL;
`

func AppliquerSchema(ctx context.Context, db *sql.DB) error {
	for _, table := range []struct{ nom, ddl string }{
		{"users", schemaProfil},
		{"rides", schemaTrajet},
		{"bookings", schemaReservation},
		{"ratings", schemaEvaluation},
		{"identite", schemaIdentite},
	} {
		if _, err := db.ExecContext(ctx, table.ddl); err != nil {
			return fmt.Errorf("application du schéma %s : %w", table.nom, err)
		}
	}
	if err := ajouterColonneSupprime(ctx, db); err != nil {
		return err
	}
	if err := indexerTrajets(ctx, db); err != nil {
		return err
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

func ajouterColonneSupprime(ctx context.Context, db *sql.DB) error {
	rows, err := db.QueryContext(ctx, "PRAGMA table_info(rides)")
	if err != nil {
		return fmt.Errorf("inspection du schéma rides : %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var cid, notnull, pk int
		var name, typ string
		var def any
		if err := rows.Scan(&cid, &name, &typ, &notnull, &def, &pk); err != nil {
			return err
		}
		if name == "deleted_at" {
			return rows.Err()
		}
	}
	if err := rows.Err(); err != nil {
		return err
	}
	if _, err := db.ExecContext(ctx, "ALTER TABLE rides ADD COLUMN deleted_at TEXT"); err != nil {
		return fmt.Errorf("ajout de rides.deleted_at : %w", err)
	}
	return nil
}

// indexerTrajets crée les index de lecture. Une base créée avant la colonne status garde sa table
// telle quelle : elle n'a pas de recherche à indexer, et rien ne serait perdu à la recréer.
// shortcut: pas d'index sur une base antérieure à status, à revoir si une telle base doit vivre.
func indexerTrajets(ctx context.Context, db *sql.DB) error {
	rows, err := db.QueryContext(ctx, "PRAGMA table_info(rides)")
	if err != nil {
		return fmt.Errorf("inspection du schéma rides : %w", err)
	}
	defer rows.Close()
	colonnes := map[string]bool{}
	for rows.Next() {
		var cid, notnull, pk int
		var name, typ string
		var def any
		if err := rows.Scan(&cid, &name, &typ, &notnull, &def, &pk); err != nil {
			return err
		}
		colonnes[name] = true
	}
	if err := rows.Err(); err != nil {
		return err
	}
	if !colonnes["status"] || !colonnes["departure"] || !colonnes["driver"] {
		return nil
	}
	if _, err := db.ExecContext(ctx, schemaIndexTrajets); err != nil {
		return fmt.Errorf("création des index de rides : %w", err)
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
