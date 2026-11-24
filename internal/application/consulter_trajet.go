package application

import (
	"context"
	"errors"

	"github.com/alaa-ilehi/wasalni/internal/domain"
)

// DetailTrajet : UC7, le trajet choisi depuis UC2 avec le profil de son conducteur.
// Profil vaut nil quand le conducteur n'a pas encore renseigné de profil (UC8 facultatif).
type DetailTrajet struct {
	Trajet domain.Trajet
	Profil *domain.Profil
}

// ConsulterTrajet : UC7, lecture seule d'un trajet, sans identité requise.
type ConsulterTrajet struct {
	trajets domain.TrajetRepository
	profils domain.ProfilRepository
}

func NouveauConsulterTrajet(trajets domain.TrajetRepository, profils domain.ProfilRepository) ConsulterTrajet {
	return ConsulterTrajet{trajets: trajets, profils: profils}
}

// Executer renvoie le trajet et, quand il existe, le profil du conducteur.
// Un trajet inconnu ou terminé est traité comme introuvable (A1 de la fiche UC7).
// L'absence de profil n'est pas une erreur : UC7 affiche alors un conducteur vide.
func (uc ConsulterTrajet) Executer(ctx context.Context, identifiant string) (DetailTrajet, error) {
	trajet, err := uc.trajets.ParIdentifiant(ctx, identifiant)
	if err != nil {
		return DetailTrajet{}, err
	}
	if trajet.Statut != domain.TrajetPublie {
		return DetailTrajet{}, domain.ErrTrajetIntrouvable
	}

	detail := DetailTrajet{Trajet: trajet}
	profil, err := uc.profils.ParIdentifiant(ctx, trajet.Conducteur)
	switch {
	case err == nil:
		detail.Profil = &profil
	case errors.Is(err, domain.ErrProfilIntrouvable):
		// Conducteur sans profil : aucune information UC8 à afficher.
	default:
		return DetailTrajet{}, err
	}
	return detail, nil
}
