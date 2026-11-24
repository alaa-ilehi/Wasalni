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

// colonnesTrajet : projection partagée par la lecture par identifiant et par la recherche.
const colonnesTrajet = `id, driver, origin, destination, departure, seats_total, seats_available,
       price, cancel_delay_hours, status, created_at`

// scanneur : ce que partagent sql.Row et sql.Rows, pour ne pas dupliquer la conversion des colonnes.
type scanneur interface {
	Scan(dest ...any) error
}

// lireTrajet convertit une ligne SQL en trajet. Les colonnes texte sont lues puis converties :
// aucune conversion implicite de SQLite vers time.Time.
func lireTrajet(row scanneur) (domain.Trajet, error) {
	var (
		trajet                    domain.Trajet
		depart, statut, creeLe    string
		delaiHeures               float64
		placesTotal, placesLibres int
	)
	if err := row.Scan(
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
	); err != nil {
		return domain.Trajet{}, err
	}

	var err error
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

// ParIdentifiant rend le trajet demandé, ou domain.ErrTrajetIntrouvable.
func (r *TrajetRepository) ParIdentifiant(ctx context.Context, identifiant string) (domain.Trajet, error) {
	requete := "SELECT " + colonnesTrajet + " FROM rides WHERE id = ?"
	trajet, err := lireTrajet(r.db.QueryRowContext(ctx, requete, identifiant))
	switch {
	case errors.Is(err, sql.ErrNoRows):
		return domain.Trajet{}, domain.ErrTrajetIntrouvable
	case err != nil:
		return domain.Trajet{}, fmt.Errorf("lecture du trajet : %w", err)
	}
	return trajet, nil
}

// Rechercher : UC2. Le SQL porte les règles métier (trajets publiés, tri par départ) et les filtres
// facultatifs ; aucun trajet trouvé n'est une erreur, c'est une liste vide.
func (r *TrajetRepository) Rechercher(ctx context.Context, criteres domain.CriteresRecherche) ([]domain.Trajet, error) {
	requete := "SELECT " + colonnesTrajet + `
FROM rides
WHERE status = 'PUBLISHED'
  AND (? = '' OR LOWER(origin) = LOWER(?))
  AND (? = '' OR LOWER(destination) = LOWER(?))
  AND (? = '' OR SUBSTR(departure, 1, 10) = ?)
ORDER BY departure ASC`
	rows, err := r.db.QueryContext(ctx, requete,
		criteres.Origine, criteres.Origine,
		criteres.Destination, criteres.Destination,
		criteres.Date, criteres.Date,
	)
	if err != nil {
		return nil, fmt.Errorf("recherche des trajets : %w", err)
	}
	defer rows.Close()

	trajets := []domain.Trajet{}
	for rows.Next() {
		trajet, err := lireTrajet(rows)
		if err != nil {
			return nil, fmt.Errorf("lecture des trajets : %w", err)
		}
		trajets = append(trajets, trajet)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("lecture des trajets : %w", err)
	}
	return trajets, nil
}
