package sqlite

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
)

// TestLesLecturesPubliquesUtilisentUnIndex : chaque lecture publique doit être servie par un index.
// Un « SCAN rides » signifie que l'index partiel n'est pas utilisable (mauvais ordre de colonnes ou
// prédicat différent de celui de l'index), donc un balayage complet à chaque recherche.
func TestLesLecturesPubliquesUtilisentUnIndex(t *testing.T) {
	ctx := context.Background()
	db, err := Ouvrir(ctx, filepath.Join(t.TempDir(), "index.db"))
	if err != nil {
		t.Fatalf("ouverture de la base : %v", err)
	}
	defer db.Close()

	cas := map[string]struct {
		requete string
		args    []any
		index   string
	}{
		"recherche UC2 paginée": {
			requete: requeteRecherche + clauseCurseur + finRecherche,
			args:    []any{"", "", "", "", "", "", "2026-12-01T08:00:00Z", "2026-12-01T08:00:00Z", "abc", LimiteRechercheParDefaut + 1},
			index:   "rides_search",
		},
		"recherche UC2 sans curseur": {
			requete: requeteRecherche + finRecherche,
			args:    []any{"Lyon", "Lyon", "Grenoble", "Grenoble", "2026-12-01", "2026-12-01", LimiteRechercheParDefaut + 1},
			index:   "rides_search",
		},
		"trajets d'un conducteur (UC10)": {
			requete: requeteParConducteur,
			args:    []any{"bob"},
			index:   "rides_driver",
		},
	}

	for nom, cas := range cas {
		t.Run(nom, func(t *testing.T) {
			rows, err := db.QueryContext(ctx, "EXPLAIN QUERY PLAN "+cas.requete, cas.args...)
			if err != nil {
				t.Fatalf("explication de la requête : %v", err)
			}
			defer rows.Close()

			var plan []string
			for rows.Next() {
				var id, parent, notused int
				var detail string
				if err := rows.Scan(&id, &parent, &notused, &detail); err != nil {
					t.Fatalf("lecture du plan : %v", err)
				}
				plan = append(plan, detail)
			}
			if err := rows.Err(); err != nil {
				t.Fatalf("lecture du plan : %v", err)
			}

			explication := strings.Join(plan, " | ")
			if strings.Contains(explication, "SCAN rides") {
				t.Fatalf("la requête balaie rides au lieu d'utiliser l'index %s : %s", cas.index, explication)
			}
			if !strings.Contains(explication, cas.index) {
				t.Fatalf("l'index %s n'apparaît pas dans le plan : %s", cas.index, explication)
			}
		})
	}
}

// TestLesIndexSurviventÀUneReapplicationDuSchema : le schéma est rejouable, les index avec.
func TestLesIndexSurviventALaReapplicationDuSchema(t *testing.T) {
	ctx := context.Background()
	db, err := Ouvrir(ctx, filepath.Join(t.TempDir(), "index.db"))
	if err != nil {
		t.Fatalf("ouverture de la base : %v", err)
	}
	defer db.Close()

	for tour := 1; tour <= 2; tour++ {
		if err := AppliquerSchema(ctx, db); err != nil {
			t.Fatalf("application du schéma (tour %d) : %v", tour, err)
		}
		for _, index := range []string{"rides_search", "rides_driver"} {
			var nom string
			err := db.QueryRowContext(ctx,
				`SELECT name FROM sqlite_master WHERE type = 'index' AND name = ?`, index).Scan(&nom)
			if err != nil {
				t.Fatalf("tour %d : index %s absent (%v)", tour, index, err)
			}
		}
	}
}
