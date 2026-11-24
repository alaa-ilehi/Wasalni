package application

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"github.com/alaa-ilehi/wasalni/internal/domain"
)

// GererTrajet regroupe les deux opérations d'un même cas d'utilisation UC9.
type GererTrajet struct {
	depot   domain.TrajetGestionRepository
	lecture domain.TrajetRepository
	horloge Horloge
	notif   NotificateurGestionTrajet
}

type NotificateurGestionTrajet interface {
	NotifierModificationTrajet(context.Context, domain.Trajet) error
	NotifierSuppressionTrajet(context.Context, domain.Trajet) error
}

func NouveauGererTrajet(depot domain.TrajetGestionRepository, lecture domain.TrajetRepository, horloge Horloge) GererTrajet {
	return GererTrajet{depot: depot, lecture: lecture, horloge: horloge}
}

func (uc GererTrajet) AvecNotificateur(n NotificateurGestionTrajet) GererTrajet {
	uc.notif = n
	return uc
}

func (uc GererTrajet) Modifier(ctx context.Context, conducteur, id string, saisie SaisieTrajet) (domain.Trajet, error) {
	if conducteur == "" {
		return domain.Trajet{}, ErrIdentiteManquante
	}
	if uc.horloge == nil {
		return domain.Trajet{}, fmt.Errorf("horloge absente")
	}
	base, err := uc.lecture.ParIdentifiant(ctx, id)
	if err != nil {
		return domain.Trajet{}, err
	}
	if err := base.ValiderTerminaison(conducteur); err != nil {
		return domain.Trajet{}, err
	}
	departBrut := saisie.Depart
	if strings.TrimSpace(departBrut) == "" {
		departBrut = saisie.DepartSecours
	}
	depart, err := instant(departBrut)
	if err != nil {
		return domain.Trajet{}, err
	}
	delai := domain.DureeDelaiAnnulationParDefaut
	if saisie.DelaiAnnulation != nil {
		delai = heures(*saisie.DelaiAnnulation)
	}
	modifie := domain.Trajet{Identifiant: id, Conducteur: conducteur, Origine: strings.TrimSpace(saisie.Origine), Destination: strings.TrimSpace(saisie.Destination), Depart: depart, PlacesTotal: saisie.PlacesTotal, Places: saisie.PlacesTotal, Prix: saisie.Prix, DelaiAnnulation: delai, Statut: domain.TrajetPublie, CreeLe: base.CreeLe}
	if err := modifie.ValiderPublication(uc.horloge.Maintenant()); err != nil {
		return domain.Trajet{}, err
	}
	resultat, err := uc.depot.Modifier(ctx, modifie, conducteur)
	if err == nil && uc.notif != nil {
		if e := uc.notif.NotifierModificationTrajet(ctx, resultat); e != nil {
			slog.Warn("notification modification trajet échouée", "trajet", id, "erreur", e)
		}
	}
	return resultat, err
}

func (uc GererTrajet) Supprimer(ctx context.Context, conducteur, id string) error {
	if conducteur == "" {
		return ErrIdentiteManquante
	}
	trajet, err := uc.lecture.ParIdentifiant(ctx, id)
	if err != nil {
		return err
	}
	if err := trajet.ValiderTerminaison(conducteur); err != nil {
		return err
	}
	if err := uc.depot.Supprimer(ctx, id, conducteur); err != nil {
		return err
	}
	if uc.notif != nil {
		if e := uc.notif.NotifierSuppressionTrajet(ctx, trajet); e != nil {
			slog.Warn("notification suppression trajet échouée", "trajet", id, "erreur", e)
		}
	}
	return nil
}
