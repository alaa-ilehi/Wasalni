package api_test

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/alaa-ilehi/wasalni/internal/api"
)

func getTrajets(t *testing.T, serveur *httptest.Server, chemin string) (int, []map[string]any, string) {
	t.Helper()
	requete, err := http.NewRequest(http.MethodGet, serveur.URL+chemin, nil)
	if err != nil {
		t.Fatalf("requête : %v", err)
	}
	requete.Header.Set(api.EnTeteAutorisation, autorisationDe(t, "lecteur"))
	reponse, err := http.DefaultClient.Do(requete)
	if err != nil {
		t.Fatalf("appel : %v", err)
	}
	defer reponse.Body.Close()
	corps, err := io.ReadAll(reponse.Body)
	if err != nil {
		t.Fatalf("lecture de la réponse : %v", err)
	}
	var trajets []map[string]any
	if err := json.Unmarshal(corps, &trajets); err != nil {
		t.Fatalf("décodage de la réponse %q : %v", corps, err)
	}
	return reponse.StatusCode, trajets, string(corps)
}

func TestRechercherTrajetsSansResultatRenvoieListeVide(t *testing.T) {
	serveur := bancTrajets(t)

	statut, trajets, corps := getTrajets(t, serveur, "/rides")
	if statut != http.StatusOK {
		t.Fatalf("statut = %d, attendu 200 (corps %s)", statut, corps)
	}
	if trajets == nil || len(trajets) != 0 {
		t.Errorf("trajets = %v, attendu une liste vide (corps %s)", trajets, corps)
	}
	if strings.TrimSpace(corps) != "[]" {
		t.Errorf("corps = %q, attendu []", corps)
	}
}

func TestRechercherTrajetsFiltreEtTrie(t *testing.T) {
	serveur := bancTrajets(t)

	statut, lyon := posterTrajet(t, serveur, "bob", corpsTrajetValide())
	if statut != http.StatusCreated {
		t.Fatalf("publication Lyon = %d, attendu 201", statut)
	}
	statut, paris := posterTrajet(t, serveur, "alice",
		`{"origin":"Paris","destination":"Lille","departure_time":"2026-12-02T09:00:00Z","seats_total":2,"price":8}`)
	if statut != http.StatusCreated {
		t.Fatalf("publication Paris = %d, attendu 201", statut)
	}

	statut, trajets, corps := getTrajets(t, serveur, "/rides")
	if statut != http.StatusOK {
		t.Fatalf("statut = %d, attendu 200 (corps %s)", statut, corps)
	}
	if len(trajets) != 2 {
		t.Fatalf("trajets = %d, attendu 2 (corps %s)", len(trajets), corps)
	}
	if trajets[0]["id"] != lyon["id"] || trajets[1]["id"] != paris["id"] {
		t.Errorf("tri par départ incorrect : %v", trajets)
	}
	if trajets[0]["seats_available"] != float64(3) || trajets[0]["price"] != 12.5 || trajets[0]["driver_id"] != "bob" {
		t.Errorf("places/prix/conducteur manquants dans %v", trajets[0])
	}

	statut, trajets, corps = getTrajets(t, serveur, "/rides?origin=lyon&destination=GRENOBLE&date=2026-12-01")
	if statut != http.StatusOK || len(trajets) != 1 || trajets[0]["id"] != lyon["id"] {
		t.Fatalf("recherche filtrée = %d %v (corps %s)", statut, trajets, corps)
	}

	statut, trajets, corps = getTrajets(t, serveur, "/rides?date=2026-12-02")
	if statut != http.StatusOK || len(trajets) != 1 || trajets[0]["id"] != paris["id"] {
		t.Fatalf("recherche par date = %d %v (corps %s)", statut, trajets, corps)
	}
}

func TestRechercherTrajetsRefuseUneDateInvalide(t *testing.T) {
	serveur := bancTrajets(t)

	requete, err := http.NewRequest(http.MethodGet, serveur.URL+"/rides?date=hier", nil)
	if err != nil {
		t.Fatalf("requête : %v", err)
	}
	requete.Header.Set(api.EnTeteAutorisation, autorisationDe(t, "lecteur"))
	reponse, err := http.DefaultClient.Do(requete)
	if err != nil {
		t.Fatalf("appel : %v", err)
	}
	defer reponse.Body.Close()
	var erreur map[string]any
	if err := json.NewDecoder(reponse.Body).Decode(&erreur); err != nil {
		t.Fatalf("décodage : %v", err)
	}
	if reponse.StatusCode != http.StatusBadRequest {
		t.Fatalf("statut = %d, attendu 400", reponse.StatusCode)
	}
	message, _ := erreur["error"].(string)
	if !strings.Contains(message, "date") {
		t.Errorf("erreur = %q, attendu un message sur la date", message)
	}
}

func TestPublierPuisRechercherPuisReserver(t *testing.T) {
	serveur := bancTrajets(t)

	statut, publie := posterTrajet(t, serveur, "bob", corpsTrajetValide())
	if statut != http.StatusCreated {
		t.Fatalf("publication = %d, attendu 201", statut)
	}
	identifiant, _ := publie["id"].(string)

	statut, trajets, corps := getTrajets(t, serveur, "/rides?origin=Lyon")
	if statut != http.StatusOK || len(trajets) != 1 || trajets[0]["id"] != identifiant {
		t.Fatalf("recherche = %d %v (corps %s)", statut, trajets, corps)
	}

	statutReservation, reservation := postReservation(t, serveur, identifiant, "alice")
	if statutReservation != http.StatusCreated {
		t.Fatalf("réservation = %d, attendu 201 (corps %v)", statutReservation, reservation)
	}
	if reservation["ride_id"] != identifiant {
		t.Errorf("ride_id = %v, attendu %q", reservation["ride_id"], identifiant)
	}
}
