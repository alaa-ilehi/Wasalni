package sqlite_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/alaa-ilehi/wasalni/internal/domain"
	"github.com/alaa-ilehi/wasalni/internal/infrastructure/sqlite"
)

// publierPartant publie un trajet Lyon → Grenoble partant à l'heure donnée et rend son identifiant.
func publierPartant(t *testing.T, depot *sqlite.TrajetRepository, depart time.Time) string {
	t.Helper()
	trajet := trajetAPublier()
	trajet.Depart = depart
	cree, err := depot.Publier(context.Background(), trajet)
	if err != nil {
		t.Fatalf("publication du trajet partant à %s : %v", depart, err)
	}
	return cree.Identifiant
}

// TestRecherchePagineeSansDoublonNiPerte : trois pages successives rendent exactement l'ensemble attendu,
// dans l'ordre, chaque trajet une seule fois.
func TestRecherchePagineeSansDoublonNiPerte(t *testing.T) {
	ctx := context.Background()
	depot, _, fermer := baseTrajet(t)
	defer fermer()

	debut := time.Date(2026, 12, 1, 8, 0, 0, 0, time.UTC)
	attendu := make([]string, 0, 5)
	for i := range 5 {
		attendu = append(attendu, publierPartant(t, depot, debut.Add(time.Duration(i)*time.Hour)))
	}

	var obtenu []string
	var curseur string
	for page := 1; page <= 3; page++ {
		resultat, err := depot.Rechercher(ctx, domain.CriteresRecherche{Limite: 2, Apres: curseur})
		if err != nil {
			t.Fatalf("page %d : %v", page, err)
		}
		if len(resultat.Trajets) > 2 {
			t.Fatalf("page %d : %d trajets, la limite est 2", page, len(resultat.Trajets))
		}
		for _, trajet := range resultat.Trajets {
			obtenu = append(obtenu, trajet.Identifiant)
		}
		curseur = resultat.CurseurSuivant
	}

	if len(obtenu) != len(attendu) {
		t.Fatalf("trajets obtenus = %v, attendu %v", obtenu, attendu)
	}
	for i := range attendu {
		if obtenu[i] != attendu[i] {
			t.Errorf("trajet %d = %q, attendu %q (liste %v)", i, obtenu[i], attendu[i], obtenu)
		}
	}
	if curseur != "" {
		t.Errorf("curseur après la dernière page = %q, attendu vide", curseur)
	}
}

// TestRecherchePagineeIgnoreUnTrajetInsereEntreDeuxPages : le curseur est un keyset, pas un décalage,
// donc une insertion en cours de pagination ne duplique ni ne saute un trajet déjà rendu.
func TestRecherchePagineeIgnoreUnTrajetInsereEntreDeuxPages(t *testing.T) {
	ctx := context.Background()
	depot, _, fermer := baseTrajet(t)
	defer fermer()

	debut := time.Date(2026, 12, 1, 8, 0, 0, 0, time.UTC)
	premier := publierPartant(t, depot, debut)
	deuxieme := publierPartant(t, depot, debut.Add(time.Hour))

	pageUn, err := depot.Rechercher(ctx, domain.CriteresRecherche{Limite: 1})
	if err != nil {
		t.Fatalf("page 1 : %v", err)
	}
	if len(pageUn.Trajets) != 1 || pageUn.Trajets[0].Identifiant != premier {
		t.Fatalf("page 1 = %v, attendu [%s]", pageUn.Trajets, premier)
	}

	// Insertion entre les deux pages, avant le curseur comme après : la suite ne doit pas bouger.
	publierPartant(t, depot, debut.Add(-time.Hour))
	publierPartant(t, depot, debut.Add(90*time.Minute))

	pageDeux, err := depot.Rechercher(ctx, domain.CriteresRecherche{Limite: 10, Apres: pageUn.CurseurSuivant})
	if err != nil {
		t.Fatalf("page 2 : %v", err)
	}
	var identifiants []string
	for _, trajet := range pageDeux.Trajets {
		identifiants = append(identifiants, trajet.Identifiant)
	}
	if len(identifiants) != 2 || identifiants[0] != deuxieme {
		t.Fatalf("page 2 = %v, attendu [%s <trajet de 09:30>]", identifiants, deuxieme)
	}
	for _, identifiant := range identifiants {
		if identifiant == premier {
			t.Fatalf("le trajet déjà rendu %s apparaît une seconde fois", premier)
		}
	}
}

// TestRechercheRefuseUnCurseurIllisible : un curseur qui ne vient pas de l'adaptateur rend une erreur
// métier, donc un 400 côté HTTP, jamais un 500.
func TestRechercheRefuseUnCurseurIllisible(t *testing.T) {
	ctx := context.Background()
	depot, _, fermer := baseTrajet(t)
	defer fermer()

	for nom, curseur := range map[string]string{
		"pas du base64":   "%%%",
		"sans séparateur": "YWJjZGVm", // « abcdef »
		"sans identifiant": func() string {
			return "MjAyNi0xMi0wMVQwODowMDowMFp8" // « 2026-12-01T08:00:00Z| »
		}(),
		"date illisible": "bmimportequoifQ", // « n'importe quoi »
	} {
		t.Run(nom, func(t *testing.T) {
			_, err := depot.Rechercher(ctx, domain.CriteresRecherche{Apres: curseur})
			if !errors.Is(err, domain.ErrCurseurInvalide) {
				t.Fatalf("erreur = %v, attendu ErrCurseurInvalide", err)
			}
		})
	}
}
