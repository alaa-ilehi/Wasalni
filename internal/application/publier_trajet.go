package application

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/alaa-ilehi/wasalni/internal/domain"
)

// Horloge : port du temps, pour que le cas d'usage reste testable sans dormir.
type Horloge interface {
	Maintenant() time.Time
}

// SaisieTrajet est le corps accepté par POST /rides : les clés JSON sont en anglais,
// les noms Go en français, comme le reste du service.
//
// DelaiAnnulation est un pointeur pour distinguer l'absence (défaut du domaine) de 0.
// DepartSecours accepte l'alias « departure » de la fiche UC1.
type SaisieTrajet struct {
	Origine         string   `json:"origin"`
	Destination     string   `json:"destination"`
	Depart          string   `json:"departure_time"`
	DepartSecours   string   `json:"departure,omitempty"`
	PlacesTotal     int      `json:"seats_total"`
	Prix            float64  `json:"price"`
	DelaiAnnulation *float64 `json:"cancel_delay_hours,omitempty"`
}

// PublierTrajet : UC1, un conducteur identifié publie un trajet.
type PublierTrajet struct {
	trajets domain.TrajetRepository
	horloge Horloge
}

func NouveauPublierTrajet(trajets domain.TrajetRepository, horloge Horloge) PublierTrajet {
	return PublierTrajet{trajets: trajets, horloge: horloge}
}

// Executer transforme la saisie en trajet publiable, applique les règles de UC1, puis écrit.
// Les places disponibles partent au maximum : leur décrément à la réservation est différé (ADR 0004).
func (uc PublierTrajet) Executer(ctx context.Context, conducteur string, saisie SaisieTrajet) (domain.Trajet, error) {
	if conducteur == "" {
		return domain.Trajet{}, ErrIdentiteManquante
	}
	if uc.horloge == nil {
		return domain.Trajet{}, fmt.Errorf("%w : horloge absente du cas d'usage", domain.ErrTrajetInvalide)
	}

	brutDepart := saisie.Depart
	if strings.TrimSpace(brutDepart) == "" {
		brutDepart = saisie.DepartSecours
	}
	depart, err := instant(brutDepart)
	if err != nil {
		return domain.Trajet{}, err
	}

	delai := domain.DureeDelaiAnnulationParDefaut
	if saisie.DelaiAnnulation != nil {
		delai = heures(*saisie.DelaiAnnulation)
	}

	trajet := domain.Trajet{
		Conducteur:      conducteur,
		Origine:         strings.TrimSpace(saisie.Origine),
		Destination:     strings.TrimSpace(saisie.Destination),
		Depart:          depart,
		PlacesTotal:     saisie.PlacesTotal,
		Places:          saisie.PlacesTotal,
		Prix:            saisie.Prix,
		DelaiAnnulation: delai,
		Statut:          domain.TrajetPublie,
	}
	if err := trajet.ValiderPublication(uc.horloge.Maintenant()); err != nil {
		return domain.Trajet{}, err
	}

	publie, err := uc.trajets.Publier(ctx, trajet)
	if err != nil {
		return domain.Trajet{}, err
	}
	return publie, nil
}

// instant lit un horodatage ISO 8601 ; une date sans fuseau est lue en UTC.
func instant(valeur string) (time.Time, error) {
	texte := strings.TrimSpace(valeur)
	if texte == "" {
		return time.Time{}, fmt.Errorf("%w : le départ est obligatoire", domain.ErrTrajetInvalide)
	}
	for _, disposition := range []string{time.RFC3339, "2006-01-02T15:04:05"} {
		if lu, err := time.Parse(disposition, texte); err == nil {
			return lu.UTC(), nil
		}
	}
	return time.Time{}, fmt.Errorf("%w : le départ doit être une date ISO 8601", domain.ErrTrajetInvalide)
}

// heures convertit un nombre d'heures décimal en durée.
func heures(valeur float64) time.Duration {
	return time.Duration(valeur * float64(time.Hour))
}
