package api_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/alaa-ilehi/wasalni/internal/api"
	"github.com/alaa-ilehi/wasalni/internal/application"
	"github.com/alaa-ilehi/wasalni/internal/infrastructure/sqlite"
)

// horlogeFigee : l'API ne lit pas le temps réel dans les tests de publication.
type horlogeFigee struct{}

func (horlogeFigee) Maintenant() time.Time {
	return time.Date(2026, 11, 24, 20, 0, 0, 0, time.UTC)
}

// bancTrajets : les deux cas d'usage câblés sur la même base SQLite réelle,
// comme dans cmd/wasalni : publier un trajet puis réserver une place dessus.
func bancTrajets(t *testing.T) *httptest.Server {
	t.Helper()
	db, err := sqlite.Ouvrir(context.Background(), filepath.Join(t.TempDir(), "trajets.db"))
	if err != nil {
		t.Fatalf("ouverture de la base : %v", err)
	}
	t.Cleanup(func() { db.Close() })

	profils := sqlite.NouveauProfilRepository(db)
	trajets := sqlite.NouveauTrajetRepository(db)
	publier := application.NouveauPublierTrajet(trajets, horlogeFigee{})
	rechercher := application.NouveauRechercherTrajets(trajets)
	consulter := application.NouveauConsulterTrajet(trajets, profils, evaluationsEnMemoire{})
	reserver := application.NouveauReserverPlace(
		trajets,
		sqlite.NouveauReservationRepository(db),
		notificateurMuet{},
	)
	serveur := httptest.NewServer(api.Routes(api.Deps{
		Profils:      application.NouveauGererProfil(profils, adaptateurPhoto{}),
		Recherches:   rechercher,
		Trajets:      publier,
		Details:      consulter,
		Reservations: reserver,
	}))
	t.Cleanup(serveur.Close)
	return serveur
}

func posterTrajet(t *testing.T, serveur *httptest.Server, acteur, corps string) (int, map[string]any) {
	t.Helper()
	requete, err := http.NewRequest(http.MethodPost, serveur.URL+"/rides", strings.NewReader(corps))
	if err != nil {
		t.Fatalf("requête : %v", err)
	}
	requete.Header.Set("Content-Type", "application/json")
	if acteur != "" {
		requete.Header.Set(api.EnTeteIdentite, acteur)
	}
	reponse, err := http.DefaultClient.Do(requete)
	if err != nil {
		t.Fatalf("appel : %v", err)
	}
	defer reponse.Body.Close()
	var corpsSortie map[string]any
	if err := json.NewDecoder(reponse.Body).Decode(&corpsSortie); err != nil {
		t.Fatalf("décodage de la réponse : %v", err)
	}
	return reponse.StatusCode, corpsSortie
}

func corpsTrajetValide() string {
	return `{"origin":"Lyon","destination":"Grenoble",` +
		`"departure_time":"2026-12-01T08:30:00Z","seats_total":3,"price":12.5,"cancel_delay_hours":1}`
}

func TestPublierTrajetSansIdentite(t *testing.T) {
	serveur := bancTrajets(t)
	statut, corps := posterTrajet(t, serveur, "", corpsTrajetValide())
	if statut != http.StatusUnauthorized {
		t.Fatalf("statut = %d, attendu 401 (corps %v)", statut, corps)
	}
	if corps["error"] != "identité manquante" {
		t.Errorf("erreur = %v, attendu « identité manquante »", corps["error"])
	}
}

func TestPublierTrajetRefuseUneSaisieInvalide(t *testing.T) {
	serveur := bancTrajets(t)
	cas := map[string]struct {
		corps string
	}{
		"origine égale à la destination": {
			corps: `{"origin":"Lyon","destination":"lyon","departure_time":"2026-12-01T08:30:00Z","seats_total":3}`,
		},
		"départ passé": {
			corps: `{"origin":"Lyon","destination":"Grenoble","departure_time":"2026-01-01T08:00:00Z","seats_total":1}`,
		},
		"aucune place": {
			corps: `{"origin":"Lyon","destination":"Grenoble","departure_time":"2026-12-01T08:30:00Z","seats_total":0}`,
		},
		"champ inconnu": {
			corps: `{"origin":"Lyon","destination":"Grenoble","departure_time":"2026-12-01T08:30:00Z","seats_total":1,"couleur":"rouge"}`,
		},
		"json illisible": {corps: `{`},
	}
	for nom, cas := range cas {
		t.Run(nom, func(t *testing.T) {
			statut, corps := posterTrajet(t, serveur, "bob", cas.corps)
			if statut != http.StatusBadRequest {
				t.Fatalf("statut = %d, attendu 400 (corps %v)", statut, corps)
			}
			if _, ok := corps["error"].(string); !ok {
				t.Errorf("corps d'erreur inattendu : %v", corps)
			}
		})
	}
}

func TestPublierTrajetPuisReserver(t *testing.T) {
	serveur := bancTrajets(t)

	statut, corps := posterTrajet(t, serveur, "bob", corpsTrajetValide())
	if statut != http.StatusCreated {
		t.Fatalf("statut = %d, attendu 201 (corps %v)", statut, corps)
	}
	identifiant, _ := corps["id"].(string)
	if identifiant == "" {
		t.Fatalf("identifiant absent de la réponse : %v", corps)
	}
	attendus := map[string]any{
		"driver_id":          "bob",
		"origin":             "Lyon",
		"destination":        "Grenoble",
		"seats_total":        float64(3),
		"seats_available":    float64(3),
		"price":              12.5,
		"cancel_delay_hours": float64(1),
		"status":             "PUBLISHED",
	}
	for cle, attendu := range attendus {
		if corps[cle] != attendu {
			t.Errorf("%s = %v, attendu %v", cle, corps[cle], attendu)
		}
	}
	for _, cle := range []string{"departure_time", "created_at"} {
		if texte, _ := corps[cle].(string); texte == "" {
			t.Errorf("%s absent de la réponse : %v", cle, corps)
		}
	}

	// Le trajet publié est immédiatement réservable : plus de 404 par adaptateur provisoire.
	statutReservation, corpsReservation := postReservation(t, serveur, identifiant, "alice")
	if statutReservation != http.StatusCreated {
		t.Fatalf("réservation = %d, attendu 201 (corps %v)", statutReservation, corpsReservation)
	}
	// Le conducteur ne peut pas réserver sa propre place.
	if statut, _ := postReservation(t, serveur, identifiant, "bob"); statut != http.StatusBadRequest {
		t.Errorf("réservation du conducteur = %d, attendu 400", statut)
	}
}

// Le décrément des places est visible par le contrat HTTP : deux places, deux réservations, puis 409.
func TestReserverConsommeLesPlacesJusquauRefus(t *testing.T) {
	serveur := bancTrajets(t)

	statut, publie := posterTrajet(t, serveur, "bob",
		`{"origin":"Lyon","destination":"Grenoble","departure_time":"2026-12-01T08:30:00Z",`+
			`"seats_total":2,"price":12.5,"cancel_delay_hours":1}`)
	if statut != http.StatusCreated {
		t.Fatalf("publication : statut = %d, attendu 201 (corps %v)", statut, publie)
	}
	identifiant, _ := publie["id"].(string)

	for _, passager := range []string{"alice", "carol"} {
		if statut, corps := postReservation(t, serveur, identifiant, passager); statut != http.StatusCreated {
			t.Fatalf("réservation de %s : statut = %d (corps %v)", passager, statut, corps)
		}
	}

	statutLecture, trajet := getTrajet(t, serveur, identifiant, "")
	if statutLecture != http.StatusOK {
		t.Fatalf("lecture du trajet : statut = %d", statutLecture)
	}
	if trajet["seats_available"] != float64(0) || trajet["seats_total"] != float64(2) {
		t.Errorf("places = %v/%v, attendu 0/2", trajet["seats_available"], trajet["seats_total"])
	}

	statut, corps := postReservation(t, serveur, identifiant, "dave")
	if statut != http.StatusConflict {
		t.Errorf("réservation sur un trajet plein : statut = %d, attendu 409 (corps %v)", statut, corps)
	}
}
