package application

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/alaa-ilehi/wasalni/internal/domain"
)

// LimiteRechercheMax : nombre maximum de trajets qu'une page peut demander.
const LimiteRechercheMax = 200

// ErrRechercheInvalide : un critère de recherche est mal formé (date, limite).
var ErrRechercheInvalide = errors.New("critères de recherche invalides")

// RechercherTrajets : UC2, un passager cherche des trajets publiés, sans identité requise.
type RechercherTrajets struct {
	trajets domain.TrajetRepository
}

func NouveauRechercherTrajets(trajets domain.TrajetRepository) RechercherTrajets {
	return RechercherTrajets{trajets: trajets}
}

// Executer normalise les critères puis délègue au dépôt le filtrage des trajets publiés, le tri par départ et
// la page demandée. Une date invalide ou une limite hors bornes est refusée avant toute lecture ; une recherche
// sans résultat rend une page vide.
func (uc RechercherTrajets) Executer(ctx context.Context, criteres domain.CriteresRecherche) (domain.PageTrajets, error) {
	criteres.Origine = strings.TrimSpace(criteres.Origine)
	criteres.Destination = strings.TrimSpace(criteres.Destination)
	date := strings.TrimSpace(criteres.Date)
	if date != "" {
		if _, err := time.Parse("2006-01-02", date); err != nil {
			return domain.PageTrajets{}, fmt.Errorf("%w : la date doit être au format AAAA-MM-JJ", ErrRechercheInvalide)
		}
	}
	criteres.Date = date
	if criteres.Limite < 0 || criteres.Limite > LimiteRechercheMax {
		return domain.PageTrajets{}, fmt.Errorf("%w : la limite doit être comprise entre 1 et %d", ErrRechercheInvalide, LimiteRechercheMax)
	}

	page, err := uc.trajets.Rechercher(ctx, criteres)
	if err != nil {
		return domain.PageTrajets{}, err
	}
	if page.Trajets == nil {
		page.Trajets = []domain.Trajet{}
	}
	return page, nil
}
