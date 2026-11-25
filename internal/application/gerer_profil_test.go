package application_test

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/alaa-ilehi/wasalni/internal/application"
	"github.com/alaa-ilehi/wasalni/internal/domain"
)

// depotFaux : port ProfilRepository en mémoire, pour tester le cas d'usage seul.
type depotFaux struct {
	profils map[string]domain.Profil
	erreur  error
}

func (d *depotFaux) Enregistrer(_ context.Context, p domain.Profil) (domain.Profil, bool, error) {
	if d.erreur != nil {
		return domain.Profil{}, false, d.erreur
	}
	_, existe := d.profils[p.Identifiant]
	d.profils[p.Identifiant] = p
	return p, !existe, nil
}

func (d *depotFaux) ParIdentifiant(_ context.Context, identifiant string) (domain.Profil, error) {
	if d.erreur != nil {
		return domain.Profil{}, d.erreur
	}
	p, ok := d.profils[identifiant]
	if !ok {
		return domain.Profil{}, domain.ErrProfilIntrouvable
	}
	return p, nil
}

type normaliseurFaux struct{ appels int }

func (n *normaliseurFaux) Normaliser(photo string) (string, error) {
	n.appels++
	if photo == "corrompue" {
		return "", errors.New("image illisible")
	}
	if photo == "trop-grande" {
		// Même forme que l'adaptateur de cmd/wasalni : photos.ErrImageTropGrande traduite en erreur d'application.
		return "", fmt.Errorf("%w : 30000 x 30000 pixels", application.ErrImageTropGrande)
	}
	return "png-normalisé", nil
}

func nouveauCas(t *testing.T) (application.GererProfil, *depotFaux, *normaliseurFaux) {
	t.Helper()
	depot := &depotFaux{profils: map[string]domain.Profil{}}
	photos := &normaliseurFaux{}
	return application.NouveauGererProfil(depot, photos), depot, photos
}

func texte(s string) *string { return &s }

func TestEnregistrerCreePuisMetAJour(t *testing.T) {
	gerer, _, _ := nouveauCas(t)
	ctx := context.Background()

	profil, cree, err := gerer.Enregistrer(ctx, "alice", application.SaisieProfil{Nom: "Alice", Age: 27})
	if err != nil || !cree {
		t.Fatalf("création : cree=%v erreur=%v", cree, err)
	}
	if profil.Identifiant != "alice" {
		t.Errorf("identifiant = %q", profil.Identifiant)
	}

	profil, cree, err = gerer.Enregistrer(ctx, "alice", application.SaisieProfil{Nom: "Alice B.", Age: 28})
	if err != nil || cree {
		t.Fatalf("mise à jour : cree=%v erreur=%v", cree, err)
	}
	if profil.Nom != "Alice B." || profil.Age != 28 {
		t.Errorf("profil mis à jour = %+v", profil)
	}
}

func TestEnregistrerRefuseSaisieInvalide(t *testing.T) {
	gerer, depot, photos := nouveauCas(t)
	_, _, err := gerer.Enregistrer(context.Background(), "alice", application.SaisieProfil{Nom: "  ", Age: 27})
	if !errors.Is(err, domain.ErrNomRequis) {
		t.Fatalf("erreur = %v, attendu ErrNomRequis", err)
	}
	if len(depot.profils) != 0 || photos.appels != 0 {
		t.Errorf("une saisie invalide ne doit rien écrire ni normaliser")
	}
}

func TestEnregistrerConserveLaPhotoQuandElleEstAbsente(t *testing.T) {
	gerer, _, _ := nouveauCas(t)
	ctx := context.Background()

	if _, _, err := gerer.Enregistrer(ctx, "alice", application.SaisieProfil{
		Nom: "Alice", Age: 27, Photo: texte("photo-initiale"),
	}); err != nil {
		t.Fatalf("création avec photo : %v", err)
	}

	profil, _, err := gerer.Enregistrer(ctx, "alice", application.SaisieProfil{Nom: "Alice", Age: 28})
	if err != nil {
		t.Fatalf("mise à jour sans photo : %v", err)
	}
	if profil.Photo != "png-normalisé" {
		t.Errorf("photo conservée = %q, attendu la photo normalisée existante", profil.Photo)
	}
}

func TestEnregistrerEffaceLaPhotoAvecUneChaineVide(t *testing.T) {
	gerer, _, _ := nouveauCas(t)
	ctx := context.Background()
	if _, _, err := gerer.Enregistrer(ctx, "alice", application.SaisieProfil{
		Nom: "Alice", Age: 27, Photo: texte("photo-initiale"),
	}); err != nil {
		t.Fatalf("création avec photo : %v", err)
	}
	profil, _, err := gerer.Enregistrer(ctx, "alice", application.SaisieProfil{
		Nom: "Alice", Age: 27, Photo: texte("   "),
	})
	if err != nil {
		t.Fatalf("effacement de la photo : %v", err)
	}
	if profil.Photo != "" {
		t.Errorf("photo = %q, attendu vide", profil.Photo)
	}
}

func TestEnregistrerPropageLErreurDuNormaliseur(t *testing.T) {
	gerer, _, photos := nouveauCas(t)
	_, _, err := gerer.Enregistrer(context.Background(), "alice", application.SaisieProfil{
		Nom: "Alice", Age: 27, Photo: texte("corrompue"),
	})
	if err == nil {
		t.Fatal("une image illisible doit être refusée")
	}
	if photos.appels != 1 {
		t.Errorf("normalisations = %d, attendu 1", photos.appels)
	}
}

func TestConsulterProfilInconnu(t *testing.T) {
	gerer, _, _ := nouveauCas(t)
	if _, err := gerer.Consulter(context.Background(), "inconnu"); !errors.Is(err, domain.ErrProfilIntrouvable) {
		t.Fatalf("erreur = %v, attendu ErrProfilIntrouvable", err)
	}
}

// TestUnePhotoTropGrandeGardeSonErreurPropre : l'API répond 400 dans les deux cas, mais ErrImageTropGrande reste
// reconnaissable derrière l'emballage ErrPhotoInvalide (deux %w).
func TestUnePhotoTropGrandeGardeSonErreurPropre(t *testing.T) {
	gerer, _, _ := nouveauCas(t)
	_, _, err := gerer.Enregistrer(context.Background(), "alice", application.SaisieProfil{
		Nom: "Alice", Age: 30, Photo: texte("trop-grande"),
	})
	if !errors.Is(err, application.ErrImageTropGrande) {
		t.Fatalf("erreur = %v, attendu ErrImageTropGrande", err)
	}
	if !errors.Is(err, application.ErrPhotoInvalide) {
		t.Fatalf("erreur = %v, attendu aussi ErrPhotoInvalide (400)", err)
	}
}
