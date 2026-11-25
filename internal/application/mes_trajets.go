package application

import (
	"context"
	"errors"

	"github.com/alaa-ilehi/wasalni/internal/domain"
)

// MesTrajets : UC10, les trajets que l'appelant conduit. Lecture seule, tous statuts, supprimés exclus.
type MesTrajets struct {
	trajets domain.TrajetRepository
}

func NouveauMesTrajets(trajets domain.TrajetRepository) MesTrajets {
	return MesTrajets{trajets: trajets}
}

// Executer rend les trajets du conducteur, du départ le plus proche au plus lointain.
// Aucun trajet n'est une liste vide, pas une erreur (A2 de la fiche UC10).
func (uc MesTrajets) Executer(ctx context.Context, conducteur string) ([]domain.Trajet, error) {
	if conducteur == "" {
		return nil, ErrIdentiteManquante
	}
	return uc.trajets.ParConducteur(ctx, conducteur)
}

// ReservationSuivie : une réservation UC10 et le trajet qu'elle concerne.
// Le trajet vaut nil quand il a été supprimé par UC9 : la réservation annulée par cette suppression reste
// visible, ce qui permet à l'utilisateur de comprendre l'historique.
type ReservationSuivie struct {
	Reservation domain.Reservation
	Trajet      *domain.Trajet
}

// MesReservations : UC10, les réservations détenues par l'appelant, les deux états compris.
type MesReservations struct {
	reservations domain.ReservationRepository
	trajets      domain.TrajetRepository
}

func NouveauMesReservations(reservations domain.ReservationRepository, trajets domain.TrajetRepository) MesReservations {
	return MesReservations{reservations: reservations, trajets: trajets}
}

// Executer rend les réservations du passager, de la plus récente à la plus ancienne, enrichies du trajet
// quand il est encore lisible.
func (uc MesReservations) Executer(ctx context.Context, passager string) ([]ReservationSuivie, error) {
	if passager == "" {
		return nil, ErrIdentiteManquante
	}
	liste, err := uc.reservations.ParPassager(ctx, passager)
	if err != nil {
		return nil, err
	}
	suivies := make([]ReservationSuivie, 0, len(liste))
	for _, reservation := range liste {
		suivie := ReservationSuivie{Reservation: reservation}
		trajet, err := uc.trajets.ParIdentifiant(ctx, reservation.Trajet)
		switch {
		case err == nil:
			suivie.Trajet = &trajet
		case errors.Is(err, domain.ErrTrajetIntrouvable):
			// Trajet supprimé (UC9) : la réservation reste listée sans son trajet.
		default:
			return nil, err
		}
		suivies = append(suivies, suivie)
	}
	return suivies, nil
}
