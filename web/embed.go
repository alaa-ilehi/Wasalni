// Package web embarque l'interface HTML/CSS/JS servie sous /app/ par le binaire.
package web

import (
	"embed"
	"io/fs"
)

//go:embed static
var fichiers embed.FS

// Statique rend le dossier static/ comme racine du système de fichiers servi.
func Statique() fs.FS {
	racine, err := fs.Sub(fichiers, "static")
	if err != nil {
		panic(err)
	}
	return racine
}
