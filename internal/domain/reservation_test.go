package domain_test

import (
	"errors"
	"testing"
	"time"

	"github.com/alaa-ilehi/wasalni/internal/domain"
)

func reservationValide() domain.Reservation {
	return domain.Reservation{Trajet: "t1", Passager: "alice", Statut: domain.ReservationActive}
}

func TestReservationValiderAnnulation(t *testing.T) {
	depart := time.Date(2026, 12, 1, 8, 0, 0, 0, time.UTC)
	reservation := domain.Reservation{
		Identifiant: "r1", Trajet: "t1", Passager: "alice", Statut: domain.ReservationActive,
	}
	trajet := domain.Trajet{Identifiant: "t1", Depart: depart, DelaiAnnulation: 2 * time.Hour}

	if err := reservation.ValiderAnnulation("alice", trajet, depart.Add(-3*time.Hour)); err != nil {
		t.Fatalf("annulation avant l'échéance refusée : %v", err)
	}
	for _, cas := range []struct {
		nom         string
		reservation domain.Reservation
		passager    string
		trajet      domain.Trajet
		maintenant  time.Time
		attendu     error
	}{
		{"autre passager", reservation, "bob", trajet, depart.Add(-3 * time.Hour), domain.ErrReservationIntrouvable},
		{"déjà annulée", domain.Reservation{Identifiant: "r1", Trajet: "t1", Passager: "alice", Statut: domain.ReservationAnnulee}, "alice", trajet, depart.Add(-3 * time.Hour), domain.ErrReservationDejaAnnulee},
		{"échéance exacte", reservation, "alice", trajet, depart.Add(-2 * time.Hour), domain.ErrAnnulationHorsDelai},
		{"mauvais trajet", reservation, "alice", domain.Trajet{Identifiant: "t2", Depart: depart, DelaiAnnulation: 2 * time.Hour}, depart.Add(-3 * time.Hour), domain.ErrReservationIntrouvable},
	} {
		t.Run(cas.nom, func(t *testing.T) {
			if err := cas.reservation.ValiderAnnulation(cas.passager, cas.trajet, cas.maintenant); !errors.Is(err, cas.attendu) {
				t.Fatalf("erreur = %v, attendu %v", err, cas.attendu)
			}
		})
	}
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
