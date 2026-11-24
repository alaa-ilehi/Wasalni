package application_test

import (
	"context"
	"errors"
	"testing"

	"github.com/alaa-ilehi/wasalni/internal/application"
	"github.com/alaa-ilehi/wasalni/internal/domain"
)

func trajetPublie(id, conducteur string) domain.Trajet {
	return domain.Trajet{
		Identifiant: id,
		Conducteur:  conducteur,
		Origine:     "Lyon",
		Destination: "Grenoble",
		Statut:      domain.TrajetPublie,
	}
}

func TestConsulterTrajetRenvoieLeProfilDuConducteur(t *testing.T) {
	trajets := &trajetsFaux{parID: map[string]domain.Trajet{"t1": trajetPublie("t1", "alice")}}
	profils := &depotFaux{profils: map[string]domain.Profil{
		"alice": {Identifiant: "alice", Nom: "Alice Ben Salah", Age: 29, Sexe: domain.SexeFeminin},
	}}
	uc := application.NouveauConsulterTrajet(trajets, profils, &evaluationsFausses{})

	detail, err := uc.Executer(context.Background(), "t1")
	if err != nil {
		t.Fatalf("consultation : %v", err)
	}
	if detail.Trajet.Identifiant != "t1" {
		t.Errorf("trajet = %q, attendu t1", detail.Trajet.Identifiant)
	}
	if detail.Profil == nil || detail.Profil.Nom != "Alice Ben Salah" {
		t.Fatalf("profil du conducteur = %+v, attendu Alice Ben Salah", detail.Profil)
	}
	if detail.Note != nil {
		t.Errorf("note = %v, attendu nil sans évaluation", *detail.Note)
	}
}

func TestConsulterTrajetRenvoieLaNoteDuConducteur(t *testing.T) {
	trajets := &trajetsFaux{parID: map[string]domain.Trajet{"t1": trajetPublie("t1", "alice")}}
	profils := &depotFaux{profils: map[string]domain.Profil{}}
	evaluations := &evaluationsFausses{notes: map[string]float64{"alice": 4.5}}
	uc := application.NouveauConsulterTrajet(trajets, profils, evaluations)

	detail, err := uc.Executer(context.Background(), "t1")
	if err != nil {
		t.Fatalf("consultation : %v", err)
	}
	if detail.Note == nil || *detail.Note != 4.5 {
		t.Errorf("note = %v, attendu 4.5", detail.Note)
	}
}

func TestConsulterTrajetSansProfilDuConducteur(t *testing.T) {
	trajets := &trajetsFaux{parID: map[string]domain.Trajet{"t1": trajetPublie("t1", "alice")}}
	profils := &depotFaux{profils: map[string]domain.Profil{}}
	uc := application.NouveauConsulterTrajet(trajets, profils, &evaluationsFausses{})

	detail, err := uc.Executer(context.Background(), "t1")
	if err != nil {
		t.Fatalf("une absence de profil ne doit pas être une erreur : %v", err)
	}
	if detail.Profil != nil {
		t.Errorf("profil = %+v, attendu nil", detail.Profil)
	}
}

func TestConsulterTrajetIntrouvableOuTermine(t *testing.T) {
	cas := map[string]map[string]domain.Trajet{
		"inconnu": {},
		"terminé": {"t1": {Identifiant: "t1", Conducteur: "alice", Statut: domain.TrajetTermine}},
	}
	for nom, parID := range cas {
		t.Run(nom, func(t *testing.T) {
			uc := application.NouveauConsulterTrajet(
				&trajetsFaux{parID: parID},
				&depotFaux{profils: map[string]domain.Profil{}},
				&evaluationsFausses{},
			)
			_, err := uc.Executer(context.Background(), "t1")
			if !errors.Is(err, domain.ErrTrajetIntrouvable) {
				t.Fatalf("erreur = %v, attendu ErrTrajetIntrouvable", err)
			}
		})
	}
}

func TestConsulterTrajetNeCachePasUneErreurProfil(t *testing.T) {
	trajets := &trajetsFaux{parID: map[string]domain.Trajet{"t1": trajetPublie("t1", "alice")}}
	profils := &depotFaux{profils: map[string]domain.Profil{}, erreur: errors.New("base indisponible")}
	uc := application.NouveauConsulterTrajet(trajets, profils, &evaluationsFausses{})

	_, err := uc.Executer(context.Background(), "t1")
	if err == nil || err.Error() != "base indisponible" {
		t.Fatalf("erreur = %v, attendu l'erreur du dépôt telle quelle", err)
	}
}
