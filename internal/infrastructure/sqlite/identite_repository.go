package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/alaa-ilehi/wasalni/internal/domain"
)

// CredentialRepository : adaptateur SQLite du port domain.CredentialRepository (UC13).
// La ligne porte le sel, le condensé et le nombre d'itérations : le coût peut donc augmenter plus
// tard sans invalider les comptes existants.
type CredentialRepository struct {
	db *sql.DB
}

func NouveauCredentialRepository(db *sql.DB) *CredentialRepository {
	return &CredentialRepository{db: db}
}

// Enregistrer compte sur la clé primaire : c'est elle, et non une lecture préalable, qui interdit
// deux mots de passe pour un même identifiant, y compris sous inscriptions simultanées.
func (r *CredentialRepository) Enregistrer(ctx context.Context, credential domain.Credential) error {
	_, err := r.db.ExecContext(ctx,
		`INSERT INTO credentials (user_id, salt, hash, iterations) VALUES (?, ?, ?, ?)`,
		credential.Identifiant, credential.Sel, credential.Condensat, credential.Iterations)
	if err != nil {
		if estViolationUnique(err) {
			return domain.ErrIdentifiantDejaInscrit
		}
		return fmt.Errorf("écriture du mot de passe : %w", err)
	}
	return nil
}

// ParIdentifiant rend le condensé, ou ErrIdentifiantsInvalides quand l'identifiant n'a pas de mot de
// passe : l'appelant ne peut donc pas distinguer « inconnu » de « mot de passe faux ».
func (r *CredentialRepository) ParIdentifiant(ctx context.Context, identifiant string) (domain.Credential, error) {
	credential := domain.Credential{Identifiant: identifiant}
	err := r.db.QueryRowContext(ctx,
		`SELECT salt, hash, iterations FROM credentials WHERE user_id = ?`, identifiant).
		Scan(&credential.Sel, &credential.Condensat, &credential.Iterations)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.Credential{}, domain.ErrIdentifiantsInvalides
	}
	if err != nil {
		return domain.Credential{}, fmt.Errorf("lecture du mot de passe : %w", err)
	}
	return credential, nil
}

// SessionRepository : adaptateur SQLite du port domain.SessionRepository (UC13).
// Seul le hachage du jeton est stocké : une copie de la base ne permet pas de rejouer une session.
type SessionRepository struct {
	db *sql.DB
}

func NouveauSessionRepository(db *sql.DB) *SessionRepository {
	return &SessionRepository{db: db}
}

func (r *SessionRepository) Creer(ctx context.Context, session domain.Session) error {
	_, err := r.db.ExecContext(ctx,
		`INSERT INTO sessions (token_hash, user_id, created_at, expires_at) VALUES (?, ?, ?, ?)`,
		session.Jeton, session.Identifiant,
		session.CreeLe.UTC().Format(formatHorodatage), session.ExpireLe.UTC().Format(formatHorodatage))
	if err != nil {
		return fmt.Errorf("création de la session : %w", err)
	}
	return nil
}

// ParJeton purge d'abord les sessions échues (purge paresseuse : aucune tâche périodique à
// exploiter), puis rend celle qui est demandée si elle est encore valable.
func (r *SessionRepository) ParJeton(ctx context.Context, jeton string, maintenant time.Time) (domain.Session, error) {
	instant := maintenant.UTC().Format(formatHorodatage)
	if _, err := r.db.ExecContext(ctx, `DELETE FROM sessions WHERE expires_at <= ?`, instant); err != nil {
		return domain.Session{}, fmt.Errorf("purge des sessions expirées : %w", err)
	}

	session := domain.Session{Jeton: jeton}
	var creeLe, expireLe string
	err := r.db.QueryRowContext(ctx,
		`SELECT user_id, created_at, expires_at FROM sessions WHERE token_hash = ?`, jeton).
		Scan(&session.Identifiant, &creeLe, &expireLe)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.Session{}, domain.ErrSessionInvalide
	}
	if err != nil {
		return domain.Session{}, fmt.Errorf("lecture de la session : %w", err)
	}

	if session.CreeLe, err = time.Parse(formatHorodatage, creeLe); err != nil {
		return domain.Session{}, fmt.Errorf("date de création illisible %q : %w", creeLe, err)
	}
	if session.ExpireLe, err = time.Parse(formatHorodatage, expireLe); err != nil {
		return domain.Session{}, fmt.Errorf("date d'expiration illisible %q : %w", expireLe, err)
	}
	return session, nil
}

// Supprimer invalide le jeton. Une session déjà absente est signalée comme invalide : la déconnexion
// n'est jamais silencieuse.
func (r *SessionRepository) Supprimer(ctx context.Context, jeton string) error {
	resultat, err := r.db.ExecContext(ctx, `DELETE FROM sessions WHERE token_hash = ?`, jeton)
	if err != nil {
		return fmt.Errorf("suppression de la session : %w", err)
	}
	supprimees, err := resultat.RowsAffected()
	if err != nil {
		return fmt.Errorf("suppression de la session : %w", err)
	}
	if supprimees == 0 {
		return domain.ErrSessionInvalide
	}
	return nil
}
