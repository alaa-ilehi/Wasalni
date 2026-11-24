package sqlite_test

import (
	"context"
	"database/sql"
	"errors"
	"path/filepath"
	"testing"

	"github.com/alaa-ilehi/wasalni/internal/domain"
	"github.com/alaa-ilehi/wasalni/internal/infrastructure/sqlite"
)

func baseEvaluation(t *testing.T) (*sql.DB, *sqlite.EvaluationRepository) {
	t.Helper()
	db, err := sqlite.Ouvrir(context.Background(), filepath.Join(t.TempDir(), "evaluations.db"))
	if err != nil {
		t.Fatalf("ouverture de la base : %v", err)
	}
	t.Cleanup(func() { db.Close() })
	if _, err := db.Exec(`INSERT INTO rides (id, driver, origin, destination, departure,
seats_total, seats_available, price, status)
VALUES ('t1', 'alice', 'Lyon', 'Grenoble', '2026-12-01T08:30:00Z', 3, 2, 12.5, 'COMPLETED')`); err != nil {
		t.Fatalf("création du trajet : %v", err)
	}
	return db, sqlite.NouveauEvaluationRepository(db)
}

func TestEnregistrerUneEvaluationEtCalculerLaMoyenne(t *testing.T) {
	ctx := context.Background()
	_, depot := baseEvaluation(t)

	evaluation, err := depot.Enregistrer(ctx, domain.Evaluation{
		Trajet: "t1", Evaluateur: "bob", Cible: "alice", Note: 4, Commentaire: "ponctuel",
	})
	if err != nil {
		t.Fatalf("enregistrement : %v", err)
	}
	if evaluation.Identifiant == "" || evaluation.CreeLe.IsZero() {
		t.Errorf("évaluation retournée incomplète : %+v", evaluation)
	}

	if _, err := depot.Enregistrer(ctx, domain.Evaluation{
		Trajet: "t1", Evaluateur: "bob", Cible: "alice", Note: 5,
	}); !errors.Is(err, domain.ErrEvaluationDejaFaite) {
		t.Fatalf("doublon : erreur = %v, attendu ErrEvaluationDejaFaite", err)
	}

	if _, err := depot.Enregistrer(ctx, domain.Evaluation{
		Trajet: "t1", Evaluateur: "carol", Cible: "alice", Note: 5,
	}); err != nil {
		t.Fatalf("deuxième évaluation : %v", err)
	}
	moyenne, nombre, err := depot.Moyenne(ctx, "alice")
	if err != nil {
		t.Fatalf("moyenne : %v", err)
	}
	if moyenne != 4.5 || nombre != 2 {
		t.Errorf("moyenne = %.1f sur %d notes, attendu 4.5 sur 2", moyenne, nombre)
	}

	if _, err := depot.Enregistrer(ctx, domain.Evaluation{
		Trajet: "t1", Evaluateur: "dave", Cible: "alice", Note: 4,
	}); err != nil {
		t.Fatalf("troisième évaluation : %v", err)
	}
	moyenne, nombre, err = depot.Moyenne(ctx, "alice")
	if err != nil {
		t.Fatalf("moyenne après arrondi : %v", err)
	}
	if moyenne != 4.3 || nombre != 3 {
		t.Errorf("moyenne = %.1f sur %d notes, attendu 4.3 sur 3", moyenne, nombre)
	}
}

func TestMoyenneSansEvaluation(t *testing.T) {
	ctx := context.Background()
	_, depot := baseEvaluation(t)

	moyenne, nombre, err := depot.Moyenne(ctx, "personne")
	if err != nil {
		t.Fatalf("moyenne : %v", err)
	}
	if moyenne != 0 || nombre != 0 {
		t.Errorf("moyenne = %.1f sur %d notes, attendu 0 sur 0", moyenne, nombre)
	}
}
