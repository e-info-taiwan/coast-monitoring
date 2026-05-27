package httpx

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"coast-monitoring/internal/auth"
	"coast-monitoring/internal/policy"
	"coast-monitoring/internal/service"

	"github.com/google/uuid"
)

func TestSessionReturnsOnlyPublicUserFields(t *testing.T) {
	user := policy.User{
		ID:     uuid.New(),
		Email:  "admin@example.com",
		Name:   "Admin",
		Role:   policy.RoleAdmin,
		Status: policy.StatusActive,
	}
	handlers := testAuthHandlers()
	handlers.Auth = &fakeHTTPAuthService{sessionUser: user}
	router := NewRouter(Dependencies{AuthHandlers: handlers})

	req := httptest.NewRequest(http.MethodGet, "/api/session", nil)
	req.AddCookie(&http.Cookie{Name: "coast_session", Value: "session-token"})
	req.AddCookie(&http.Cookie{Name: "coast_csrf", Value: "csrf-token"})
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	var body map[string]any
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("Decode body error = %v", err)
	}
	userBody, ok := body["user"].(map[string]any)
	if !ok {
		t.Fatalf("user body = %#v, want object", body["user"])
	}
	wantKeys := map[string]bool{"id": true, "email": true, "name": true, "role": true}
	if len(userBody) != len(wantKeys) {
		t.Fatalf("user field count = %d, body = %#v", len(userBody), userBody)
	}
	for key := range wantKeys {
		if _, ok := userBody[key]; !ok {
			t.Fatalf("user body missing %q: %#v", key, userBody)
		}
	}
	for _, forbidden := range []string{"password_hash", "passwordHash", "google_sub", "googleSub", "status", "created_at", "createdAt", "updated_at", "updatedAt"} {
		if _, ok := userBody[forbidden]; ok {
			t.Fatalf("session response leaked %q: %#v", forbidden, userBody)
		}
		if _, ok := body[forbidden]; ok {
			t.Fatalf("top-level session response leaked %q: %#v", forbidden, body)
		}
	}
}

func TestPasswordLoginCreatesSessionCookiesAndRecordsSuccessfulAttempt(t *testing.T) {
	passwordHash, err := auth.HashPassword("correct-password")
	if err != nil {
		t.Fatalf("HashPassword error = %v", err)
	}
	user := service.LoginUser{
		ID:           uuid.New(),
		Email:        "user@example.com",
		Name:         "User",
		Role:         policy.RoleVolunteer,
		Status:       policy.StatusActive,
		PasswordHash: passwordHash,
	}
	authSvc := &fakeHTTPAuthService{loginUserByEmail: user}
	sessions := &fakeHTTPSessionStore{}
	attempts := &fakeHTTPLoginAttemptRecorder{}
	handlers := testAuthHandlers()
	handlers.Auth = authSvc
	handlers.Sessions = sessions
	handlers.LoginAttempts = attempts

	reqBody := bytes.NewBufferString(`{"email":" USER@example.com ","password":"correct-password"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/auth/password", reqBody)
	req.Header.Set("Content-Type", "application/json")
	req.RemoteAddr = "192.0.2.10:12345"
	rec := httptest.NewRecorder()

	handlers.PasswordLogin(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d, body %s", rec.Code, http.StatusOK, rec.Body.String())
	}
	if authSvc.email != "user@example.com" {
		t.Fatalf("lookup email = %q, want normalized email", authSvc.email)
	}
	if len(attempts.records) != 1 || !attempts.records[0].Success || attempts.records[0].Email != "user@example.com" {
		t.Fatalf("login attempts = %+v, want one successful normalized attempt", attempts.records)
	}

	sessionCookie := findCookie(rec.Result().Cookies(), "coast_session")
	if sessionCookie == nil {
		t.Fatal("coast_session cookie was not set")
	}
	csrfCookie := findCookie(rec.Result().Cookies(), "coast_csrf")
	if csrfCookie == nil {
		t.Fatal("coast_csrf cookie was not set")
	}
	assertSessionCookieAttrs(t, sessionCookie, true)
	assertSessionCookieAttrs(t, csrfCookie, false)
	if sessionCookie.Value == "" || csrfCookie.Value == "" || sessionCookie.Value == csrfCookie.Value {
		t.Fatalf("cookie token values are invalid: session=%q csrf=%q", sessionCookie.Value, csrfCookie.Value)
	}
	if sessions.created.UserID != user.ID {
		t.Fatalf("session user id = %s, want %s", sessions.created.UserID, user.ID)
	}
	if !bytes.Equal(sessions.created.TokenHash, auth.HashToken(sessionCookie.Value)) {
		t.Fatal("stored session token hash does not match cookie token")
	}
	if !bytes.Equal(sessions.created.CSRFTokenHash, auth.HashToken(csrfCookie.Value)) {
		t.Fatal("stored csrf token hash does not match cookie token")
	}
	if bytes.Equal(sessions.created.TokenHash, []byte(sessionCookie.Value)) {
		t.Fatal("stored raw session token instead of hash")
	}

	var body SessionResponse
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("Decode body error = %v", err)
	}
	if !body.Authenticated || body.User == nil || body.User.ID != user.ID.String() {
		t.Fatalf("session response = %+v, want authenticated user", body)
	}
	if body.CSRFToken != csrfCookie.Value {
		t.Fatalf("csrf response token = %q, want cookie value", body.CSRFToken)
	}
}

func TestPasswordLoginRejectsDisabledUserAndRecordsFailedAttempt(t *testing.T) {
	passwordHash, err := auth.HashPassword("correct-password")
	if err != nil {
		t.Fatalf("HashPassword error = %v", err)
	}
	handlers := testAuthHandlers()
	handlers.Auth = &fakeHTTPAuthService{loginUserByEmail: service.LoginUser{
		ID:           uuid.New(),
		Email:        "disabled@example.com",
		Name:         "Disabled",
		Role:         policy.RoleVolunteer,
		Status:       policy.StatusDisabled,
		PasswordHash: passwordHash,
	}}
	attempts := &fakeHTTPLoginAttemptRecorder{}
	handlers.LoginAttempts = attempts

	req := httptest.NewRequest(http.MethodPost, "/api/auth/password", bytes.NewBufferString(`{"email":"disabled@example.com","password":"correct-password"}`))
	rec := httptest.NewRecorder()

	handlers.PasswordLogin(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
	if len(attempts.records) != 1 || attempts.records[0].Success {
		t.Fatalf("login attempts = %+v, want one failed attempt", attempts.records)
	}
}

func TestGoogleStartStoresHashedStateAndRedirects(t *testing.T) {
	oauthStates := &fakeHTTPOAuthStateStore{}
	google := &fakeHTTPGoogleOAuth{}
	handlers := testAuthHandlers()
	handlers.OAuthStates = oauthStates
	handlers.Google = google

	req := httptest.NewRequest(http.MethodGet, "/api/auth/google/start", nil)
	rec := httptest.NewRecorder()

	handlers.GoogleStart(rec, req)

	if rec.Code != http.StatusFound {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusFound)
	}
	if google.state == "" {
		t.Fatal("google auth URL was not built with state")
	}
	if got := rec.Header().Get("Location"); got != "https://accounts.example/auth?state="+url.QueryEscape(google.state) {
		t.Fatalf("redirect location = %q", got)
	}
	if !bytes.Equal(oauthStates.created.StateHash, auth.HashToken(google.state)) {
		t.Fatal("stored oauth state hash does not match state token")
	}
	if bytes.Equal(oauthStates.created.StateHash, []byte(google.state)) {
		t.Fatal("stored raw oauth state instead of hash")
	}
}

func TestGoogleCallbackAttachesExistingEmailUserAndCreatesSession(t *testing.T) {
	userID := uuid.New()
	authSvc := &fakeHTTPAuthService{
		loginUserByGoogleSubErr: service.ErrNotFound,
		loginUserByEmail: service.LoginUser{
			ID:     userID,
			Email:  "user@example.com",
			Name:   "User",
			Role:   policy.RoleVolunteer,
			Status: policy.StatusActive,
		},
		attachedGoogleUser: service.LoginUser{
			ID:        userID,
			Email:     "user@example.com",
			Name:      "User",
			Role:      policy.RoleVolunteer,
			Status:    policy.StatusActive,
			GoogleSub: stringPtr("google-sub"),
		},
	}
	sessions := &fakeHTTPSessionStore{}
	handlers := testAuthHandlers()
	handlers.Auth = authSvc
	handlers.Sessions = sessions
	handlers.OAuthStates = &fakeHTTPOAuthStateStore{}
	handlers.Google = &fakeHTTPGoogleOAuth{identity: GoogleIdentity{
		Subject:       "google-sub",
		Email:         "USER@example.com",
		Name:          "Google User",
		EmailVerified: true,
	}}

	req := httptest.NewRequest(http.MethodGet, "/api/auth/google/callback?state=state-token&code=auth-code", nil)
	rec := httptest.NewRecorder()

	handlers.GoogleCallback(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d, body %s", rec.Code, http.StatusOK, rec.Body.String())
	}
	if authSvc.googleSub != "google-sub" {
		t.Fatalf("google sub lookup = %q, want google-sub", authSvc.googleSub)
	}
	if authSvc.email != "user@example.com" {
		t.Fatalf("email lookup = %q, want normalized email", authSvc.email)
	}
	if authSvc.attachedUserID != userID || authSvc.attachedGoogleSub != "google-sub" {
		t.Fatalf("attached user = %s/%q, want %s/google-sub", authSvc.attachedUserID, authSvc.attachedGoogleSub, userID)
	}
	if sessions.created.UserID != userID {
		t.Fatalf("created session user id = %s, want %s", sessions.created.UserID, userID)
	}
	if findCookie(rec.Result().Cookies(), "coast_session") == nil || findCookie(rec.Result().Cookies(), "coast_csrf") == nil {
		t.Fatal("google callback did not set session cookies")
	}
}

func TestGoogleCallbackCreatesBootstrapAdmin(t *testing.T) {
	authSvc := &fakeHTTPAuthService{
		loginUserByGoogleSubErr: service.ErrNotFound,
		loginUserByEmailErr:     service.ErrNotFound,
		bootstrapUser: service.LoginUser{
			ID:        uuid.New(),
			Email:     "admin@example.com",
			Name:      "Admin",
			Role:      policy.RoleAdmin,
			Status:    policy.StatusActive,
			GoogleSub: stringPtr("google-sub"),
		},
	}
	handlers := testAuthHandlers()
	handlers.Auth = authSvc
	handlers.Sessions = &fakeHTTPSessionStore{}
	handlers.OAuthStates = &fakeHTTPOAuthStateStore{}
	handlers.Google = &fakeHTTPGoogleOAuth{identity: GoogleIdentity{
		Subject:       "google-sub",
		Email:         "admin@example.com",
		Name:          "Admin",
		EmailVerified: true,
	}}
	handlers.Config.BootstrapAdminEmail = "admin@example.com"

	req := httptest.NewRequest(http.MethodGet, "/api/auth/google/callback?state=state-token&code=auth-code", nil)
	rec := httptest.NewRecorder()

	handlers.GoogleCallback(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d, body %s", rec.Code, http.StatusOK, rec.Body.String())
	}
	if authSvc.bootstrapEmail != "admin@example.com" || authSvc.bootstrapGoogleSub != "google-sub" {
		t.Fatalf("bootstrap input = email %q sub %q", authSvc.bootstrapEmail, authSvc.bootstrapGoogleSub)
	}
}

func testAuthHandlers() *AuthHandlers {
	now := time.Date(2026, 5, 27, 12, 0, 0, 0, time.UTC)
	return &AuthHandlers{
		Auth:          &fakeHTTPAuthService{},
		Sessions:      &fakeHTTPSessionStore{},
		LoginAttempts: &fakeHTTPLoginAttemptRecorder{},
		OAuthStates:   &fakeHTTPOAuthStateStore{},
		Google:        &fakeHTTPGoogleOAuth{},
		Config: AuthHandlerConfig{
			SessionCookieName: "coast_session",
			CSRFCookieName:    "coast_csrf",
			SecureCookies:     boolPtr(false),
			SessionTTL:        24 * time.Hour,
			OAuthStateTTL:     10 * time.Minute,
		},
		Now: func() time.Time { return now },
	}
}

func findCookie(cookies []*http.Cookie, name string) *http.Cookie {
	for _, cookie := range cookies {
		if cookie.Name == name {
			return cookie
		}
	}
	return nil
}

func assertSessionCookieAttrs(t *testing.T, cookie *http.Cookie, httpOnly bool) {
	t.Helper()
	if cookie.Path != "/" {
		t.Fatalf("%s path = %q, want /", cookie.Name, cookie.Path)
	}
	if cookie.HttpOnly != httpOnly {
		t.Fatalf("%s HttpOnly = %v, want %v", cookie.Name, cookie.HttpOnly, httpOnly)
	}
	if cookie.Secure {
		t.Fatalf("%s Secure = true, want false for test config", cookie.Name)
	}
	if cookie.SameSite != http.SameSiteLaxMode {
		t.Fatalf("%s SameSite = %v, want Lax", cookie.Name, cookie.SameSite)
	}
}

type fakeHTTPAuthService struct {
	sessionUser             policy.User
	sessionErr              error
	loginUserByEmail        service.LoginUser
	loginUserByEmailErr     error
	loginUserByGoogleSub    service.LoginUser
	loginUserByGoogleSubErr error
	attachedGoogleUser      service.LoginUser
	anyAdminExists          bool
	anyAdminExistsErr       error
	bootstrapUser           service.LoginUser
	bootstrapErr            error

	email              string
	googleSub          string
	attachedUserID     uuid.UUID
	attachedGoogleSub  string
	bootstrapEmail     string
	bootstrapName      string
	bootstrapGoogleSub string
}

func (s *fakeHTTPAuthService) AuthenticateSession(ctx context.Context, sessionToken, csrfToken string) (policy.User, error) {
	if s.sessionErr != nil {
		return policy.User{}, s.sessionErr
	}
	return s.sessionUser, nil
}

func (s *fakeHTTPAuthService) FindLoginUserByEmail(ctx context.Context, email string) (service.LoginUser, error) {
	s.email = email
	if s.loginUserByEmailErr != nil {
		return service.LoginUser{}, s.loginUserByEmailErr
	}
	return s.loginUserByEmail, nil
}

func (s *fakeHTTPAuthService) FindLoginUserByGoogleSub(ctx context.Context, googleSub string) (service.LoginUser, error) {
	s.googleSub = googleSub
	if s.loginUserByGoogleSubErr != nil {
		return service.LoginUser{}, s.loginUserByGoogleSubErr
	}
	return s.loginUserByGoogleSub, nil
}

func (s *fakeHTTPAuthService) AttachGoogleSub(ctx context.Context, userID uuid.UUID, googleSub string) (service.LoginUser, error) {
	s.attachedUserID = userID
	s.attachedGoogleSub = googleSub
	return s.attachedGoogleUser, nil
}

func (s *fakeHTTPAuthService) AnyAdminExists(ctx context.Context) (bool, error) {
	return s.anyAdminExists, s.anyAdminExistsErr
}

func (s *fakeHTTPAuthService) CreateBootstrapAdmin(ctx context.Context, email, name, googleSub string) (service.LoginUser, error) {
	s.bootstrapEmail = email
	s.bootstrapName = name
	s.bootstrapGoogleSub = googleSub
	if s.bootstrapErr != nil {
		return service.LoginUser{}, s.bootstrapErr
	}
	return s.bootstrapUser, nil
}

type fakeHTTPSessionStore struct {
	created service.CreateSessionRecord
	session service.Session
}

func (s *fakeHTTPSessionStore) CreateSession(ctx context.Context, input service.CreateSessionRecord) (service.Session, error) {
	s.created = input
	if s.session.ID == uuid.Nil {
		s.session = service.Session{ID: uuid.New(), UserID: input.UserID, ExpiresAt: input.ExpiresAt}
	}
	return s.session, nil
}

func (s *fakeHTTPSessionStore) GetSessionByTokenHash(ctx context.Context, tokenHash []byte) (service.Session, error) {
	return s.session, nil
}

func (s *fakeHTTPSessionStore) RevokeSession(ctx context.Context, id uuid.UUID) error {
	return nil
}

type fakeHTTPLoginAttemptRecorder struct {
	records []service.LoginAttemptRecord
}

func (r *fakeHTTPLoginAttemptRecorder) RecordLoginAttempt(ctx context.Context, input service.LoginAttemptRecord) error {
	r.records = append(r.records, input)
	return nil
}

type fakeHTTPOAuthStateStore struct {
	created service.CreateOAuthStateRecord
}

func (s *fakeHTTPOAuthStateStore) CreateOAuthState(ctx context.Context, input service.CreateOAuthStateRecord) (service.OAuthState, error) {
	s.created = input
	return service.OAuthState{ID: uuid.New(), ExpiresAt: input.ExpiresAt}, nil
}

func (s *fakeHTTPOAuthStateStore) ConsumeOAuthState(ctx context.Context, stateHash []byte, now time.Time) (service.OAuthState, error) {
	if len(stateHash) == 0 {
		return service.OAuthState{}, errors.New("empty state hash")
	}
	return service.OAuthState{ID: uuid.New(), ExpiresAt: now.Add(time.Minute)}, nil
}

type fakeHTTPGoogleOAuth struct {
	state    string
	identity GoogleIdentity
}

func (g *fakeHTTPGoogleOAuth) AuthCodeURL(state string) string {
	g.state = state
	return "https://accounts.example/auth?state=" + url.QueryEscape(state)
}

func (g *fakeHTTPGoogleOAuth) Exchange(ctx context.Context, code string) (OAuthToken, error) {
	if code == "" {
		return OAuthToken{}, errors.New("missing code")
	}
	return OAuthToken{IDToken: "id-token"}, nil
}

func (g *fakeHTTPGoogleOAuth) VerifyIDToken(ctx context.Context, token OAuthToken) (GoogleIdentity, error) {
	if g.identity.Subject == "" {
		return GoogleIdentity{Subject: "google-sub", Email: "user@example.com", Name: "User", EmailVerified: true}, nil
	}
	return g.identity, nil
}

func stringPtr(value string) *string {
	return &value
}

func boolPtr(value bool) *bool {
	return &value
}
