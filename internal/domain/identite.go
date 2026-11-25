package domain

import (
	"context"
	"errors"
	"time"
)

// Longueurs admises pour un mot de passe : assez long pour être utile, assez court pour rester sous
// la limite de PBKDF2 en octets.
const (
	MotDePasseLongueurMin = 8
	MotDePasseLongueurMax = 72
)

var (
	ErrIdentifiantDejaInscrit = errors.New("cet identifiant a déjà un mot de passe")
	ErrIdentifiantsInvalides  = errors.New("identifiant ou mot de passe incorrect")
	ErrMotDePasseInvalide     = errors.New("le mot de passe doit contenir entre 8 et 72 caractères")
	ErrSessionInvalide        = errors.New("session absente, inconnue ou expirée")
)

// Credential : le mot de passe haché d'un identifiant. Le sel est propre à chaque compte et rend
// deux mots de passe identiques impossibles à comparer entre eux (ADR 0011).
type Credential struct {
	Identifiant string
	Sel         []byte
	Condensat   []byte
	Iterations  int
}

// Session : un jeton de session, conservé sous forme hachée. Le jeton en clair n'existe que chez
// le client et dans la réponse qui l'a créé.
type Session struct {
	Jeton       string // hachage SHA-256 du jeton en hexadécimal
	Identifiant string
	CreeLe      time.Time
	ExpireLe    time.Time
}

// CredentialRepository : persistance des mots de passe hachés.
// Contrat : Enregistrer échoue avec ErrIdentifiantDejaInscrit si l'identifiant a déjà un mot de passe ;
// ParIdentifiant échoue avec ErrIdentifiantsInvalides pour un identifiant inconnu.
type CredentialRepository interface {
	Enregistrer(ctx context.Context, credential Credential) error
	ParIdentifiant(ctx context.Context, identifiant string) (Credential, error)
}

// SessionRepository : persistance des sessions. Les sessions expirées ne sont jamais rendues : elles
// sont supprimées à la lecture, ce qui évite un balayage périodique (ADR 0011).
type SessionRepository interface {
	Creer(ctx context.Context, session Session) error
	ParJeton(ctx context.Context, jeton string, maintenant time.Time) (Session, error)
	Supprimer(ctx context.Context, jeton string) error
}
