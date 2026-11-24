// Package photos normalise la photo d'un profil : décodage, réduction et encodage PNG sans perte.
//
// Choix de conception (ADR 0008) : PNG plutôt que JPEG, pour rester sans perte. La réduction se fait
// par moyenne de blocs (box filter), sans dépendance externe. Si le PNG dépasse encore le poids
// maximal, le côté est réduit par paliers jusqu'à CoteMin ; au-delà l'image est refusée.
package photos

import (
	"bytes"
	"encoding/base64"
	"errors"
	"fmt"
	"image"
	"image/png"
	"math"
	"strings"

	_ "image/gif"  // formats acceptés en entrée
	_ "image/jpeg" //
)

const (
	// TailleMaxCorps est la limite du corps HTTP accepté par l'adaptateur (10 Mio).
	TailleMaxCorps = 10 << 20
	// CoteMax est le côté maximal du PNG stocké.
	CoteMax = 512
	// CoteMin est le plus petit côté essayé avant de refuser l'image.
	CoteMin = 128
	// PasReduction est le pas de réduction successif.
	PasReduction = 64
	// PoidsMax est le poids maximal du PNG stocké (512 Kio).
	PoidsMax = 512 << 10
)

var (
	ErrImageIllisible  = errors.New("image illisible ou format non pris en charge")
	ErrImageTropLourde = fmt.Errorf("image impossible à ramener sous %d Kio", PoidsMax/1024)
)

// Normaliser transforme une image base64 (éventuellement préfixée « data:image/…;base64, »)
// en PNG de côté ≤ CoteMax et de poids ≤ PoidsMax.
func Normaliser(photoBase64 string) (string, error) {
	brut, err := decoder(photoBase64)
	if err != nil {
		return "", ErrImageIllisible
	}
	source, _, err := image.Decode(bytes.NewReader(brut))
	if err != nil {
		return "", ErrImageIllisible
	}
	if source.Bounds().Empty() {
		return "", ErrImageIllisible
	}

	for cote := CoteMax; ; cote -= PasReduction {
		if cote < CoteMin {
			cote = CoteMin
		}
		reduite := redimensionner(source, cote)
		var tampon bytes.Buffer
		encodeur := png.Encoder{CompressionLevel: png.BestCompression}
		if err := encodeur.Encode(&tampon, reduite); err != nil {
			return "", ErrImageIllisible
		}
		if tampon.Len() <= PoidsMax {
			return base64.StdEncoding.EncodeToString(tampon.Bytes()), nil
		}
		if cote == CoteMin {
			return "", ErrImageTropLourde
		}
	}
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

// redimensionner réduit l'image pour que son plus grand côté vaille au plus coteMax,
// par moyenne des pixels source de chaque bloc. Une image déjà petite est renvoyée telle quelle.
func redimensionner(source image.Image, coteMax int) image.Image {
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
		debutY := b.Min.Y + y*hauteur/nvHauteur
		finY := b.Min.Y + (y+1)*hauteur/nvHauteur
		if finY <= debutY {
			finY = debutY + 1
		}
		for x := 0; x < nvLargeur; x++ {
			debutX := b.Min.X + x*largeur/nvLargeur
			finX := b.Min.X + (x+1)*largeur/nvLargeur
			if finX <= debutX {
				finX = debutX + 1
			}
			var sommeR, sommeV, sommeB, sommeA, n uint64
			for sy := debutY; sy < finY; sy++ {
				for sx := debutX; sx < finX; sx++ {
					r, v, bl, a := source.At(sx, sy).RGBA()
					sommeR += uint64(r)
					sommeV += uint64(v)
					sommeB += uint64(bl)
					sommeA += uint64(a)
					n++
				}
			}
			dest.SetRGBA(x, y, colorRGBA(sommeR, sommeV, sommeB, sommeA, n))
		}
	}
	return dest
}
