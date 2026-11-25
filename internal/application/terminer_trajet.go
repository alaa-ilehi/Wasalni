package application

import (
	"context"
	"log/slog"

	"github.com/alaa-ilehi/wasalni/internal/domain"
)

// NotificateurFinTrajet est le port best-effort de l'événement UC11; la livraison est déléguée à UC12.
type NotificateurFinTrajet interface {
	NotifierFinTrajet(ctx context.Context, trajet domain.Trajet) error
}

// TerminerTrajet : UC11, le conducteur clôt son trajet publié.
type TerminerTrajet struct {
	trajets      domain.TrajetFinRepository
	notificateur NotificateurFinTrajet
}

func NouveauTerminerTrajet(trajets domain.TrajetFinRepository, notificateur NotificateurFinTrajet) TerminerTrajet {
	return TerminerTrajet{trajets: trajets, notificateur: notificateur}
}

// Executer valide l'identité, termine le trajet, puis journalise l'événement sans défaire la mise à jour.
func (uc TerminerTrajet) Executer(ctx context.Context, conducteur, trajetID string) (domain.Trajet, error) {
	if conducteur == "" {
		return domain.Trajet{}, ErrIdentiteManquante
	}
	termine, err := uc.trajets.Terminer(ctx, trajetID, conducteur)
	if err != nil {
		return domain.Trajet{}, err
	}
	if uc.notificateur != nil {
		if err := uc.notificateur.NotifierFinTrajet(ctx, termine); err != nil {
			slog.Warn("notification de fin de trajet échouée",
				"trajet", termine.Identifiant, "conducteur", termine.Conducteur, "erreur", err)
		}
	}
	return termine, nil
}
