package api_test

import (
	"context"
	"database/sql"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/alaa-ilehi/wasalni/internal/api"
	"github.com/alaa-ilehi/wasalni/internal/application"
	"github.com/alaa-ilehi/wasalni/internal/domain"
	"github.com/alaa-ilehi/wasalni/internal/infrastructure/sqlite"
)

// evaluationsEnMemoire : port EvaluationRepository en mémoire pour les bancs sans base.
type evaluationsEnMemoire struct {
	notes map[string]float64
}

func (e evaluationsEnMemoire) Enregistrer(_ context.Context, evaluation domain.Evaluation) (domain.Evaluation, error) {
	return evaluation, nil
}

func (e evaluationsEnMemoire) Moyenne(_ context.Context, identifiant string) (float64, int, error) {
	if note, ok := e.notes[identifiant]; ok {
		return note, 1, nil
	}
	return 0, 0, nil
}

// bancEvaluations : tous les cas câblés sur la même base SQLite, comme dans cmd/wasalni.
func bancEvaluations(t *testing.T) (*httptest.Server, *sql.DB) {
	t.Helper()
	db, err := sqlite.Ouvrir(context.Background(), filepath.Join(t.TempDir(), "evaluations.db"))
	if err != nil {
		t.Fatalf("ouverture de la base : %v", err)
	}
	t.Cleanup(func() { db.Close() })

	profils := sqlite.NouveauProfilRepository(db)
	trajets := sqlite.NouveauTrajetRepository(db)
	reservations := sqlite.NouveauReservationRepository(db)
	evaluations := sqlite.NouveauEvaluationRepository(db)
	serveur := httptest.NewServer(api.Routes(api.Deps{
		Authentification: identitePartagee.auth,
		Profils:          application.NouveauGererProfil(profils, adaptateurPhoto{}),
		Trajets:          application.NouveauPublierTrajet(trajets, horlogeFigee{}),
		Details:          application.NouveauConsulterTrajet(trajets, profils, evaluations),
		Reservations:     application.NouveauReserverPlace(trajets, reservations, notificateurMuet{}),
		Evaluations:      application.NouveauEvaluerParticipant(trajets, reservations, evaluations),
	}))
	t.Cleanup(serveur.Close)
	return serveur, db
}

func postEvaluation(t *testing.T, serveur *httptest.Server, trajet, acteur, corps string) (int, map[string]any) {
	t.Helper()
	requete, err := http.NewRequest(http.MethodPost,
		serveur.URL+"/rides/"+trajet+"/ratings", strings.NewReader(corps))
	if err != nil {
		t.Fatalf("requête : %v", err)
	}
	requete.Header.Set("Content-Type", "application/json")
	if acteur != "" {
		requete.Header.Set(api.EnTeteAutorisation, autorisationDe(t, acteur))
	}
	reponse, err := http.DefaultClient.Do(requete)
	if err != nil {
		t.Fatalf("appel : %v", err)
	}
	defer reponse.Body.Close()
	brut, _ := io.ReadAll(reponse.Body)
	charge := map[string]any{}
	if len(brut) > 0 {
		if err := json.Unmarshal(brut, &charge); err != nil {
			t.Fatalf("décodage de la réponse : %v", err)
		}
	}
	return reponse.StatusCode, charge
}

func publierEtTerminer(t *testing.T, serveur *httptest.Server, db *sql.DB, passagers ...string) string {
	t.Helper()
	statut, publie := posterTrajet(t, serveur, "alice", corpsTrajetValide())
	if statut != http.StatusCreated {
		t.Fatalf("publication : statut = %d, corps = %v", statut, publie)
	}
	identifiant, _ := publie["id"].(string)
	for _, passager := range passagers {
		if statut, _ := postReservation(t, serveur, identifiant, passager); statut != http.StatusCreated {
			t.Fatalf("réservation de %s : statut = %d", passager, statut)
		}
	}
	if _, err := db.Exec(`UPDATE rides SET status = 'COMPLETED' WHERE id = ?`, identifiant); err != nil {
		t.Fatalf("clôture du trajet : %v", err)
	}
	return identifiant
}

func TestEvaluerParticipantsEtLireLaMoyenneDansUC7(t *testing.T) {
	serveur, db := bancEvaluations(t)
	if statut, _ := putProfil(t, serveur, "alice", `{"name":"Alice Ben Salah","age":29,"sex":"FEMALE"}`); statut != http.StatusCreated {
		t.Fatalf("profil du conducteur : statut = %d", statut)
	}
	trajet := publierEtTerminer(t, serveur, db, "bob", "carol")

	cas := []struct{ acteur, corps string }{
		{"bob", `{"score":4,"comment":"ponctuel","target":"alice"}`},
		{"carol", `{"score":5,"target":"alice"}`},
		{"alice", `{"score":3,"target":"bob"}`},
	}
	for _, c := range cas {
		statut, charge := postEvaluation(t, serveur, trajet, c.acteur, c.corps)
		if statut != http.StatusCreated {
			t.Fatalf("évaluation de %s : statut = %d, corps = %v", c.acteur, statut, charge)
		}
		if charge["ride_id"] != trajet || charge["evaluator"] != c.acteur {
			t.Errorf("évaluation = %v", charge)
		}
	}

	// UC7 refuse un trajet terminé : on relit une publication du même conducteur pour vérifier la note.
	if _, err := db.Exec(`UPDATE rides SET status = 'PUBLISHED' WHERE id = ?`, trajet); err != nil {
		t.Fatalf("réouverture du trajet : %v", err)
	}
	statut, detail := getTrajet(t, serveur, trajet, "")
	if statut != http.StatusOK {
		t.Fatalf("consultation : statut = %d", statut)
	}
	conducteur, ok := detail["driver"].(map[string]any)
	if !ok {
		t.Fatalf("conducteur = %v", detail["driver"])
	}
	if note := conducteur["rating"]; note != float64(4.5) {
		t.Errorf("note du conducteur = %v, attendu 4.5", note)
	}
}

func TestEvaluerParticipantRefus(t *testing.T) {
	cas := []struct {
		libelle   string
		preparer  func(*testing.T, *httptest.Server, *sql.DB) string
		acteur    string
		corps     string
		trajetArg bool
		attendu   int
	}{
		{
			libelle: "identité absente", acteur: "", corps: `{"score":4,"target":"alice"}`,
			trajetArg: true, attendu: http.StatusUnauthorized,
		},
		{
			libelle: "note hors bornes", acteur: "bob", corps: `{"score":6,"target":"alice"}`,
			trajetArg: true, attendu: http.StatusBadRequest,
		},
		{
			libelle: "commentaire trop long", acteur: "bob",
			corps:     `{"score":4,"target":"alice","comment":"` + strings.Repeat("x", domain.CommentaireLongueurMax+1) + `"}`,
			trajetArg: true, attendu: http.StatusBadRequest,
		},
		{
			libelle: "trajet inconnu", acteur: "bob", corps: `{"score":4,"target":"alice"}`,
			attendu: http.StatusNotFound,
		},
		{
			libelle: "auto-évaluation", acteur: "bob", corps: `{"score":4,"target":"bob"}`,
			trajetArg: true, attendu: http.StatusBadRequest,
		},
		{
			libelle: "non participant", acteur: "carol", corps: `{"score":4,"target":"alice"}`,
			trajetArg: true, attendu: http.StatusForbidden,
		},
		{
			libelle: "cible non participante", acteur: "alice", corps: `{"score":4,"target":"carol"}`,
			trajetArg: true, attendu: http.StatusForbidden,
		},
		{
			libelle: "doublon", acteur: "bob", corps: `{"score":4,"target":"alice"}`,
			trajetArg: true, attendu: http.StatusConflict,
		},
	}

	for _, c := range cas {
		t.Run(c.libelle, func(t *testing.T) {
			serveur, db := bancEvaluations(t)
			trajet := publierEtTerminer(t, serveur, db, "bob")
			if c.libelle == "doublon" {
				statut, _ := postEvaluation(t, serveur, trajet, "bob", `{"score":4,"target":"alice"}`)
				if statut != http.StatusCreated {
					t.Fatalf("première évaluation : statut = %d", statut)
				}
			}
			chemin := trajet
			if !c.trajetArg {
				chemin = "inconnu"
			}
			statut, corps := postEvaluation(t, serveur, chemin, c.acteur, c.corps)
			if statut != c.attendu {
				t.Fatalf("statut = %d, attendu %d (corps %v)", statut, c.attendu, corps)
			}
			if message, ok := corps["error"].(string); !ok || message == "" {
				t.Errorf("corps d'erreur = %v", corps)
			}
		})
	}
}

func TestEvaluerRefuseUnTrajetNonTermine(t *testing.T) {
	serveur, _ := bancEvaluations(t)
	statut, publie := posterTrajet(t, serveur, "alice", corpsTrajetValide())
	if statut != http.StatusCreated {
		t.Fatalf("publication : statut = %d", statut)
	}
	trajet, _ := publie["id"].(string)

	statut, corps := postEvaluation(t, serveur, trajet, "bob", `{"score":4,"target":"alice"}`)
	if statut != http.StatusConflict {
		t.Fatalf("statut = %d, attendu 409 (corps %v)", statut, corps)
	}
	if corps["error"] == nil {
		t.Error("message d'erreur absent")
	}
}

func TestConsulterTrajetNoteNulleSansEvaluation(t *testing.T) {
	serveur := httptest.NewServer(api.Routes(api.Deps{
		Authentification: identitePartagee.auth,
		Details: application.NouveauConsulterTrajet(
			trajetsEnMemoire{"t1": {Identifiant: "t1", Conducteur: "alice", Statut: domain.TrajetPublie}},
			profilsEnMemoire{"alice": {Identifiant: "alice", Nom: "Alice", Age: 29}},
			evaluationsEnMemoire{},
		),
	}))
	defer serveur.Close()

	statut, corps := getTrajet(t, serveur, "t1", "")
	if statut != http.StatusOK {
		t.Fatalf("statut = %d, attendu 200", statut)
	}
	conducteur := corps["driver"].(map[string]any)
	if note, present := conducteur["rating"]; !present || note != nil {
		t.Errorf("note = %v (présent : %v), attendu null sans évaluation", note, present)
	}
}
