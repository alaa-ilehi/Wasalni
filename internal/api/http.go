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
	case errors.Is(err, application.ErrIdentiteManquante),
		errors.Is(err, domain.ErrIdentifiantsInvalides),
		errors.Is(err, domain.ErrSessionInvalide):
		return http.StatusUnauthorized
	case errors.Is(err, domain.ErrProfilIntrouvable),
		errors.Is(err, domain.ErrTrajetIntrouvable),
		errors.Is(err, domain.ErrReservationIntrouvable):
		return http.StatusNotFound
	case errors.Is(err, domain.ErrNonParticipant),
		errors.Is(err, domain.ErrTrajetNonProprietaire):
		return http.StatusForbidden
	case errors.Is(err, domain.ErrTrajetComplet),
		errors.Is(err, domain.ErrPlacesInsuffisantes),
		errors.Is(err, domain.ErrTrajetSupprime),
		errors.Is(err, domain.ErrTrajetDejaTermine),
		errors.Is(err, domain.ErrReservationDejaActive),
		errors.Is(err, domain.ErrReservationDejaAnnulee),
		errors.Is(err, domain.ErrAnnulationHorsDelai),
		errors.Is(err, domain.ErrTrajetNonTermine),
		errors.Is(err, domain.ErrEvaluationDejaFaite),
		errors.Is(err, domain.ErrIdentifiantDejaInscrit):
		return http.StatusConflict
	case errors.Is(err, domain.ErrNomRequis),
		errors.Is(err, domain.ErrNomInvalide),
		errors.Is(err, domain.ErrAgeInvalide),
		errors.Is(err, domain.ErrSexeInvalide),
		errors.Is(err, domain.ErrMotDePasseInvalide),
		errors.Is(err, domain.ErrTrajetInvalide),
		errors.Is(err, domain.ErrConducteurNePeutPasReserver),
		errors.Is(err, application.ErrPhotoInvalide),
		errors.Is(err, application.ErrImageTropGrande),
		errors.Is(err, application.ErrRechercheInvalide),
		errors.Is(err, domain.ErrCurseurInvalide),
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
