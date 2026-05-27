package repository

import (
	"context"
	"database/sql"

	"coast-monitoring/internal/service"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

type SessionRepository struct {
	db *pgxpool.Pool
}

func NewSessionRepository(db *pgxpool.Pool) SessionRepository {
	return SessionRepository{db: db}
}

func (r SessionRepository) CreateSession(ctx context.Context, input service.CreateSessionRecord) (service.Session, error) {
	var ip any
	if input.IP != "" {
		ip = input.IP
	}
	return scanSession(r.db.QueryRow(ctx, `
		INSERT INTO sessions (user_id, token_hash, csrf_token_hash, user_agent, ip, expires_at)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING id, user_id, expires_at, created_at, revoked_at
	`, input.UserID, input.TokenHash, input.CSRFTokenHash, input.UserAgent, ip, input.ExpiresAt))
}

func (r SessionRepository) GetSessionByTokenHash(ctx context.Context, tokenHash []byte) (service.Session, error) {
	return scanSession(r.db.QueryRow(ctx, `
		SELECT id, user_id, expires_at, created_at, revoked_at
		FROM sessions
		WHERE token_hash = $1
	`, tokenHash))
}

func (r SessionRepository) RevokeSession(ctx context.Context, id uuid.UUID) error {
	_, err := r.db.Exec(ctx, `UPDATE sessions SET revoked_at = now() WHERE id = $1`, id)
	return err
}

type sessionScanner interface {
	Scan(dest ...any) error
}

func scanSession(row sessionScanner) (service.Session, error) {
	var session service.Session
	var revokedAt sql.NullTime
	if err := row.Scan(&session.ID, &session.UserID, &session.ExpiresAt, &session.CreatedAt, &revokedAt); err != nil {
		return service.Session{}, translateError(err)
	}
	if revokedAt.Valid {
		session.RevokedAt = &revokedAt.Time
	}
	return session, nil
}
