package domain

import "context"

// ProfilRepository : persistance des profils, vue depuis le domaine.
// Contrat : Enregistrer crée ou remplace le profil de l'identifiant fourni.
type ProfilRepository interface {
	Enregistrer(ctx context.Context, p Profil) (Profil, bool, error) // booléen : true si création
	ParIdentifiant(ctx context.Context, identifiant string) (Profil, error)
}
