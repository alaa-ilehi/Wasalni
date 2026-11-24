package sqlite

import (
	"context"
	"database/sql"
	"fmt"
	"math"
	"time"

	"github.com/alaa-ilehi/wasalni/internal/domain"
)

// EvaluationRepository : adaptateur SQLite du port domain.EvaluationRepository (UC6).
type EvaluationRepository struct {
	db *sql.DB
}

func NouveauEvaluationRepository(db *sql.DB) *EvaluationRepository {
	return &EvaluationRepository{db: db}
}

// Enregistrer insère l'évaluation. L'index unique porte l'interdiction du doublon :
// aucune lecture préalable n'est nécessaire, y compris sous appels simultanés.
func (r *EvaluationRepository) Enregistrer(ctx context.Context, evaluation domain.Evaluation) (domain.Evaluation, error) {
	identifiant, err := nouvelIdentifiant()
	if err != nil {
		return domain.Evaluation{}, err
	}

	const requete = `
INSERT INTO ratings (id, ride_id, evaluator, target, score, comment)
VALUES (?, ?, ?, ?, ?, ?)
RETURNING created_at`
	var commentaire any
	if evaluation.Commentaire != "" {
		commentaire = evaluation.Commentaire
	}
	var creeLe string
	err = r.db.QueryRowContext(ctx, requete,
		identifiant, evaluation.Trajet, evaluation.Evaluateur, evaluation.Cible,
		evaluation.Note, commentaire).Scan(&creeLe)
	if err != nil {
		if estViolationUnique(err) {
			return domain.Evaluation{}, domain.ErrEvaluationDejaFaite
		}
		return domain.Evaluation{}, fmt.Errorf("écriture de l'évaluation : %w", err)
	}
	horodatage, err := time.Parse(formatHorodatage, creeLe)
	if err != nil {
		return domain.Evaluation{}, fmt.Errorf("horodatage de création illisible %q : %w", creeLe, err)
	}
	evaluation.Identifiant = identifiant
	evaluation.CreeLe = horodatage
	return evaluation, nil
}

// Moyenne renvoie la note reçue arrondie à une décimale et le nombre d'évaluations.
// Sans évaluation, le compteur vaut zéro et UC7 affiche une note absente.
func (r *EvaluationRepository) Moyenne(ctx context.Context, identifiant string) (float64, int, error) {
	const requete = `SELECT AVG(score), COUNT(*) FROM ratings WHERE target = ?`
	var (
		moyenne sql.NullFloat64
		nombre  int
	)
	if err := r.db.QueryRowContext(ctx, requete, identifiant).Scan(&moyenne, &nombre); err != nil {
		return 0, 0, fmt.Errorf("lecture de la note moyenne : %w", err)
	}
	if nombre == 0 {
		return 0, 0, nil
	}
	return math.Round(moyenne.Float64*10) / 10, nombre, nil
}
