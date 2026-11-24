package photos_test

import (
	"bytes"
	"encoding/base64"
	"encoding/binary"
	"errors"
	"hash/crc32"
	"image"
	"image/color"
	"image/png"
	"math/rand"
	"testing"

	"github.com/alaa-ilehi/wasalni/internal/infrastructure/photos"
)

// bruitRN fabrique une image de bruit pseudo-aléatoire : mal compressible, donc lourde.
func bruitRN(t *testing.T, largeur, hauteur int) *image.RGBA {
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
	return source
}

// bruitPNG fabrique le même bruit, encodé en PNG : le format d'entrée des tests.
func bruitPNG(t *testing.T, largeur, hauteur int) []byte {
	t.Helper()
	var tampon bytes.Buffer
	if err := png.Encode(&tampon, bruitRN(t, largeur, hauteur)); err != nil {
		t.Fatalf("encodage de l'image de test : %v", err)
	}
	return tampon.Bytes()
}

// decoderSortie vérifie le format et rend les octets et l'image de la sortie normalisée.
func decoderSortie(t *testing.T, base64Sortie string) ([]byte, image.Image) {
	t.Helper()
	brut, err := base64.StdEncoding.DecodeString(base64Sortie)
	if err != nil {
		t.Fatalf("la sortie doit être du base64 valide : %v", err)
	}
	if len(brut) < 12 || string(brut[:4]) != "RIFF" || string(brut[8:12]) != "WEBP" {
		t.Fatalf("la sortie doit être un conteneur WebP, obtenu %q", brut[:min(12, len(brut))])
	}
	img, format, err := image.Decode(bytes.NewReader(brut))
	if err != nil {
		t.Fatalf("la sortie doit être une image décodable : %v", err)
	}
	if format != "webp" {
		t.Fatalf("format de sortie = %q, attendu webp", format)
	}
	return brut, img
}

func TestNormaliserReduitUneGrandePhoto(t *testing.T) {
	entree := base64.StdEncoding.EncodeToString(bruitPNG(t, 1400, 1000))

	sortie, err := photos.Normaliser(entree)
	if err != nil {
		t.Fatalf("normalisation : %v", err)
	}
	brut, img := decoderSortie(t, sortie)
	b := img.Bounds()
	if b.Dx() > photos.CoteMax || b.Dy() > photos.CoteMax {
		t.Errorf("dimensions = %dx%d, attendu au plus %d", b.Dx(), b.Dy(), photos.CoteMax)
	}
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
	b := mustImage(t, sortie).Bounds()
	if b.Dx() != 10 || b.Dy() != 10 {
		t.Errorf("dimensions = %dx%d, attendu 10x10 (aucun agrandissement)", b.Dx(), b.Dy())
	}
}

// Une photo déjà sous CoteMax n'est pas rééchantillonnée : la sortie doit être identique au pixel près,
// c'est la raison du choix du sans perte (ADR 0008).
func TestNormaliserConserveLesPixelsSansPerte(t *testing.T) {
	source := bruitRN(t, 200, 150)
	sortie, err := photos.Normaliser(base64.StdEncoding.EncodeToString(bruitPNG(t, 200, 150)))
	if err != nil {
		t.Fatalf("normalisation : %v", err)
	}

	relue := mustImage(t, sortie)
	if relue.Bounds() != source.Bounds() {
		t.Fatalf("dimensions = %v, attendu %v", relue.Bounds(), source.Bounds())
	}
	for y := 0; y < 150; y++ {
		for x := 0; x < 200; x++ {
			// comparaison sur les composantes : les types concrets de color.Color diffèrent d'une image à l'autre
			r1, v1, b1, a1 := relue.At(x, y).RGBA()
			r2, v2, b2, a2 := source.At(x, y).RGBA()
			if r1 != r2 || v1 != v2 || b1 != b2 || a1 != a2 {
				t.Fatalf("pixel (%d,%d) = %v, attendu %v : la sortie n'est pas sans perte",
					x, y, relue.At(x, y), source.At(x, y))
			}
		}
	}
}

// Le WebP sans perte doit être plus léger que le PNG à qualité identique : c'est le but du changement.
func TestNormaliserWebPPeseMoinsQueLePNG(t *testing.T) {
	entree := bruitPNG(t, 512, 512)

	sortie, err := photos.Normaliser(base64.StdEncoding.EncodeToString(entree))
	if err != nil {
		t.Fatalf("normalisation : %v", err)
	}
	brut, _ := decoderSortie(t, sortie)
	t.Logf("même image de 512x512 : PNG=%d octets, WebP=%d octets", len(entree), len(brut))
	if len(brut) >= len(entree) {
		t.Errorf("WebP = %d octets, PNG = %d : le WebP devrait être plus léger", len(brut), len(entree))
	}
	if ratio := float64(len(brut)) / float64(len(entree)); ratio >= 1 {
		t.Errorf("ratio WebP/PNG = %.2f, attendu < 1", ratio)
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

// bombePNG fabrique une bombe de décompression : 33 octets qui déclarent 20 000 × 20 000 pixels, sans
// en fournir un seul. Décodée, elle demanderait 1,6 Go (20000*20000*4).
func bombePNG(t *testing.T) []byte {
	t.Helper()
	var ihdr bytes.Buffer
	for _, valeur := range []uint32{20000, 20000} {
		if err := binary.Write(&ihdr, binary.BigEndian, valeur); err != nil {
			t.Fatalf("en-tête : %v", err)
		}
	}
	ihdr.Write([]byte{8, 2, 0, 0, 0}) // 8 bits, RVB, sans filtrage ni entrelacement

	var png bytes.Buffer
	png.WriteString("\x89PNG\r\n\x1a\n")
	chunk := func(nom string, donnees []byte) {
		if err := binary.Write(&png, binary.BigEndian, uint32(len(donnees))); err != nil {
			t.Fatalf("longueur du bloc %s : %v", nom, err)
		}
		png.WriteString(nom)
		png.Write(donnees)
		somme := crc32.NewIEEE()
		somme.Write([]byte(nom))
		somme.Write(donnees)
		if err := binary.Write(&png, binary.BigEndian, somme.Sum32()); err != nil {
			t.Fatalf("somme de contrôle du bloc %s : %v", nom, err)
		}
	}
	chunk("IHDR", ihdr.Bytes())
	chunk("IDAT", nil)
	chunk("IEND", nil)
	return png.Bytes()
}

// Le budget de pixels est vérifié sur les dimensions déclarées, avant tout décodage : une bombe de
// 33 octets est refusée sans qu'un pixel ne soit alloué.
func TestNormaliserRefuseUneBombeDeDecompression(t *testing.T) {
	bombe := bombePNG(t)
	t.Logf("bombe de %d octets déclarant 20000x20000 (1,6 Go une fois décodée)", len(bombe))

	if _, err := photos.Normaliser(base64.StdEncoding.EncodeToString(bombe)); !errors.Is(err, photos.ErrImageTropGrande) {
		t.Fatalf("erreur = %v, attendu ErrImageTropGrande", err)
	}
}

func TestNormaliserRefuseUneImageTropLourde(t *testing.T) {
	budget := photos.PoidsMax
	photos.PoidsMax = 1 // aucun palier ne peut tenir sous un octet
	t.Cleanup(func() { photos.PoidsMax = budget })

	entree := base64.StdEncoding.EncodeToString(bruitPNG(t, 400, 400))
	if _, err := photos.Normaliser(entree); !errors.Is(err, photos.ErrImageTropLourde) {
		t.Fatalf("erreur = %v, attendu ErrImageTropLourde", err)
	}
}

func mustImage(t *testing.T, base64Sortie string) image.Image {
	t.Helper()
	_, img := decoderSortie(t, base64Sortie)
	return img
}
