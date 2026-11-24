// Package domain : entités, règles métier et ports. Aucune dépendance technique.
package domain

import (
	"errors"
	"fmt"
	"strings"
	"time"
)

// Sexe d'un profil. Facultatif : la valeur vide signifie « non renseigné ».
type Sexe string

const (
	SexeMasculin Sexe = "MALE"
	SexeFeminin  Sexe = "FEMALE"
	SexeAutre    Sexe = "OTHER"
)

// Bornes de validation du profil (UC8).
const (
	NomLongueurMin = 2
	NomLongueurMax = 80
	AgeMin         = 18
	AgeMax         = 99
)

// Erreurs métier du profil. Elles ne portent aucun code HTTP (voir l'adaptateur HTTP).
var (
	ErrNomRequis         = errors.New("le nom est requis")
	ErrNomInvalide       = fmt.Errorf("le nom doit contenir entre %d et %d caractères", NomLongueurMin, NomLongueurMax)
	ErrAgeInvalide       = fmt.Errorf("l'âge doit être compris entre %d et %d ans", AgeMin, AgeMax)
	ErrSexeInvalide      = errors.New("le sexe doit valoir MALE, FEMALE ou OTHER")
	ErrProfilIntrouvable = errors.New("profil introuvable")
)

// Profil est le profil d'un utilisateur identifié (UC8).
// L'identifiant vient du système d'authentification externe : il n'est jamais créé ici.
type Profil struct {
	Identifiant string
	Nom         string
	Age         int
	Sexe        Sexe
	Photo       string // base64 d'un PNG déjà normalisé ; vide si absent
	CreeLe      time.Time
	MajLe       time.Time
}

// Valider applique les règles métier de UC8 : nom et âge requis, sexe facultatif.
func (p Profil) Valider() error {
	nom := strings.TrimSpace(p.Nom)
	if nom == "" {
		return ErrNomRequis
	}
	if n := len([]rune(nom)); n < NomLongueurMin || n > NomLongueurMax {
		return ErrNomInvalide
	}
	if p.Age < AgeMin || p.Age > AgeMax {
		return ErrAgeInvalide
	}
	switch p.Sexe {
	case "", SexeMasculin, SexeFeminin, SexeAutre:
	default:
		return ErrSexeInvalide
	}
	return nil
}
