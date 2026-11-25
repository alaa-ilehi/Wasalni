package application

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"strings"
	"time"

	"github.com/alaa-ilehi/wasalni/internal/domain"
)

// IdentifiantLongueurMax borne la clé d'un compte, comme le nom d'un profil.
const IdentifiantLongueurMax = 64

// DureeSession : 24 h. Passé ce délai le jeton est refusé et sa ligne supprimée à la lecture (ADR 0011).
const DureeSession = 24 * time.Hour

// HacheurMotDePasse : port vers l'adaptateur de hachage, déclaré par l'application comme
// NormaliseurPhoto l'est pour UC8. L'application ne connaît ni PBKDF2 ni son coût.
type HacheurMotDePasse interface {
	Hacher(motDePasse string) (domain.Credential, error)
	Verifier(motDePasse string, credential domain.Credential) bool
}

// Authentification : UC13, inscription, connexion, déconnexion et résolution d'un jeton.
// Aucun jeton en clair n'est conservé : la base ne voit que son hachage SHA-256.
type Authentification struct {
	credentials domain.CredentialRepository
	sessions    domain.SessionRepository
	hacheur     HacheurMotDePasse
	horloge     Horloge
}

func NouvelleAuthentification(
	credentials domain.CredentialRepository,
	sessions domain.SessionRepository,
	hacheur HacheurMotDePasse,
	horloge Horloge,
) Authentification {
	return Authentification{credentials: credentials, sessions: sessions, hacheur: hacheur, horloge: horloge}
}

// Inscrire crée le mot de passe d'un identifiant puis ouvre une session. Un identifiant qui possède
// déjà un profil peut s'inscrire : le compte et le profil sont deux objets distincts (ADR 0011).
func (uc Authentification) Inscrire(ctx context.Context, identifiant, motDePasse string) (string, error) {
	identifiant = strings.TrimSpace(identifiant)
	if identifiant == "" || len(identifiant) > IdentifiantLongueurMax {
		return "", domain.ErrIdentifiantsInvalides
	}
	if err := validerMotDePasse(motDePasse); err != nil {
		return "", err
	}

	credential, err := uc.hacheur.Hacher(motDePasse)
	if err != nil {
		return "", fmt.Errorf("hachage du mot de passe : %w", err)
	}
	credential.Identifiant = identifiant
	if err := uc.credentials.Enregistrer(ctx, credential); err != nil {
		return "", err
	}
	return uc.ouvrirSession(ctx, identifiant)
}

// Connecter vérifie le couple (identifiant, mot de passe) et ouvre une session. L'identifiant inconnu
// et le mot de passe faux rendent la même erreur : la réponse ne dit pas lequel des deux est faux.
func (uc Authentification) Connecter(ctx context.Context, identifiant, motDePasse string) (string, error) {
	credential, err := uc.credentials.ParIdentifiant(ctx, strings.TrimSpace(identifiant))
	if err != nil {
		return "", err
	}
	if !uc.hacheur.Verifier(motDePasse, credential) {
		return "", domain.ErrIdentifiantsInvalides
	}
	return uc.ouvrirSession(ctx, credential.Identifiant)
}

// Deconnecter invalide le jeton présenté. Un jeton inconnu est une session invalide, pas un succès.
func (uc Authentification) Deconnecter(ctx context.Context, jeton string) error {
	return uc.sessions.Supprimer(ctx, hacherJeton(jeton))
}

// Resoudre rend l'identifiant porté par un jeton valide. C'est le seul point que l'adaptateur HTTP
// utilise pour connaître l'acteur d'une requête.
func (uc Authentification) Resoudre(ctx context.Context, jeton string) (string, error) {
	if strings.TrimSpace(jeton) == "" {
		return "", domain.ErrSessionInvalide
	}
	session, err := uc.sessions.ParJeton(ctx, hacherJeton(jeton), uc.horloge.Maintenant())
	if err != nil {
		return "", err
	}
	return session.Identifiant, nil
}

func (uc Authentification) ouvrirSession(ctx context.Context, identifiant string) (string, error) {
	jeton, err := jetonNeuf()
	if err != nil {
		return "", err
	}
	maintenant := uc.horloge.Maintenant()
	session := domain.Session{
		Jeton:       hacherJeton(jeton),
		Identifiant: identifiant,
		CreeLe:      maintenant,
		ExpireLe:    maintenant.Add(DureeSession),
	}
	if err := uc.sessions.Creer(ctx, session); err != nil {
		return "", fmt.Errorf("ouverture de la session : %w", err)
	}
	return jeton, nil
}

// validerMotDePasse borne la taille avant tout hachage : un mot de passe vide ou démesuré est refusé
// en 400, sans coût de calcul.
func validerMotDePasse(motDePasse string) error {
	if len(motDePasse) < domain.MotDePasseLongueurMin || len(motDePasse) > domain.MotDePasseLongueurMax {
		return domain.ErrMotDePasseInvalide
	}
	return nil
}

// jetonNeuf : 32 octets aléatoires en base64url, soit 43 caractères. Le jeton n'est rendu qu'une fois.
func jetonNeuf() (string, error) {
	brut := make([]byte, 32)
	if _, err := rand.Read(brut); err != nil {
		return "", fmt.Errorf("jeton aléatoire : %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(brut), nil
}

// hacherJeton : la base ne stocke que cette empreinte, jamais le jeton lui-même.
func hacherJeton(jeton string) string {
	somme := sha256.Sum256([]byte(jeton))
	return hex.EncodeToString(somme[:])
}
