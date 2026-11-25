package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/alaa-ilehi/wasalni/internal/application"
	"github.com/alaa-ilehi/wasalni/internal/domain"
)

// TailleMaxCorps borne le corps HTTP accepté par PUT /profile (10 Mio, ADR 0008).
const TailleMaxCorps = 10 << 20

// Deps regroupe les cas d'utilisation exposés par l'API. La racine de composition les fournit.
type Deps struct {
	Profils        application.GererProfil
	Recherches     application.RechercherTrajets
	Trajets        application.PublierTrajet
	Details        application.ConsulterTrajet
	Reservations   application.ReserverPlace
	Annulations    application.AnnulerReservation
	Terminaisons   application.TerminerTrajet
	GestionTrajets application.GererTrajet
	Evaluations    application.EvaluerParticipant
	// Authentification : UC13, seule source d'identité ; aucune route protégée ne lit un en-tête de confiance.
	Authentification application.Authentification
}

// Routes construit le routeur HTTP du service (Go 1.22 : motifs avec méthode).
func Routes(deps Deps) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, _ *http.Request) {
		repondreJSON(w, http.StatusOK, map[string]string{"statut": "ok"})
	})
	mux.HandleFunc("POST /auth/register", inscrire(deps.Authentification))
	mux.HandleFunc("POST /auth/login", connecter(deps.Authentification))
	mux.HandleFunc("DELETE /auth/session", deconnecter(deps.Authentification))
	mux.HandleFunc("GET /profile", consulterProfil(deps.Profils, deps.Authentification))
	mux.HandleFunc("PUT /profile", enregistrerProfil(deps.Profils, deps.Authentification))
	mux.HandleFunc("GET /rides", rechercherTrajets(deps.Recherches, deps.Authentification))
	mux.HandleFunc("GET /rides/{id}", consulterTrajet(deps.Details, deps.Authentification))
	mux.HandleFunc("POST /rides", publierTrajet(deps.Trajets, deps.Authentification))
	mux.HandleFunc("POST /rides/{id}/bookings", reserverPlace(deps.Reservations, deps.Authentification))
	mux.HandleFunc("POST /rides/{id}/complete", terminerTrajet(deps.Terminaisons, deps.Authentification))
	mux.HandleFunc("PUT /rides/{id}", modifierTrajet(deps.GestionTrajets, deps.Authentification))
	mux.HandleFunc("DELETE /rides/{id}", supprimerTrajet(deps.GestionTrajets, deps.Authentification))
	mux.HandleFunc("DELETE /bookings/{id}", annulerReservation(deps.Annulations, deps.Authentification))
	mux.HandleFunc("POST /rides/{id}/ratings", enregistrerEvaluation(deps.Evaluations, deps.Authentification))
	return mux
}

// identifiantsJSON : corps commun à l'inscription et à la connexion (UC13).
type identifiantsJSON struct {
	Identifiant string `json:"id"`
	MotDePasse  string `json:"password"`
}

// sessionJSON : jeton rendu une seule fois au client ; il n'est jamais relu depuis la base en clair.
type sessionJSON struct {
	Identifiant string `json:"id"`
	Jeton       string `json:"token"`
}

func inscrire(auth application.Authentification) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		r.Body = http.MaxBytesReader(w, r.Body, TailleMaxCorps)
		var saisie identifiantsJSON
		decodeur := json.NewDecoder(r.Body)
		decodeur.DisallowUnknownFields()
		if err := decodeur.Decode(&saisie); err != nil {
			repondreJSON(w, http.StatusBadRequest, erreurJSON{Erreur: "corps JSON invalide"})
			return
		}
		jeton, err := auth.Inscrire(r.Context(), saisie.Identifiant, saisie.MotDePasse)
		if err != nil {
			repondreErreur(w, err)
			return
		}
		repondreJSON(w, http.StatusCreated, sessionJSON{Identifiant: strings.TrimSpace(saisie.Identifiant), Jeton: jeton})
	}
}

func connecter(auth application.Authentification) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		r.Body = http.MaxBytesReader(w, r.Body, TailleMaxCorps)
		var saisie identifiantsJSON
		decodeur := json.NewDecoder(r.Body)
		decodeur.DisallowUnknownFields()
		if err := decodeur.Decode(&saisie); err != nil {
			repondreJSON(w, http.StatusBadRequest, erreurJSON{Erreur: "corps JSON invalide"})
			return
		}
		jeton, err := auth.Connecter(r.Context(), saisie.Identifiant, saisie.MotDePasse)
		if err != nil {
			repondreErreur(w, err)
			return
		}
		repondreJSON(w, http.StatusOK, sessionJSON{Identifiant: strings.TrimSpace(saisie.Identifiant), Jeton: jeton})
	}
}

func deconnecter(auth application.Authentification) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if _, err := identite(r, auth); err != nil {
			repondreErreur(w, err)
			return
		}
		if err := auth.Deconnecter(r.Context(), jetonBearer(r)); err != nil {
			repondreErreur(w, err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

func modifierTrajet(gerer application.GererTrajet, auth application.Authentification) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		acteur, err := identite(r, auth)
		if err != nil {
			repondreErreur(w, err)
			return
		}
		r.Body = http.MaxBytesReader(w, r.Body, TailleMaxCorps)
		var saisie application.SaisieTrajet
		decodeur := json.NewDecoder(r.Body)
		decodeur.DisallowUnknownFields()
		if err := decodeur.Decode(&saisie); err != nil {
			repondreJSON(w, http.StatusBadRequest, erreurJSON{Erreur: "corps JSON invalide"})
			return
		}
		trajet, err := gerer.Modifier(r.Context(), acteur, r.PathValue("id"), saisie)
		if err != nil {
			repondreErreur(w, err)
			return
		}
		repondreJSON(w, http.StatusOK, trajetVersJSON(trajet))
	}
}

func supprimerTrajet(gerer application.GererTrajet, auth application.Authentification) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		acteur, err := identite(r, auth)
		if err != nil {
			repondreErreur(w, err)
			return
		}
		if err := gerer.Supprimer(r.Context(), acteur, r.PathValue("id")); err != nil {
			repondreErreur(w, err)
			return
		}
		repondreJSON(w, http.StatusNoContent, nil)
	}
}

// terminerTrajet : POST /rides/{id}/complete, clôt le trajet du conducteur identifié.
func terminerTrajet(terminer application.TerminerTrajet, auth application.Authentification) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		conducteur, err := identite(r, auth)
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
func consulterProfil(gerer application.GererProfil, auth application.Authentification) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		acteur, err := identite(r, auth)
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
func enregistrerProfil(gerer application.GererProfil, auth application.Authentification) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		acteur, err := identite(r, auth)
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

// consulterTrajet : GET /rides/{id}, session requise mais aucun rôle particulier (UC7, aucune précondition).
func consulterTrajet(consulter application.ConsulterTrajet, auth application.Authentification) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if _, err := identite(r, auth); err != nil {
			repondreErreur(w, err)
			return
		}
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

// rechercherTrajets : GET /rides, session requise mais aucun identifiant transmis (UC2, aucune précondition).
func rechercherTrajets(rechercher application.RechercherTrajets, auth application.Authentification) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// La recherche ne sert pas l'identité, mais une session est exigée : la seule exception au
		// jeton est /health et les routes /auth/* (ADR 0011).
		if _, err := identite(r, auth); err != nil {
			repondreErreur(w, err)
			return
		}
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
func publierTrajet(publier application.PublierTrajet, auth application.Authentification) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		acteur, err := identite(r, auth)
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
func reserverPlace(reserver application.ReserverPlace, auth application.Authentification) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		acteur, err := identite(r, auth)
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

// annulerReservation : DELETE /bookings/{id}, annule la réservation de l'acteur identifié.
func annulerReservation(annuler application.AnnulerReservation, auth application.Authentification) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		acteur, err := identite(r, auth)
		if err != nil {
			repondreErreur(w, err)
			return
		}
		if _, err := annuler.Executer(r.Context(), acteur, r.PathValue("id")); err != nil {
			repondreErreur(w, err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}
