package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/alaa-ilehi/wasalni/internal/domain"
)

// TrajetRepository : adaptateur SQLite du port domain.TrajetRepository (UC1 et UC3).
type TrajetRepository struct {
	db *sql.DB
}

func NouveauTrajetRepository(db *sql.DB) *TrajetRepository {
	return &TrajetRepository{db: db}
}

// Publier écrit un nouveau trajet et rend l'enregistrement créé, identifiant et horodatages compris.
func (r *TrajetRepository) Publier(ctx context.Context, trajet domain.Trajet) (domain.Trajet, error) {
	identifiant, err := nouvelIdentifiant()
	if err != nil {
		return domain.Trajet{}, err
	}

	const requete = `
INSERT INTO rides (id, driver, origin, destination, departure, seats_total, seats_available,
                   price, cancel_delay_hours, status)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
RETURNING created_at`
	var creeLe string
	err = r.db.QueryRowContext(ctx, requete,
		identifiant,
		trajet.Conducteur,
		trajet.Origine,
		trajet.Destination,
		trajet.Depart.UTC().Format(time.RFC3339),
		trajet.PlacesTotal,
		trajet.Places,
		trajet.Prix,
		trajet.DelaiAnnulation.Hours(),
		string(trajet.Statut),
	).Scan(&creeLe)
	if err != nil {
		return domain.Trajet{}, fmt.Errorf("écriture du trajet : %w", err)
	}

	horodatage, err := time.Parse(time.RFC3339, creeLe)
	if err != nil {
		return domain.Trajet{}, fmt.Errorf("horodatage de création illisible %q : %w", creeLe, err)
	}
	trajet.Identifiant = identifiant
	trajet.CreeLe = horodatage
	return trajet, nil
}

// ParIdentifiant rend le trajet demandé, ou domain.ErrTrajetIntrouvable.
func (r *TrajetRepository) ParIdentifiant(ctx context.Context, identifiant string) (domain.Trajet, error) {
	const requete = `
SELECT id, driver, origin, destination, departure, seats_total, seats_available,
       price, cancel_delay_hours, status, created_at
FROM rides
WHERE id = ?`
	var (
		trajet                    domain.Trajet
		depart, statut, creeLe    string
		delaiHeures               float64
		placesTotal, placesLibres int
	)
	// Les colonnes texte sont lues puis converties : aucune conversion implicite de SQLite vers time.Time.
	err := r.db.QueryRowContext(ctx, requete, identifiant).Scan(
		&trajet.Identifiant,
		&trajet.Conducteur,
		&trajet.Origine,
		&trajet.Destination,
		&depart,
		&placesTotal,
		&placesLibres,
		&trajet.Prix,
		&delaiHeures,
		&statut,
		&creeLe,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.Trajet{}, domain.ErrTrajetIntrouvable
	}
	if err != nil {
		return domain.Trajet{}, fmt.Errorf("lecture du trajet : %w", err)
	}

	if trajet.Depart, err = time.Parse(time.RFC3339, depart); err != nil {
		return domain.Trajet{}, fmt.Errorf("départ illisible %q : %w", depart, err)
	}
	if trajet.CreeLe, err = time.Parse(time.RFC3339, creeLe); err != nil {
		return domain.Trajet{}, fmt.Errorf("horodatage de création illisible %q : %w", creeLe, err)
	}
	trajet.PlacesTotal = placesTotal
	trajet.Places = placesLibres
	trajet.DelaiAnnulation = time.Duration(delaiHeures * float64(time.Hour))
	trajet.Statut = domain.StatutTrajet(statut)
	return trajet, nil
}
