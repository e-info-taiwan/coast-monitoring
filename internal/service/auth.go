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

type LoginUser struct {
	ID           uuid.UUID
	Email        string
	Name         string
	Role         policy.Role
	Status       policy.Status
	GoogleSub    *string
	PasswordHash string
}

type AuthUserRepository interface {
	FindLoginUserByEmail(ctx context.Context, email string) (LoginUser, error)
	FindLoginUserByGoogleSub(ctx context.Context, googleSub string) (LoginUser, error)
	AttachGoogleSub(ctx context.Context, userID uuid.UUID, googleSub string) (LoginUser, error)
	AnyAdminExists(ctx context.Context) (bool, error)
}

type AuthService struct {
	Sessions SessionAuthenticator
	Users    AuthUserRepository
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

func (s AuthService) FindLoginUserByEmail(ctx context.Context, email string) (LoginUser, error) {
	email = strings.ToLower(strings.TrimSpace(email))
	if email == "" {
		return LoginUser{}, ErrValidation
	}
	return s.Users.FindLoginUserByEmail(ctx, email)
}

func (s AuthService) FindLoginUserByGoogleSub(ctx context.Context, googleSub string) (LoginUser, error) {
	googleSub = strings.TrimSpace(googleSub)
	if googleSub == "" {
		return LoginUser{}, ErrValidation
	}
	return s.Users.FindLoginUserByGoogleSub(ctx, googleSub)
}

func (s AuthService) AttachGoogleSub(ctx context.Context, userID uuid.UUID, googleSub string) (LoginUser, error) {
	googleSub = strings.TrimSpace(googleSub)
	if userID == uuid.Nil || googleSub == "" {
		return LoginUser{}, ErrValidation
	}
	return s.Users.AttachGoogleSub(ctx, userID, googleSub)
}

func (s AuthService) AnyAdminExists(ctx context.Context) (bool, error) {
	return s.Users.AnyAdminExists(ctx)
}
