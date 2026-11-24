package api

import (
	"errors"
	"net/http"
	"strings"
)

// ErrIdentiteManquante : l'en-tête X-User est absent ou vide (adaptateur d'identité, ADR 0002).
var ErrIdentiteManquante = errors.New("identité manquante : en-tête X-User requis")

// EnTeteIdentite est l'en-tête porté par le système d'authentification externe.
const EnTeteIdentite = "X-User"

// identite lit l'acteur courant. L'identité n'est jamais créée ici : elle vient du système externe.
func identite(r *http.Request) (string, error) {
	acteur := strings.TrimSpace(r.Header.Get(EnTeteIdentite))
	if acteur == "" {
		return "", ErrIdentiteManquante
	}
	return acteur, nil
}
