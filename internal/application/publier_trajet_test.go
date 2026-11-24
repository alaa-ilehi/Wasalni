package application_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/alaa-ilehi/wasalni/internal/application"
	"github.com/alaa-ilehi/wasalni/internal/domain"
)

// horlogeFixe : port Horloge, pour que la publication ne dépende pas du temps réel.
type horlogeFixe struct{ instant time.Time }

func (h horlogeFixe) Maintenant() time.Time { return h.instant }

func maintenant() time.Time {
	return time.Date(2026, 11, 24, 20, 0, 0, 0, time.UTC)
}

func saisieValide() application.SaisieTrajet {
	return application.SaisieTrajet{
		Origine:     "  Lyon ",
		Destination: "Grenoble",
		Depart:      maintenant().Add(3 * time.Hour).Format(time.RFC3339),
		PlacesTotal: 3,
		Prix:        12.5,
	}
}

func publier(t *testing.T) (application.PublierTrajet, *trajetsFaux) {
	t.Helper()
	trajets := &trajetsFaux{parID: map[string]domain.Trajet{}}
	return application.NouveauPublierTrajet(trajets, horlogeFixe{instant: maintenant()}), trajets
}

func TestPublierTrajetEcritUnTrajetPublie(t *testing.T) {
	publierTrajet, trajets := publier(t)

	trajet, err := publierTrajet.Executer(context.Background(), "bob", saisieValide())
	if err != nil {
		t.Fatalf("publication : %v", err)
	}
	if trajet.Identifiant == "" {
		t.Error("identifiant vide")
	}
	if trajet.Conducteur != "bob" {
		t.Errorf("conducteur = %q, attendu bob", trajet.Conducteur)
	}
	if trajet.Origine != "Lyon" {
		t.Errorf("origine = %q, attendu Lyon (espaces retirés)", trajet.Origine)
	}
	if trajet.PlacesTotal != 3 || trajet.Places != 3 {
		t.Errorf("places = %d/%d, attendu 3/3", trajet.Places, trajet.PlacesTotal)
	}
	if trajet.Statut != domain.TrajetPublie {
		t.Errorf("statut = %q, attendu %q", trajet.Statut, domain.TrajetPublie)
	}
	if trajet.DelaiAnnulation != domain.DureeDelaiAnnulationParDefaut {
		t.Errorf("délai = %v, attendu le défaut %v", trajet.DelaiAnnulation, domain.DureeDelaiAnnulationParDefaut)
	}
	if len(trajets.parID) != 1 {
		t.Errorf("trajets écrits = %d, attendu 1", len(trajets.parID))
	}
}

func TestPublierTrajetAccepteUnDelaiExpliciteEtLAliasDeparture(t *testing.T) {
	publierTrajet, _ := publier(t)
	saisie := saisieValide()
	saisie.Depart = ""
	saisie.DepartSecours = maintenant().Add(4 * time.Hour).Format(time.RFC3339)
	delai := 1.5
	saisie.DelaiAnnulation = &delai

	trajet, err := publierTrajet.Executer(context.Background(), "bob", saisie)
	if err != nil {
		t.Fatalf("publication : %v", err)
	}
	if trajet.DelaiAnnulation != 90*time.Minute {
		t.Errorf("délai = %v, attendu 1h30", trajet.DelaiAnnulation)
	}
	if !trajet.Depart.Equal(maintenant().Add(4 * time.Hour)) {
		t.Errorf("départ = %v, attendu +4h", trajet.Depart)
	}
}

func TestPublierTrajetLitUneDateSansFuseauEnUTC(t *testing.T) {
	publierTrajet, _ := publier(t)
	saisie := saisieValide()
	saisie.Depart = "2026-11-24T23:30:00"

	trajet, err := publierTrajet.Executer(context.Background(), "bob", saisie)
	if err != nil {
		t.Fatalf("publication : %v", err)
	}
	if trajet.Depart.Location() != time.UTC {
		t.Errorf("fuseau = %v, attendu UTC", trajet.Depart.Location())
	}
}

func TestPublierTrajetRefuse(t *testing.T) {
	cas := map[string]struct {
		acteur   string
		modifier func(*application.SaisieTrajet)
		attendu  error
	}{
		"sans identité": {
			acteur:   "",
			modifier: func(*application.SaisieTrajet) {},
			attendu:  application.ErrIdentiteManquante,
		},
		"sans départ": {
			acteur:   "bob",
			modifier: func(s *application.SaisieTrajet) { s.Depart = "  " },
			attendu:  domain.ErrTrajetInvalide,
		},
		"départ illisible": {
			acteur:   "bob",
			modifier: func(s *application.SaisieTrajet) { s.Depart = "demain matin" },
			attendu:  domain.ErrTrajetInvalide,
		},
		"départ passé": {
			acteur:   "bob",
			modifier: func(s *application.SaisieTrajet) { s.Depart = maintenant().Add(-time.Hour).Format(time.RFC3339) },
			attendu:  domain.ErrTrajetInvalide,
		},
		"origine égale à la destination": {
			acteur:   "bob",
			modifier: func(s *application.SaisieTrajet) { s.Destination = "lyon" },
			attendu:  domain.ErrTrajetInvalide,
		},
		"aucune place": {
			acteur:   "bob",
			modifier: func(s *application.SaisieTrajet) { s.PlacesTotal = 0 },
			attendu:  domain.ErrTrajetInvalide,
		},
		"prix négatif": {
			acteur:   "bob",
			modifier: func(s *application.SaisieTrajet) { s.Prix = -0.5 },
			attendu:  domain.ErrTrajetInvalide,
		},
		"délai plus long que le temps restant": {
			acteur: "bob",
			modifier: func(s *application.SaisieTrajet) {
				trop := 5.0
				s.DelaiAnnulation = &trop
			},
			attendu: domain.ErrTrajetInvalide,
		},
	}
	for nom, cas := range cas {
		t.Run(nom, func(t *testing.T) {
			publierTrajet, trajets := publier(t)
			saisie := saisieValide()
			cas.modifier(&saisie)

			if _, err := publierTrajet.Executer(context.Background(), cas.acteur, saisie); !errors.Is(err, cas.attendu) {
				t.Fatalf("erreur = %v, attendu %v", err, cas.attendu)
			}
			if len(trajets.parID) != 0 {
				t.Errorf("un refus ne doit rien écrire, %d trajets écrits", len(trajets.parID))
			}
		})
	}
}

func TestPublierTrajetNeCachePasUneErreurDepot(t *testing.T) {
	trajets := &trajetsFaux{erreur: errors.New("base indisponible")}
	publierTrajet := application.NouveauPublierTrajet(trajets, horlogeFixe{instant: maintenant()})

	_, err := publierTrajet.Executer(context.Background(), "bob", saisieValide())
	if err == nil || errors.Is(err, domain.ErrTrajetInvalide) {
		t.Fatalf("erreur = %v, attendu l'erreur du dépôt telle quelle", err)
	}
}
