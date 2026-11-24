package application

import (
	"context"
	"fmt"
	"strings"

	"github.com/alaa-ilehi/wasalni/internal/domain"
)

// EvaluerParticipant : UC6, une évaluation par participant et par trajet, après la fin du trajet.
type EvaluerParticipant struct {
	trajets      domain.TrajetRepository
	reservations domain.ReservationRepository
	evaluations  domain.EvaluationRepository
}

func NouveauEvaluerParticipant(
	trajets domain.TrajetRepository,
	reservations domain.ReservationRepository,
	evaluations domain.EvaluationRepository,
) EvaluerParticipant {
	return EvaluerParticipant{trajets: trajets, reservations: reservations, evaluations: evaluations}
}

// Executer vérifie d'abord les champs, puis le trajet terminé et la participation des deux parties.
// Le doublon est arbitré par l'index unique du dépôt, pas par une lecture préalable.
func (uc EvaluerParticipant) Executer(
	ctx context.Context,
	evaluateur, trajetID, cible string,
	note int,
	commentaire string,
) (domain.Evaluation, error) {
	evaluation := domain.Evaluation{
		Trajet:      trajetID,
		Evaluateur:  evaluateur,
		Cible:       cible,
		Note:        note,
		Commentaire: strings.TrimSpace(commentaire),
	}
	if err := evaluation.Valider(); err != nil {
		return domain.Evaluation{}, err
	}

	trajet, err := uc.trajets.ParIdentifiant(ctx, trajetID)
	if err != nil {
		return domain.Evaluation{}, err
	}
	if trajet.Statut != domain.TrajetTermine {
		return domain.Evaluation{}, domain.ErrTrajetNonTermine
	}

	var participant string
	switch {
	case cible == trajet.Conducteur:
		// Un passager évalue le conducteur : l'évaluateur doit avoir réservé.
		participant = evaluateur
	case evaluateur == trajet.Conducteur:
		// Le conducteur évalue un passager : la cible doit avoir réservé.
		participant = cible
	default:
		// Un passager ne peut pas évaluer un autre passager.
		return domain.Evaluation{}, domain.ErrNonParticipant
	}
	ok, err := participe(ctx, uc.reservations, participant, trajetID)
	if err != nil {
		return domain.Evaluation{}, err
	}
	if !ok {
		return domain.Evaluation{}, domain.ErrNonParticipant
	}

	return uc.evaluations.Enregistrer(ctx, evaluation)
}

// participe vérifie qu'une réservation existe pour le couple (passager, trajet).
// UC5 n'exclut pas les réservations annulées : la ligne atteste la participation antérieure au trajet.
func participe(ctx context.Context, depot domain.ReservationRepository, passager, trajetID string) (bool, error) {
	reservations, err := depot.ParPassager(ctx, passager)
	if err != nil {
		return false, fmt.Errorf("lecture des réservations : %w", err)
	}
	for _, reservation := range reservations {
		if reservation.Trajet == trajetID {
			return true, nil
		}
	}
	return false, nil
}
