package notifications_test

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"testing"

	"github.com/alaa-ilehi/wasalni/internal/application"
	"github.com/alaa-ilehi/wasalni/internal/domain"
	"github.com/alaa-ilehi/wasalni/internal/infrastructure/notifications"
)

// Conformité aux ports : ajouter une méthode à un port casse la compilation ici plutôt qu'en production.
var (
	_ application.Notificateur              = (*notifications.Journal)(nil)
	_ application.NotificateurAnnulation    = (*notifications.Journal)(nil)
	_ application.NotificateurGestionTrajet = (*notifications.Journal)(nil)
	_ application.NotificateurFinTrajet     = (*notifications.Journal)(nil)
)

// journalDeTest : un journal bavard dans un tampon, pour relire ce qui a été inscrit.
func journalDeTest(t *testing.T) (*notifications.Journal, *bytes.Buffer) {
	t.Helper()
	tampon := &bytes.Buffer{}
	return notifications.NouveauJournal(slog.New(slog.NewJSONHandler(tampon, nil))), tampon
}

// derniereLigne : la seule ligne JSON inscrite, décodée.
func derniereLigne(t *testing.T, tampon *bytes.Buffer) map[string]any {
	t.Helper()
	var ligne map[string]any
	if err := json.Unmarshal(bytes.TrimSpace(tampon.Bytes()), &ligne); err != nil {
		t.Fatalf("décodage du journal %q : %v", tampon.String(), err)
	}
	return ligne
}

func TestLeJournalInscritChaqueEvenement(t *testing.T) {
	reservation := domain.Reservation{Identifiant: "r1", Trajet: "t1", Passager: "alice"}
	trajet := domain.Trajet{Identifiant: "t1", Conducteur: "bob"}

	cas := map[string]struct {
		appeler   func(*notifications.Journal) error
		message   string
		attributs map[string]string
	}{
		"réservation confirmée": {
			appeler:   func(j *notifications.Journal) error { return j.NotifierReservation(context.Background(), reservation) },
			message:   "réservation confirmée au passager",
			attributs: map[string]string{"reservation": "r1", "trajet": "t1", "passager": "alice"},
		},
		"réservation annulée": {
			appeler:   func(j *notifications.Journal) error { return j.NotifierAnnulation(context.Background(), reservation) },
			message:   "réservation annulée par le passager",
			attributs: map[string]string{"reservation": "r1", "trajet": "t1", "passager": "alice"},
		},
		"trajet terminé": {
			appeler:   func(j *notifications.Journal) error { return j.NotifierFinTrajet(context.Background(), trajet) },
			message:   "trajet terminé; notification aux passagers à transmettre par UC12",
			attributs: map[string]string{"trajet": "t1", "conducteur": "bob"},
		},
		"trajet modifié": {
			appeler: func(j *notifications.Journal) error {
				return j.NotifierModificationTrajet(context.Background(), trajet)
			},
			message:   "trajet modifié; notification aux passagers à transmettre par UC12",
			attributs: map[string]string{"trajet": "t1", "conducteur": "bob"},
		},
		"trajet supprimé": {
			appeler:   func(j *notifications.Journal) error { return j.NotifierSuppressionTrajet(context.Background(), trajet) },
			message:   "trajet supprimé; notification aux passagers à transmettre par UC12",
			attributs: map[string]string{"trajet": "t1", "conducteur": "bob"},
		},
	}

	for nom, cas := range cas {
		t.Run(nom, func(t *testing.T) {
			journal, tampon := journalDeTest(t)
			if err := cas.appeler(journal); err != nil {
				t.Fatalf("notification : %v, attendu nil (ADR 0007 : au mieux)", err)
			}

			ligne := derniereLigne(t, tampon)
			if ligne["level"] != "INFO" {
				t.Errorf("niveau = %v, attendu INFO", ligne["level"])
			}
			if ligne["msg"] != cas.message {
				t.Errorf("message = %q, attendu %q", ligne["msg"], cas.message)
			}
			for attribut, attendu := range cas.attributs {
				if ligne[attribut] != attendu {
					t.Errorf("attribut %s = %v, attendu %q", attribut, ligne[attribut], attendu)
				}
			}
		})
	}
}

// TestUnJournalSansJournaliseSansPanne : NouveauJournal(nil) retombe sur le journal par défaut.
func TestUnJournalSansJournaliseSansPanne(t *testing.T) {
	journal := notifications.NouveauJournal(nil)
	if err := journal.NotifierFinTrajet(context.Background(), domain.Trajet{Identifiant: "t1", Conducteur: "bob"}); err != nil {
		t.Fatalf("notification : %v, attendu nil", err)
	}
}
