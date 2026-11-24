// Package sqlite : adaptateurs de persistance sur SQLite, sans ORM.
package sqlite

import (
	"context"
	"database/sql"
	"fmt"

	_ "modernc.org/sqlite" // pilote SQLite sans cgo
)

// Ouvrir ouvre (et crée si besoin) le fichier de base, applique les pragmas puis le schéma.
func Ouvrir(ctx context.Context, chemin string) (*sql.DB, error) {
	db, err := sql.Open("sqlite", chemin)
	if err != nil {
		return nil, fmt.Errorf("ouverture de la base : %w", err)
	}
	// SQLite n'accepte qu'un écrivain à la fois : une seule connexion sérialise les accès.
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)

	for _, pragma := range []string{
		"PRAGMA journal_mode = WAL",
		"PRAGMA busy_timeout = 5000",
		"PRAGMA foreign_keys = ON",
	} {
		if _, err := db.ExecContext(ctx, pragma); err != nil {
			db.Close()
			return nil, fmt.Errorf("pragma %q : %w", pragma, err)
		}
	}
	if err := AppliquerSchema(ctx, db); err != nil {
		db.Close()
		return nil, err
	}
	return db, nil
}
