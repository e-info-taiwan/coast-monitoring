package repository

import (
	"context"
	"time"

	"coast-monitoring/internal/service"

	"github.com/jackc/pgx/v5/pgxpool"
)

type LoginAttemptRepository struct {
	db *pgxpool.Pool
}

func NewLoginAttemptRepository(db *pgxpool.Pool) LoginAttemptRepository {
	return LoginAttemptRepository{db: db}
}

func (r LoginAttemptRepository) RecordLoginAttempt(ctx context.Context, input service.LoginAttemptRecord) error {
	var ip any
	if input.IP != "" {
		ip = input.IP
	}
	_, err := r.db.Exec(ctx, `
		INSERT INTO login_attempts (email, ip, success)
		VALUES ($1, $2, $3)
	`, input.Email, ip, input.Success)
	return translateError(err)
}

func (r LoginAttemptRepository) CountRecentFailedLoginAttempts(ctx context.Context, email, ip string, since time.Time) (int, error) {
	var count int
	var err error
	if ip == "" {
		err = r.db.QueryRow(ctx, `
			SELECT count(*)
			FROM login_attempts
			WHERE success = false
				AND attempted_at >= $2
				AND email = $1
		`, email, since).Scan(&count)
		return count, translateError(err)
	}
	err = r.db.QueryRow(ctx, `
		SELECT count(*)
		FROM login_attempts
		WHERE success = false
			AND attempted_at >= $3
			AND (email = $1 OR ip = $2)
	`, email, ip, since).Scan(&count)
	return count, translateError(err)
}
