package api_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/alaa-ilehi/wasalni/internal/api"
	"github.com/alaa-ilehi/wasalni/internal/application"
	"github.com/alaa-ilehi/wasalni/internal/domain"
)

// profilsEnMemoire : port ProfilRepository en mémoire pour les cas HTTP sans base.
type profilsEnMemoire map[string]domain.Profil

func (p profilsEnMemoire) Enregistrer(_ context.Context, profil domain.Profil) (domain.Profil, bool, error) {
	_, existe := p[profil.Identifiant]
	p[profil.Identifiant] = profil
	return profil, !existe, nil
}

func (p profilsEnMemoire) ParIdentifiant(_ context.Context, identifiant string) (domain.Profil, error) {
	profil, ok := p[identifiant]
	if !ok {
		return domain.Profil{}, domain.ErrProfilIntrouvable
	}
	return profil, nil
}

func getTrajet(t *testing.T, serveur *httptest.Server, identifiant, acteur string) (int, map[string]any) {
	t.Helper()
	if acteur == "" {
		acteur = "lecteur"
	}
	requete, err := http.NewRequest(http.MethodGet, serveur.URL+"/rides/"+identifiant, nil)
	if err != nil {
		t.Fatalf("requête : %v", err)
	}
	if acteur != "" {
		requete.Header.Set(api.EnTeteAutorisation, autorisationDe(t, acteur))
	}
	reponse, err := http.DefaultClient.Do(requete)
	if err != nil {
		t.Fatalf("appel : %v", err)
	}
	defer reponse.Body.Close()
	var corps map[string]any
	if err := json.NewDecoder(reponse.Body).Decode(&corps); err != nil {
		t.Fatalf("décodage de la réponse : %v", err)
	}
	return reponse.StatusCode, corps
}

func putProfil(t *testing.T, serveur *httptest.Server, acteur, corps string) (int, map[string]any) {
	t.Helper()
	requete, err := http.NewRequest(http.MethodPut, serveur.URL+"/profile", strings.NewReader(corps))
	if err != nil {
		t.Fatalf("requête : %v", err)
	}
	requete.Header.Set("Content-Type", "application/json")
	requete.Header.Set(api.EnTeteAutorisation, autorisationDe(t, acteur))
	reponse, err := http.DefaultClient.Do(requete)
	if err != nil {
		t.Fatalf("appel : %v", err)
	}
	defer reponse.Body.Close()
	var sortie map[string]any
	if err := json.NewDecoder(reponse.Body).Decode(&sortie); err != nil {
		t.Fatalf("décodage de la réponse : %v", err)
	}
	return reponse.StatusCode, sortie
}

func TestConsulterTrajetSansIdentiteNiProfil(t *testing.T) {
	serveur := bancTrajets(t)
	statut, publie := posterTrajet(t, serveur, "alice", corpsTrajetValide())
	if statut != http.StatusCreated {
		t.Fatalf("publication : statut = %d, corps = %v", statut, publie)
	}
	identifiant, _ := publie["id"].(string)

	statut, corps := getTrajet(t, serveur, identifiant, "")
	if statut != http.StatusOK {
		t.Fatalf("statut = %d, attendu 200 (corps %v)", statut, corps)
	}
	if corps["id"] != identifiant || corps["origin"] != "Lyon" || corps["destination"] != "Grenoble" {
		t.Errorf("trajet = %v", corps)
	}
	if corps["departure_time"] != "2026-12-01T08:30:00Z" {
		t.Errorf("départ = %v", corps["departure_time"])
	}
	if corps["driver"] != nil {
		t.Errorf("conducteur = %v, attendu nil sans profil UC8", corps["driver"])
	}
}

func TestConsulterTrajetRenvoieLeConducteurEtUneNoteNeutre(t *testing.T) {
	serveur := bancTrajets(t)
	statut, _ := putProfil(t, serveur, "alice", `{"name":"Alice Ben Salah","age":29,"sex":"FEMALE"}`)
	if statut != http.StatusCreated {
		t.Fatalf("création du profil : statut = %d", statut)
	}
	statut, publie := posterTrajet(t, serveur, "alice", corpsTrajetValide())
	if statut != http.StatusCreated {
		t.Fatalf("publication : statut = %d", statut)
	}
	identifiant, _ := publie["id"].(string)

	statut, corps := getTrajet(t, serveur, identifiant, "")
	if statut != http.StatusOK {
		t.Fatalf("statut = %d, attendu 200 (corps %v)", statut, corps)
	}
	conducteur, ok := corps["driver"].(map[string]any)
	if !ok {
		t.Fatalf("conducteur = %v, attendu un objet", corps["driver"])
	}
	if conducteur["name"] != "Alice Ben Salah" || conducteur["sex"] != "FEMALE" || conducteur["age"] != float64(29) {
		t.Errorf("conducteur = %v", conducteur)
	}
	note, present := conducteur["rating"]
	if !present || note != nil {
		t.Errorf("note = %v (présent : %v), attendu null sans évaluation", note, present)
	}
}

func TestConsulterTrajetIntrouvableOuTermineRenvoie404(t *testing.T) {
	serveurInconnu := bancTrajets(t)
	statut, corps := getTrajet(t, serveurInconnu, "inconnu", "")
	if statut != http.StatusNotFound {
		t.Fatalf("trajet inconnu : statut = %d, attendu 404 (corps %v)", statut, corps)
	}
	if _, ok := corps["error"].(string); !ok {
		t.Errorf("corps d'erreur = %v", corps)
	}

	serveurTermine := httptest.NewServer(api.Routes(api.Deps{
		Authentification: identitePartagee.auth,
		Details: application.NouveauConsulterTrajet(
			trajetsEnMemoire{"t1": {Identifiant: "t1", Conducteur: "alice", Statut: domain.TrajetTermine}},
			profilsEnMemoire{},
			evaluationsEnMemoire{},
		),
	}))
	defer serveurTermine.Close()
	statut, corps = getTrajet(t, serveurTermine, "t1", "")
	if statut != http.StatusNotFound {
		t.Fatalf("trajet terminé : statut = %d, attendu 404 (corps %v)", statut, corps)
	}
}
