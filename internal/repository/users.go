package repository

import (
	"context"
	"database/sql"

	"coast-monitoring/internal/policy"
	"coast-monitoring/internal/service"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

type UserRepository struct {
	db *pgxpool.Pool
}

func NewUserRepository(db *pgxpool.Pool) UserRepository {
	return UserRepository{db: db}
}

func (r UserRepository) ListUsers(ctx context.Context) ([]service.User, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id, email::text, name, role::text, status::text, google_sub, password_hash IS NOT NULL, created_at, updated_at
		FROM users
		ORDER BY email
	`)
	if err != nil {
		return nil, translateError(err)
	}
	defer rows.Close()

	var users []service.User
	for rows.Next() {
		user, err := scanUser(rows)
		if err != nil {
			return nil, err
		}
		users = append(users, user)
	}
	return users, translateError(rows.Err())
}

func (r UserRepository) CreateUser(ctx context.Context, input service.CreateUserRecord) (service.User, error) {
	return scanUser(r.db.QueryRow(ctx, `
		INSERT INTO users (email, name, role, status, google_sub, password_hash)
		VALUES ($1, $2, $3, $4, $5, NULLIF($6, ''))
		RETURNING id, email::text, name, role::text, status::text, google_sub, password_hash IS NOT NULL, created_at, updated_at
	`, input.Email, input.Name, input.Role, input.Status, input.GoogleSub, input.PasswordHash))
}

func (r UserRepository) UpdateUser(ctx context.Context, id uuid.UUID, input service.UpdateUserRecord) (service.User, error) {
	if input.PasswordHash == nil {
		return scanUser(r.db.QueryRow(ctx, `
			UPDATE users
			SET email = $2, name = $3, role = $4, status = $5, google_sub = $6, updated_at = now()
			WHERE id = $1
			RETURNING id, email::text, name, role::text, status::text, google_sub, password_hash IS NOT NULL, created_at, updated_at
		`, id, input.Email, input.Name, input.Role, input.Status, input.GoogleSub))
	}
	return scanUser(r.db.QueryRow(ctx, `
		UPDATE users
		SET email = $2, name = $3, role = $4, status = $5, google_sub = $6, password_hash = NULLIF($7, ''), updated_at = now()
		WHERE id = $1
		RETURNING id, email::text, name, role::text, status::text, google_sub, password_hash IS NOT NULL, created_at, updated_at
	`, id, input.Email, input.Name, input.Role, input.Status, input.GoogleSub, *input.PasswordHash))
}

func (r UserRepository) DisableUser(ctx context.Context, id uuid.UUID) error {
	tag, err := r.db.Exec(ctx, `UPDATE users SET status = 'disabled', updated_at = now() WHERE id = $1`, id)
	return requireRowsAffected(tag, err)
}

type userScanner interface {
	Scan(dest ...any) error
}

func scanUser(row userScanner) (service.User, error) {
	var user service.User
	var role string
	var status string
	var googleSub sql.NullString
	if err := row.Scan(
		&user.ID,
		&user.Email,
		&user.Name,
		&role,
		&status,
		&googleSub,
		&user.HasPassword,
		&user.CreatedAt,
		&user.UpdatedAt,
	); err != nil {
		return service.User{}, translateError(err)
	}
	user.Role = policy.Role(role)
	user.Status = policy.Status(status)
	if googleSub.Valid {
		user.GoogleSub = &googleSub.String
	}
	return user, nil
}
