package application

import (
	"context"
	"log/slog"

	"github.com/alaa-ilehi/wasalni/internal/domain"
)

// Notificateur : port vers le système de notification externe (UC12). L'échec est toléré (ADR 0007).
type Notificateur interface {
	NotifierReservation(ctx context.Context, reservation domain.Reservation) error
}

// ReserverPlace : UC3, un passager réserve une place sur un trajet.
type ReserverPlace struct {
	trajets       domain.TrajetRepository
	reservations  domain.ReservationRepository
	notifications Notificateur
}

func NouveauReserverPlace(
	trajets domain.TrajetRepository,
	reservations domain.ReservationRepository,
	notifications Notificateur,
) ReserverPlace {
	return ReserverPlace{trajets: trajets, reservations: reservations, notifications: notifications}
}

// Executer applique les règles de UC3 dans cet ordre : trajet connu, passager distinct du conducteur,
// place disponible, puis écriture. La lecture de places n'est qu'un refus rapide : l'arbitrage de la
// dernière place et du doublon se fait dans la transaction de l'adaptateur (ADR 0004).
func (uc ReserverPlace) Executer(ctx context.Context, trajetID, passager string) (domain.Reservation, error) {
	if passager == "" {
		return domain.Reservation{}, ErrIdentiteManquante
	}

	trajet, err := uc.trajets.ParIdentifiant(ctx, trajetID)
	if err != nil {
		return domain.Reservation{}, err
	}
	if err := trajet.Valider(); err != nil {
		return domain.Reservation{}, err
	}
	if trajet.Conducteur == passager {
		return domain.Reservation{}, domain.ErrConducteurNePeutPasReserver
	}
	if trajet.Places <= 0 {
		return domain.Reservation{}, domain.ErrTrajetComplet
	}

	reservation := domain.Reservation{
		Trajet:   trajet.Identifiant,
		Passager: passager,
		Statut:   domain.ReservationActive,
	}
	if err := reservation.Valider(); err != nil {
		return domain.Reservation{}, err
	}

	creee, err := uc.reservations.Reserver(ctx, reservation)
	if err != nil {
		return domain.Reservation{}, err
	}

	uc.notifier(ctx, creee)
	return creee, nil
}

// notifier prévient le passager sans jamais annuler la réservation déjà écrite (ADR 0007).
func (uc ReserverPlace) notifier(ctx context.Context, reservation domain.Reservation) {
	if uc.notifications == nil {
		return
	}
	if err := uc.notifications.NotifierReservation(ctx, reservation); err != nil {
		slog.Warn("notification de réservation échouée",
			"reservation", reservation.Identifiant, "trajet", reservation.Trajet, "erreur", err)
	}
}
