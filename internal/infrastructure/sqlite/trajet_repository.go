package sqlite

import (
	"context"
	"database/sql"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/alaa-ilehi/wasalni/internal/domain"
)

// TrajetRepository : adaptateur SQLite des ports trajet (UC1, UC3, UC7 et UC11).
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

// Terminer réalise la transition UC11 une seule fois et uniquement pour le conducteur propriétaire.
func (r *TrajetRepository) Terminer(ctx context.Context, identifiant, conducteur string) (domain.Trajet, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return domain.Trajet{}, fmt.Errorf("début de transaction : %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	var trajet domain.Trajet
	if trajet, err = lireTrajet(tx.QueryRowContext(ctx,
		"SELECT "+colonnesTrajet+" FROM rides WHERE id = ? AND deleted_at IS NULL", identifiant)); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return domain.Trajet{}, domain.ErrTrajetIntrouvable
		}
		return domain.Trajet{}, fmt.Errorf("lecture du trajet à terminer : %w", err)
	}
	if err := trajet.ValiderTerminaison(conducteur); err != nil {
		return domain.Trajet{}, err
	}

	resultat, err := tx.ExecContext(ctx,
		`UPDATE rides SET status = 'COMPLETED' WHERE id = ? AND driver = ? AND status = 'PUBLISHED'`,
		identifiant, conducteur,
	)
	if err != nil {
		return domain.Trajet{}, fmt.Errorf("terminaison du trajet : %w", err)
	}
	modifiees, err := resultat.RowsAffected()
	if err != nil {
		return domain.Trajet{}, fmt.Errorf("vérification de la terminaison : %w", err)
	}
	if modifiees != 1 {
		return domain.Trajet{}, domain.ErrTrajetDejaTermine
	}
	if err := tx.Commit(); err != nil {
		return domain.Trajet{}, fmt.Errorf("validation de la transaction : %w", err)
	}

	trajet.Statut = domain.TrajetTermine
	return trajet, nil
}

// Modifier met à jour un trajet publié en conservant le nombre de places déjà réservées.
func (r *TrajetRepository) Modifier(ctx context.Context, trajet domain.Trajet, conducteur string) (domain.Trajet, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return domain.Trajet{}, err
	}
	defer func() { _ = tx.Rollback() }()
	actuel, err := lireTrajet(tx.QueryRowContext(ctx, "SELECT "+colonnesTrajet+" FROM rides WHERE id = ? AND deleted_at IS NULL", trajet.Identifiant))
	if errors.Is(err, sql.ErrNoRows) {
		return domain.Trajet{}, domain.ErrTrajetIntrouvable
	}
	if err != nil {
		return domain.Trajet{}, err
	}
	if err := actuel.ValiderTerminaison(conducteur); err != nil {
		return domain.Trajet{}, err
	}
	var reservees int
	if err := tx.QueryRowContext(ctx, "SELECT COUNT(*) FROM bookings WHERE ride_id = ? AND status = 'ACTIVE'", trajet.Identifiant).Scan(&reservees); err != nil {
		return domain.Trajet{}, err
	}
	if trajet.PlacesTotal < reservees {
		return domain.Trajet{}, domain.ErrPlacesInsuffisantes
	}
	trajet.Places = trajet.PlacesTotal - reservees
	_, err = tx.ExecContext(ctx, `UPDATE rides SET origin=?, destination=?, departure=?, seats_total=?, seats_available=?, price=?, cancel_delay_hours=? WHERE id=? AND driver=? AND status='PUBLISHED' AND deleted_at IS NULL`,
		trajet.Origine, trajet.Destination, trajet.Depart.UTC().Format(time.RFC3339), trajet.PlacesTotal, trajet.Places, trajet.Prix, trajet.DelaiAnnulation.Hours(), trajet.Identifiant, conducteur)
	if err != nil {
		return domain.Trajet{}, err
	}
	if err := tx.Commit(); err != nil {
		return domain.Trajet{}, err
	}
	trajet.Conducteur, trajet.Statut, trajet.CreeLe = actuel.Conducteur, actuel.Statut, actuel.CreeLe
	return trajet, nil
}

// Supprimer conserve la ligne et l'historique des réservations, tout en annulant les réservations actives.
func (r *TrajetRepository) Supprimer(ctx context.Context, identifiant, conducteur string) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	trajet, err := lireTrajet(tx.QueryRowContext(ctx, "SELECT "+colonnesTrajet+" FROM rides WHERE id=? AND deleted_at IS NULL", identifiant))
	if errors.Is(err, sql.ErrNoRows) {
		return domain.ErrTrajetIntrouvable
	}
	if err != nil {
		return err
	}
	if err := trajet.ValiderTerminaison(conducteur); err != nil {
		return err
	}
	annulees, err := tx.ExecContext(ctx, "UPDATE bookings SET status='CANCELLED' WHERE ride_id=? AND status='ACTIVE'", identifiant)
	if err != nil {
		return err
	}
	rendues, err := annulees.RowsAffected()
	if err != nil {
		return err
	}
	// Les réservations annulées rendent leurs places, comme le fait UC5 ; MIN garde la ligne valide même
	// sur une base déjà incohérente (ADR 0004 : places + réservations actives = places totales).
	if _, err := tx.ExecContext(ctx, "UPDATE rides SET seats_available=MIN(seats_available+?, seats_total), deleted_at=strftime('%Y-%m-%dT%H:%M:%SZ','now') WHERE id=?", rendues, identifiant); err != nil {
		return err
	}
	return tx.Commit()
}

// colonnesTrajet : projection partagée par la lecture par identifiant et par la recherche.
const colonnesTrajet = `id, driver, origin, destination, departure, seats_total, seats_available,
       price, cancel_delay_hours, status, created_at, deleted_at`

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
		supprimeLe                sql.NullString
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
		&supprimeLe,
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
	trajet.Supprime = supprimeLe.Valid
	return trajet, nil
}

// ParIdentifiant rend le trajet demandé, ou domain.ErrTrajetIntrouvable.
func (r *TrajetRepository) ParIdentifiant(ctx context.Context, identifiant string) (domain.Trajet, error) {
	requete := "SELECT " + colonnesTrajet + " FROM rides WHERE id = ? AND deleted_at IS NULL"
	trajet, err := lireTrajet(r.db.QueryRowContext(ctx, requete, identifiant))
	switch {
	case errors.Is(err, sql.ErrNoRows):
		return domain.Trajet{}, domain.ErrTrajetIntrouvable
	case err != nil:
		return domain.Trajet{}, fmt.Errorf("lecture du trajet : %w", err)
	}
	return trajet, nil
}

// LimiteRechercheParDefaut : nombre de trajets rendus quand le client ne demande pas de limite.
const LimiteRechercheParDefaut = 50

const (
	requeteRecherche = "SELECT " + colonnesTrajet + `
FROM rides
WHERE status = 'PUBLISHED'
  AND deleted_at IS NULL
  AND (? = '' OR LOWER(origin) = LOWER(?))
  AND (? = '' OR LOWER(destination) = LOWER(?))
  AND (? = '' OR SUBSTR(departure, 1, 10) = ?)`
	clauseCurseur = `
  AND (departure > ? OR (departure = ? AND id > ?))`
	finRecherche = `
ORDER BY departure ASC, id ASC
LIMIT ?`
	requeteParConducteur = "SELECT " + colonnesTrajet + `
FROM rides
WHERE driver = ?
  AND deleted_at IS NULL
ORDER BY departure ASC, id ASC`
)

// Rechercher : UC2, une page de trajets publiés triés par départ puis identifiant.
// Le SQL porte les règles métier et les filtres facultatifs ; aucun trajet trouvé n'est une erreur, c'est une page vide.
// La pagination est un keyset : le curseur porte le couple (departure, id) de la dernière ligne rendue, donc ni
// comptage ni décalage, et une insertion entre deux pages ne duplique rien.
func (r *TrajetRepository) Rechercher(ctx context.Context, criteres domain.CriteresRecherche) (domain.PageTrajets, error) {
	limite := criteres.Limite
	if limite <= 0 {
		limite = LimiteRechercheParDefaut
	}
	requete := requeteRecherche
	arguments := []any{
		criteres.Origine, criteres.Origine,
		criteres.Destination, criteres.Destination,
		criteres.Date, criteres.Date,
	}
	if criteres.Apres != "" {
		depart, identifiant, err := decoderCurseur(criteres.Apres)
		if err != nil {
			return domain.PageTrajets{}, err
		}
		requete += clauseCurseur
		arguments = append(arguments, depart, depart, identifiant)
	}
	// Une ligne de plus que demandé : sa présence dit s'il reste une page, sans requête de comptage.
	requete += finRecherche
	arguments = append(arguments, limite+1)

	rows, err := r.db.QueryContext(ctx, requete, arguments...)
	if err != nil {
		return domain.PageTrajets{}, fmt.Errorf("recherche des trajets : %w", err)
	}
	defer rows.Close()

	trajets := []domain.Trajet{}
	for rows.Next() {
		trajet, err := lireTrajet(rows)
		if err != nil {
			return domain.PageTrajets{}, fmt.Errorf("lecture des trajets : %w", err)
		}
		trajets = append(trajets, trajet)
	}
	if err := rows.Err(); err != nil {
		return domain.PageTrajets{}, fmt.Errorf("lecture des trajets : %w", err)
	}

	page := domain.PageTrajets{Trajets: trajets}
	if len(trajets) > limite {
		page.Trajets = trajets[:limite]
		dernier := page.Trajets[limite-1]
		page.CurseurSuivant = encoderCurseur(dernier.Depart, dernier.Identifiant)
	}
	return page, nil
}

// encoderCurseur : le curseur reste opaque pour le client, base64url de <departement RFC 3339>|<identifiant>.
func encoderCurseur(depart time.Time, identifiant string) string {
	brut := depart.UTC().Format(time.RFC3339) + "|" + identifiant
	return base64.RawURLEncoding.EncodeToString([]byte(brut))
}

// decoderCurseur refuse tout curseur qui ne vient pas de encoderCurseur : le client reçoit un 400, pas un 500.
func decoderCurseur(curseur string) (string, string, error) {
	brut, err := base64.RawURLEncoding.DecodeString(curseur)
	if err != nil {
		return "", "", domain.ErrCurseurInvalide
	}
	depart, identifiant, trouve := strings.Cut(string(brut), "|")
	if !trouve || identifiant == "" {
		return "", "", domain.ErrCurseurInvalide
	}
	if _, err := time.Parse(time.RFC3339, depart); err != nil {
		return "", "", domain.ErrCurseurInvalide
	}
	return depart, identifiant, nil
}

// ParConducteur : trajets d'un conducteur, supprimés exclus, tous statuts (UC10).
func (r *TrajetRepository) ParConducteur(ctx context.Context, conducteur string) ([]domain.Trajet, error) {
	rows, err := r.db.QueryContext(ctx, requeteParConducteur, conducteur)
	if err != nil {
		return nil, fmt.Errorf("lecture des trajets du conducteur : %w", err)
	}
	defer rows.Close()

	trajets := []domain.Trajet{}
	for rows.Next() {
		trajet, err := lireTrajet(rows)
		if err != nil {
			return nil, fmt.Errorf("lecture des trajets du conducteur : %w", err)
		}
		trajets = append(trajets, trajet)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("lecture des trajets du conducteur : %w", err)
	}
	return trajets, nil
}
