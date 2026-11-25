package api_test

import (
	"context"
	"encoding/json"
	"errors"
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

type notificateurFinTrajetTest struct {
	err    error
	appels int
}

func (n *notificateurFinTrajetTest) NotifierFinTrajet(_ context.Context, _ domain.Trajet) error {
	n.appels++
	return n.err
}

func TestTerminerTrajetHTTPSQLite(t *testing.T) {
	ctx := context.Background()
	db, err := sqlite.Ouvrir(ctx, filepath.Join(t.TempDir(), "terminer.db"))
	if err != nil {
		t.Fatalf("ouvrir SQLite : %v", err)
	}
	defer db.Close()
	depot := sqlite.NouveauTrajetRepository(db)
	notif := &notificateurFinTrajetTest{err: errors.New("UC12 indisponible")}
	uc := application.NouveauTerminerTrajet(depot, notif)
	serveur := httptest.NewServer(api.Routes(api.Deps{Authentification: identitePartagee.auth, Terminaisons: uc}))
	defer serveur.Close()

	creer := func() domain.Trajet {
		t.Helper()
		trajet, err := depot.Publier(ctx, domain.Trajet{
			Conducteur: "alice", Origine: "Lyon", Destination: "Grenoble",
			PlacesTotal: 2, Places: 2, Statut: domain.TrajetPublie,
		})
		if err != nil {
			t.Fatalf("publier le trajet de test : %v", err)
		}
		return trajet
	}
	appel := func(id, acteur string) (int, map[string]any) {
		t.Helper()
		requete, err := http.NewRequest(http.MethodPost, serveur.URL+"/rides/"+id+"/complete", strings.NewReader(""))
		if err != nil {
			t.Fatalf("créer la requête : %v", err)
		}
		if acteur != "" {
			requete.Header.Set(api.EnTeteAutorisation, autorisationDe(t, acteur))
		}
		reponse, err := http.DefaultClient.Do(requete)
		if err != nil {
			t.Fatalf("appeler l'API : %v", err)
		}
		defer reponse.Body.Close()
		var corps map[string]any
		if reponse.StatusCode == http.StatusOK {
			if err := json.NewDecoder(reponse.Body).Decode(&corps); err != nil {
				t.Fatalf("décoder le trajet terminé : %v", err)
			}
		}
		return reponse.StatusCode, corps
	}

	t.Run("identité absente", func(t *testing.T) {
		trajet := creer()
		if statut, _ := appel(trajet.Identifiant, ""); statut != http.StatusUnauthorized {
			t.Fatalf("statut = %d, attendu 401", statut)
		}
		relu, err := depot.ParIdentifiant(ctx, trajet.Identifiant)
		if err != nil || relu.Statut != domain.TrajetPublie {
			t.Fatalf("trajet après refus = %+v, erreur = %v", relu, err)
		}
	})

	t.Run("inconnu ou non propriétaire", func(t *testing.T) {
		if statut, _ := appel("inconnu", "alice"); statut != http.StatusNotFound {
			t.Fatalf("trajet inconnu : statut = %d, attendu 404", statut)
		}
		trajet := creer()
		if statut, _ := appel(trajet.Identifiant, "bob"); statut != http.StatusForbidden {
			t.Fatalf("autre conducteur : statut = %d, attendu 403", statut)
		}
	})

	t.Run("termine une seule fois", func(t *testing.T) {
		trajet := creer()
		statut, corps := appel(trajet.Identifiant, "alice")
		if statut != http.StatusOK || corps["status"] != string(domain.TrajetTermine) {
			t.Fatalf("statut = %d, corps = %v", statut, corps)
		}
		relu, err := depot.ParIdentifiant(ctx, trajet.Identifiant)
		if err != nil || relu.Statut != domain.TrajetTermine {
			t.Fatalf("trajet stocké = %+v, erreur = %v", relu, err)
		}
		if statut, _ := appel(trajet.Identifiant, "alice"); statut != http.StatusConflict {
			t.Fatalf("seconde terminaison : statut = %d, attendu 409", statut)
		}
		if notif.appels != 1 {
			t.Fatalf("notifications = %d, attendu 1 (uniquement après le commit)", notif.appels)
		}
	})
}
