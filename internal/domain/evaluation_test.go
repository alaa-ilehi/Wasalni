package domain_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/alaa-ilehi/wasalni/internal/domain"
)

func TestEvaluationValider(t *testing.T) {
	base := domain.Evaluation{Trajet: "t1", Evaluateur: "bob", Cible: "alice", Note: 4}
	cas := []struct {
		libelle  string
		modifier func(*domain.Evaluation)
		attendu  error
	}{
		{"valide", func(*domain.Evaluation) {}, nil},
		{"note basse", func(e *domain.Evaluation) { e.Note = 0 }, domain.ErrEvaluationInvalide},
		{"note haute", func(e *domain.Evaluation) { e.Note = 6 }, domain.ErrEvaluationInvalide},
		{"cible absente", func(e *domain.Evaluation) { e.Cible = "" }, domain.ErrEvaluationInvalide},
		{"auto-évaluation", func(e *domain.Evaluation) { e.Cible = "bob" }, domain.ErrEvaluationInvalide},
		{"commentaire trop long", func(e *domain.Evaluation) {
			e.Commentaire = strings.Repeat("x", domain.CommentaireLongueurMax+1)
		}, domain.ErrEvaluationInvalide},
	}
	for _, c := range cas {
		t.Run(c.libelle, func(t *testing.T) {
			evaluation := base
			c.modifier(&evaluation)
			err := evaluation.Valider()
			if !errors.Is(err, c.attendu) {
				t.Fatalf("erreur = %v, attendu %v", err, c.attendu)
			}
		})
	}
}
