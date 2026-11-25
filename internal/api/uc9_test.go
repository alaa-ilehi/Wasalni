package api_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/alaa-ilehi/wasalni/internal/api"
	"github.com/alaa-ilehi/wasalni/internal/application"
	"github.com/alaa-ilehi/wasalni/internal/domain"
	"github.com/alaa-ilehi/wasalni/internal/infrastructure/sqlite"
)

type horlogeUC9 struct{}

func (horlogeUC9) Maintenant() time.Time { return time.Date(2026, 11, 25, 8, 0, 0, 0, time.UTC) }

func TestUC9HTTPModifierEtSupprimer(t *testing.T) {
	ctx := context.Background()
	db, err := sqlite.Ouvrir(ctx, filepath.Join(t.TempDir(), "uc9.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	trajets, reservations := sqlite.NouveauTrajetRepository(db), sqlite.NouveauReservationRepository(db)
	uc := application.NouveauGererTrajet(trajets, trajets, horlogeUC9{})
	handler := api.Routes(api.Deps{GestionTrajets: uc})
	cree, err := trajets.Publier(ctx, domain.Trajet{Conducteur: "bob", Origine: "Lyon", Destination: "Grenoble", Depart: time.Date(2026, 12, 1, 8, 0, 0, 0, time.UTC), PlacesTotal: 2, Places: 2, Statut: domain.TrajetPublie})
	if err != nil {
		t.Fatal(err)
	}
	faire := func(method, body, acteur string) *httptest.ResponseRecorder {
		r := httptest.NewRequest(method, "/rides/"+cree.Identifiant, strings.NewReader(body))
		if acteur != "" {
			r.Header.Set(api.EnTeteIdentite, acteur)
		}
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, r)
		return w
	}
	if got := faire(http.MethodPut, `{"origin":"Paris","destination":"Grenoble","departure_time":"2026-12-01T08:00:00Z","seats_total":2,"price":5}`, "bob"); got.Code != http.StatusOK {
		t.Fatalf("PUT status=%d body=%s", got.Code, got.Body)
	}
	if _, err := reservations.Reserver(ctx, domain.Reservation{Trajet: cree.Identifiant, Passager: "alice", Statut: domain.ReservationActive}); err != nil {
		t.Fatal(err)
	}
	if got := faire(http.MethodDelete, "", "bob"); got.Code != http.StatusNoContent {
		t.Fatalf("DELETE status=%d body=%s", got.Code, got.Body)
	}
	if got := faire(http.MethodDelete, "", "bob"); got.Code != http.StatusNotFound {
		t.Fatalf("second DELETE status=%d, want 404", got.Code)
	}
}
