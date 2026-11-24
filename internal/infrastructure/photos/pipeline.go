// Package photos normalise la photo d'un profil : décodage, réduction et encodage WebP sans perte.
//
// Choix de conception (ADR 0008) : WebP sans perte (VP8L) plutôt que PNG ou JPEG, via
// github.com/SeriousBug/webp-go-pure — pure Go, sans cgo. La réduction se fait par moyenne de blocs
// (box filter), sans bibliothèque de rééchantillonnage. Trois paliers d'encodage au plus
// (512, 256, 128 px) ; au-delà l'image est refusée.
package photos

import (
	"bytes"
	"encoding/base64"
	"errors"
	"fmt"
	"image"
	"image/draw"
	"math"
	"strings"

	webp "github.com/SeriousBug/webp-go-pure/std"

	_ "image/gif"  // formats acceptés en entrée
	_ "image/jpeg" //
	_ "image/png"  //
)

const (
	// CoteMax est le côté maximal de l'image stockée.
	CoteMax = 512
	// CoteMin est le plus petit côté essayé avant de refuser l'image.
	CoteMin = 128
)

// Paliers d'encodage : le côté maximal essayé successivement, du plus grand au plus petit.
var paliers = [...]int{CoteMax, 256, CoteMin}

// PixelsMax borne les pixels décodés (24 Mpx). Le corps de la requête est borné en octets, pas en
// pixels : sans ce plafond, quelques dizaines d'octets peuvent déclarer 20 000 × 20 000 pixels et faire
// allouer un gigaoctet et demi avant la réduction.
const PixelsMax = 24_000_000

// PoidsMax est le poids maximal de l'image stockée (512 Kio). Variable pour que les tests puissent
// simuler un budget inatteignable sans créer d'image énorme.
var PoidsMax = 512 << 10

var (
	ErrImageIllisible  = errors.New("image illisible ou format non pris en charge")
	ErrImageTropGrande = errors.New("image trop grande")
	ErrImageTropLourde = fmt.Errorf("image impossible à ramener sous %d Kio", PoidsMax/1024)
)

// Normaliser transforme une image base64 (éventuellement préfixée « data:image/…;base64, »)
// en WebP sans perte de côté ≤ CoteMax et de poids ≤ PoidsMax.
func Normaliser(photoBase64 string) (string, error) {
	brut, err := decoder(photoBase64)
	if err != nil {
		return "", ErrImageIllisible
	}

	// Les dimensions sont lues avant le décodage complet : c'est le seul moment où l'on peut refuser
	// une bombe de décompression sans avoir déjà alloué ses pixels.
	config, _, err := image.DecodeConfig(bytes.NewReader(brut))
	if err != nil {
		return "", ErrImageIllisible
	}
	if config.Width <= 0 || config.Height <= 0 {
		return "", ErrImageIllisible
	}
	if config.Width*config.Height > PixelsMax {
		return "", fmt.Errorf("%w : %d x %d pixels, au-delà de %d", ErrImageTropGrande,
			config.Width, config.Height, PixelsMax)
	}

	source, _, err := image.Decode(bytes.NewReader(brut))
	if err != nil {
		return "", ErrImageIllisible
	}
	premiere := versRGBA(source)

	for _, cote := range paliers {
		reduite := redimensionner(premiere, cote)
		var tampon bytes.Buffer
		if err := webp.Encode(&tampon, reduite, &webp.Options{Lossless: true}); err != nil {
			return "", ErrImageIllisible
		}
		if tampon.Len() <= PoidsMax {
			return base64.StdEncoding.EncodeToString(tampon.Bytes()), nil
		}
	}
	return "", ErrImageTropLourde
}

// decoder accepte le base64 nu ou préfixé d'une URL de données, avec ou sans retour à la ligne.
func decoder(photoBase64 string) ([]byte, error) {
	valeur := strings.TrimSpace(photoBase64)
	if i := strings.Index(valeur, ";base64,"); i >= 0 {
		valeur = valeur[i+len(";base64,"):]
	}
	valeur = strings.Map(func(r rune) rune {
		switch r {
		case '\n', '\r', '\t', ' ':
			return -1
		}
		return r
	}, valeur)
	if valeur == "" {
		return nil, ErrImageIllisible
	}
	if brut, err := base64.StdEncoding.DecodeString(valeur); err == nil {
		return brut, nil
	}
	return base64.RawStdEncoding.DecodeString(valeur)
}

// versRGBA ramène la source en *image.RGBA d'origine (0,0), une seule fois : le filtre lit ensuite
// Pix directement, au lieu d'appeler image.Image.At() — une interface et une conversion par pixel,
// prémultipliées qui plus est. Le seul cas sans copie est une image déjà RGBA à l'origine.
func versRGBA(source image.Image) *image.RGBA {
	b := source.Bounds()
	if rgba, ok := source.(*image.RGBA); ok && b.Min == (image.Point{}) {
		return rgba
	}
	dest := image.NewRGBA(image.Rect(0, 0, b.Dx(), b.Dy()))
	draw.Draw(dest, dest.Bounds(), source, b.Min, draw.Src)
	return dest
}

// redimensionner réduit l'image pour que son plus grand côté vaille au plus coteMax, par moyenne des
// pixels source de chaque bloc. Une image déjà petite est renvoyée telle quelle.
func redimensionner(source *image.RGBA, coteMax int) *image.RGBA {
	b := source.Bounds()
	largeur, hauteur := b.Dx(), b.Dy()
	plusGrand := largeur
	if hauteur > plusGrand {
		plusGrand = hauteur
	}
	if plusGrand <= coteMax {
		return source
	}

	ratio := float64(coteMax) / float64(plusGrand)
	nvLargeur := int(math.Round(float64(largeur) * ratio))
	nvHauteur := int(math.Round(float64(hauteur) * ratio))
	if nvLargeur < 1 {
		nvLargeur = 1
	}
	if nvHauteur < 1 {
		nvHauteur = 1
	}

	dest := image.NewRGBA(image.Rect(0, 0, nvLargeur, nvHauteur))
	for y := 0; y < nvHauteur; y++ {
		debutY := y * hauteur / nvHauteur
		finY := (y + 1) * hauteur / nvHauteur
		if finY <= debutY {
			finY = debutY + 1
		}
		for x := 0; x < nvLargeur; x++ {
			debutX := x * largeur / nvLargeur
			finX := (x + 1) * largeur / nvLargeur
			if finX <= debutX {
				finX = debutX + 1
			}
			var sommeR, sommeV, sommeB, sommeA, n uint64
			for sy := debutY; sy < finY; sy++ {
				ligne := source.Pix[sy*source.Stride:]
				for sx := debutX; sx < finX; sx++ {
					i := sx * 4
					sommeR += uint64(ligne[i])
					sommeV += uint64(ligne[i+1])
					sommeB += uint64(ligne[i+2])
					sommeA += uint64(ligne[i+3])
					n++
				}
			}
			j := y*dest.Stride + x*4
			dest.Pix[j] = uint8(sommeR / n)
			dest.Pix[j+1] = uint8(sommeV / n)
			dest.Pix[j+2] = uint8(sommeB / n)
			dest.Pix[j+3] = uint8(sommeA / n)
		}
	}
	return dest
}
