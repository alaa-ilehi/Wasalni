package application_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/alaa-ilehi/wasalni/internal/application"
	"github.com/alaa-ilehi/wasalni/internal/domain"
)

// evaluationsFausses : port EvaluationRepository en mémoire, avec l'index unique simulé.
type evaluationsFausses struct {
	creees  []domain.Evaluation
	couples map[string]bool
	notes   map[string]float64
	erreur  error
}

func (e *evaluationsFausses) Enregistrer(_ context.Context, evaluation domain.Evaluation) (domain.Evaluation, error) {
	if e.erreur != nil {
		return domain.Evaluation{}, e.erreur
	}
	if e.couples == nil {
		e.couples = map[string]bool{}
	}
	cle := evaluation.Trajet + "|" + evaluation.Evaluateur + "|" + evaluation.Cible
	if e.couples[cle] {
		return domain.Evaluation{}, domain.ErrEvaluationDejaFaite
	}
	e.couples[cle] = true
	evaluation.Identifiant = "eval-1"
	e.creees = append(e.creees, evaluation)
	return evaluation, nil
}

func (e *evaluationsFausses) Moyenne(_ context.Context, identifiant string) (float64, int, error) {
	if e.erreur != nil {
		return 0, 0, e.erreur
	}
	if note, ok := e.notes[identifiant]; ok {
		return note, 1, nil
	}
	return 0, 0, nil
}

func trajetTermine(id, conducteur string) domain.Trajet {
	trajet := trajetPublie(id, conducteur)
	trajet.Statut = domain.TrajetTermine
	return trajet
}

func bancEvaluation() (application.EvaluerParticipant, *trajetsFaux, *reservationsFausses, *evaluationsFausses) {
	trajets := &trajetsFaux{parID: map[string]domain.Trajet{
		"t1": trajetTermine("t1", "alice"),
	}}
	reservations := &reservationsFausses{creees: []domain.Reservation{
		{Trajet: "t1", Passager: "bob", Statut: domain.ReservationActive},
	}}
	evaluations := &evaluationsFausses{}
	return application.NouveauEvaluerParticipant(trajets, reservations, evaluations),
		trajets, reservations, evaluations
}

func TestEvaluerPassagerNoteLeConducteur(t *testing.T) {
	evaluer, _, _, evaluations := bancEvaluation()

	evaluation, err := evaluer.Executer(context.Background(), "bob", "t1", "alice", 4, "  trajet agréable  ")
	if err != nil {
		t.Fatalf("évaluation : %v", err)
	}
	if evaluation.Identifiant != "eval-1" || evaluation.Note != 4 || evaluation.Commentaire != "trajet agréable" {
		t.Errorf("évaluation = %+v", evaluation)
	}
	if len(evaluations.creees) != 1 || evaluations.creees[0].Cible != "alice" {
		t.Errorf("évaluations enregistrées = %+v", evaluations.creees)
	}
}

func TestEvaluerConducteurNoteUnPassager(t *testing.T) {
	evaluer, _, _, evaluations := bancEvaluation()

	if _, err := evaluer.Executer(context.Background(), "alice", "t1", "bob", 5, ""); err != nil {
		t.Fatalf("évaluation du passager : %v", err)
	}
	if evaluations.creees[0].Evaluateur != "alice" || evaluations.creees[0].Cible != "bob" {
		t.Errorf("évaluation = %+v", evaluations.creees[0])
	}
}

func TestEvaluerParticipantRefus(t *testing.T) {
	cas := []struct {
		libelle     string
		evaluateur  string
		trajet      string
		cible       string
		note        int
		commentaire string
		preparer    func(*trajetsFaux, *evaluationsFausses)
		attendu     error
	}{
		{
			libelle: "note trop basse", evaluateur: "bob", trajet: "t1", cible: "alice", note: 0,
			attendu: domain.ErrEvaluationInvalide,
		},
		{
			libelle: "note trop haute", evaluateur: "bob", trajet: "t1", cible: "alice", note: 6,
			attendu: domain.ErrEvaluationInvalide,
		},
		{
			libelle: "évaluateur absent", evaluateur: "", trajet: "t1", cible: "alice", note: 4,
			attendu: domain.ErrEvaluationInvalide,
		},
		{
			libelle: "auto-évaluation", evaluateur: "bob", trajet: "t1", cible: "bob", note: 4,
			attendu: domain.ErrEvaluationInvalide,
		},
		{
			libelle: "commentaire trop long", evaluateur: "bob", trajet: "t1", cible: "alice", note: 4,
			commentaire: strings.Repeat("x", domain.CommentaireLongueurMax+1),
			attendu:     domain.ErrEvaluationInvalide,
		},
		{
			libelle: "trajet inconnu", evaluateur: "bob", trajet: "inconnu", cible: "alice", note: 4,
			attendu: domain.ErrTrajetIntrouvable,
		},
		{
			libelle: "trajet non terminé", evaluateur: "bob", trajet: "t1", cible: "alice", note: 4,
			preparer: func(trajets *trajetsFaux, _ *evaluationsFausses) {
				trajets.parID["t1"] = trajetPublie("t1", "alice")
			},
			attendu: domain.ErrTrajetNonTermine,
		},
		{
			libelle: "évaluateur non participant", evaluateur: "carol", trajet: "t1", cible: "alice", note: 4,
			attendu: domain.ErrNonParticipant,
		},
		{
			libelle: "cible non participante", evaluateur: "alice", trajet: "t1", cible: "carol", note: 4,
			attendu: domain.ErrNonParticipant,
		},
		{
			libelle: "passager évalue un passager", evaluateur: "bob", trajet: "t1", cible: "carol", note: 4,
			attendu: domain.ErrNonParticipant,
		},
		{
			libelle: "doublon", evaluateur: "bob", trajet: "t1", cible: "alice", note: 4,
			preparer: func(_ *trajetsFaux, evaluations *evaluationsFausses) {
				evaluations.couples = map[string]bool{"t1|bob|alice": true}
			},
			attendu: domain.ErrEvaluationDejaFaite,
		},
	}

	for _, c := range cas {
		t.Run(c.libelle, func(t *testing.T) {
			evaluer, trajets, _, evaluations := bancEvaluation()
			if c.preparer != nil {
				c.preparer(trajets, evaluations)
			}
			_, err := evaluer.Executer(context.Background(), c.evaluateur, c.trajet, c.cible, c.note, c.commentaire)
			if !errors.Is(err, c.attendu) {
				t.Fatalf("erreur = %v, attendu %v", err, c.attendu)
			}
		})
	}
}

func TestEvaluerParticipantNeCachePasUneErreurDeDepot(t *testing.T) {
	evaluer, _, reservations, _ := bancEvaluation()
	reservations.erreur = errors.New("base indisponible")

	_, err := evaluer.Executer(context.Background(), "bob", "t1", "alice", 4, "")
	if err == nil || !strings.Contains(err.Error(), "base indisponible") {
		t.Fatalf("erreur = %v, attendu l'erreur du dépôt", err)
	}
}
