// Package api : adaptateur HTTP (routage, identité, correspondance erreur → code HTTP).
package api

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"

	"github.com/alaa-ilehi/wasalni/internal/application"
	"github.com/alaa-ilehi/wasalni/internal/domain"
)

// statutHTTP est la seule table de correspondance erreur → code HTTP (ADR 0006).
// Le domaine et l'application n'en connaissent aucun code.
func statutHTTP(err error) int {
	switch {
	case errors.Is(err, application.ErrIdentiteManquante):
		return http.StatusUnauthorized
	case errors.Is(err, domain.ErrProfilIntrouvable),
		errors.Is(err, domain.ErrTrajetIntrouvable):
		return http.StatusNotFound
	case errors.Is(err, domain.ErrNonParticipant):
		return http.StatusForbidden
	case errors.Is(err, domain.ErrTrajetComplet),
		errors.Is(err, domain.ErrReservationDejaActive),
		errors.Is(err, domain.ErrTrajetNonTermine),
		errors.Is(err, domain.ErrEvaluationDejaFaite):
		return http.StatusConflict
	case errors.Is(err, domain.ErrNomRequis),
		errors.Is(err, domain.ErrNomInvalide),
		errors.Is(err, domain.ErrAgeInvalide),
		errors.Is(err, domain.ErrSexeInvalide),
		errors.Is(err, domain.ErrTrajetInvalide),
		errors.Is(err, domain.ErrConducteurNePeutPasReserver),
		errors.Is(err, application.ErrPhotoInvalide),
		errors.Is(err, application.ErrRechercheInvalide),
		errors.Is(err, domain.ErrEvaluationInvalide):
		return http.StatusBadRequest
	default:
		return http.StatusInternalServerError
	}
}

// erreurJSON est le corps d'erreur unique de l'API.
type erreurJSON struct {
	Erreur string `json:"error"`
}

func repondreJSON(w http.ResponseWriter, statut int, corps any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(statut)
	if corps == nil {
		return
	}
	if err := json.NewEncoder(w).Encode(corps); err != nil {
		slog.Error("écriture de la réponse impossible", "erreur", err)
	}
}

// repondreErreur traduit l'erreur en code HTTP et journalise les erreurs inattendues.
func repondreErreur(w http.ResponseWriter, err error) {
	statut := statutHTTP(err)
	if statut == http.StatusInternalServerError {
		slog.Error("erreur inattendue", "erreur", err)
		repondreJSON(w, statut, erreurJSON{Erreur: "erreur interne"})
		return
	}
	repondreJSON(w, statut, erreurJSON{Erreur: err.Error()})
}
