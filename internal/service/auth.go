package service

import (
	"context"
	"time"

	"github.com/google/uuid"
)

type Session struct {
	ID        uuid.UUID
	UserID    uuid.UUID
	ExpiresAt time.Time
	CreatedAt time.Time
	RevokedAt *time.Time
}

type CreateSessionRecord struct {
	UserID        uuid.UUID
	TokenHash     []byte
	CSRFTokenHash []byte
	UserAgent     string
	IP            string
	ExpiresAt     time.Time
}

type SessionRepository interface {
	CreateSession(ctx context.Context, input CreateSessionRecord) (Session, error)
	GetSessionByTokenHash(ctx context.Context, tokenHash []byte) (Session, error)
	RevokeSession(ctx context.Context, id uuid.UUID) error
}
