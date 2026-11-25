package domain

import (
	"context"
	"time"
)

// ProfilRepository : persistance des profils, vue depuis le domaine.
// Contrat : Enregistrer crée ou remplace le profil de l'identifiant fourni.
type ProfilRepository interface {
	Enregistrer(ctx context.Context, p Profil) (Profil, bool, error) // booléen : true si création
	ParIdentifiant(ctx context.Context, identifiant string) (Profil, error)
}

// TrajetRepository : publication (UC1), recherche (UC2) et lecture pour la réservation (UC3).
// Contrat : Publier écrit un nouveau trajet et rend l'enregistrement créé, identifiant compris ;
// ParIdentifiant échoue avec domain.ErrTrajetIntrouvable pour un identifiant inconnu ;
// Rechercher ne rapporte que les trajets publiés, triés par départ, et rend une liste vide si rien ne correspond.
type TrajetRepository interface {
	Publier(ctx context.Context, t Trajet) (Trajet, error)
	ParIdentifiant(ctx context.Context, identifiant string) (Trajet, error)
	// Rechercher rend une page de trajets publiés et le curseur de la suivante (vide sur la dernière page).
	Rechercher(ctx context.Context, criteres CriteresRecherche) (PageTrajets, error)
	// ParConducteur liste les trajets d'un conducteur, trajets supprimés exclus, du départ le plus proche
	// au plus lointain (UC10). Une liste vide n'est pas une erreur.
	ParConducteur(ctx context.Context, conducteur string) ([]Trajet, error)
}

// TrajetFinRepository porte la transition UC11 vers COMPLETED.
// Terminer échoue avec ErrTrajetIntrouvable, ErrTrajetNonProprietaire ou ErrTrajetDejaTermine.
type TrajetFinRepository interface {
	Terminer(ctx context.Context, identifiant, conducteur string) (Trajet, error)
}

// TrajetGestionRepository porte la modification et la suppression logique UC9.
type TrajetGestionRepository interface {
	Modifier(ctx context.Context, trajet Trajet, conducteur string) (Trajet, error)
	Supprimer(ctx context.Context, identifiant, conducteur string) error
}

// ReservationRepository : écriture et lecture des réservations.
// Contrat : Reserver échoue avec domain.ErrReservationDejaActive si une réservation ACTIVE existe déjà
// pour le couple (trajet, passager) ; la garantie vient de l'index unique partiel, pas d'une lecture préalable.
type ReservationRepository interface {
	Reserver(ctx context.Context, r Reservation) (Reservation, error)
	// ParPassager liste les réservations d'un passager, de la plus récente à la plus ancienne.
	ParPassager(ctx context.Context, passager string) ([]Reservation, error)
	// ParPassagerAvecTrajet fait la même lecture en y joignant le trajet, en une seule requête. Le
	// trajet vaut nil quand il a été supprimé (UC9).
	ParPassagerAvecTrajet(ctx context.Context, passager string) ([]ReservationAvecTrajet, error)
	// Annuler vérifie propriété, état et échéance, puis passe à CANCELLED et rend la place
	// dans la même transaction. Une répétition ne modifie ni réservation ni compteur.
	Annuler(ctx context.Context, identifiant, passager string, maintenant time.Time) (Reservation, error)
}

// EvaluationRepository : écriture et agrégation des évaluations de UC6.
// Contrat : Enregistrer échoue avec domain.ErrEvaluationDejaFaite si le couple
// (trajet, évaluateur, cible) existe déjà ; Moyenne rend la note moyenne arrondie à une décimale
// et le nombre d'évaluations reçues par l'utilisateur (zéro si aucune).
type EvaluationRepository interface {
	Enregistrer(ctx context.Context, e Evaluation) (Evaluation, error)
	Moyenne(ctx context.Context, identifiant string) (float64, int, error)
}
