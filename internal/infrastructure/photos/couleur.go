package photos

import "image/color"

// colorRGBA convertit des sommes de composantes 16 bits (prémultipliées, comme RGBA()) en une couleur 8 bits.
func colorRGBA(sommeR, sommeV, sommeB, sommeA, n uint64) color.RGBA {
	if n == 0 {
		return color.RGBA{A: 0xff}
	}
	return color.RGBA{
		R: uint8(sommeR / n >> 8),
		G: uint8(sommeV / n >> 8),
		B: uint8(sommeB / n >> 8),
		A: uint8(sommeA / n >> 8),
	}
}
