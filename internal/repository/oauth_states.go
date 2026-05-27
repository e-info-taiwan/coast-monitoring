package repository

import (
	"context"
	"database/sql"
	"time"

	"coast-monitoring/internal/service"

	"github.com/jackc/pgx/v5/pgxpool"
)

type OAuthStateRepository struct {
	db *pgxpool.Pool
}

func NewOAuthStateRepository(db *pgxpool.Pool) OAuthStateRepository {
	return OAuthStateRepository{db: db}
}

func (r OAuthStateRepository) CreateOAuthState(ctx context.Context, input service.CreateOAuthStateRecord) (service.OAuthState, error) {
	return scanOAuthState(r.db.QueryRow(ctx, `
		INSERT INTO oauth_states (state_hash, redirect_path, expires_at)
		VALUES ($1, $2, $3)
		RETURNING id, redirect_path, expires_at, created_at, consumed_at
	`, input.StateHash, input.RedirectPath, input.ExpiresAt))
}

func (r OAuthStateRepository) ConsumeOAuthState(ctx context.Context, stateHash []byte, now time.Time) (service.OAuthState, error) {
	return scanOAuthState(r.db.QueryRow(ctx, `
		UPDATE oauth_states
		SET consumed_at = $2
		WHERE state_hash = $1
			AND consumed_at IS NULL
			AND expires_at > $2
		RETURNING id, redirect_path, expires_at, created_at, consumed_at
	`, stateHash, now))
}

func scanOAuthState(row sessionScanner) (service.OAuthState, error) {
	var state service.OAuthState
	var consumedAt sql.NullTime
	if err := row.Scan(&state.ID, &state.RedirectPath, &state.ExpiresAt, &state.CreatedAt, &consumedAt); err != nil {
		return service.OAuthState{}, translateError(err)
	}
	if consumedAt.Valid {
		state.ConsumedAt = &consumedAt.Time
	}
	return state, nil
}
