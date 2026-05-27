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

func TestFindLoginUserByEmailNormalizesEmail(t *testing.T) {
	repo := &fakeAuthUserRepository{loginUser: LoginUser{
		ID:           uuid.New(),
		Email:        "user@example.com",
		Name:         "User",
		Role:         policy.RoleVolunteer,
		Status:       policy.StatusActive,
		PasswordHash: "hash",
	}}
	svc := AuthService{Users: repo}

	user, err := svc.FindLoginUserByEmail(context.Background(), " USER@EXAMPLE.COM ")
	if err != nil {
		t.Fatalf("FindLoginUserByEmail error = %v", err)
	}

	if user.PasswordHash != "hash" {
		t.Fatal("login user did not include password hash for auth flow")
	}
	if repo.email != "user@example.com" {
		t.Fatalf("repository email = %q, want normalized email", repo.email)
	}
}

func TestFindLoginUserByGoogleSubTrimsSub(t *testing.T) {
	repo := &fakeAuthUserRepository{loginUser: LoginUser{
		ID:        uuid.New(),
		Email:     "user@example.com",
		Name:      "User",
		Role:      policy.RoleVolunteer,
		Status:    policy.StatusActive,
		GoogleSub: stringPtr("google-sub"),
	}}
	svc := AuthService{Users: repo}

	_, err := svc.FindLoginUserByGoogleSub(context.Background(), " google-sub ")
	if err != nil {
		t.Fatalf("FindLoginUserByGoogleSub error = %v", err)
	}

	if repo.googleSub != "google-sub" {
		t.Fatalf("repository google sub = %q, want trimmed value", repo.googleSub)
	}
}

func TestAttachGoogleSubTrimsSub(t *testing.T) {
	userID := uuid.New()
	repo := &fakeAuthUserRepository{loginUser: LoginUser{ID: userID}}
	svc := AuthService{Users: repo}

	_, err := svc.AttachGoogleSub(context.Background(), userID, " google-sub ")
	if err != nil {
		t.Fatalf("AttachGoogleSub error = %v", err)
	}

	if repo.userID != userID {
		t.Fatalf("repository userID = %s, want %s", repo.userID, userID)
	}
	if repo.googleSub != "google-sub" {
		t.Fatalf("repository google sub = %q, want trimmed value", repo.googleSub)
	}
}

func TestAnyAdminExistsDelegates(t *testing.T) {
	repo := &fakeAuthUserRepository{anyAdminExists: true}
	svc := AuthService{Users: repo}

	exists, err := svc.AnyAdminExists(context.Background())
	if err != nil {
		t.Fatalf("AnyAdminExists error = %v", err)
	}
	if !exists {
		t.Fatal("AnyAdminExists = false, want true")
	}
}

func TestCreateBootstrapAdminCreatesFirstActiveAdmin(t *testing.T) {
	repo := &fakeAuthUserRepository{loginUser: LoginUser{ID: uuid.New(), Email: "admin@example.com"}}
	svc := AuthService{Users: repo}

	_, err := svc.CreateBootstrapAdmin(context.Background(), " Admin@Example.COM ", " Admin ", " google-sub ")
	if err != nil {
		t.Fatalf("CreateBootstrapAdmin error = %v", err)
	}

	if repo.created.Email != "admin@example.com" {
		t.Fatalf("created email = %q, want normalized email", repo.created.Email)
	}
	if repo.created.Name != "Admin" {
		t.Fatalf("created name = %q, want trimmed name", repo.created.Name)
	}
	if repo.created.Role != policy.RoleAdmin || repo.created.Status != policy.StatusActive {
		t.Fatalf("created role/status = %s/%s, want active admin", repo.created.Role, repo.created.Status)
	}
	if repo.created.GoogleSub == nil || *repo.created.GoogleSub != "google-sub" {
		t.Fatalf("created google sub = %v, want google-sub", repo.created.GoogleSub)
	}
}

func TestCreateBootstrapAdminRejectsWhenAdminExists(t *testing.T) {
	repo := &fakeAuthUserRepository{anyAdminExists: true}
	svc := AuthService{Users: repo}

	_, err := svc.CreateBootstrapAdmin(context.Background(), "admin@example.com", "Admin", "google-sub")

	if !errors.Is(err, ErrConflict) {
		t.Fatalf("CreateBootstrapAdmin error = %v, want %v", err, ErrConflict)
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

type fakeAuthUserRepository struct {
	loginUser      LoginUser
	anyAdminExists bool
	email          string
	googleSub      string
	userID         uuid.UUID
	created        CreateUserRecord
}

func (r *fakeAuthUserRepository) FindLoginUserByEmail(ctx context.Context, email string) (LoginUser, error) {
	r.email = email
	return r.loginUser, nil
}

func (r *fakeAuthUserRepository) FindLoginUserByGoogleSub(ctx context.Context, googleSub string) (LoginUser, error) {
	r.googleSub = googleSub
	return r.loginUser, nil
}

func (r *fakeAuthUserRepository) AttachGoogleSub(ctx context.Context, userID uuid.UUID, googleSub string) (LoginUser, error) {
	r.userID = userID
	r.googleSub = googleSub
	return r.loginUser, nil
}

func (r *fakeAuthUserRepository) AnyAdminExists(ctx context.Context) (bool, error) {
	return r.anyAdminExists, nil
}

func (r *fakeAuthUserRepository) CreateBootstrapAdmin(ctx context.Context, input CreateUserRecord) (LoginUser, error) {
	r.created = input
	return r.loginUser, nil
}
