package domain_test

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/alaa-ilehi/wasalni/internal/domain"
)

// trajetValide : base de test, départ dans deux heures.
func trajetValide(maintenant time.Time) domain.Trajet {
	return domain.Trajet{
		Conducteur:      "bob",
		Origine:         "Lyon",
		Destination:     "Grenoble",
		Depart:          maintenant.Add(2 * time.Hour),
		PlacesTotal:     3,
		Places:          3,
		Prix:            12.5,
		DelaiAnnulation: time.Hour,
		Statut:          domain.TrajetPublie,
	}
}

func TestValiderPublicationAccepteUnTrajetComplet(t *testing.T) {
	maintenant := time.Date(2026, 11, 24, 20, 0, 0, 0, time.UTC)
	if err := trajetValide(maintenant).ValiderPublication(maintenant); err != nil {
		t.Fatalf("trajet valide refusé : %v", err)
	}
}

func TestValiderTerminaison(t *testing.T) {
	cases := []struct {
		nom        string
		conducteur string
		statut     domain.StatutTrajet
		erreur     error
	}{
		{nom: "propriétaire et publié", conducteur: "bob", statut: domain.TrajetPublie},
		{nom: "autre conducteur", conducteur: "alice", statut: domain.TrajetPublie, erreur: domain.ErrTrajetNonProprietaire},
		{nom: "déjà terminé", conducteur: "bob", statut: domain.TrajetTermine, erreur: domain.ErrTrajetDejaTermine},
		{nom: "statut inconnu", conducteur: "bob", statut: "CANCELLED", erreur: domain.ErrTrajetInvalide},
	}
	for _, cas := range cases {
		t.Run(cas.nom, func(t *testing.T) {
			trajet := domain.Trajet{Conducteur: "bob", Statut: cas.statut}
			if err := trajet.ValiderTerminaison(cas.conducteur); !errors.Is(err, cas.erreur) {
				t.Fatalf("erreur = %v, attendu %v", err, cas.erreur)
			}
		})
	}
}

func TestValiderPublicationRefuse(t *testing.T) {
	maintenant := time.Date(2026, 11, 24, 20, 0, 0, 0, time.UTC)
	cas := map[string]struct {
		modifier  func(*domain.Trajet)
		messageAt string
	}{
		"sans conducteur": {
			modifier:  func(tj *domain.Trajet) { tj.Conducteur = "" },
			messageAt: "conducteur",
		},
		"origine vide": {
			modifier:  func(tj *domain.Trajet) { tj.Origine = "   " },
			messageAt: "origine",
		},
		"destination vide": {
			modifier:  func(tj *domain.Trajet) { tj.Destination = "" },
			messageAt: "destination",
		},
		"origine égale à la destination": {
			modifier:  func(tj *domain.Trajet) { tj.Destination = "  lyon " },
			messageAt: "différentes",
		},
		"aucune place": {
			modifier:  func(tj *domain.Trajet) { tj.PlacesTotal = 0; tj.Places = 0 },
			messageAt: "places",
		},
		"places disponibles incohérentes": {
			modifier:  func(tj *domain.Trajet) { tj.Places = 4 },
			messageAt: "places",
		},
		"prix négatif": {
			modifier:  func(tj *domain.Trajet) { tj.Prix = -1 },
			messageAt: "prix",
		},
		"délai d'annulation négatif": {
			modifier:  func(tj *domain.Trajet) { tj.DelaiAnnulation = -time.Minute },
			messageAt: "délai",
		},
		"départ passé": {
			modifier:  func(tj *domain.Trajet) { tj.Depart = maintenant.Add(-time.Minute) },
			messageAt: "futur",
		},
		"départ à l'instant": {
			modifier:  func(tj *domain.Trajet) { tj.Depart = maintenant },
			messageAt: "futur",
		},
		"délai plus long que le temps restant": {
			modifier:  func(tj *domain.Trajet) { tj.DelaiAnnulation = 2 * time.Hour },
			messageAt: "temps restant",
		},
		"statut inconnu": {
			modifier:  func(tj *domain.Trajet) { tj.Statut = "BROUILLON" },
			messageAt: "statut",
		},
	}
	for nom, cas := range cas {
		t.Run(nom, func(t *testing.T) {
			trajet := trajetValide(maintenant)
			cas.modifier(&trajet)
			err := trajet.ValiderPublication(maintenant)
			if !errors.Is(err, domain.ErrTrajetInvalide) {
				t.Fatalf("erreur = %v, attendu ErrTrajetInvalide", err)
			}
			if !strings.Contains(err.Error(), cas.messageAt) {
				t.Errorf("message %q ne parle pas de %q", err.Error(), cas.messageAt)
			}
		})
	}
}

func TestResteAvantDepartEstSigne(t *testing.T) {
	maintenant := time.Date(2026, 11, 24, 20, 0, 0, 0, time.UTC)
	trajet := trajetValide(maintenant)
	if reste := trajet.ResteAvantDepart(maintenant); reste != 2*time.Hour {
		t.Errorf("reste = %v, attendu 2h", reste)
	}
	trajet.Depart = maintenant.Add(-30 * time.Minute)
	if reste := trajet.ResteAvantDepart(maintenant); reste != -30*time.Minute {
		t.Errorf("reste = %v, attendu -30m", reste)
	}
}
