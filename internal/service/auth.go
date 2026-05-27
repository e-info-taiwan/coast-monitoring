package service

import (
	"context"
	"errors"
	"strings"
	"time"

	"coast-monitoring/internal/auth"
	"coast-monitoring/internal/policy"

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

type SessionAuthenticator interface {
	GetUserByValidSession(ctx context.Context, sessionTokenHash, csrfTokenHash []byte) (policy.User, error)
}

type AuthService struct {
	Sessions SessionAuthenticator
}

func (s AuthService) AuthenticateSession(ctx context.Context, sessionToken, csrfToken string) (policy.User, error) {
	sessionToken = strings.TrimSpace(sessionToken)
	csrfToken = strings.TrimSpace(csrfToken)
	if sessionToken == "" || csrfToken == "" {
		return policy.User{}, ErrUnauthorized
	}

	user, err := s.Sessions.GetUserByValidSession(ctx, auth.HashToken(sessionToken), auth.HashToken(csrfToken))
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return policy.User{}, ErrUnauthorized
		}
		return policy.User{}, err
	}
	if user.Status != policy.StatusActive {
		return policy.User{}, ErrUnauthorized
	}
	return user, nil
}
