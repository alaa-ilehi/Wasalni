package domain

import (
	"errors"
	"fmt"
	"strings"
	"time"
)

// CommentaireLongueurMax borne le commentaire facultatif de UC6.
const CommentaireLongueurMax = 500

// Erreurs métier de l'évaluation. Aucun code HTTP ici : la correspondance vit dans l'adaptateur HTTP.
var (
	ErrEvaluationInvalide  = errors.New("évaluation invalide")
	ErrTrajetNonTermine    = errors.New("le trajet n'est pas terminé")
	ErrNonParticipant      = errors.New("l'utilisateur n'a pas participé à ce trajet")
	ErrEvaluationDejaFaite = errors.New("une évaluation existe déjà pour ce couple sur ce trajet")
)

// Evaluation : le jugement porté par un participant sur l'autre partie, après la fin du trajet (UC6).
// Evaluateur vient de l'identité externe ; Cible est le conducteur ou un passager du même trajet.
type Evaluation struct {
	Identifiant string
	Trajet      string
	Evaluateur  string
	Cible       string
	Note        int
	Commentaire string
	CreeLe      time.Time
}

// Valider applique les règles indépendantes du trajet : note entière de 1 à 5, participants distincts,
// commentaire facultatif borné. La participation elle-même est vérifiée par le cas d'utilisation.
func (e Evaluation) Valider() error {
	switch {
	case e.Trajet == "" || e.Evaluateur == "" || e.Cible == "":
		return fmt.Errorf("%w : le trajet, l'évaluateur et la cible sont obligatoires", ErrEvaluationInvalide)
	case e.Evaluateur == e.Cible:
		return fmt.Errorf("%w : un utilisateur ne peut pas s'évaluer lui-même", ErrEvaluationInvalide)
	case e.Note < 1 || e.Note > 5:
		return fmt.Errorf("%w : la note doit être comprise entre 1 et 5", ErrEvaluationInvalide)
	case len([]rune(strings.TrimSpace(e.Commentaire))) > CommentaireLongueurMax:
		return fmt.Errorf("%w : le commentaire ne peut pas dépasser %d caractères",
			ErrEvaluationInvalide, CommentaireLongueurMax)
	}
	return nil
}
