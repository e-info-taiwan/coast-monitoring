package service

import (
	"bytes"
	"context"
	"errors"
	"testing"

	"coast-monitoring/internal/auth"
	"coast-monitoring/internal/policy"

	"github.com/google/uuid"
)

func TestAuthenticateSessionReturnsActiveUser(t *testing.T) {
	sessionToken := "session-token"
	csrfToken := "csrf-token"
	user := policy.User{
		ID:     uuid.New(),
		Email:  "user@example.com",
		Name:   "User",
		Role:   policy.RoleVolunteer,
		Status: policy.StatusActive,
	}
	repo := &fakeSessionAuthenticator{user: user}
	svc := AuthService{Sessions: repo}

	got, err := svc.AuthenticateSession(context.Background(), sessionToken, csrfToken)
	if err != nil {
		t.Fatalf("AuthenticateSession error = %v", err)
	}

	if got != user {
		t.Fatalf("AuthenticateSession user = %+v, want %+v", got, user)
	}
	if !bytes.Equal(repo.sessionTokenHash, auth.HashToken(sessionToken)) {
		t.Fatal("session token was not hashed before lookup")
	}
	if !bytes.Equal(repo.csrfTokenHash, auth.HashToken(csrfToken)) {
		t.Fatal("csrf token was not hashed before lookup")
	}
}

func TestAuthenticateSessionRejectsEmptyTokens(t *testing.T) {
	svc := AuthService{Sessions: &fakeSessionAuthenticator{}}

	_, err := svc.AuthenticateSession(context.Background(), "", "csrf-token")
	if !errors.Is(err, ErrUnauthorized) {
		t.Fatalf("empty session token error = %v, want %v", err, ErrUnauthorized)
	}

	_, err = svc.AuthenticateSession(context.Background(), "session-token", "")
	if !errors.Is(err, ErrUnauthorized) {
		t.Fatalf("empty csrf token error = %v, want %v", err, ErrUnauthorized)
	}
}

func TestAuthenticateSessionRejectsExpiredSession(t *testing.T) {
	svc := AuthService{Sessions: &fakeSessionAuthenticator{err: ErrNotFound}}

	_, err := svc.AuthenticateSession(context.Background(), "session-token", "csrf-token")

	if !errors.Is(err, ErrUnauthorized) {
		t.Fatalf("expired session error = %v, want %v", err, ErrUnauthorized)
	}
}

func TestAuthenticateSessionRejectsRevokedSession(t *testing.T) {
	svc := AuthService{Sessions: &fakeSessionAuthenticator{err: ErrNotFound}}

	_, err := svc.AuthenticateSession(context.Background(), "session-token", "csrf-token")

	if !errors.Is(err, ErrUnauthorized) {
		t.Fatalf("revoked session error = %v, want %v", err, ErrUnauthorized)
	}
}

func TestAuthenticateSessionRejectsCSRFMismatch(t *testing.T) {
	svc := AuthService{Sessions: &fakeSessionAuthenticator{err: ErrNotFound}}

	_, err := svc.AuthenticateSession(context.Background(), "session-token", "wrong-csrf")

	if !errors.Is(err, ErrUnauthorized) {
		t.Fatalf("csrf mismatch error = %v, want %v", err, ErrUnauthorized)
	}
}

func TestAuthenticateSessionRejectsDisabledUser(t *testing.T) {
	svc := AuthService{Sessions: &fakeSessionAuthenticator{user: policy.User{
		ID:     uuid.New(),
		Email:  "disabled@example.com",
		Name:   "Disabled",
		Role:   policy.RoleVolunteer,
		Status: policy.StatusDisabled,
	}}}

	_, err := svc.AuthenticateSession(context.Background(), "session-token", "csrf-token")

	if !errors.Is(err, ErrUnauthorized) {
		t.Fatalf("disabled user error = %v, want %v", err, ErrUnauthorized)
	}
}

type fakeSessionAuthenticator struct {
	user             policy.User
	err              error
	sessionTokenHash []byte
	csrfTokenHash    []byte
}

func (r *fakeSessionAuthenticator) GetUserByValidSession(ctx context.Context, sessionTokenHash, csrfTokenHash []byte) (policy.User, error) {
	r.sessionTokenHash = append([]byte(nil), sessionTokenHash...)
	r.csrfTokenHash = append([]byte(nil), csrfTokenHash...)
	if r.err != nil {
		return policy.User{}, r.err
	}
	return r.user, nil
}
