package application_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/alaa-ilehi/wasalni/internal/application"
	"github.com/alaa-ilehi/wasalni/internal/domain"
)

// mesTrajetsFaux : trajets par conducteur en mémoire ; l'erreur injectée vérifie la propagation.
type mesTrajetsFaux struct {
	parConducteur map[string][]domain.Trajet
	erreur        error
	demande       string
}

func (f *mesTrajetsFaux) Publier(_ context.Context, t domain.Trajet) (domain.Trajet, error) {
	return t, nil
}

func (f *mesTrajetsFaux) ParIdentifiant(_ context.Context, identifiant string) (domain.Trajet, error) {
	for _, trajets := range f.parConducteur {
		for _, trajet := range trajets {
			if trajet.Identifiant == identifiant {
				return trajet, nil
			}
		}
	}
	return domain.Trajet{}, domain.ErrTrajetIntrouvable
}

func (f *mesTrajetsFaux) Rechercher(_ context.Context, _ domain.CriteresRecherche) ([]domain.Trajet, error) {
	return nil, nil
}

func (f *mesTrajetsFaux) ParConducteur(_ context.Context, conducteur string) ([]domain.Trajet, error) {
	f.demande = conducteur
	if f.erreur != nil {
		return nil, f.erreur
	}
	return f.parConducteur[conducteur], nil
}

// mesReservationsFaux : réservations par passager en mémoire.
type mesReservationsFaux struct {
	parPassager map[string][]domain.Reservation
	erreur      error
	demande     string
}

func (f *mesReservationsFaux) Reserver(_ context.Context, r domain.Reservation) (domain.Reservation, error) {
	return r, nil
}

func (f *mesReservationsFaux) ParPassager(_ context.Context, passager string) ([]domain.Reservation, error) {
	f.demande = passager
	if f.erreur != nil {
		return nil, f.erreur
	}
	return f.parPassager[passager], nil
}

func (f *mesReservationsFaux) Annuler(_ context.Context, _ string, _ string, _ time.Time) (domain.Reservation, error) {
	return domain.Reservation{}, nil
}

func trajetUC10(identifiant, conducteur string) domain.Trajet {
	return domain.Trajet{
		Identifiant: identifiant,
		Conducteur:  conducteur,
		Origine:     "Lyon",
		Destination: "Grenoble",
		Depart:      time.Date(2026, 12, 1, 8, 30, 0, 0, time.UTC),
		PlacesTotal: 3,
		Places:      3,
		Prix:        12.5,
		Statut:      domain.TrajetPublie,
	}
}

func TestMesTrajetsRendSeulementCeuxDuConducteur(t *testing.T) {
	trajets := &mesTrajetsFaux{parConducteur: map[string][]domain.Trajet{
		"bob": {trajetUC10("T1", "bob"), trajetUC10("T2", "bob")},
	}}
	uc := application.NouveauMesTrajets(trajets)

	liste, err := uc.Executer(context.Background(), "bob")
	if err != nil {
		t.Fatalf("exécution : %v", err)
	}
	if len(liste) != 2 || liste[0].Identifiant != "T1" || liste[1].Identifiant != "T2" {
		t.Fatalf("trajets = %+v, attendu T1 puis T2", liste)
	}
	if trajets.demande != "bob" {
		t.Fatalf("lecture demandée pour %q, attendu bob", trajets.demande)
	}
}

// A2 de la fiche UC10 : aucun trajet est une liste vide, pas une erreur.
func TestMesTrajetsSansTrajetRendUneListeVide(t *testing.T) {
	uc := application.NouveauMesTrajets(&mesTrajetsFaux{parConducteur: map[string][]domain.Trajet{}})
	liste, err := uc.Executer(context.Background(), "bob")
	if err != nil || len(liste) != 0 {
		t.Fatalf("liste = %+v, %v, attendu vide sans erreur", liste, err)
	}
}

func TestMesTrajetsSansIdentiteEchoue(t *testing.T) {
	uc := application.NouveauMesTrajets(&mesTrajetsFaux{})
	if _, err := uc.Executer(context.Background(), ""); !errors.Is(err, application.ErrIdentiteManquante) {
		t.Fatalf("erreur = %v, attendu ErrIdentiteManquante", err)
	}
}

func TestMesTrajetsPropageLErreurDuDepot(t *testing.T) {
	panne := errors.New("base indisponible")
	uc := application.NouveauMesTrajets(&mesTrajetsFaux{erreur: panne})
	if _, err := uc.Executer(context.Background(), "bob"); !errors.Is(err, panne) {
		t.Fatalf("erreur = %v, attendu la panne du dépôt", err)
	}
}

func TestMesReservationsEnrichitAvecLeTrajet(t *testing.T) {
	reservations := &mesReservationsFaux{parPassager: map[string][]domain.Reservation{
		"alice": {
			{Identifiant: "B1", Trajet: "T1", Passager: "alice", Statut: domain.ReservationActive, CreeLe: time.Date(2026, 11, 20, 9, 0, 0, 0, time.UTC)},
			{Identifiant: "B2", Trajet: "T-supprime", Passager: "alice", Statut: domain.ReservationAnnulee, CreeLe: time.Date(2026, 11, 19, 9, 0, 0, 0, time.UTC)},
		},
	}}
	trajets := &mesTrajetsFaux{parConducteur: map[string][]domain.Trajet{"bob": {trajetUC10("T1", "bob")}}}
	uc := application.NouveauMesReservations(reservations, trajets)

	suivies, err := uc.Executer(context.Background(), "alice")
	if err != nil {
		t.Fatalf("exécution : %v", err)
	}
	if len(suivies) != 2 {
		t.Fatalf("réservations = %+v, attendu 2", suivies)
	}
	if suivies[0].Trajet == nil || suivies[0].Trajet.Identifiant != "T1" {
		t.Fatalf("réservation active sans trajet : %+v", suivies[0])
	}
	// Trajet supprimé par UC9 : la réservation reste listée, sans trajet.
	if suivies[1].Trajet != nil || suivies[1].Reservation.Statut != domain.ReservationAnnulee {
		t.Fatalf("réservation dont le trajet est supprimé : %+v", suivies[1])
	}
}

func TestMesReservationsSansReservationRendUneListeVide(t *testing.T) {
	uc := application.NouveauMesReservations(
		&mesReservationsFaux{parPassager: map[string][]domain.Reservation{}},
		&mesTrajetsFaux{},
	)
	suivies, err := uc.Executer(context.Background(), "alice")
	if err != nil || len(suivies) != 0 {
		t.Fatalf("liste = %+v, %v, attendu vide sans erreur", suivies, err)
	}
}

func TestMesReservationsSansIdentiteEchoue(t *testing.T) {
	uc := application.NouveauMesReservations(&mesReservationsFaux{}, &mesTrajetsFaux{})
	if _, err := uc.Executer(context.Background(), ""); !errors.Is(err, application.ErrIdentiteManquante) {
		t.Fatalf("erreur = %v, attendu ErrIdentiteManquante", err)
	}
}
