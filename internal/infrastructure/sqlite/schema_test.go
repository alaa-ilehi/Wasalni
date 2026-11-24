package sqlite_test

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"

	"github.com/alaa-ilehi/wasalni/internal/infrastructure/sqlite"
)

// TestOuvrirMigreLaCleEtrangereDesReservations vérifie le rattrapage sur une base créée
// avant ADR 0004 : la contrainte est ajoutée sans perdre les réservations existantes.
func TestOuvrirMigreLaCleEtrangereDesReservations(t *testing.T) {
	ctx := context.Background()
	chemin := filepath.Join(t.TempDir(), "ancienne.db")

	ancienne, err := sql.Open("sqlite", chemin)
	if err != nil {
		t.Fatalf("ouverture de l'ancienne base : %v", err)
	}
	etapes := []string{
		`CREATE TABLE rides (id TEXT PRIMARY KEY)`,
		`CREATE TABLE bookings (
            id TEXT PRIMARY KEY,
            ride_id TEXT NOT NULL,
            passenger TEXT NOT NULL,
            status TEXT NOT NULL,
            created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now'))
        )`,
		`INSERT INTO rides (id) VALUES ('t1')`,
		`INSERT INTO bookings (id, ride_id, passenger, status) VALUES ('res-1', 't1', 'alice', 'ACTIVE')`,
	}
	for _, etape := range etapes {
		if _, err := ancienne.ExecContext(ctx, etape); err != nil {
			t.Fatalf("préparation de l'ancienne base : %v", err)
		}
	}
	if err := ancienne.Close(); err != nil {
		t.Fatalf("fermeture de l'ancienne base : %v", err)
	}

	db, err := sqlite.Ouvrir(ctx, chemin)
	if err != nil {
		t.Fatalf("ouverture avec migration : %v", err)
	}
	defer db.Close()

	const presence = `SELECT count(*) FROM pragma_foreign_key_list('bookings') WHERE "table" = 'rides' AND "from" = 'ride_id'`
	var cle int
	if err := db.QueryRowContext(ctx, presence).Scan(&cle); err != nil {
		t.Fatalf("lecture des clés étrangères : %v", err)
	}
	if cle == 0 {
		t.Error("la clé étrangère vers rides n'a pas été ajoutée")
	}

	var reservations int
	if err := db.QueryRowContext(ctx, `SELECT count(*) FROM bookings`).Scan(&reservations); err != nil {
		t.Fatalf("comptage des réservations : %v", err)
	}
	if reservations != 1 {
		t.Errorf("réservations conservées = %d, attendu 1", reservations)
	}

	if _, err := db.ExecContext(ctx, `INSERT INTO bookings (id, ride_id, passenger, status)
        VALUES ('orphelin', 'trajet-inconnu', 'bob', 'ACTIVE')`); err == nil {
		t.Error("une réservation vers un trajet inexistant doit être refusée")
	}
}
