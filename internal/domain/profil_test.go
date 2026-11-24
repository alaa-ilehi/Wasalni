package domain_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/alaa-ilehi/wasalni/internal/domain"
)

func profilValide() domain.Profil {
	return domain.Profil{Identifiant: "alice", Nom: "Alice Ben Salah", Age: 27, Sexe: domain.SexeFeminin}
}

func TestValiderNomEtAge(t *testing.T) {
	cas := []struct {
		nom     string
		age     int
		attendu error
		libelle string
	}{
		{"", 27, domain.ErrNomRequis, "nom vide"},
		{"   ", 27, domain.ErrNomRequis, "nom fait d'espaces"},
		{"A", 27, domain.ErrNomInvalide, "nom trop court"},
		{strings.Repeat("a", 81), 27, domain.ErrNomInvalide, "nom trop long"},
		{"Alice", 17, domain.ErrAgeInvalide, "âge sous la borne"},
		{"Alice", 100, domain.ErrAgeInvalide, "âge au-dessus de la borne"},
		{"Alice", -1, domain.ErrAgeInvalide, "âge négatif"},
	}
	for _, c := range cas {
		p := domaineAvec(c.nom, c.age)
		if err := p.Valider(); !errors.Is(err, c.attendu) {
			t.Errorf("%s : erreur = %v, attendu %v", c.libelle, err, c.attendu)
		}
	}
}

func TestValiderSexeFacultatif(t *testing.T) {
	for _, sexe := range []domain.Sexe{"", domain.SexeMasculin, domain.SexeFeminin, domain.SexeAutre} {
		p := profilValide()
		p.Sexe = sexe
		if err := p.Valider(); err != nil {
			t.Errorf("sexe %q refusé : %v", sexe, err)
		}
	}
	p := profilValide()
	p.Sexe = "HOMME"
	if err := p.Valider(); !errors.Is(err, domain.ErrSexeInvalide) {
		t.Errorf("sexe inconnu accepté, erreur = %v", err)
	}
}

func TestValiderAccepteLesBornes(t *testing.T) {
	for _, age := range []int{domain.AgeMin, domain.AgeMax} {
		p := domaineAvec("Alice", age)
		if err := p.Valider(); err != nil {
			t.Errorf("âge %d refusé : %v", age, err)
		}
	}
}

func domaineAvec(nom string, age int) domain.Profil {
	p := profilValide()
	p.Nom = nom
	p.Age = age
	return p
}
