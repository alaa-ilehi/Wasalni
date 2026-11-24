package api

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/alaa-ilehi/wasalni/internal/application"
	"github.com/alaa-ilehi/wasalni/internal/domain"
)

// evaluationJSON : entrée de POST /rides/{id}/ratings.
type evaluationJSON struct {
	Score       int    `json:"score"`
	Commentaire string `json:"comment"`
	Cible       string `json:"target"`
}

// evaluationReponseJSON : sortie de POST /rides/{id}/ratings, dans le même vocabulaire anglais que le reste de l'API.
type evaluationReponseJSON struct {
	Identifiant string `json:"id"`
	Trajet      string `json:"ride_id"`
	Evaluateur  string `json:"evaluator"`
	Cible       string `json:"target"`
	Note        int    `json:"score"`
	Commentaire string `json:"comment,omitempty"`
	CreeLe      string `json:"created_at"`
}

func evaluationVersJSON(e domain.Evaluation) evaluationReponseJSON {
	return evaluationReponseJSON{
		Identifiant: e.Identifiant,
		Trajet:      e.Trajet,
		Evaluateur:  e.Evaluateur,
		Cible:       e.Cible,
		Note:        e.Note,
		Commentaire: e.Commentaire,
		CreeLe:      e.CreeLe.UTC().Format(time.RFC3339),
	}
}

// enregistrerEvaluation : POST /rides/{id}/ratings (UC6), identité externe requise.
func enregistrerEvaluation(evaluer application.EvaluerParticipant) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		identifiant, err := identite(r)
		if err != nil {
			repondreErreur(w, err)
			return
		}

		var entree evaluationJSON
		corps := http.MaxBytesReader(w, r.Body, TailleMaxCorps)
		if err := json.NewDecoder(corps).Decode(&entree); err != nil {
			repondreErreur(w, domain.ErrEvaluationInvalide)
			return
		}

		evaluation, err := evaluer.Executer(
			r.Context(),
			identifiant,
			r.PathValue("id"),
			entree.Cible,
			entree.Score,
			entree.Commentaire,
		)
		if err != nil {
			repondreErreur(w, err)
			return
		}
		repondreJSON(w, http.StatusCreated, evaluationVersJSON(evaluation))
	}
}
