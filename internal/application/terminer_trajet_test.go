package application_test

import (
	"context"
	"errors"
	"testing"

	"github.com/alaa-ilehi/wasalni/internal/application"
	"github.com/alaa-ilehi/wasalni/internal/domain"
)

type depotFinFaux struct {
	trajet     domain.Trajet
	err        error
	appels     int
	conducteur string
}

func (d *depotFinFaux) Terminer(_ context.Context, identifiant, conducteur string) (domain.Trajet, error) {
	d.appels++
	d.conducteur = conducteur
	if d.err != nil {
		return domain.Trajet{}, d.err
	}
	d.trajet.Identifiant = identifiant
	d.trajet.Conducteur = conducteur
	d.trajet.Statut = domain.TrajetTermine
	return d.trajet, nil
}

type notificateurFinFaux struct {
	appels int
	err    error
}

func (n *notificateurFinFaux) NotifierFinTrajet(_ context.Context, _ domain.Trajet) error {
	n.appels++
	return n.err
}

func TestTerminerTrajet(t *testing.T) {
	t.Run("identité requise", func(t *testing.T) {
		depot := &depotFinFaux{}
		uc := application.NouveauTerminerTrajet(depot, nil)
		if _, err := uc.Executer(context.Background(), "", "t1"); !errors.Is(err, application.ErrIdentiteManquante) {
			t.Fatalf("erreur = %v, attendu ErrIdentiteManquante", err)
		}
		if depot.appels != 0 {
			t.Fatalf("appels au dépôt = %d, attendu 0", depot.appels)
		}
	})

	t.Run("succès et notification au mieux", func(t *testing.T) {
		depot := &depotFinFaux{trajet: domain.Trajet{Identifiant: "t1"}}
		notif := &notificateurFinFaux{err: errors.New("UC12 indisponible")}
		uc := application.NouveauTerminerTrajet(depot, notif)
		termine, err := uc.Executer(context.Background(), "bob", "t1")
		if err != nil {
			t.Fatalf("terminer : %v", err)
		}
		if termine.Statut != domain.TrajetTermine || termine.Identifiant != "t1" || depot.conducteur != "bob" {
			t.Fatalf("trajet terminé = %+v; conducteur transmis = %q", termine, depot.conducteur)
		}
		if notif.appels != 1 {
			t.Fatalf("notifications = %d, attendu 1", notif.appels)
		}
	})

	t.Run("erreur du dépôt propagée sans notification", func(t *testing.T) {
		erreur := domain.ErrTrajetDejaTermine
		depot := &depotFinFaux{err: erreur}
		notif := &notificateurFinFaux{}
		uc := application.NouveauTerminerTrajet(depot, notif)
		if _, err := uc.Executer(context.Background(), "bob", "t1"); !errors.Is(err, erreur) {
			t.Fatalf("erreur = %v, attendu %v", err, erreur)
		}
		if notif.appels != 0 {
			t.Fatalf("notifications = %d, attendu 0", notif.appels)
		}
	})
}
