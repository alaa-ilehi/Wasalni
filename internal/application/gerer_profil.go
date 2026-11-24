// Package application : cas d'utilisation. Orchestre le domaine et les ports, sans connaître HTTP ni SQL.
package application

import (
	"context"
	"errors"
	"strings"

	"github.com/alaa-ilehi/wasalni/internal/domain"
)

// NormaliseurPhoto : port de traitement d'image (décodage, réduction, encodage).
type NormaliseurPhoto interface {
	Normaliser(photoBase64 string) (string, error)
}

// GererProfil : UC8, créer ou modifier le profil de l'utilisateur identifié.
type GererProfil struct {
	profils domain.ProfilRepository
	photos  NormaliseurPhoto
}

func NouveauGererProfil(profils domain.ProfilRepository, photos NormaliseurPhoto) GererProfil {
	return GererProfil{profils: profils, photos: photos}
}

// SaisieProfil est le corps accepté par le cas d'utilisation.
// Photo nil : la photo existante est conservée ; "" : la photo est effacée ; sinon la valeur est normalisée.
type SaisieProfil struct {
	Nom   string  `json:"name"`
	Age   int     `json:"age"`
	Sexe  string  `json:"sex,omitempty"`
	Photo *string `json:"photo"`
}

// Consulter renvoie le profil de l'identifiant, ou domain.ErrProfilIntrouvable.
func (g GererProfil) Consulter(ctx context.Context, identifiant string) (domain.Profil, error) {
	return g.profils.ParIdentifiant(ctx, identifiant)
}

// Enregistrer valide la saisie, normalise la photo si elle est fournie, puis enregistre le profil.
// Le booléen du résultat vaut true quand le profil vient d'être créé.
func (g GererProfil) Enregistrer(ctx context.Context, identifiant string, saisie SaisieProfil) (domain.Profil, bool, error) {
	profil := domain.Profil{
		Identifiant: identifiant,
		Nom:         strings.TrimSpace(saisie.Nom),
		Age:         saisie.Age,
		Sexe:        domain.Sexe(strings.ToUpper(strings.TrimSpace(saisie.Sexe))),
	}
	if err := profil.Valider(); err != nil {
		return domain.Profil{}, false, err
	}

	existant, err := g.profils.ParIdentifiant(ctx, identifiant)
	switch {
	case err == nil:
		profil.Photo = existant.Photo // une mise à jour partielle conserve la photo
	case errors.Is(err, domain.ErrProfilIntrouvable):
	default:
		return domain.Profil{}, false, err
	}

	if saisie.Photo != nil {
		if strings.TrimSpace(*saisie.Photo) == "" {
			profil.Photo = ""
		} else {
			normalisee, err := g.photos.Normaliser(*saisie.Photo)
			if err != nil {
				return domain.Profil{}, false, err
			}
			profil.Photo = normalisee
		}
	}

	return g.profils.Enregistrer(ctx, profil)
}
