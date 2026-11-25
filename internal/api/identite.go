package api

import (
	"net/http"
	"strings"

	"github.com/alaa-ilehi/wasalni/internal/application"
	"github.com/alaa-ilehi/wasalni/internal/domain"
)

// EnTeteAutorisation : en-tête standard du jeton de session, `Authorization: Bearer <jeton>` (ADR 0011).
const EnTeteAutorisation = "Authorization"

const prefixeBearer = "Bearer "

// identite rend l'acteur courant à partir du jeton de session. L'identité n'est jamais créée ici :
// elle vient d'une session ouverte par UC13, et un jeton absent ou échu rend ErrSessionInvalide (401).
func identite(r *http.Request, auth application.Authentification) (string, error) {
	jeton := jetonBearer(r)
	if jeton == "" {
		return "", domain.ErrSessionInvalide
	}
	return auth.Resoudre(r.Context(), jeton)
}

// jetonBearer extrait le jeton de l'en-tête, en tolérant une casse différente sur le préfixe.
func jetonBearer(r *http.Request) string {
	valeur := strings.TrimSpace(r.Header.Get(EnTeteAutorisation))
	if len(valeur) <= len(prefixeBearer) || !strings.EqualFold(valeur[:len(prefixeBearer)], prefixeBearer) {
		return ""
	}
	return strings.TrimSpace(valeur[len(prefixeBearer):])
}
