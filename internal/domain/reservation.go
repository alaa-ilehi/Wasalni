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
