package application_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/alaa-ilehi/wasalni/internal/application"
	"github.com/alaa-ilehi/wasalni/internal/domain"
)

// hacheurFaux : coût nul, mais même contrat que l'adaptateur réel (sel rendu, vérification stricte).
// Le coût réel de PBKDF2 est vérifié dans internal/infrastructure/passwords.
type hacheurFaux struct{}

func (hacheurFaux) Hacher(motDePasse string) (domain.Credential, error) {
	return domain.Credential{Sel: []byte("sel"), Condensat: []byte("condensat:" + motDePasse), Iterations: 1}, nil
}

func (hacheurFaux) Verifier(motDePasse string, credential domain.Credential) bool {
	return string(credential.Condensat) == "condensat:"+motDePasse
}

type credentialsEnMemoire struct {
	parIdentifiant map[string]domain.Credential
}

func nouveauCredentials() *credentialsEnMemoire {
	return &credentialsEnMemoire{parIdentifiant: map[string]domain.Credential{}}
}

func (c *credentialsEnMemoire) Enregistrer(_ context.Context, credential domain.Credential) error {
	if _, existe := c.parIdentifiant[credential.Identifiant]; existe {
		return domain.ErrIdentifiantDejaInscrit
	}
	c.parIdentifiant[credential.Identifiant] = credential
	return nil
}

func (c *credentialsEnMemoire) ParIdentifiant(_ context.Context, identifiant string) (domain.Credential, error) {
	credential, existe := c.parIdentifiant[identifiant]
	if !existe {
		return domain.Credential{}, domain.ErrIdentifiantsInvalides
	}
	return credential, nil
}

type sessionsEnMemoire struct {
	parJeton map[string]domain.Session
}

func nouvellesSessions() *sessionsEnMemoire {
	return &sessionsEnMemoire{parJeton: map[string]domain.Session{}}
}

func (s *sessionsEnMemoire) Creer(_ context.Context, session domain.Session) error {
	s.parJeton[session.Jeton] = session
	return nil
}

func (s *sessionsEnMemoire) ParJeton(_ context.Context, jeton string, maintenant time.Time) (domain.Session, error) {
	session, existe := s.parJeton[jeton]
	if !existe {
		return domain.Session{}, domain.ErrSessionInvalide
	}
	if !session.ExpireLe.After(maintenant) {
		delete(s.parJeton, jeton)
		return domain.Session{}, domain.ErrSessionInvalide
	}
	return session, nil
}

func (s *sessionsEnMemoire) Supprimer(_ context.Context, jeton string) error {
	if _, existe := s.parJeton[jeton]; !existe {
		return domain.ErrSessionInvalide
	}
	delete(s.parJeton, jeton)
	return nil
}

func bancAuthentification(maintenant time.Time) (application.Authentification, *sessionsEnMemoire) {
	sessions := nouvellesSessions()
	return application.NouvelleAuthentification(nouveauCredentials(), sessions, hacheurFaux{}, horlogeFixe{instant: maintenant}), sessions
}

func TestInscrireOuvreUneSessionUtilisable(t *testing.T) {
	ctx := context.Background()
	uc, sessions := bancAuthentification(time.Date(2026, 11, 25, 8, 0, 0, 0, time.UTC))

	jeton, err := uc.Inscrire(ctx, "amine", "motdepasse")
	if err != nil {
		t.Fatalf("inscription : %v", err)
	}
	if len(jeton) < 40 {
		t.Fatalf("jeton de %d caractères, attendu au moins 40 (32 octets en base64url)", len(jeton))
	}
	if len(sessions.parJeton) != 1 {
		t.Fatalf("%d session(s) enregistrée(s), attendu 1", len(sessions.parJeton))
	}
	for empreinte, session := range sessions.parJeton {
		if empreinte == jeton {
			t.Fatal("le jeton est stocké en clair : seule son empreinte doit être conservée")
		}
		if session.Identifiant != "amine" {
			t.Fatalf("session liée à %q, attendu amine", session.Identifiant)
		}
		if session.ExpireLe.Sub(session.CreeLe) != application.DureeSession {
			t.Fatalf("durée de session %v, attendu %v", session.ExpireLe.Sub(session.CreeLe), application.DureeSession)
		}
	}

	acteur, err := uc.Resoudre(ctx, jeton)
	if err != nil || acteur != "amine" {
		t.Fatalf("résolution = (%q, %v), attendu (amine, nil)", acteur, err)
	}
}

func TestInscrireRefuseUnIdentifiantDejaInscrit(t *testing.T) {
	ctx := context.Background()
	uc, _ := bancAuthentification(time.Now().UTC())

	if _, err := uc.Inscrire(ctx, "amine", "motdepasse"); err != nil {
		t.Fatalf("première inscription : %v", err)
	}
	if _, err := uc.Inscrire(ctx, "  amine ", "autrepass"); !errors.Is(err, domain.ErrIdentifiantDejaInscrit) {
		t.Fatalf("erreur = %v, attendu ErrIdentifiantDejaInscrit", err)
	}
}

func TestInscrireValideIdentifiantEtMotDePasse(t *testing.T) {
	ctx := context.Background()
	uc, _ := bancAuthentification(time.Now().UTC())

	if _, err := uc.Inscrire(ctx, "   ", "motdepasse"); !errors.Is(err, domain.ErrIdentifiantsInvalides) {
		t.Fatalf("identifiant vide : erreur = %v", err)
	}
	cas := []struct {
		nom        string
		motDePasse string
	}{
		{"trop court", "court"},
		{"vide", ""},
		{"trop long", "m" + string(make([]byte, domain.MotDePasseLongueurMax))},
	}
	for _, c := range cas {
		t.Run(c.nom, func(t *testing.T) {
			if _, err := uc.Inscrire(ctx, "amine", c.motDePasse); !errors.Is(err, domain.ErrMotDePasseInvalide) {
				t.Fatalf("erreur = %v, attendu ErrMotDePasseInvalide", err)
			}
		})
	}
}

func TestConnecterRendUneErreurIdentiquePourIdentifiantEtMotDePasse(t *testing.T) {
	ctx := context.Background()
	uc, _ := bancAuthentification(time.Now().UTC())
	if _, err := uc.Inscrire(ctx, "amine", "motdepasse"); err != nil {
		t.Fatalf("inscription : %v", err)
	}

	jeton, err := uc.Connecter(ctx, "amine", "motdepasse")
	if err != nil {
		t.Fatalf("connexion : %v", err)
	}
	if acteur, err := uc.Resoudre(ctx, jeton); err != nil || acteur != "amine" {
		t.Fatalf("résolution = (%q, %v)", acteur, err)
	}

	_, errMotDePasse := uc.Connecter(ctx, "amine", "mauvaispass")
	_, errIdentifiant := uc.Connecter(ctx, "inconnu", "motdepasse")
	if !errors.Is(errMotDePasse, domain.ErrIdentifiantsInvalides) || !errors.Is(errIdentifiant, domain.ErrIdentifiantsInvalides) {
		t.Fatalf("erreurs = (%v, %v), attendu deux fois ErrIdentifiantsInvalides", errMotDePasse, errIdentifiant)
	}
	if errMotDePasse.Error() != errIdentifiant.Error() {
		t.Fatalf("les deux refus doivent être indiscernables : %q vs %q", errMotDePasse, errIdentifiant)
	}
}

func TestDeconnecterInvalideLeJeton(t *testing.T) {
	ctx := context.Background()
	uc, _ := bancAuthentification(time.Now().UTC())
	jeton, err := uc.Inscrire(ctx, "amine", "motdepasse")
	if err != nil {
		t.Fatalf("inscription : %v", err)
	}

	if err := uc.Deconnecter(ctx, jeton); err != nil {
		t.Fatalf("déconnexion : %v", err)
	}
	if _, err := uc.Resoudre(ctx, jeton); !errors.Is(err, domain.ErrSessionInvalide) {
		t.Fatalf("erreur = %v, attendu ErrSessionInvalide", err)
	}
	if err := uc.Deconnecter(ctx, jeton); !errors.Is(err, domain.ErrSessionInvalide) {
		t.Fatalf("seconde déconnexion : erreur = %v, attendu ErrSessionInvalide", err)
	}
}

func TestResoudreRefuseUneSessionExpiree(t *testing.T) {
	ctx := context.Background()
	maintenant := time.Date(2026, 11, 25, 8, 0, 0, 0, time.UTC)
	sessions := nouvellesSessions()
	uc := application.NouvelleAuthentification(nouveauCredentials(), sessions, hacheurFaux{}, horlogeFixe{instant: maintenant})

	jeton, err := uc.Inscrire(ctx, "amine", "motdepasse")
	if err != nil {
		t.Fatalf("inscription : %v", err)
	}

	apresExpiration := maintenant.Add(application.DureeSession + time.Minute)
	ucTardive := application.NouvelleAuthentification(nouveauCredentials(), sessions, hacheurFaux{}, horlogeFixe{instant: apresExpiration})
	if _, err := ucTardive.Resoudre(ctx, jeton); !errors.Is(err, domain.ErrSessionInvalide) {
		t.Fatalf("erreur = %v, attendu ErrSessionInvalide", err)
	}
	if len(sessions.parJeton) != 0 {
		t.Fatalf("%d session(s) restante(s), la session expirée doit être supprimée", len(sessions.parJeton))
	}
}

func TestResoudreRefuseUnJetonVide(t *testing.T) {
	uc, _ := bancAuthentification(time.Now().UTC())
	if _, err := uc.Resoudre(context.Background(), "  "); !errors.Is(err, domain.ErrSessionInvalide) {
		t.Fatalf("erreur = %v, attendu ErrSessionInvalide", err)
	}
}
