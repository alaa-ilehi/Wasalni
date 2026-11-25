// Package notifications : les événements sont inscrits au journal en attendant le système externe UC12 (ADR 0007).
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

func (j *Journal) NotifierFinTrajet(_ context.Context, trajet domain.Trajet) error {
	j.journal.Info("trajet terminé; notification aux passagers à transmettre par UC12",
		"trajet", trajet.Identifiant,
		"conducteur", trajet.Conducteur)
	return nil
}

func (j *Journal) NotifierModificationTrajet(_ context.Context, trajet domain.Trajet) error {
	j.journal.Info("trajet modifié; notification aux passagers à transmettre par UC12", "trajet", trajet.Identifiant, "conducteur", trajet.Conducteur)
	return nil
}

func (j *Journal) NotifierSuppressionTrajet(_ context.Context, trajet domain.Trajet) error {
	j.journal.Info("trajet supprimé; notification aux passagers à transmettre par UC12", "trajet", trajet.Identifiant, "conducteur", trajet.Conducteur)
	return nil
}
