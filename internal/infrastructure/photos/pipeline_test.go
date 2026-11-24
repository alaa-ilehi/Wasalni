package photos_test

import (
	"bytes"
	"encoding/base64"
	"errors"
	"image"
	"image/color"
	"image/png"
	"math/rand"
	"testing"

	"github.com/alaa-ilehi/wasalni/internal/infrastructure/photos"
)

// bruitPNG fabrique une image de bruit pseudo-aléatoire : mal compressible, donc lourde en PNG.
func bruitPNG(t *testing.T, largeur, hauteur int) []byte {
	t.Helper()
	source := image.NewRGBA(image.Rect(0, 0, largeur, hauteur))
	alea := rand.New(rand.NewSource(42))
	for y := 0; y < hauteur; y++ {
		for x := 0; x < largeur; x++ {
			source.SetRGBA(x, y, color.RGBA{
				R: uint8(alea.Intn(256)),
				G: uint8(alea.Intn(256)),
				B: uint8(alea.Intn(256)),
				A: 0xff,
			})
		}
	}
	var tampon bytes.Buffer
	if err := png.Encode(&tampon, source); err != nil {
		t.Fatalf("encodage de l'image de test : %v", err)
	}
	return tampon.Bytes()
}

func decoderPNG(t *testing.T, base64PNG string) image.Image {
	t.Helper()
	brut, err := base64.StdEncoding.DecodeString(base64PNG)
	if err != nil {
		t.Fatalf("la sortie doit être du base64 valide : %v", err)
	}
	img, format, err := image.Decode(bytes.NewReader(brut))
	if err != nil {
		t.Fatalf("la sortie doit être une image décodable : %v", err)
	}
	if format != "png" {
		t.Fatalf("format de sortie = %q, attendu png (sans perte)", format)
	}
	return img
}

func TestNormaliserReduitUneGrandePhoto(t *testing.T) {
	entree := base64.StdEncoding.EncodeToString(bruitPNG(t, 1400, 1000))

	sortie, err := photos.Normaliser(entree)
	if err != nil {
		t.Fatalf("normalisation : %v", err)
	}
	img := decoderPNG(t, sortie)
	b := img.Bounds()
	if b.Dx() > photos.CoteMax || b.Dy() > photos.CoteMax {
		t.Errorf("dimensions = %dx%d, attendu au plus %d", b.Dx(), b.Dy(), photos.CoteMax)
	}
	brut, _ := base64.StdEncoding.DecodeString(sortie)
	if len(brut) > photos.PoidsMax {
		t.Errorf("poids = %d octets, attendu au plus %d", len(brut), photos.PoidsMax)
	}
}

func TestNormaliserConserveUnePetitePhoto(t *testing.T) {
	entree := base64.StdEncoding.EncodeToString(bruitPNG(t, 10, 10))

	sortie, err := photos.Normaliser(entree)
	if err != nil {
		t.Fatalf("normalisation : %v", err)
	}
	b := decoderPNG(t, sortie).Bounds()
	if b.Dx() != 10 || b.Dy() != 10 {
		t.Errorf("dimensions = %dx%d, attendu 10x10 (aucun agrandissement)", b.Dx(), b.Dy())
	}
}

func TestNormaliserAccepteLePrefixeDataURL(t *testing.T) {
	entree := "data:image/png;base64," + base64.StdEncoding.EncodeToString(bruitPNG(t, 32, 32))

	if _, err := photos.Normaliser(entree); err != nil {
		t.Fatalf("préfixe data: refusé : %v", err)
	}
}

func TestNormaliserRefuseUneImageIllisible(t *testing.T) {
	cas := map[string]string{
		"base64 invalide": "%%%pas-du-base64%%%",
		"pas une image":   base64.StdEncoding.EncodeToString([]byte("bonjour")),
		"vide":            "",
	}
	for libelle, entree := range cas {
		if _, err := photos.Normaliser(entree); !errors.Is(err, photos.ErrImageIllisible) {
			t.Errorf("%s : erreur = %v, attendu ErrImageIllisible", libelle, err)
		}
	}
}
