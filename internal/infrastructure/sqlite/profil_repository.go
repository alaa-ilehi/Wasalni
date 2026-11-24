package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/alaa-ilehi/wasalni/internal/domain"
)

const formatHorodatage = time.RFC3339

// ProfilRepository : adaptateur SQLite du port domain.ProfilRepository.
type ProfilRepository struct {
	db *sql.DB
}

func NouveauProfilRepository(db *sql.DB) *ProfilRepository {
	return &ProfilRepository{db: db}
}

// Enregistrer insère ou met à jour le profil dans une transaction.
// Les horodatages sont posés par la base ; le booléen indique une création.
func (r *ProfilRepository) Enregistrer(ctx context.Context, p domain.Profil) (domain.Profil, bool, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return domain.Profil{}, false, fmt.Errorf("début de transaction : %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	var existe int
	switch err := tx.QueryRowContext(ctx, `SELECT 1 FROM users WHERE id = ?`, p.Identifiant).Scan(&existe); {
	case errors.Is(err, sql.ErrNoRows):
	case err != nil:
		return domain.Profil{}, false, fmt.Errorf("lecture du profil : %w", err)
	default:
		// le profil existe : mise à jour
		const requete = `
UPDATE users SET name = ?, age = ?, sex = ?, photo = ?,
       updated_at = strftime('%Y-%m-%dT%H:%M:%SZ', 'now')
WHERE id = ?
RETURNING created_at, updated_at`
		var creeLe, majLe string
		if err := tx.QueryRowContext(ctx, requete, p.Nom, p.Age, sexeSQL(p.Sexe), photoSQL(p.Photo), p.Identifiant).
			Scan(&creeLe, &majLe); err != nil {
			return domain.Profil{}, false, fmt.Errorf("mise à jour du profil : %w", err)
		}
		if err := tx.Commit(); err != nil {
			return domain.Profil{}, false, fmt.Errorf("validation de la transaction : %w", err)
		}
		if err := horodater(&p, creeLe, majLe); err != nil {
			return domain.Profil{}, false, err
		}
		return p, false, nil
	}

	const requete = `
INSERT INTO users (id, name, age, sex, photo)
VALUES (?, ?, ?, ?, ?)
RETURNING created_at, updated_at`
	var creeLe, majLe string
	if err := tx.QueryRowContext(ctx, requete, p.Identifiant, p.Nom, p.Age, sexeSQL(p.Sexe), photoSQL(p.Photo)).
		Scan(&creeLe, &majLe); err != nil {
		return domain.Profil{}, false, fmt.Errorf("création du profil : %w", err)
	}
	if err := tx.Commit(); err != nil {
		return domain.Profil{}, false, fmt.Errorf("validation de la transaction : %w", err)
	}
	if err := horodater(&p, creeLe, majLe); err != nil {
		return domain.Profil{}, false, err
	}
	return p, true, nil
}

// ParIdentifiant renvoie le profil, ou domain.ErrProfilIntrouvable.
func (r *ProfilRepository) ParIdentifiant(ctx context.Context, identifiant string) (domain.Profil, error) {
	const requete = `
SELECT id, name, age, sex, photo, created_at, updated_at
FROM users WHERE id = ?`
	var (
		profil domain.Profil
		sexe   sql.NullString
		photo  sql.NullString
		creeLe string
		majLe  string
	)
	err := r.db.QueryRowContext(ctx, requete, identifiant).
		Scan(&profil.Identifiant, &profil.Nom, &profil.Age, &sexe, &photo, &creeLe, &majLe)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.Profil{}, domain.ErrProfilIntrouvable
	}
	if err != nil {
		return domain.Profil{}, fmt.Errorf("lecture du profil : %w", err)
	}
	profil.Sexe = domain.Sexe(sexe.String)
	profil.Photo = photo.String
	if err := horodater(&profil, creeLe, majLe); err != nil {
		return domain.Profil{}, err
	}
	return profil, nil
}

// sexeSQL écrit NULL quand le sexe n'est pas renseigné (colonne facultative).
func sexeSQL(sexe domain.Sexe) any {
	if sexe == "" {
		return nil
	}
	return string(sexe)
}

// photoSQL écrit NULL quand le profil n'a pas de photo.
func photoSQL(photo string) any {
	if photo == "" {
		return nil
	}
	return photo
}

func horodater(p *domain.Profil, creeLe, majLe string) error {
	cree, err := time.Parse(formatHorodatage, creeLe)
	if err != nil {
		return fmt.Errorf("horodatage de création illisible %q : %w", creeLe, err)
	}
	maj, err := time.Parse(formatHorodatage, majLe)
	if err != nil {
		return fmt.Errorf("horodatage de mise à jour illisible %q : %w", majLe, err)
	}
	p.CreeLe = cree
	p.MajLe = maj
	return nil
}
