package sqlite

import (
	"context"
	"database/sql"
	"fmt"
)

// schemaProfil : table des profils (UC8). Le schéma est idempotent, il est appliqué à chaque démarrage.
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

// AppliquerSchema crée la table des profils si elle n'existe pas encore.
func AppliquerSchema(ctx context.Context, db *sql.DB) error {
	if _, err := db.ExecContext(ctx, schemaProfil); err != nil {
		return fmt.Errorf("application du schéma : %w", err)
	}
	return nil
}
