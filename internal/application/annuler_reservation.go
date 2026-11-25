package application

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/alaa-ilehi/wasalni/internal/domain"
)

// NotificateurAnnulation est le port de notification best-effort de UC5.
type NotificateurAnnulation interface {
	NotifierAnnulation(ctx context.Context, reservation domain.Reservation) error
}

// AnnulerReservation : UC5, un passager libère sa réservation active avant l'échéance.
type AnnulerReservation struct {
	reservations domain.ReservationRepository
	horloge      Horloge
	notificateur NotificateurAnnulation
}

func NouveauAnnulerReservation(
	reservations domain.ReservationRepository,
	horloge Horloge,
	notificateur NotificateurAnnulation,
) AnnulerReservation {
	return AnnulerReservation{reservations: reservations, horloge: horloge, notificateur: notificateur}
}

// Executer délègue au dépôt la transition atomique, y compris le contrôle de l'échéance.
// La notification part après validation et ne peut pas annuler la transaction (ADR 0007).
func (uc AnnulerReservation) Executer(ctx context.Context, passager, reservationID string) (domain.Reservation, error) {
	if passager == "" {
		return domain.Reservation{}, ErrIdentiteManquante
	}
	if uc.horloge == nil {
		return domain.Reservation{}, fmt.Errorf("horloge absente du cas d'usage UC5")
	}

	annulee, err := uc.reservations.Annuler(ctx, reservationID, passager, uc.horloge.Maintenant())
	if err != nil {
		return domain.Reservation{}, err
	}
	if uc.notificateur != nil {
		if err := uc.notificateur.NotifierAnnulation(ctx, annulee); err != nil {
			slog.Warn("notification d'annulation échouée",
				"reservation", annulee.Identifiant, "trajet", annulee.Trajet,
				"passager", annulee.Passager, "erreur", err)
		}
	}
	return annulee, nil
}
