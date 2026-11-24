package application_test

import (
	"context"
	"errors"
	"testing"

	"github.com/alaa-ilehi/wasalni/internal/application"
	"github.com/alaa-ilehi/wasalni/internal/domain"
)

// trajetsFaux : port TrajetRepository en mémoire.
type trajetsFaux struct {
	parID  map[string]domain.Trajet
	erreur error
}

func (t *trajetsFaux) ParIdentifiant(_ context.Context, identifiant string) (domain.Trajet, error) {
	if t.erreur != nil {
		return domain.Trajet{}, t.erreur
	}
	trajet, ok := t.parID[identifiant]
	if !ok {
		return domain.Trajet{}, domain.ErrTrajetIntrouvable
	}
	return trajet, nil
}

// reservationsFausses : port ReservationRepository en mémoire, avec l'index unique simulé.
type reservationsFausses struct {
	actives map[string]bool
	creees  []domain.Reservation
	erreur  error
}

func (r *reservationsFausses) Reserver(_ context.Context, reservation domain.Reservation) (domain.Reservation, error) {
	if r.erreur != nil {
		return domain.Reservation{}, r.erreur
	}
	if r.actives == nil {
		r.actives = map[string]bool{}
	}
	cle := reservation.Trajet + "|" + reservation.Passager
	if r.actives[cle] {
		return domain.Reservation{}, domain.ErrReservationDejaActive
	}
	r.actives[cle] = true
	reservation.Identifiant = "res-1"
	r.creees = append(r.creees, reservation)
	return reservation, nil
}

// ParPassager : vue mémoire du listage, pour couvrir le port complet.
func (r *reservationsFausses) ParPassager(_ context.Context, passager string) ([]domain.Reservation, error) {
	if r.erreur != nil {
		return nil, r.erreur
	}
	resultat := make([]domain.Reservation, 0, len(r.creees))
	for i := len(r.creees) - 1; i >= 0; i-- {
		if r.creees[i].Passager == passager {
			resultat = append(resultat, r.creees[i])
		}
	}
	return resultat, nil
}

// notificateurFaux : trace les appels et peut échouer.
type notificateurFaux struct {
	appels int
	erreur error
}

func (n *notificateurFaux) NotifierReservation(context.Context, domain.Reservation) error {
	n.appels++
	return n.erreur
}

func bancReservation() (application.ReserverPlace, *trajetsFaux, *reservationsFausses, *notificateurFaux) {
	trajets := &trajetsFaux{parID: map[string]domain.Trajet{
		"t1":      {Identifiant: "t1", Conducteur: "bob", Places: 2},
		"complet": {Identifiant: "complet", Conducteur: "bob", Places: 0},
	}}
	reservations := &reservationsFausses{}
	notifications := &notificateurFaux{}
	return application.NouveauReserverPlace(trajets, reservations, notifications),
		trajets, reservations, notifications
}

func TestReserverPlaceSucces(t *testing.T) {
	reserver, _, reservations, notifications := bancReservation()

	reservation, err := reserver.Executer(context.Background(), "t1", "alice")
	if err != nil {
		t.Fatalf("réservation : %v", err)
	}
	if reservation.Statut != domain.ReservationActive {
		t.Errorf("statut = %q, attendu ACTIVE", reservation.Statut)
	}
	if reservation.Passager != "alice" || reservation.Trajet != "t1" || reservation.Identifiant == "" {
		t.Errorf("réservation = %+v", reservation)
	}
	if len(reservations.creees) != 1 {
		t.Errorf("écritures = %d, attendu 1", len(reservations.creees))
	}
	if notifications.appels != 1 {
		t.Errorf("notifications = %d, attendu 1", notifications.appels)
	}
}

func TestReserverPlaceRefus(t *testing.T) {
	cas := []struct {
		libelle string
		trajet  string
		acteur  string
		attendu error
	}{
		{"trajet inconnu", "inconnu", "alice", domain.ErrTrajetIntrouvable},
		{"conducteur", "t1", "bob", domain.ErrConducteurNePeutPasReserver},
		{"trajet complet", "complet", "alice", domain.ErrTrajetComplet},
		{"identité absente", "t1", "", application.ErrIdentiteManquante},
	}
	for _, c := range cas {
		reserver, _, reservations, notifications := bancReservation()
		_, err := reserver.Executer(context.Background(), c.trajet, c.acteur)
		if !errors.Is(err, c.attendu) {
			t.Errorf("%s : erreur = %v, attendu %v", c.libelle, err, c.attendu)
		}
		if len(reservations.creees) != 0 {
			t.Errorf("%s : une réservation a été écrite", c.libelle)
		}
		if notifications.appels != 0 {
			t.Errorf("%s : une notification a été envoyée", c.libelle)
		}
	}
}

func TestReserverPlaceDoublon(t *testing.T) {
	reserver, _, _, notifications := bancReservation()
	ctx := context.Background()

	if _, err := reserver.Executer(ctx, "t1", "alice"); err != nil {
		t.Fatalf("première réservation : %v", err)
	}
	_, err := reserver.Executer(ctx, "t1", "alice")
	if !errors.Is(err, domain.ErrReservationDejaActive) {
		t.Fatalf("seconde réservation : erreur = %v, attendu ErrReservationDejaActive", err)
	}
	if notifications.appels != 1 {
		t.Errorf("notifications = %d, attendu 1 (pas de notification pour un refus)", notifications.appels)
	}
}

func TestEchecNotificationNeDefaitPasLaReservation(t *testing.T) {
	reserver, _, reservations, notifications := bancReservation()
	notifications.erreur = errors.New("service de notification indisponible")

	reservation, err := reserver.Executer(context.Background(), "t1", "alice")
	if err != nil {
		t.Fatalf("la réservation doit survivre à l'échec de notification : %v", err)
	}
	if reservation.Identifiant == "" {
		t.Error("réservation non écrite")
	}
	if len(reservations.creees) != 1 || notifications.appels != 1 {
		t.Errorf("écritures = %d, notifications = %d", len(reservations.creees), notifications.appels)
	}
}

func TestReserverPlacePropageLErreurDEcriture(t *testing.T) {
	reserver, _, reservations, _ := bancReservation()
	reservations.erreur = errors.New("base indisponible")

	if _, err := reserver.Executer(context.Background(), "t1", "alice"); err == nil {
		t.Fatal("une erreur d'écriture doit remonter")
	}
}
