// Package notifications : adaptateur du port Notificateur. Le système externe d'UC12 n'existe pas encore :
// la notification se limite à une trace, l'échec restant sans effet sur la réservation (ADR 0007).
package notifications

import (
	"context"
	"log/slog"

	"github.com/alaa-ilehi/wasalni/internal/domain"
)

// Journal écrit une ligne par notification.
type Journal struct {
	journal *slog.Logger
}

func NouveauJournal(journal *slog.Logger) *Journal {
	if journal == nil {
		journal = slog.Default()
	}
	return &Journal{journal: journal}
}

func (j *Journal) NotifierReservation(_ context.Context, reservation domain.Reservation) error {
	j.journal.Info("réservation confirmée au passager",
		"reservation", reservation.Identifiant,
		"trajet", reservation.Trajet,
		"passager", reservation.Passager)
	return nil
}

func (j *Journal) NotifierAnnulation(_ context.Context, reservation domain.Reservation) error {
	j.journal.Info("réservation annulée par le passager",
		"reservation", reservation.Identifiant,
		"trajet", reservation.Trajet,
		"passager", reservation.Passager)
	return nil
}
