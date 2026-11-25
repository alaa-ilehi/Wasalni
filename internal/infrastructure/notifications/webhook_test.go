package notifications

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/alaa-ilehi/wasalni/internal/domain"
)

// capturer : un récepteur qui enregistre la requête et répond le statut demandé.
func capturer(t *testing.T, statut int, corps *string) *httptest.Server {
	t.Helper()
	serveur := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("méthode = %s, attendu POST", r.Method)
		}
		if entete := r.Header.Get("Content-Type"); entete != "application/json" {
			t.Errorf("Content-Type = %q, attendu application/json", entete)
		}
		lus, err := io.ReadAll(r.Body)
		if err != nil {
			t.Errorf("lecture du corps : %v", err)
		}
		*corps = string(lus)
		w.WriteHeader(statut)
	}))
	t.Cleanup(serveur.Close)
	return serveur
}

func TestChaqueEvenementEstLivre(t *testing.T) {
	reservation := domain.Reservation{Identifiant: "r1", Trajet: "t1", Passager: "alice"}
	trajet := domain.Trajet{Identifiant: "t1", Conducteur: "bob"}

	cas := map[string]struct {
		appeler func(*Webhook) error
		attendu string
	}{
		"réservation acceptée": {
			appeler: func(w *Webhook) error { return w.NotifierReservation(context.Background(), reservation) },
			attendu: `{"event":"reservation_accepted","reservation":"r1","ride":"t1","passenger":"alice"}`,
		},
		"réservation annulée": {
			appeler: func(w *Webhook) error { return w.NotifierAnnulation(context.Background(), reservation) },
			attendu: `{"event":"reservation_cancelled","reservation":"r1","ride":"t1","passenger":"alice"}`,
		},
		"trajet terminé": {
			appeler: func(w *Webhook) error { return w.NotifierFinTrajet(context.Background(), trajet) },
			attendu: `{"event":"ride_completed","ride":"t1","driver":"bob"}`,
		},
		"trajet modifié": {
			appeler: func(w *Webhook) error { return w.NotifierModificationTrajet(context.Background(), trajet) },
			attendu: `{"event":"ride_modified","ride":"t1","driver":"bob"}`,
		},
		"trajet supprimé": {
			appeler: func(w *Webhook) error { return w.NotifierSuppressionTrajet(context.Background(), trajet) },
			attendu: `{"event":"ride_deleted","ride":"t1","driver":"bob"}`,
		},
	}

	for nom, cas := range cas {
		t.Run(nom, func(t *testing.T) {
			var corps string
			serveur := capturer(t, http.StatusNoContent, &corps)
			webhook := NouveauWebhook(serveur.URL)

			if err := cas.appeler(webhook); err != nil {
				t.Fatalf("livraison : %v, attendu nil", err)
			}
			if corps != cas.attendu {
				t.Errorf("corps = %s, attendu %s", corps, cas.attendu)
			}
		})
	}
}

func TestUnStatutNonDeuxCentRendUneErreur(t *testing.T) {
	for _, statut := range []int{http.StatusInternalServerError, http.StatusBadGateway, http.StatusNotFound} {
		var corps string
		serveur := capturer(t, statut, &corps)
		if err := NouveauWebhook(serveur.URL).NotifierFinTrajet(context.Background(), domain.Trajet{Identifiant: "t1"}); err == nil {
			t.Fatalf("statut %d : erreur nil, attendu une erreur", statut)
		}
	}
}

func TestUneCibleInjoignableRendUneErreur(t *testing.T) {
	serveur := httptest.NewServer(http.NotFoundHandler())
	url := serveur.URL
	serveur.Close()

	err := NouveauWebhook(url).NotifierFinTrajet(context.Background(), domain.Trajet{Identifiant: "t1"})
	if err == nil {
		t.Fatal("connexion refusée : erreur nil, attendu une erreur")
	}
}

// TestLeDelaiBorneLaLivraison : une cible qui ne répond pas fait échouer l'appel au délai du client, pas au-delà.
func TestLeDelaiBorneLaLivraison(t *testing.T) {
	bloque := make(chan struct{})
	serveur := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		<-bloque
	}))
	defer serveur.Close()
	defer close(bloque)

	webhook := &Webhook{url: serveur.URL, client: &http.Client{Timeout: 50 * time.Millisecond}}
	debut := time.Now()
	err := webhook.NotifierFinTrajet(context.Background(), domain.Trajet{Identifiant: "t1"})
	ecoule := time.Since(debut)

	if err == nil {
		t.Fatal("délai dépassé : erreur nil, attendu une erreur")
	}
	if ecoule > 2*time.Second {
		t.Errorf("échec après %s, attendu au plus le délai du client", ecoule)
	}
	if !strings.Contains(err.Error(), "livraison") {
		t.Errorf("erreur = %v, attendu une erreur de livraison", err)
	}
}

// TestLeCorpsEstDuJSONValide : l'événement est relu comme un objet JSON, pas comme une chaîne.
func TestLeCorpsEstDuJSONValide(t *testing.T) {
	var corps string
	serveur := capturer(t, http.StatusOK, &corps)

	if err := NouveauWebhook(serveur.URL).NotifierReservation(context.Background(),
		domain.Reservation{Identifiant: "r1", Trajet: "t1", Passager: "alice"}); err != nil {
		t.Fatalf("livraison : %v", err)
	}
	var recu map[string]string
	if err := json.Unmarshal([]byte(corps), &recu); err != nil {
		t.Fatalf("corps %q n'est pas un objet JSON : %v", corps, err)
	}
	if recu["event"] != "reservation_accepted" || recu["reservation"] != "r1" {
		t.Errorf("objet reçu = %v", recu)
	}
}
