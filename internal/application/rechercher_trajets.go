package application

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/alaa-ilehi/wasalni/internal/domain"
)

// ErrRechercheInvalide : un critère de recherche est mal formé (date).
var ErrRechercheInvalide = errors.New("critères de recherche invalides")

// RechercherTrajets : UC2, un passager cherche des trajets publiés, sans identité requise.
type RechercherTrajets struct {
	trajets domain.TrajetRepository
}

func NouveauRechercherTrajets(trajets domain.TrajetRepository) RechercherTrajets {
	return RechercherTrajets{trajets: trajets}
}

// Executer normalise les critères puis délègue au dépôt le filtrage des trajets publiés et le tri par départ.
// Une date invalide est refusée avant toute lecture ; une recherche sans résultat rend une liste vide.
func (uc RechercherTrajets) Executer(ctx context.Context, criteres domain.CriteresRecherche) ([]domain.Trajet, error) {
	criteres.Origine = strings.TrimSpace(criteres.Origine)
	criteres.Destination = strings.TrimSpace(criteres.Destination)
	date := strings.TrimSpace(criteres.Date)
	if date != "" {
		if _, err := time.Parse("2006-01-02", date); err != nil {
			return nil, fmt.Errorf("%w : la date doit être au format AAAA-MM-JJ", ErrRechercheInvalide)
		}
	}
	criteres.Date = date

	trajets, err := uc.trajets.Rechercher(ctx, criteres)
	if err != nil {
		return nil, err
	}
	if trajets == nil {
		return []domain.Trajet{}, nil
	}
	return trajets, nil
}
