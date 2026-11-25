package domain

import (
	"errors"
	"time"
)

// StatutReservation : deux états seulement, sans flux financier (ADR 0003).
type StatutReservation string

const (
	ReservationActive  StatutReservation = "ACTIVE"
	ReservationAnnulee StatutReservation = "CANCELLED"
)

// Erreurs métier de la réservation. Aucun code HTTP ici : la correspondance vit dans l'adaptateur HTTP.
var (
	ErrTrajetIntrouvable           = errors.New("trajet introuvable")
	ErrTrajetComplet               = errors.New("plus aucune place disponible sur ce trajet")
	ErrReservationDejaActive       = errors.New("une réservation active existe déjà pour ce trajet")
	ErrReservationIntrouvable      = errors.New("réservation introuvable")
	ErrReservationDejaAnnulee      = errors.New("la réservation est déjà annulée")
	ErrAnnulationHorsDelai         = errors.New("le délai d'annulation est dépassé")
	ErrConducteurNePeutPasReserver = errors.New("le conducteur ne peut pas réserver sa propre place")
	ErrTrajetInvalide              = errors.New("trajet invalide")
)

// Reservation : une place retenue par un passager sur un trajet.
type Reservation struct {
	Identifiant string
	Trajet      string
	Passager    string
	Statut      StatutReservation
	CreeLe      time.Time
}

// ReservationAvecTrajet : une réservation et le trajet qu'elle vise, lus ensemble. Trajet vaut nil quand
// le conducteur a supprimé le trajet (UC9) : la réservation reste visible, sans trajet.
type ReservationAvecTrajet struct {
	Reservation Reservation
	Trajet      *Trajet
}

// Valider vérifie les champs indispensables avant écriture.
func (r Reservation) Valider() error {
	if r.Trajet == "" || r.Passager == "" {
		return ErrTrajetInvalide
	}
	switch r.Statut {
	case ReservationActive, ReservationAnnulee:
	default:
		return ErrTrajetInvalide
	}
	return nil
}

// ValiderAnnulation applique les règles de UC5. Une réservation d'un autre passager
// est volontairement indistinguable d'une réservation inconnue.
func (r Reservation) ValiderAnnulation(passager string, trajet Trajet, maintenant time.Time) error {
	if r.Identifiant == "" || r.Passager != passager || r.Trajet != trajet.Identifiant {
		return ErrReservationIntrouvable
	}
	if r.Statut != ReservationActive {
		return ErrReservationDejaAnnulee
	}
	if !maintenant.Before(trajet.Depart.Add(-trajet.DelaiAnnulation)) {
		return ErrAnnulationHorsDelai
	}
	return nil
}
