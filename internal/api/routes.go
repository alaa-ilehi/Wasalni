package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/alaa-ilehi/wasalni/internal/application"
	"github.com/alaa-ilehi/wasalni/internal/domain"
	"github.com/alaa-ilehi/wasalni/internal/infrastructure/photos"
)

// profilJSON : représentation publique du profil, en anglais comme le reste du contrat HTTP.
type profilJSON struct {
	Identifiant string `json:"id"`
	Nom         string `json:"name"`
	Age         int    `json:"age"`
	Sexe        string `json:"sex,omitempty"`
	Photo       string `json:"photo,omitempty"`
	CreeLe      string `json:"created_at"`
	MajLe       string `json:"updated_at"`
}

func versJSON(p domain.Profil) profilJSON {
	return profilJSON{
		Identifiant: p.Identifiant,
		Nom:         p.Nom,
		Age:         p.Age,
		Sexe:        string(p.Sexe),
		Photo:       p.Photo,
		CreeLe:      p.CreeLe.UTC().Format(time.RFC3339),
		MajLe:       p.MajLe.UTC().Format(time.RFC3339),
	}
}

// Routes construit le routeur HTTP du service (Go 1.22 : motifs avec méthode).
func Routes(gerer application.GererProfil) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, _ *http.Request) {
		repondreJSON(w, http.StatusOK, map[string]string{"statut": "ok"})
	})
	mux.HandleFunc("GET /profile", consulterProfil(gerer))
	mux.HandleFunc("PUT /profile", enregistrerProfil(gerer))
	return mux
}

// consulterProfil : GET /profile, le profil de l'identité courante.
func consulterProfil(gerer application.GererProfil) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		acteur, err := identite(r)
		if err != nil {
			repondreErreur(w, err)
			return
		}
		profil, err := gerer.Consulter(r.Context(), acteur)
		if err != nil {
			repondreErreur(w, err)
			return
		}
		repondreJSON(w, http.StatusOK, versJSON(profil))
	}
}

// enregistrerProfil : PUT /profile, crée (201) ou met à jour (200) le profil de l'identité courante.
func enregistrerProfil(gerer application.GererProfil) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		acteur, err := identite(r)
		if err != nil {
			repondreErreur(w, err)
			return
		}

		r.Body = http.MaxBytesReader(w, r.Body, photos.TailleMaxCorps)
		var saisie application.SaisieProfil
		decodeur := json.NewDecoder(r.Body)
		decodeur.DisallowUnknownFields()
		if err := decodeur.Decode(&saisie); err != nil {
			var tropGrand *http.MaxBytesError
			if errors.As(err, &tropGrand) {
				repondreJSON(w, http.StatusRequestEntityTooLarge,
					erreurJSON{Erreur: "corps de requête trop volumineux"})
				return
			}
			repondreJSON(w, http.StatusBadRequest, erreurJSON{Erreur: "corps JSON invalide"})
			return
		}

		profil, cree, err := gerer.Enregistrer(r.Context(), acteur, saisie)
		if err != nil {
			repondreErreur(w, err)
			return
		}
		statut := http.StatusOK
		if cree {
			statut = http.StatusCreated
		}
		repondreJSON(w, statut, versJSON(profil))
	}
}
