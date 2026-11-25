package domain

import (
	"errors"
	"fmt"
	"strings"
	"time"
)

// StatutTrajet : états d'un trajet publiable et terminé par son conducteur (UC11).
type StatutTrajet string

const (
	TrajetPublie  StatutTrajet = "PUBLISHED"
	TrajetTermine StatutTrajet = "COMPLETED"
)

var (
	ErrTrajetNonProprietaire = errors.New("le trajet appartient à un autre conducteur")
	ErrTrajetDejaTermine     = errors.New("le trajet est déjà terminé")
	ErrPlacesInsuffisantes   = errors.New("le nombre de places est inférieur aux réservations actives")
	ErrTrajetSupprime        = errors.New("le trajet est supprimé")
)

// DureeDelaiAnnulationParDefaut : valeur retenue quand la publication n'en fournit pas (fiche UC1).
const DureeDelaiAnnulationParDefaut = 2 * time.Hour

// Trajet : un trajet publié par un conducteur (UC1), sur lequel un passager réserve une place (UC3).
//
// Places est le compteur encore disponible ; il vaut PlacesTotal à la publication et est mis à jour
// atomiquement avec chaque réservation.
type Trajet struct {
	Identifiant     string
	Conducteur      string
	Origine         string
	Destination     string
	Depart          time.Time
	PlacesTotal     int
	Places          int
	Prix            float64
	DelaiAnnulation time.Duration
	Statut          StatutTrajet
	Supprime        bool
	CreeLe          time.Time
}

// Valider refuse un trajet inexploitable pour une réservation : ce dont UC3 a besoin, rien de plus.
func (t Trajet) Valider() error {
	if t.Identifiant == "" || t.Conducteur == "" {
		return ErrTrajetInvalide
	}
	return nil
}

// ValiderTerminaison applique les règles UC11 après lecture du trajet.
func (t Trajet) ValiderTerminaison(conducteur string) error {
	if t.Conducteur != conducteur {
		return ErrTrajetNonProprietaire
	}
	if t.Statut == TrajetTermine {
		return ErrTrajetDejaTermine
	}
	if t.Statut != TrajetPublie {
		return ErrTrajetInvalide
	}
	return nil
}

// ValiderPublication applique les règles de UC1 au moment de la publication.
// L'horloge est fournie par l'application : le domaine ne lit pas l'heure système.
// Chaque refus enveloppe ErrTrajetInvalide, ce qui laisse un seul code HTTP côté interface.
func (t Trajet) ValiderPublication(maintenant time.Time) error {
	origine := strings.TrimSpace(t.Origine)
	destination := strings.TrimSpace(t.Destination)
	switch {
	case t.Conducteur == "":
		return fmt.Errorf("%w : le conducteur est obligatoire", ErrTrajetInvalide)
	case origine == "":
		return fmt.Errorf("%w : l'origine est obligatoire", ErrTrajetInvalide)
	case destination == "":
		return fmt.Errorf("%w : la destination est obligatoire", ErrTrajetInvalide)
	case strings.EqualFold(origine, destination):
		return fmt.Errorf("%w : l'origine et la destination doivent être différentes", ErrTrajetInvalide)
	case t.PlacesTotal < 1:
		return fmt.Errorf("%w : le nombre de places doit être au moins 1", ErrTrajetInvalide)
	case t.Places < 0 || t.Places > t.PlacesTotal:
		return fmt.Errorf("%w : les places disponibles doivent rester entre 0 et %d", ErrTrajetInvalide, t.PlacesTotal)
	case t.Prix < 0:
		return fmt.Errorf("%w : le prix ne peut pas être négatif", ErrTrajetInvalide)
	case t.DelaiAnnulation < 0:
		return fmt.Errorf("%w : le délai d'annulation ne peut pas être négatif", ErrTrajetInvalide)
	case !t.Depart.After(maintenant):
		return fmt.Errorf("%w : le départ doit être dans le futur", ErrTrajetInvalide)
	case t.DelaiAnnulation >= t.Depart.Sub(maintenant):
		return fmt.Errorf("%w : le délai d'annulation doit être inférieur au temps restant avant le départ", ErrTrajetInvalide)
	}
	switch t.Statut {
	case TrajetPublie, TrajetTermine:
	default:
		return fmt.Errorf("%w : statut de trajet inconnu %q", ErrTrajetInvalide, t.Statut)
	}
	return nil
}

// ResteAvantDepart : temps restant avant le départ, négatif si le départ est passé.
func (t Trajet) ResteAvantDepart(maintenant time.Time) time.Duration {
	return t.Depart.Sub(maintenant)
}

// CriteresRecherche : filtres facultatifs de UC2. Une chaîne vide signifie « aucun filtre ».
// Date est une date calendaire AAAA-MM-JJ, sans heure ni fuseau.
// Limite borne le nombre de résultats rendus (0 : la limite par défaut de l'adaptateur).
// Apres est le curseur opaque rendu par la page précédente ; sa forme ne regarde que l'adaptateur.
type CriteresRecherche struct {
	Origine     string
	Destination string
	Date        string
	Limite      int
	Apres       string
}

// ErrCurseurInvalide : le curseur de pagination est illisible ou mal formé.
var ErrCurseurInvalide = errors.New("curseur de pagination invalide")

// PageTrajets : une page de résultats UC2 et le curseur de la suivante.
// CurseurSuivant est vide quand la page est la dernière.
type PageTrajets struct {
	Trajets        []Trajet
	CurseurSuivant string
}
