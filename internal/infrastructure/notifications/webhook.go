package notifications

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/alaa-ilehi/wasalni/internal/domain"
)

// DelaiLivraison borne chaque livraison : une cible lente ne doit pas retenir la requête métier plus longtemps.
const DelaiLivraison = 5 * time.Second

// Webhook livre un événement par POST JSON vers l'URL configurée (ADR 0013). La sémantique reste celle d'ADR 0007 :
// un échec est rendu à l'appelant, qui le journalise sans annuler l'opération métier.
type Webhook struct {
	url    string
	client *http.Client
}

// NouveauWebhook : un adaptateur prêt à livrer vers url.
func NouveauWebhook(url string) *Webhook {
	return &Webhook{url: url, client: &http.Client{Timeout: DelaiLivraison}}
}

// evenement est le corps envoyé : le nom de l'événement et les identifiants du journal.
type evenement struct {
	Evenement   string `json:"event"`
	Reservation string `json:"reservation,omitempty"`
	Trajet      string `json:"ride,omitempty"`
	Passager    string `json:"passenger,omitempty"`
	Conducteur  string `json:"driver,omitempty"`
}

func (w *Webhook) NotifierReservation(ctx context.Context, reservation domain.Reservation) error {
	return w.livrer(ctx, evenement{
		Evenement:   "reservation_accepted",
		Reservation: reservation.Identifiant,
		Trajet:      reservation.Trajet,
		Passager:    reservation.Passager,
	})
}

func (w *Webhook) NotifierAnnulation(ctx context.Context, reservation domain.Reservation) error {
	return w.livrer(ctx, evenement{
		Evenement:   "reservation_cancelled",
		Reservation: reservation.Identifiant,
		Trajet:      reservation.Trajet,
		Passager:    reservation.Passager,
	})
}

func (w *Webhook) NotifierFinTrajet(ctx context.Context, trajet domain.Trajet) error {
	return w.livrer(ctx, evenement{
		Evenement:  "ride_completed",
		Trajet:     trajet.Identifiant,
		Conducteur: trajet.Conducteur,
	})
}

func (w *Webhook) NotifierModificationTrajet(ctx context.Context, trajet domain.Trajet) error {
	return w.livrer(ctx, evenement{
		Evenement:  "ride_modified",
		Trajet:     trajet.Identifiant,
		Conducteur: trajet.Conducteur,
	})
}

func (w *Webhook) NotifierSuppressionTrajet(ctx context.Context, trajet domain.Trajet) error {
	return w.livrer(ctx, evenement{
		Evenement:  "ride_deleted",
		Trajet:     trajet.Identifiant,
		Conducteur: trajet.Conducteur,
	})
}

// livrer : POST JSON borné ; 2xx rend nil, tout le reste une erreur que l'appelant journalise.
func (w *Webhook) livrer(ctx context.Context, e evenement) error {
	corps, err := json.Marshal(e)
	if err != nil {
		return fmt.Errorf("encodage de la notification : %w", err)
	}
	requete, err := http.NewRequestWithContext(ctx, http.MethodPost, w.url, bytes.NewReader(corps))
	if err != nil {
		return fmt.Errorf("préparation de la notification : %w", err)
	}
	requete.Header.Set("Content-Type", "application/json")

	reponse, err := w.client.Do(requete)
	if err != nil {
		return fmt.Errorf("livraison de la notification : %w", err)
	}
	defer reponse.Body.Close()
	// Le corps est lu même en succès : la connexion reste réutilisable.
	_, _ = io.Copy(io.Discard, reponse.Body)

	if reponse.StatusCode < http.StatusOK || reponse.StatusCode >= http.StatusMultipleChoices {
		return fmt.Errorf("livraison de la notification : statut %d", reponse.StatusCode)
	}
	return nil
}
