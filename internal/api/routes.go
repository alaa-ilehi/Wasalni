package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/alaa-ilehi/wasalni/internal/application"
	"github.com/alaa-ilehi/wasalni/internal/domain"
)

// TailleMaxCorps borne le corps HTTP accepté par PUT /profile (10 Mio, ADR 0008).
const TailleMaxCorps = 10 << 20

// Deps regroupe les cas d'utilisation exposés par l'API. La racine de composition les fournit.
type Deps struct {
	Profils      application.GererProfil
	Recherches   application.RechercherTrajets
	Trajets      application.PublierTrajet
	Details      application.ConsulterTrajet
	Reservations application.ReserverPlace
	Terminaisons application.TerminerTrajet
	Evaluations  application.EvaluerParticipant
}

// Routes construit le routeur HTTP du service (Go 1.22 : motifs avec méthode).
func Routes(deps Deps) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, _ *http.Request) {
		repondreJSON(w, http.StatusOK, map[string]string{"statut": "ok"})
	})
	mux.HandleFunc("GET /profile", consulterProfil(deps.Profils))
	mux.HandleFunc("PUT /profile", enregistrerProfil(deps.Profils))
	mux.HandleFunc("GET /rides", rechercherTrajets(deps.Recherches))
	mux.HandleFunc("GET /rides/{id}", consulterTrajet(deps.Details))
	mux.HandleFunc("POST /rides", publierTrajet(deps.Trajets))
	mux.HandleFunc("POST /rides/{id}/bookings", reserverPlace(deps.Reservations))
	mux.HandleFunc("POST /rides/{id}/complete", terminerTrajet(deps.Terminaisons))
	mux.HandleFunc("POST /rides/{id}/ratings", enregistrerEvaluation(deps.Evaluations))
	return mux
}

// terminerTrajet : POST /rides/{id}/complete, clôt le trajet du conducteur identifié.
func terminerTrajet(terminer application.TerminerTrajet) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		conducteur, err := identite(r)
		if err != nil {
			repondreErreur(w, err)
			return
		}
		trajet, err := terminer.Executer(r.Context(), conducteur, r.PathValue("id"))
		if err != nil {
			repondreErreur(w, err)
			return
		}
		repondreJSON(w, http.StatusOK, trajetVersJSON(trajet))
	}
}

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

		r.Body = http.MaxBytesReader(w, r.Body, TailleMaxCorps)
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

// trajetJSON : représentation publique d'un trajet. Les clés suivent la fiche UC1
// (driver_id, departure_time) ; le délai d'annulation est en heures décimales.
type trajetJSON struct {
	Identifiant     string  `json:"id"`
	Conducteur      string  `json:"driver_id"`
	Origine         string  `json:"origin"`
	Destination     string  `json:"destination"`
	Depart          string  `json:"departure_time"`
	PlacesTotal     int     `json:"seats_total"`
	Places          int     `json:"seats_available"`
	Prix            float64 `json:"price"`
	DelaiAnnulation float64 `json:"cancel_delay_hours"`
	Statut          string  `json:"status"`
	CreeLe          string  `json:"created_at"`
}

func trajetVersJSON(t domain.Trajet) trajetJSON {
	return trajetJSON{
		Identifiant:     t.Identifiant,
		Conducteur:      t.Conducteur,
		Origine:         t.Origine,
		Destination:     t.Destination,
		Depart:          t.Depart.UTC().Format(time.RFC3339),
		PlacesTotal:     t.PlacesTotal,
		Places:          t.Places,
		Prix:            t.Prix,
		DelaiAnnulation: t.DelaiAnnulation.Hours(),
		Statut:          string(t.Statut),
		CreeLe:          t.CreeLe.UTC().Format(time.RFC3339),
	}
}

// conducteurJSON : informations UC8 montrées par UC7. Note nil → aucune évaluation UC6 reçue.
type conducteurJSON struct {
	Nom   string   `json:"name"`
	Sexe  string   `json:"sex,omitempty"`
	Age   int      `json:"age"`
	Photo string   `json:"photo,omitempty"`
	Note  *float64 `json:"rating"`
}

// detailTrajetJSON reprend les champs de trajetJSON et ajoute le conducteur (UC7).
type detailTrajetJSON struct {
	trajetJSON
	Conducteur *conducteurJSON `json:"driver"`
}

// consulterTrajet : GET /rides/{id}, lecture sans identité requise (UC7, aucune précondition).
func consulterTrajet(consulter application.ConsulterTrajet) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		detail, err := consulter.Executer(r.Context(), r.PathValue("id"))
		if err != nil {
			repondreErreur(w, err)
			return
		}

		reponse := detailTrajetJSON{trajetJSON: trajetVersJSON(detail.Trajet)}
		if detail.Profil != nil {
			reponse.Conducteur = &conducteurJSON{
				Nom:   detail.Profil.Nom,
				Sexe:  string(detail.Profil.Sexe),
				Age:   detail.Profil.Age,
				Photo: detail.Profil.Photo,
				Note:  detail.Note,
			}
		}
		repondreJSON(w, http.StatusOK, reponse)
	}
}

// rechercherTrajets : GET /rides, aucun identifiant requis (UC2, aucune précondition).
func rechercherTrajets(rechercher application.RechercherTrajets) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		parametres := r.URL.Query()
		trajets, err := rechercher.Executer(r.Context(), domain.CriteresRecherche{
			Origine:     parametres.Get("origin"),
			Destination: parametres.Get("destination"),
			Date:        parametres.Get("date"),
		})
		if err != nil {
			repondreErreur(w, err)
			return
		}
		resultats := make([]trajetJSON, 0, len(trajets))
		for _, trajet := range trajets {
			resultats = append(resultats, trajetVersJSON(trajet))
		}
		repondreJSON(w, http.StatusOK, resultats)
	}
}

// publierTrajet : POST /rides, le conducteur identifié publie un trajet.
func publierTrajet(publier application.PublierTrajet) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		acteur, err := identite(r)
		if err != nil {
			repondreErreur(w, err)
			return
		}

		r.Body = http.MaxBytesReader(w, r.Body, TailleMaxCorps)
		var saisie application.SaisieTrajet
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

		trajet, err := publier.Executer(r.Context(), acteur, saisie)
		if err != nil {
			repondreErreur(w, err)
			return
		}
		repondreJSON(w, http.StatusCreated, trajetVersJSON(trajet))
	}
}

// reservationJSON : représentation publique d'une réservation.
type reservationJSON struct {
	Identifiant string `json:"id"`
	Trajet      string `json:"ride_id"`
	Passager    string `json:"passenger"`
	Statut      string `json:"status"`
	CreeLe      string `json:"created_at"`
}

func reservationVersJSON(r domain.Reservation) reservationJSON {
	return reservationJSON{
		Identifiant: r.Identifiant,
		Trajet:      r.Trajet,
		Passager:    r.Passager,
		Statut:      string(r.Statut),
		CreeLe:      r.CreeLe.UTC().Format(time.RFC3339),
	}
}

// reserverPlace : POST /rides/{id}/bookings, le passager identifié réserve une place.
func reserverPlace(reserver application.ReserverPlace) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		acteur, err := identite(r)
		if err != nil {
			repondreErreur(w, err)
			return
		}
		reservation, err := reserver.Executer(r.Context(), r.PathValue("id"), acteur)
		if err != nil {
			repondreErreur(w, err)
			return
		}
		repondreJSON(w, http.StatusCreated, reservationVersJSON(reservation))
	}
}
