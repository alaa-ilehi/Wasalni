package application_test

import (
	"context"
	"errors"
	"testing"

	"github.com/alaa-ilehi/wasalni/internal/application"
	"github.com/alaa-ilehi/wasalni/internal/domain"
)

// trajetsRechercheFaux : le port TrajetRepository, avec un œil sur les critères reçus.
type trajetsRechercheFaux struct {
	*trajetsFaux
	criteres  domain.CriteresRecherche
	resultats []domain.Trajet
	erreur    error
	appels    int
}

func (f *trajetsRechercheFaux) Rechercher(_ context.Context, criteres domain.CriteresRecherche) ([]domain.Trajet, error) {
	f.appels++
	f.criteres = criteres
	if f.erreur != nil {
		return nil, f.erreur
	}
	return f.resultats, nil
}

func depotRecherche() *trajetsRechercheFaux {
	return &trajetsRechercheFaux{trajetsFaux: &trajetsFaux{parID: map[string]domain.Trajet{}}}
}

func TestRechercherTrajetsNormaliseLesCriteres(t *testing.T) {
	depot := depotRecherche()
	uc := application.NouveauRechercherTrajets(depot)

	_, err := uc.Executer(context.Background(), domain.CriteresRecherche{
		Origine: "  Lyon  ", Destination: " Grenoble ", Date: " 2026-12-01 ",
	})
	if err != nil {
		t.Fatalf("recherche : %v", err)
	}
	if depot.appels != 1 {
		t.Fatalf("appels au dépôt = %d, attendu 1", depot.appels)
	}
	if depot.criteres.Origine != "Lyon" || depot.criteres.Destination != "Grenoble" || depot.criteres.Date != "2026-12-01" {
		t.Errorf("critères transmis = %+v", depot.criteres)
	}
}

func TestRechercherTrajetsRefuseUneDateMalFormee(t *testing.T) {
	for _, date := range []string{"hier", "2026-13-01", "2026-02-30", "01/12/2026"} {
		t.Run(date, func(t *testing.T) {
			depot := depotRecherche()
			uc := application.NouveauRechercherTrajets(depot)

			_, err := uc.Executer(context.Background(), domain.CriteresRecherche{Date: date})
			if !errors.Is(err, application.ErrRechercheInvalide) {
				t.Fatalf("erreur = %v, attendu ErrRechercheInvalide", err)
			}
			if depot.appels != 0 {
				t.Errorf("le dépôt a été appelé %d fois alors que la date est invalide", depot.appels)
			}
		})
	}
}

func TestRechercherTrajetsSansResultatRendUneListeVide(t *testing.T) {
	uc := application.NouveauRechercherTrajets(depotRecherche())

	trajets, err := uc.Executer(context.Background(), domain.CriteresRecherche{})
	if err != nil {
		t.Fatalf("recherche : %v", err)
	}
	if trajets == nil {
		t.Fatal("une recherche sans résultat doit rendre une liste vide, pas nil")
	}
	if len(trajets) != 0 {
		t.Errorf("trajets = %v, attendu une liste vide", trajets)
	}
}

func TestRechercherTrajetsNeCachePasUneErreurDepot(t *testing.T) {
	depot := depotRecherche()
	erreur := errors.New("base indisponible")
	depot.erreur = erreur
	uc := application.NouveauRechercherTrajets(depot)

	_, err := uc.Executer(context.Background(), domain.CriteresRecherche{})
	if !errors.Is(err, erreur) {
		t.Fatalf("erreur = %v, attendu l'erreur du dépôt telle quelle", err)
	}
}
