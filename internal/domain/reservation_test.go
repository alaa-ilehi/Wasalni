package domain_test

import (
	"errors"
	"testing"

	"github.com/alaa-ilehi/wasalni/internal/domain"
)

func reservationValide() domain.Reservation {
	return domain.Reservation{Trajet: "t1", Passager: "alice", Statut: domain.ReservationActive}
}

func TestReservationValider(t *testing.T) {
	if err := reservationValide().Valider(); err != nil {
		t.Fatalf("réservation valide refusée : %v", err)
	}

	cas := map[string]domain.Reservation{
		"trajet vide":    {Passager: "alice", Statut: domain.ReservationActive},
		"passager vide":  {Trajet: "t1", Statut: domain.ReservationActive},
		"statut inconnu": {Trajet: "t1", Passager: "alice", Statut: "INCONNU"},
		"statut vide":    {Trajet: "t1", Passager: "alice"},
	}
	for libelle, reservation := range cas {
		if err := reservation.Valider(); !errors.Is(err, domain.ErrTrajetInvalide) {
			t.Errorf("%s : erreur = %v, attendu ErrTrajetInvalide", libelle, err)
		}
	}
}

func TestDeuxStatutsSeulement(t *testing.T) {
	if domain.ReservationActive != "ACTIVE" || domain.ReservationAnnulee != "CANCELLED" {
		t.Fatalf("statuts = %q / %q, attendu ACTIVE / CANCELLED",
			domain.ReservationActive, domain.ReservationAnnulee)
	}
}

func TestTrajetValider(t *testing.T) {
	if err := (domain.Trajet{Identifiant: "t1", Conducteur: "bob", Places: 2}).Valider(); err != nil {
		t.Fatalf("trajet valide refusé : %v", err)
	}
	for libelle, trajet := range map[string]domain.Trajet{
		"sans identifiant": {},
		"sans conducteur":  {Identifiant: "t1"},
	} {
		if err := trajet.Valider(); !errors.Is(err, domain.ErrTrajetInvalide) {
			t.Errorf("%s : erreur = %v, attendu ErrTrajetInvalide", libelle, err)
		}
	}
}
