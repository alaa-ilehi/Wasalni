package passwords_test

import (
	"crypto/sha1"
	"crypto/sha256"
	"encoding/hex"
	"testing"

	"github.com/alaa-ilehi/wasalni/internal/infrastructure/passwords"
)

// Vecteurs publiés dans la RFC 6070 (PBKDF2-HMAC-SHA1) : ils valident la construction PBKDF2
// elle-même, indépendamment du PRF retenu en production.
func TestDerivationSuitLesVecteursRFC6070(t *testing.T) {
	cas := []struct {
		nom        string
		motDePasse string
		sel        string
		iterations int
		longueur   int
		attendu    string
	}{
		{"c=1", "password", "salt", 1, 20, "0c60c80f961f0e71f3a9b524af6012062fe037a6"},
		{"c=2", "password", "salt", 2, 20, "ea6c014dc72d6f8ccd1ed92ace1d41f0d8de8957"},
		{"c=4096", "password", "salt", 4096, 20, "4b007901b765489abead49d926f721d065a429c1"},
		{"clé longue", "passwordPASSWORDpassword", "saltSALTsaltSALTsaltSALTsaltSALTsalt",
			4096, 25, "3d2eec4fe41c849b80c8d83662c0e44a8b291a964cf2f07038"},
	}
	for _, c := range cas {
		t.Run(c.nom, func(t *testing.T) {
			obtenu := passwords.Deriver(sha1.New, []byte(c.motDePasse), []byte(c.sel), c.iterations, c.longueur)
			if hex.EncodeToString(obtenu) != c.attendu {
				t.Fatalf("PBKDF2-SHA1 = %x, attendu %s", obtenu, c.attendu)
			}
		})
	}
}

// Mêmes cas avec le PRF de production : PBKDF2-HMAC-SHA256, valeurs de référence publiées.
func TestDerivationSuitLesVecteursSHA256(t *testing.T) {
	cas := []struct {
		nom        string
		motDePasse string
		sel        string
		iterations int
		longueur   int
		attendu    string
	}{
		{"c=1", "password", "salt", 1, 32, "120fb6cffcf8b32c43e7225256c4f837a86548c92ccc35480805987cb70be17b"},
		{"c=2", "password", "salt", 2, 32, "ae4d0c95af6b46d32d0adff928f06dd02a303f8ef3c251dfd6e2d85a95474c43"},
		{"c=4096", "password", "salt", 4096, 32, "c5e478d59288c841aa530db6845c4c8d962893a001ce4e11a4963873aa98134a"},
		{"clé longue", "passwordPASSWORDpassword", "saltSALTsaltSALTsaltSALTsaltSALTsalt",
			4096, 40, "348c89dbcbd32b2f32d814b8116e84cf2b17347ebc1800181c4e2a1fb8dd53e1c635518c7dac47e9"},
	}
	for _, c := range cas {
		t.Run(c.nom, func(t *testing.T) {
			obtenu := passwords.Deriver(sha256.New, []byte(c.motDePasse), []byte(c.sel), c.iterations, c.longueur)
			if hex.EncodeToString(obtenu) != c.attendu {
				t.Fatalf("PBKDF2-SHA256 = %x, attendu %s", obtenu, c.attendu)
			}
		})
	}
}

func TestHacherRendUnSelNeufEtLeVerifie(t *testing.T) {
	premier, err := passwords.Hacher("motdepasse")
	if err != nil {
		t.Fatalf("hachage : %v", err)
	}
	second, err := passwords.Hacher("motdepasse")
	if err != nil {
		t.Fatalf("hachage : %v", err)
	}

	if len(premier.Sel) != passwords.SelLongueur {
		t.Fatalf("sel de %d octets, attendu %d", len(premier.Sel), passwords.SelLongueur)
	}
	if premier.Iterations != passwords.IterationsParDefaut || premier.Iterations < 100_000 {
		t.Fatalf("itérations = %d, attendu %d", premier.Iterations, passwords.IterationsParDefaut)
	}
	if string(premier.Sel) == string(second.Sel) {
		t.Fatal("deux hachages du même mot de passe ont reçu le même sel")
	}
	if string(premier.Condensat) == string(second.Condensat) {
		t.Fatal("deux hachages du même mot de passe ont produit le même condensé : sel ignoré")
	}
	if !passwords.Verifier("motdepasse", premier) || !passwords.Verifier("motdepasse", second) {
		t.Fatal("le bon mot de passe est refusé")
	}
	if passwords.Verifier("motdepassE", premier) {
		t.Fatal("un mot de passe faux est accepté")
	}
	if passwords.Verifier("", premier) {
		t.Fatal("une chaîne vide est acceptée")
	}
}
