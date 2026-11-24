package domain

import "context"

// ProfilRepository : persistance des profils, vue depuis le domaine.
// Contrat : Enregistrer crée ou remplace le profil de l'identifiant fourni.
type ProfilRepository interface {
	Enregistrer(ctx context.Context, p Profil) (Profil, bool, error) // booléen : true si création
	ParIdentifiant(ctx context.Context, identifiant string) (Profil, error)
}

// TrajetRepository : lecture des trajets. En écriture seule côté UC3 : la publication appartient à UC1.
type TrajetRepository interface {
	ParIdentifiant(ctx context.Context, identifiant string) (Trajet, error)
}

// ReservationRepository : écriture et lecture des réservations.
// Contrat : Reserver échoue avec domain.ErrReservationDejaActive si une réservation ACTIVE existe déjà
// pour le couple (trajet, passager) ; la garantie vient de l'index unique partiel, pas d'une lecture préalable.
type ReservationRepository interface {
	Reserver(ctx context.Context, r Reservation) (Reservation, error)
	// ParPassager liste les réservations d'un passager, de la plus récente à la plus ancienne.
	ParPassager(ctx context.Context, passager string) ([]Reservation, error)
}
