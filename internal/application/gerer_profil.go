// Package application : cas d'utilisation. Orchestre le domaine et les ports, sans connaître HTTP ni SQL.
package application

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/alaa-ilehi/wasalni/internal/domain"
)

// ErrIdentiteManquante : aucun acteur identifié (en-tête absent ou vide), quel que soit l'appel.
var ErrIdentiteManquante = errors.New("identité manquante")

// Erreur d'application : la photo fournie est inutilisable (illisible ou impossible à ramener sous la limite).
// Elle traduit l'erreur du port de normalisation pour que l'interface n'ait pas à connaître l'infrastructure.
var ErrPhotoInvalide = errors.New("photo invalide")

// ErrImageTropGrande : la photo déclare plus de pixels que le budget (ADR 0008). Distincte d'ErrPhotoInvalide pour que
// le client sache que le refus vient de la taille et non d'un fichier illisible ; l'adaptateur de `cmd/wasalni` la pose
// en traduisant `photos.ErrImageTropGrande`, application n'importe pas l'infrastructure.
var ErrImageTropGrande = errors.New("photo trop grande")

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
				// Deux %w : l'API garde ErrPhotoInvalide (400) et peut distinguer ErrImageTropGrande.
				return domain.Profil{}, false, fmt.Errorf("%w : %w", ErrPhotoInvalide, err)
			}
			profil.Photo = normalisee
		}
	}

	return g.profils.Enregistrer(ctx, profil)
}
