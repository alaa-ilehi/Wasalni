package api_test

import (
	"context"
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

// bancAuth : le routeur complet des profils et de l'authentification, sur une base temporaire et un
// annuaire d'identités en mémoire (le hachage réel est testé dans internal/infrastructure/passwords).
type bancAuth struct {
	serveur  *httptest.Server
	identite *identiteDeTest
}

func nouveauBancAuth(t *testing.T) *bancAuth {
	t.Helper()
	ctx := context.Background()
	db, err := sqlite.Ouvrir(ctx, filepath.Join(t.TempDir(), "auth.db"))
	if err != nil {
		t.Fatalf("ouverture de la base : %v", err)
	}
	t.Cleanup(func() { db.Close() })

	identite := nouvelleIdentite()
	serveur := httptest.NewServer(api.Routes(api.Deps{
		Profils:          application.NouveauGererProfil(sqlite.NouveauProfilRepository(db), adaptateurPhoto{}),
		Authentification: identite.auth,
	}))
	t.Cleanup(serveur.Close)
	return &bancAuth{serveur: serveur, identite: identite}
}

func (b *bancAuth) appel(t *testing.T, methode, chemin, autorisation, corps string) (int, map[string]any) {
	t.Helper()
	var lecteur io.Reader
	if corps != "" {
		lecteur = strings.NewReader(corps)
	}
	requete, err := http.NewRequest(methode, b.serveur.URL+chemin, lecteur)
	if err != nil {
		t.Fatalf("requête : %v", err)
	}
	if corps != "" {
		requete.Header.Set("Content-Type", "application/json")
	}
	if autorisation != "" {
		requete.Header.Set(api.EnTeteAutorisation, autorisation)
	}
	reponse, err := http.DefaultClient.Do(requete)
	if err != nil {
		t.Fatalf("appel %s %s : %v", methode, chemin, err)
	}
	defer reponse.Body.Close()
	brut, _ := io.ReadAll(reponse.Body)
	var charge map[string]any
	if len(brut) > 0 {
		if err := json.Unmarshal(brut, &charge); err != nil {
			t.Fatalf("réponse non JSON (%d) : %s", reponse.StatusCode, brut)
		}
	}
	return reponse.StatusCode, charge
}

func TestInscriptionRendUnJetonEtUnCompte(t *testing.T) {
	banc := nouveauBancAuth(t)
	statut, corps := banc.appel(t, http.MethodPost, "/auth/register", "", `{"id":"amine","password":"motdepasse"}`)
	if statut != http.StatusCreated {
		t.Fatalf("statut = %d, attendu 201 (%v)", statut, corps)
	}
	if corps["id"] != "amine" {
		t.Errorf("id = %v, attendu amine", corps["id"])
	}
	jeton, _ := corps["token"].(string)
	if len(jeton) < 40 {
		t.Fatalf("jeton de %d caractères, attendu au moins 40", len(jeton))
	}

	// Le jeton rendu ouvre bien une route protégée.
	if statut, _ := banc.appel(t, http.MethodPut, "/profile", "Bearer "+jeton, `{"name":"Amine","age":24}`); statut != http.StatusCreated {
		t.Fatalf("PUT /profile avec le jeton : statut = %d, attendu 201", statut)
	}
}

func TestInscriptionRefuseUnDoublonEtUneSaisieInvalide(t *testing.T) {
	banc := nouveauBancAuth(t)
	if statut, corps := banc.appel(t, http.MethodPost, "/auth/register", "", `{"id":"amine","password":"motdepasse"}`); statut != http.StatusCreated {
		t.Fatalf("première inscription : %d (%v)", statut, corps)
	}
	statut, corps := banc.appel(t, http.MethodPost, "/auth/register", "", `{"id":"amine","password":"autrepass"}`)
	if statut != http.StatusConflict {
		t.Fatalf("doublon : statut = %d, attendu 409 (%v)", statut, corps)
	}

	statut, _ = banc.appel(t, http.MethodPost, "/auth/register", "", `{"id":"bob","password":"court"}`)
	if statut != http.StatusBadRequest {
		t.Fatalf("mot de passe court : statut = %d, attendu 400", statut)
	}
	statut, _ = banc.appel(t, http.MethodPost, "/auth/register", "", `{"id":"bob"`)
	if statut != http.StatusBadRequest {
		t.Fatalf("corps illisible : statut = %d, attendu 400", statut)
	}
	statut, _ = banc.appel(t, http.MethodPost, "/auth/register", "", `{"id":"bob","password":"motdepasse","role":"admin"}`)
	if statut != http.StatusBadRequest {
		t.Fatalf("champ inconnu : statut = %d, attendu 400", statut)
	}
}

func TestConnexionRendUnJetonEtRefuseSansDirePourquoi(t *testing.T) {
	banc := nouveauBancAuth(t)
	if statut, _ := banc.appel(t, http.MethodPost, "/auth/register", "", `{"id":"amine","password":"motdepasse"}`); statut != http.StatusCreated {
		t.Fatalf("inscription : statut = %d", statut)
	}

	statut, corps := banc.appel(t, http.MethodPost, "/auth/login", "", `{"id":"amine","password":"motdepasse"}`)
	if statut != http.StatusOK {
		t.Fatalf("connexion : statut = %d, attendu 200 (%v)", statut, corps)
	}
	if jeton, _ := corps["token"].(string); len(jeton) < 40 {
		t.Fatalf("jeton de connexion invalide : %v", corps["token"])
	}

	statut, mauvaisMotDePasse := banc.appel(t, http.MethodPost, "/auth/login", "", `{"id":"amine","password":"mauvaispass"}`)
	if statut != http.StatusUnauthorized {
		t.Fatalf("mauvais mot de passe : statut = %d, attendu 401", statut)
	}
	statut, identifiantInconnu := banc.appel(t, http.MethodPost, "/auth/login", "", `{"id":"inconnu","password":"motdepasse"}`)
	if statut != http.StatusUnauthorized {
		t.Fatalf("identifiant inconnu : statut = %d, attendu 401", statut)
	}
	if mauvaisMotDePasse["error"] != identifiantInconnu["error"] {
		t.Fatalf("les deux refus doivent être indiscernables : %v vs %v", mauvaisMotDePasse["error"], identifiantInconnu["error"])
	}
}

func TestDeconnexionInvalideLeJeton(t *testing.T) {
	banc := nouveauBancAuth(t)
	jeton := banc.identite.jeton(t, "amine")

	if statut, corps := banc.appel(t, http.MethodDelete, "/auth/session", "Bearer "+jeton, ""); statut != http.StatusNoContent {
		t.Fatalf("déconnexion : statut = %d, attendu 204 (%v)", statut, corps)
	}
	if statut, _ := banc.appel(t, http.MethodDelete, "/auth/session", "Bearer "+jeton, ""); statut != http.StatusUnauthorized {
		t.Fatalf("seconde déconnexion : statut = %d, attendu 401", statut)
	}
	if statut, _ := banc.appel(t, http.MethodPut, "/profile", "Bearer "+jeton, `{"name":"Amine","age":24}`); statut != http.StatusUnauthorized {
		t.Fatalf("route protégée après déconnexion : statut = %d, attendu 401", statut)
	}
	if statut, _ := banc.appel(t, http.MethodDelete, "/auth/session", "", ""); statut != http.StatusUnauthorized {
		t.Fatalf("déconnexion sans jeton : statut = %d, attendu 401", statut)
	}
}

// Toutes les routes protégées refusent une requête sans jeton : une route oubliée se verrait ici.
func TestToutesLesRoutesProtegeesRefusentSansJeton(t *testing.T) {
	banc := nouveauBancAuth(t)
	routes := []struct{ methode, chemin string }{
		{http.MethodGet, "/profile"},
		{http.MethodPut, "/profile"},
		{http.MethodGet, "/rides"},
		{http.MethodGet, "/rides/inconnu"},
		{http.MethodPost, "/rides"},
		{http.MethodPost, "/rides/inconnu/bookings"},
		{http.MethodPost, "/rides/inconnu/complete"},
		{http.MethodPut, "/rides/inconnu"},
		{http.MethodDelete, "/rides/inconnu"},
		{http.MethodDelete, "/bookings/inconnu"},
		{http.MethodPost, "/rides/inconnu/ratings"},
		{http.MethodDelete, "/auth/session"},
	}
	for _, route := range routes {
		t.Run(route.methode+" "+route.chemin, func(t *testing.T) {
			// Toutes les routes protégées, y compris la recherche et le détail d'un trajet : la seule
			// exception au jeton est /health et les trois routes /auth/* (ADR 0011).
			statut, corps := banc.appel(t, route.methode, route.chemin, "", "{}")
			if statut != http.StatusUnauthorized {
				t.Fatalf("statut = %d, attendu 401 pour %s %s (%v)", statut, route.methode, route.chemin, corps)
			}
		})
	}

	if statut, _ := banc.appel(t, http.MethodGet, "/health", "", ""); statut != http.StatusOK {
		t.Fatalf("GET /health : statut = %d, attendu 200 (route publique)", statut)
	}
}

// Deux sessions distinctes ne se voient pas : le jeton porte l'identité, pas le client.
func TestDeuxSessionsNePartagentPasLeProfil(t *testing.T) {
	banc := nouveauBancAuth(t)
	amine := banc.identite.jeton(t, "amine")
	bob := banc.identite.jeton(t, "bob")

	if statut, _ := banc.appel(t, http.MethodPut, "/profile", "Bearer "+amine, `{"name":"Amine","age":24}`); statut != http.StatusCreated {
		t.Fatalf("profil d'amine : statut = %d", statut)
	}
	statut, corps := banc.appel(t, http.MethodGet, "/profile", "Bearer "+bob, "")
	if statut != http.StatusNotFound {
		t.Fatalf("profil vu par bob : statut = %d, attendu 404 (%v)", statut, corps)
	}
}

// Un jeton expiré est refusé et sa session disparaît (même règle que l'adaptateur SQLite).
func TestJetonExpireRefuse(t *testing.T) {
	banc := nouveauBancAuth(t)
	jeton := banc.identite.jeton(t, "amine")
	if statut, _ := banc.appel(t, http.MethodPut, "/profile", "Bearer "+jeton, `{"name":"Amine","age":24}`); statut != http.StatusCreated {
		t.Fatalf("PUT /profile : statut = %d", statut)
	}

	banc.identite.annuaire.instant = banc.identite.annuaire.instant.Add(application.DureeSession)
	if statut, corps := banc.appel(t, http.MethodGet, "/profile", "Bearer "+jeton, ""); statut != http.StatusUnauthorized {
		t.Fatalf("statut = %d, attendu 401 après expiration (%v)", statut, corps)
	}
}

var _ = domain.ErrSessionInvalide
