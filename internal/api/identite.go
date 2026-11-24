package api

import (
	"net/http"
	"strings"

	"github.com/alaa-ilehi/wasalni/internal/application"
)

// EnTeteIdentite est l'en-tête porté par le système d'authentification externe (ADR 0002).
const EnTeteIdentite = "X-User"

// identite lit l'acteur courant. L'identité n'est jamais créée ici : elle vient du système externe.
func identite(r *http.Request) (string, error) {
	acteur := strings.TrimSpace(r.Header.Get(EnTeteIdentite))
	if acteur == "" {
		return "", application.ErrIdentiteManquante
	}
	return acteur, nil
}
