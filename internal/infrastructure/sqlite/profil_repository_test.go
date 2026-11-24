package sqlite_test

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/alaa-ilehi/wasalni/internal/domain"
	"github.com/alaa-ilehi/wasalni/internal/infrastructure/sqlite"
)

func baseDeTest(t *testing.T) (*sqlite.ProfilRepository, func() error) {
	t.Helper()
	ctx := context.Background()
	chemin := filepath.Join(t.TempDir(), "test.db")
	db, err := sqlite.Ouvrir(ctx, chemin)
	if err != nil {
		t.Fatalf("ouverture de la base : %v", err)
	}
	return sqlite.NouveauProfilRepository(db), db.Close
}

func TestSchemaIdempotent(t *testing.T) {
	ctx := context.Background()
	chemin := filepath.Join(t.TempDir(), "test.db")
	db, err := sqlite.Ouvrir(ctx, chemin)
	if err != nil {
		t.Fatalf("première ouverture : %v", err)
	}
	if err := sqlite.AppliquerSchema(ctx, db); err != nil {
		t.Fatalf("seconde application du schéma : %v", err)
	}
	db.Close()

	// réouverture du même fichier : la table existe déjà
	db2, err := sqlite.Ouvrir(ctx, chemin)
	if err != nil {
		t.Fatalf("réouverture : %v", err)
	}
	defer db2.Close()
	if err := sqlite.AppliquerSchema(ctx, db2); err != nil {
		t.Fatalf("schéma réappliqué sur une base existante : %v", err)
	}
}

func TestEnregistrerCreePuisMetAJour(t *testing.T) {
	ctx := context.Background()
	depot, fermer := baseDeTest(t)
	defer fermer()

	profil := domain.Profil{Identifiant: "alice", Nom: "Alice", Age: 27, Sexe: domain.SexeFeminin, Photo: "cG5n"}
	cree, creation, err := depot.Enregistrer(ctx, profil)
	if err != nil {
		t.Fatalf("création : %v", err)
	}
	if !creation {
		t.Error("le premier enregistrement doit être une création")
	}
	if cree.CreeLe.IsZero() || cree.MajLe.IsZero() {
		t.Error("les horodatages doivent être renseignés")
	}

	profil.Nom = "Alice B."
	profil.Age = 31
	profil.Sexe = ""
	profil.Photo = ""
	misAJour, creation, err := depot.Enregistrer(ctx, profil)
	if err != nil {
		t.Fatalf("mise à jour : %v", err)
	}
	if creation {
		t.Error("le second enregistrement ne doit pas être une création")
	}
	if misAJour.Nom != "Alice B." || misAJour.Age != 31 || misAJour.Sexe != "" || misAJour.Photo != "" {
		t.Errorf("profil mis à jour = %+v", misAJour)
	}
	if !misAJour.CreeLe.Equal(cree.CreeLe) {
		t.Errorf("created_at a changé : %v puis %v", cree.CreeLe, misAJour.CreeLe)
	}
}

func TestParIdentifiant(t *testing.T) {
	ctx := context.Background()
	depot, fermer := baseDeTest(t)
	defer fermer()

	if _, err := depot.ParIdentifiant(ctx, "inconnu"); !errors.Is(err, domain.ErrProfilIntrouvable) {
		t.Fatalf("erreur = %v, attendu ErrProfilIntrouvable", err)
	}

	profil := domain.Profil{Identifiant: "bob", Nom: "Bob", Age: 30, Photo: "cG5n"}
	if _, _, err := depot.Enregistrer(ctx, profil); err != nil {
		t.Fatalf("création : %v", err)
	}

	lu, err := depot.ParIdentifiant(ctx, "bob")
	if err != nil {
		t.Fatalf("lecture : %v", err)
	}
	if lu.Nom != "Bob" || lu.Age != 30 || lu.Photo != "cG5n" || lu.Sexe != "" {
		t.Errorf("profil lu = %+v", lu)
	}
	if time.Since(lu.MajLe) > time.Minute {
		t.Errorf("updated_at incohérent : %v", lu.MajLe)
	}
}
