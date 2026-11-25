package api_test

import (
	"fmt"
	"net/http"
	"net/url"
	"testing"
)

// TestLimiteBorneeSurLaRecherche : la limite par défaut s'applique quand le paramètre est absent, les
// bornes 1 et 200 passent, tout le reste est une saisie invalide (400).
func TestLimiteBorneeSurLaRecherche(t *testing.T) {
	serveur := bancTrajets(t)

	cas := map[string]struct {
		requete string
		statut  int
	}{
		"sans paramètre": {"", http.StatusOK},
		"un":             {"?limit=1", http.StatusOK},
		"maximum":        {"?limit=200", http.StatusOK},
		"zéro":           {"?limit=0", http.StatusBadRequest},
		"au-dessus":      {"?limit=201", http.StatusBadRequest},
		"pas un nombre":  {"?limit=abc", http.StatusBadRequest},
		"négatif":        {"?limit=-5", http.StatusBadRequest},
	}
	for nom, cas := range cas {
		t.Run(nom, func(t *testing.T) {
			statut, _, corps := getPageTrajets(t, serveur, "/rides"+cas.requete)
			if statut != cas.statut {
				t.Fatalf("statut = %d, attendu %d (corps %s)", statut, cas.statut, corps)
			}
		})
	}
}

// TestRecherchePagineeParCurseur : la page suivante reprend exactement la suite, et la dernière page
// rend un curseur nul.
func TestRecherchePagineeParCurseur(t *testing.T) {
	serveur := bancTrajets(t)
	for jour := 1; jour <= 3; jour++ {
		corps := fmt.Sprintf(`{"origin":"Lyon","destination":"Grenoble",`+
			`"departure_time":"2026-12-0%dT08:30:00Z","seats_total":3,"price":12.5}`, jour)
		if statut, _ := posterTrajet(t, serveur, "bob", corps); statut != http.StatusCreated {
			t.Fatalf("publication du jour %d = %d, attendu 201", jour, statut)
		}
	}

	statut, premiere, corps := getPageTrajets(t, serveur, "/rides?limit=2")
	if statut != http.StatusOK {
		t.Fatalf("page 1 = %d, attendu 200 (corps %s)", statut, corps)
	}
	if len(premiere.Trajets) != 2 {
		t.Fatalf("page 1 = %d trajets, attendu 2 (corps %s)", len(premiere.Trajets), corps)
	}
	if premiere.CurseurSuivant == nil || *premiere.CurseurSuivant == "" {
		t.Fatalf("page 1 : curseur suivant absent (corps %s)", corps)
	}

	statut, seconde, corps := getPageTrajets(t, serveur,
		"/rides?limit=2&after="+url.QueryEscape(*premiere.CurseurSuivant))
	if statut != http.StatusOK {
		t.Fatalf("page 2 = %d, attendu 200 (corps %s)", statut, corps)
	}
	if len(seconde.Trajets) != 1 {
		t.Fatalf("page 2 = %d trajets, attendu 1 (corps %s)", len(seconde.Trajets), corps)
	}
	if seconde.CurseurSuivant != nil {
		t.Errorf("page 2 : curseur suivant = %q, attendu null", *seconde.CurseurSuivant)
	}

	vu := map[any]bool{}
	for _, trajet := range append(premiere.Trajets, seconde.Trajets...) {
		if vu[trajet["id"]] {
			t.Errorf("trajet %v rendu deux fois sur les deux pages", trajet["id"])
		}
		vu[trajet["id"]] = true
	}
	if len(vu) != 3 {
		t.Errorf("trajets distincts vus = %d, attendu 3", len(vu))
	}
}

// TestCurseurIllisibleRendQuatreCent : un curseur abîmé est une saisie invalide, pas une panne serveur.
func TestCurseurIllisibleRendQuatreCent(t *testing.T) {
	serveur := bancTrajets(t)

	for nom, curseur := range map[string]string{
		"pas du base64":   "%%%",
		"sans séparateur": "YWJjZGVm",
		"date illisible":  "bmimportequoifQ",
	} {
		t.Run(nom, func(t *testing.T) {
			statut, _, corps := getPageTrajets(t, serveur, "/rides?after="+url.QueryEscape(curseur))
			if statut != http.StatusBadRequest {
				t.Fatalf("statut = %d, attendu 400 (corps %s)", statut, corps)
			}
		})
	}
}
