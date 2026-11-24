package application_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/alaa-ilehi/wasalni/internal/application"
	"github.com/alaa-ilehi/wasalni/internal/domain"
)

type depotAnnulationFaux struct {
	resultat domain.Reservation
	erreur   error
	appels   int
	id       string
	passager string
	instant  time.Time
}

func (d *depotAnnulationFaux) Reserver(context.Context, domain.Reservation) (domain.Reservation, error) {
	return domain.Reservation{}, nil
}

func (d *depotAnnulationFaux) ParPassager(context.Context, string) ([]domain.Reservation, error) {
	return nil, nil
}

func (d *depotAnnulationFaux) Annuler(_ context.Context, id, passager string, maintenant time.Time) (domain.Reservation, error) {
	d.appels++
	d.id, d.passager, d.instant = id, passager, maintenant
	return d.resultat, d.erreur
}

type horlogeAnnulation struct{ instant time.Time }

func (h horlogeAnnulation) Maintenant() time.Time { return h.instant }

type notificateurAnnulationFaux struct {
	appels      int
	reservation domain.Reservation
	erreur      error
}

func (n *notificateurAnnulationFaux) NotifierAnnulation(_ context.Context, reservation domain.Reservation) error {
	n.appels++
	n.reservation = reservation
	return n.erreur
}

func TestAnnulerReservationSuccesEtNotification(t *testing.T) {
	maintenant := time.Date(2026, 11, 25, 12, 0, 0, 0, time.UTC)
	annulee := domain.Reservation{Identifiant: "r1", Trajet: "t1", Passager: "alice", Statut: domain.ReservationAnnulee}
	depot := &depotAnnulationFaux{resultat: annulee}
	notif := &notificateurAnnulationFaux{}
	uc := application.NouveauAnnulerReservation(depot, horlogeAnnulation{maintenant}, notif)

	resultat, err := uc.Executer(context.Background(), "alice", "r1")
	if err != nil {
		t.Fatalf("annulation : %v", err)
	}
	if resultat != annulee {
		t.Errorf("résultat = %+v, attendu %+v", resultat, annulee)
	}
	if depot.appels != 1 || depot.id != "r1" || depot.passager != "alice" || !depot.instant.Equal(maintenant) {
		t.Errorf("appel dépôt = %+v, appels %d", depot, depot.appels)
	}
	if notif.appels != 1 || notif.reservation != annulee {
		t.Errorf("notification = %+v, appels %d", notif.reservation, notif.appels)
	}
}

func TestAnnulerReservationRefuseIdentiteManquante(t *testing.T) {
	depot := &depotAnnulationFaux{}
	uc := application.NouveauAnnulerReservation(depot, horlogeAnnulation{time.Now()}, nil)
	if _, err := uc.Executer(context.Background(), "", "r1"); !errors.Is(err, application.ErrIdentiteManquante) {
		t.Fatalf("erreur = %v, attendu ErrIdentiteManquante", err)
	}
	if depot.appels != 0 {
		t.Errorf("le dépôt a été appelé %d fois", depot.appels)
	}
}

func TestAnnulerReservationPreserveErreurDuDepot(t *testing.T) {
	for _, attendue := range []error{domain.ErrReservationIntrouvable, domain.ErrReservationDejaAnnulee, domain.ErrAnnulationHorsDelai} {
		t.Run(attendue.Error(), func(t *testing.T) {
			depot := &depotAnnulationFaux{erreur: attendue}
			notif := &notificateurAnnulationFaux{}
			uc := application.NouveauAnnulerReservation(depot, horlogeAnnulation{time.Now()}, notif)
			if _, err := uc.Executer(context.Background(), "alice", "r1"); !errors.Is(err, attendue) {
				t.Fatalf("erreur = %v, attendu %v", err, attendue)
			}
			if notif.appels != 0 {
				t.Errorf("notification envoyée après refus (%d appels)", notif.appels)
			}
		})
	}
}

func TestEchecNotificationNeDefaitPasAnnulation(t *testing.T) {
	annulee := domain.Reservation{Identifiant: "r1", Trajet: "t1", Passager: "alice", Statut: domain.ReservationAnnulee}
	depot := &depotAnnulationFaux{resultat: annulee}
	notif := &notificateurAnnulationFaux{erreur: errors.New("service indisponible")}
	uc := application.NouveauAnnulerReservation(depot, horlogeAnnulation{time.Now()}, notif)
	if _, err := uc.Executer(context.Background(), "alice", "r1"); err != nil {
		t.Fatalf("l'échec de notification doit rester sans effet : %v", err)
	}
	if notif.appels != 1 {
		t.Errorf("notifications = %d, attendu 1", notif.appels)
	}
}
