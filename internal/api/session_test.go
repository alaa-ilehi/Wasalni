package api_test

import (
	"context"
	"testing"
	"time"

	"github.com/alaa-ilehi/wasalni/internal/api"
	"github.com/alaa-ilehi/wasalni/internal/application"
	"github.com/alaa-ilehi/wasalni/internal/domain"
)

// hachageRapide : PBKDF2 coûte une centaine de millisecondes par hachage, et les tests HTTP vérifient
// le contrat, pas le coût. Le hachage réel (sel, itérations, vecteurs RFC 6070) est couvert par
// internal/infrastructure/passwords et par le test de fumée manuel.
type hachageRapide struct{}

func (hachageRapide) Hacher(motDePasse string) (domain.Credential, error) {
	return domain.Credential{Sel: []byte("sel-de-test"), Condensat: []byte("condensat:" + motDePasse), Iterations: 1}, nil
}

func (hachageRapide) Verifier(motDePasse string, credential domain.Credential) bool {
	return string(credential.Condensat) == "condensat:"+motDePasse
}

// annuaireDeTest : implémente les deux ports d'identité en mémoire, avec la même purge paresseuse
// des sessions échues que l'adaptateur SQLite.
type annuaireDeTest struct {
	credentials map[string]domain.Credential
	sessions    map[string]domain.Session
	instant     time.Time
}

func (a *annuaireDeTest) Enregistrer(_ context.Context, credential domain.Credential) error {
	if _, existe := a.credentials[credential.Identifiant]; existe {
		return domain.ErrIdentifiantDejaInscrit
	}
	a.credentials[credential.Identifiant] = credential
	return nil
}

func (a *annuaireDeTest) ParIdentifiant(_ context.Context, identifiant string) (domain.Credential, error) {
	credential, existe := a.credentials[identifiant]
	if !existe {
		return domain.Credential{}, domain.ErrIdentifiantsInvalides
	}
	return credential, nil
}

func (a *annuaireDeTest) Creer(_ context.Context, session domain.Session) error {
	a.sessions[session.Jeton] = session
	return nil
}

func (a *annuaireDeTest) ParJeton(_ context.Context, jeton string, maintenant time.Time) (domain.Session, error) {
	session, existe := a.sessions[jeton]
	if !existe || !session.ExpireLe.After(maintenant) {
		delete(a.sessions, jeton)
		return domain.Session{}, domain.ErrSessionInvalide
	}
	return session, nil
}

func (a *annuaireDeTest) Supprimer(_ context.Context, jeton string) error {
	if _, existe := a.sessions[jeton]; !existe {
		return domain.ErrSessionInvalide
	}
	delete(a.sessions, jeton)
	return nil
}

func (a *annuaireDeTest) Maintenant() time.Time { return a.instant }

type identiteDeTest struct {
	annuaire *annuaireDeTest
	auth     application.Authentification
}

func nouvelleIdentite() *identiteDeTest {
	annuaire := &annuaireDeTest{
		credentials: map[string]domain.Credential{},
		sessions:    map[string]domain.Session{},
		instant:     time.Date(2026, 11, 25, 8, 0, 0, 0, time.UTC),
	}
	return &identiteDeTest{
		annuaire: annuaire,
		auth:     application.NouvelleAuthentification(annuaire, annuaire, hachageRapide{}, annuaire),
	}
}

// jeton inscrit l'acteur au besoin puis rend un jeton de session utilisable dans l'en-tête.
func (i *identiteDeTest) jeton(t *testing.T, acteur string) string {
	t.Helper()
	if _, erreur := i.annuaire.ParIdentifiant(context.Background(), acteur); erreur == nil {
		jeton, err := i.auth.Connecter(context.Background(), acteur, "motdepasse")
		if err != nil {
			t.Fatalf("connexion de %q : %v", acteur, err)
		}
		return jeton
	}
	jeton, err := i.auth.Inscrire(context.Background(), acteur, "motdepasse")
	if err != nil {
		t.Fatalf("inscription de %q : %v", acteur, err)
	}
	return jeton
}

// autorisation rend la valeur exacte de l'en-tête Authorization pour cet acteur.
func (i *identiteDeTest) autorisation(t *testing.T, acteur string) string {
	t.Helper()
	return "Bearer " + i.jeton(t, acteur)
}

// identitePartagee : une instance pour tous les routeurs de test. Les tests ne partagent aucun état
// métier ; seul l'annuaire d'identités est commun, ce qui évite de le recâbler dans chaque banc.
var identitePartagee = nouvelleIdentite()

// autorisationDe : en-tête à poser pour agir au nom de l'acteur donné.
func autorisationDe(t *testing.T, acteur string) string {
	t.Helper()
	return identitePartagee.autorisation(t, acteur)
}

// deps remplit le seul champ d'identité d'un Deps de test.
func depsAuthentifies(deps api.Deps) api.Deps {
	deps.Authentification = identitePartagee.auth
	return deps
}
