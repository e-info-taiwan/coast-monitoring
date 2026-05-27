package repository

import (
	"context"

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
