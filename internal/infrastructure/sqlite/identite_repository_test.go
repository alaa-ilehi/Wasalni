package sqlite_test

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/alaa-ilehi/wasalni/internal/domain"
	"github.com/alaa-ilehi/wasalni/internal/infrastructure/passwords"
	"github.com/alaa-ilehi/wasalni/internal/infrastructure/sqlite"
)

func baseIdentite(t *testing.T) (*sqlite.CredentialRepository, *sqlite.SessionRepository) {
	t.Helper()
	ctx := context.Background()
	chemin := filepath.Join(t.TempDir(), "identite.db")
	db, err := sqlite.Ouvrir(ctx, chemin)
	if err != nil {
		t.Fatalf("ouverture de la base : %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return sqlite.NouveauCredentialRepository(db), sqlite.NouveauSessionRepository(db)
}

func TestMotDePasseEnregistrePuisRelu(t *testing.T) {
	ctx := context.Background()
	credentials, _ := baseIdentite(t)

	credential, err := passwords.Hacher("motdepasse")
	if err != nil {
		t.Fatalf("hachage : %v", err)
	}
	credential.Identifiant = "amine"
	if err := credentials.Enregistrer(ctx, credential); err != nil {
		t.Fatalf("enregistrement : %v", err)
	}

	relu, err := credentials.ParIdentifiant(ctx, "amine")
	if err != nil {
		t.Fatalf("relecture : %v", err)
	}
	if string(relu.Sel) != string(credential.Sel) {
		t.Fatal("le sel relu diffère de celui enregistré")
	}
	if string(relu.Condensat) != string(credential.Condensat) {
		t.Fatal("le condensé relu diffère de celui enregistré")
	}
	if relu.Iterations != credential.Iterations {
		t.Fatalf("itérations relues = %d, attendu %d", relu.Iterations, credential.Iterations)
	}
	if !passwords.Verifier("motdepasse", relu) || passwords.Verifier("mauvaispass", relu) {
		t.Fatal("la vérification du mot de passe relu est fausse")
	}
}

func TestMotDePasseEnDoubleRefuse(t *testing.T) {
	ctx := context.Background()
	credentials, _ := baseIdentite(t)

	for i := 0; i < 2; i++ {
		credential, err := passwords.Hacher("motdepasse")
		if err != nil {
			t.Fatalf("hachage : %v", err)
		}
		credential.Identifiant = "amine"
		err = credentials.Enregistrer(ctx, credential)
		if i == 0 && err != nil {
			t.Fatalf("premier enregistrement : %v", err)
		}
		if i == 1 && !errors.Is(err, domain.ErrIdentifiantDejaInscrit) {
			t.Fatalf("second enregistrement : erreur = %v, attendu ErrIdentifiantDejaInscrit", err)
		}
	}
}

func TestMotDePasseDIdentifiantInconnu(t *testing.T) {
	credentials, _ := baseIdentite(t)
	if _, err := credentials.ParIdentifiant(context.Background(), "inconnu"); !errors.Is(err, domain.ErrIdentifiantsInvalides) {
		t.Fatalf("erreur = %v, attendu ErrIdentifiantsInvalides", err)
	}
}

func TestSessionCreeeRelueEtSupprimee(t *testing.T) {
	ctx := context.Background()
	_, sessions := baseIdentite(t)
	maintenant := time.Date(2026, 11, 25, 8, 0, 0, 0, time.UTC)

	session := domain.Session{
		Jeton:       "empreinte-du-jeton",
		Identifiant: "amine",
		CreeLe:      maintenant,
		ExpireLe:    maintenant.Add(time.Hour),
	}
	if err := sessions.Creer(ctx, session); err != nil {
		t.Fatalf("création : %v", err)
	}

	relue, err := sessions.ParJeton(ctx, session.Jeton, maintenant)
	if err != nil {
		t.Fatalf("relecture : %v", err)
	}
	if relue.Identifiant != "amine" {
		t.Fatalf("identifiant relu = %q, attendu amine", relue.Identifiant)
	}
	if !relue.ExpireLe.Equal(session.ExpireLe) {
		t.Fatalf("expiration relue = %v, attendu %v", relue.ExpireLe, session.ExpireLe)
	}

	if err := sessions.Supprimer(ctx, session.Jeton); err != nil {
		t.Fatalf("suppression : %v", err)
	}
	if _, err := sessions.ParJeton(ctx, session.Jeton, maintenant); !errors.Is(err, domain.ErrSessionInvalide) {
		t.Fatalf("après suppression : erreur = %v, attendu ErrSessionInvalide", err)
	}
	if err := sessions.Supprimer(ctx, session.Jeton); !errors.Is(err, domain.ErrSessionInvalide) {
		t.Fatalf("seconde suppression : erreur = %v, attendu ErrSessionInvalide", err)
	}
}

// Une session échue n'est jamais rendue, et sa ligne disparaît à la lecture : la purge est paresseuse.
func TestSessionExpireeSupprimeeALaLecture(t *testing.T) {
	ctx := context.Background()
	_, sessions := baseIdentite(t)
	creation := time.Date(2026, 11, 25, 8, 0, 0, 0, time.UTC)

	session := domain.Session{
		Jeton:       "empreinte-expiree",
		Identifiant: "amine",
		CreeLe:      creation,
		ExpireLe:    creation.Add(time.Minute),
	}
	if err := sessions.Creer(ctx, session); err != nil {
		t.Fatalf("création : %v", err)
	}

	apres := creation.Add(2 * time.Minute)
	if _, err := sessions.ParJeton(ctx, session.Jeton, apres); !errors.Is(err, domain.ErrSessionInvalide) {
		t.Fatalf("erreur = %v, attendu ErrSessionInvalide", err)
	}

	// La session échue a été purgée : même à l'instant de création, elle n'existe plus.
	if _, err := sessions.ParJeton(ctx, session.Jeton, creation); !errors.Is(err, domain.ErrSessionInvalide) {
		t.Fatalf("la session expirée n'a pas été supprimée : erreur = %v", err)
	}
}
