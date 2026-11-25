// Package passwords hache et vérifie les mots de passe (PBKDF2-HMAC-SHA256, RFC 8018).
// Aucune dépendance : la construction est écrite sur crypto/hmac et crypto/sha256, et vérifiée
// contre les vecteurs publiés de la RFC 6070 (construction) et les vecteurs SHA-256 (PRF retenu).
package passwords

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/binary"
	"fmt"
	"hash"

	"github.com/alaa-ilehi/wasalni/internal/domain"
)

const (
	// SelLongueur : 16 octets aléatoires par compte. Deux comptes de même mot de passe obtiennent
	// donc deux condensés différents, ce qui interdit toute comparaison entre comptes.
	SelLongueur = 16
	// CondensatLongueur : taille de clé dérivée (SHA-256 ⇒ un seul bloc PBKDF2).
	CondensatLongueur = 32
	// IterationsParDefaut : au-dessus du plancher de 100 000 exigé, en deçà du coût qui rendrait
	// chaque connexion sensible (~70 ms ici).
	IterationsParDefaut = 120_000
)

// Hacher dérive un condensé du mot de passe avec un sel neuf. Les paramètres (sel, condensé,
// itérations) sont rendus ensemble : ils sont tous nécessaires à la vérification, et les stocker
// permet d'augmenter le coût plus tard sans invalider les comptes existants (ADR 0011).
func Hacher(motDePasse string) (domain.Credential, error) {
	sel := make([]byte, SelLongueur)
	if _, err := rand.Read(sel); err != nil {
		return domain.Credential{}, fmt.Errorf("sel aléatoire : %w", err)
	}
	return domain.Credential{
		Sel:        sel,
		Condensat:  Deriver(sha256.New, []byte(motDePasse), sel, IterationsParDefaut, CondensatLongueur),
		Iterations: IterationsParDefaut,
	}, nil
}

// Verifier recalcule le condensé et le compare en temps constant : la durée ne dépend pas du nombre
// d'octets corrects, donc l'erreur ne fuit pas d'information exploitable.
func Verifier(motDePasse string, credential domain.Credential) bool {
	attendu := Deriver(sha256.New, []byte(motDePasse), credential.Sel, credential.Iterations, len(credential.Condensat))
	return subtle.ConstantTimeCompare(attendu, credential.Condensat) == 1
}

// Deriver implémente PBKDF2 (RFC 8018 §5.2) sur le PRF fourni. Le PRF est un paramètre exporté pour
// que le test rejoue les vecteurs publiés de la RFC 6070, qui sont calculés avec HMAC-SHA1.
func Deriver(prf func() hash.Hash, motDePasse, sel []byte, iterations, longueur int) []byte {
	taille := prf().Size()
	blocs := (longueur + taille - 1) / taille
	cle := make([]byte, 0, blocs*taille)
	u := make([]byte, taille)
	bloc := make([]byte, taille)

	for numero := 1; numero <= blocs; numero++ {
		mac := hmac.New(prf, motDePasse)
		mac.Write(sel)
		var compteur [4]byte
		binary.BigEndian.PutUint32(compteur[:], uint32(numero))
		mac.Write(compteur[:])
		u = mac.Sum(u[:0])
		copy(bloc, u)

		for i := 1; i < iterations; i++ {
			mac = hmac.New(prf, motDePasse)
			mac.Write(u)
			u = mac.Sum(u[:0])
			for j := range bloc {
				bloc[j] ^= u[j]
			}
		}
		cle = append(cle, bloc...)
	}
	return cle[:longueur]
}

// Hacheur : adaptateur du port application.HacheurMotDePasse (UC13). Un type, deux méthodes : la
// racine de composition le câble comme normaliseurPhoto l'est pour UC8.
type Hacheur struct{}

func (Hacheur) Hacher(motDePasse string) (domain.Credential, error) { return Hacher(motDePasse) }

func (Hacheur) Verifier(motDePasse string, credential domain.Credential) bool {
	return Verifier(motDePasse, credential)
}
