package httpx

import (
	"context"
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"strings"
	"time"

	"coast-monitoring/internal/auth"
	"coast-monitoring/internal/policy"
	"coast-monitoring/internal/service"

	"github.com/google/uuid"
)

const (
	defaultSessionCookieName = "coast_session"
	defaultCSRFCookieName    = "coast_csrf"
	defaultSessionTTL        = 7 * 24 * time.Hour
	defaultOAuthStateTTL     = 10 * time.Minute
)

type AuthHandlers struct {
	Auth          AuthService
	Sessions      SessionStore
	LoginAttempts LoginAttemptRecorder
	OAuthStates   OAuthStateStore
	Google        GoogleOAuthProvider
	Config        AuthHandlerConfig
	Now           func() time.Time
}

type AuthHandlerConfig struct {
	SessionCookieName   string
	CSRFCookieName      string
	SecureCookies       *bool
	SessionTTL          time.Duration
	OAuthStateTTL       time.Duration
	BootstrapAdminEmail string
}

type AuthService interface {
	AuthenticateSession(ctx context.Context, sessionToken, csrfToken string) (policy.User, error)
	FindLoginUserByEmail(ctx context.Context, email string) (service.LoginUser, error)
	FindLoginUserByGoogleSub(ctx context.Context, googleSub string) (service.LoginUser, error)
	AttachGoogleSub(ctx context.Context, userID uuid.UUID, googleSub string) (service.LoginUser, error)
	AnyAdminExists(ctx context.Context) (bool, error)
	CreateBootstrapAdmin(ctx context.Context, email, name, googleSub string) (service.LoginUser, error)
}

type SessionStore interface {
	CreateSession(ctx context.Context, input service.CreateSessionRecord) (service.Session, error)
	GetSessionByTokenHash(ctx context.Context, tokenHash []byte) (service.Session, error)
	RevokeSession(ctx context.Context, id uuid.UUID) error
}

type LoginAttemptRecorder interface {
	RecordLoginAttempt(ctx context.Context, input service.LoginAttemptRecord) error
}

type OAuthStateStore interface {
	CreateOAuthState(ctx context.Context, input service.CreateOAuthStateRecord) (service.OAuthState, error)
	ConsumeOAuthState(ctx context.Context, stateHash []byte, now time.Time) (service.OAuthState, error)
}

type OAuthToken struct {
	IDToken string
}

type GoogleIdentity struct {
	Subject       string
	Email         string
	Name          string
	EmailVerified bool
}

type GoogleOAuthProvider interface {
	AuthCodeURL(state string) string
	Exchange(ctx context.Context, code string) (OAuthToken, error)
	VerifyIDToken(ctx context.Context, token OAuthToken) (GoogleIdentity, error)
}

func (h *AuthHandlers) Session(w http.ResponseWriter, r *http.Request) {
	if h == nil || h.Auth == nil {
		writeError(w, http.StatusServiceUnavailable, "auth is not configured")
		return
	}
	sessionToken, ok := cookieValue(r, h.config().SessionCookieName)
	if !ok {
		writeJSON(w, http.StatusOK, SessionResponse{Authenticated: false})
		return
	}
	csrfToken, ok := cookieValue(r, h.config().CSRFCookieName)
	if !ok {
		writeJSON(w, http.StatusOK, SessionResponse{Authenticated: false})
		return
	}
	user, err := h.Auth.AuthenticateSession(r.Context(), sessionToken, csrfToken)
	if err != nil {
		if errors.Is(err, service.ErrUnauthorized) || errors.Is(err, service.ErrNotFound) {
			writeJSON(w, http.StatusOK, SessionResponse{Authenticated: false})
			return
		}
		writeError(w, http.StatusInternalServerError, "session lookup failed")
		return
	}
	writeJSON(w, http.StatusOK, SessionResponse{
		Authenticated: true,
		User:          currentUserResponse(user),
		CSRFToken:     csrfToken,
	})
}

func (h *AuthHandlers) PasswordLogin(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		writeError(w, http.StatusBadRequest, "invalid login request")
		return
	}
	email := normalizeEmail(input.Email)
	password := strings.TrimSpace(input.Password)
	if email == "" || password == "" {
		_ = h.recordLoginAttempt(r.Context(), email, remoteIP(r), false)
		writeError(w, http.StatusBadRequest, "email and password are required")
		return
	}
	if h == nil || h.Auth == nil || h.Sessions == nil {
		writeError(w, http.StatusServiceUnavailable, "auth is not configured")
		return
	}

	user, err := h.Auth.FindLoginUserByEmail(r.Context(), email)
	if err != nil {
		if errors.Is(err, service.ErrNotFound) || errors.Is(err, service.ErrValidation) {
			_ = h.recordLoginAttempt(r.Context(), email, remoteIP(r), false)
			writeError(w, http.StatusUnauthorized, "invalid email or password")
			return
		}
		writeError(w, http.StatusInternalServerError, "login failed")
		return
	}
	if user.Status != policy.StatusActive || !auth.VerifyPassword(user.PasswordHash, password) {
		_ = h.recordLoginAttempt(r.Context(), email, remoteIP(r), false)
		writeError(w, http.StatusUnauthorized, "invalid email or password")
		return
	}

	if err := h.recordLoginAttempt(r.Context(), email, remoteIP(r), true); err != nil {
		writeError(w, http.StatusInternalServerError, "login attempt recording failed")
		return
	}
	response, err := h.createSession(r.Context(), w, r, loginUserToPolicyUser(user))
	if err != nil {
		writeError(w, http.StatusInternalServerError, "session creation failed")
		return
	}
	writeJSON(w, http.StatusOK, response)
}

func (h *AuthHandlers) GoogleStart(w http.ResponseWriter, r *http.Request) {
	if h == nil || h.OAuthStates == nil || h.Google == nil {
		writeError(w, http.StatusServiceUnavailable, "google auth is not configured")
		return
	}
	stateToken, err := auth.GenerateToken(auth.MinTokenBytes)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "oauth state creation failed")
		return
	}
	_, err = h.OAuthStates.CreateOAuthState(r.Context(), service.CreateOAuthStateRecord{
		StateHash:    auth.HashToken(stateToken),
		RedirectPath: "/",
		ExpiresAt:    h.now().Add(h.config().OAuthStateTTL),
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "oauth state creation failed")
		return
	}
	http.Redirect(w, r, h.Google.AuthCodeURL(stateToken), http.StatusFound)
}

func (h *AuthHandlers) GoogleCallback(w http.ResponseWriter, r *http.Request) {
	if h == nil || h.Auth == nil || h.Sessions == nil || h.OAuthStates == nil || h.Google == nil {
		writeError(w, http.StatusServiceUnavailable, "google auth is not configured")
		return
	}
	stateToken := strings.TrimSpace(r.URL.Query().Get("state"))
	code := strings.TrimSpace(r.URL.Query().Get("code"))
	if stateToken == "" || code == "" {
		writeError(w, http.StatusBadRequest, "state and code are required")
		return
	}
	if _, err := h.OAuthStates.ConsumeOAuthState(r.Context(), auth.HashToken(stateToken), h.now()); err != nil {
		writeError(w, http.StatusUnauthorized, "invalid oauth state")
		return
	}
	token, err := h.Google.Exchange(r.Context(), code)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "google code exchange failed")
		return
	}
	identity, err := h.Google.VerifyIDToken(r.Context(), token)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "google id token verification failed")
		return
	}
	user, err := h.resolveGoogleUser(r.Context(), identity)
	if err != nil {
		if errors.Is(err, service.ErrUnauthorized) || errors.Is(err, service.ErrNotFound) || errors.Is(err, service.ErrValidation) {
			writeError(w, http.StatusUnauthorized, "google account is not allowed")
			return
		}
		writeError(w, http.StatusInternalServerError, "google login failed")
		return
	}
	response, err := h.createSession(r.Context(), w, r, loginUserToPolicyUser(user))
	if err != nil {
		writeError(w, http.StatusInternalServerError, "session creation failed")
		return
	}
	writeJSON(w, http.StatusOK, response)
}

func (h *AuthHandlers) Logout(w http.ResponseWriter, r *http.Request) {
	cfg := h.config()
	if h != nil && h.Sessions != nil {
		if token, ok := cookieValue(r, cfg.SessionCookieName); ok {
			session, err := h.Sessions.GetSessionByTokenHash(r.Context(), auth.HashToken(token))
			if err == nil {
				_ = h.Sessions.RevokeSession(r.Context(), session.ID)
			}
		}
	}
	h.clearCookie(w, cfg.SessionCookieName, true)
	h.clearCookie(w, cfg.CSRFCookieName, false)
	writeJSON(w, http.StatusOK, SessionResponse{Authenticated: false})
}

func (h *AuthHandlers) resolveGoogleUser(ctx context.Context, identity GoogleIdentity) (service.LoginUser, error) {
	googleSub := strings.TrimSpace(identity.Subject)
	email := normalizeEmail(identity.Email)
	name := strings.TrimSpace(identity.Name)
	if googleSub == "" || email == "" || !identity.EmailVerified {
		return service.LoginUser{}, service.ErrUnauthorized
	}

	user, err := h.Auth.FindLoginUserByGoogleSub(ctx, googleSub)
	if err == nil {
		if user.Status != policy.StatusActive {
			return service.LoginUser{}, service.ErrUnauthorized
		}
		return user, nil
	}
	if !errors.Is(err, service.ErrNotFound) {
		return service.LoginUser{}, err
	}

	user, err = h.Auth.FindLoginUserByEmail(ctx, email)
	if err == nil {
		if user.Status != policy.StatusActive {
			return service.LoginUser{}, service.ErrUnauthorized
		}
		return h.Auth.AttachGoogleSub(ctx, user.ID, googleSub)
	}
	if !errors.Is(err, service.ErrNotFound) {
		return service.LoginUser{}, err
	}

	cfg := h.config()
	if cfg.BootstrapAdminEmail == "" || email != normalizeEmail(cfg.BootstrapAdminEmail) {
		return service.LoginUser{}, service.ErrUnauthorized
	}
	adminExists, err := h.Auth.AnyAdminExists(ctx)
	if err != nil {
		return service.LoginUser{}, err
	}
	if adminExists {
		return service.LoginUser{}, service.ErrUnauthorized
	}
	return h.Auth.CreateBootstrapAdmin(ctx, email, name, googleSub)
}

func (h *AuthHandlers) createSession(ctx context.Context, w http.ResponseWriter, r *http.Request, user policy.User) (SessionResponse, error) {
	sessionToken, err := auth.GenerateToken(auth.MinTokenBytes)
	if err != nil {
		return SessionResponse{}, err
	}
	csrfToken, err := auth.GenerateToken(auth.MinTokenBytes)
	if err != nil {
		return SessionResponse{}, err
	}
	cfg := h.config()
	expiresAt := h.now().Add(cfg.SessionTTL)
	if _, err := h.Sessions.CreateSession(ctx, service.CreateSessionRecord{
		UserID:        user.ID,
		TokenHash:     auth.HashToken(sessionToken),
		CSRFTokenHash: auth.HashToken(csrfToken),
		UserAgent:     r.UserAgent(),
		IP:            remoteIP(r),
		ExpiresAt:     expiresAt,
	}); err != nil {
		return SessionResponse{}, err
	}
	h.setCookie(w, cfg.SessionCookieName, sessionToken, true, expiresAt)
	h.setCookie(w, cfg.CSRFCookieName, csrfToken, false, expiresAt)
	return SessionResponse{
		Authenticated: true,
		User:          currentUserResponse(user),
		CSRFToken:     csrfToken,
	}, nil
}

func (h *AuthHandlers) recordLoginAttempt(ctx context.Context, email, ip string, success bool) error {
	if h == nil || h.LoginAttempts == nil {
		return nil
	}
	return h.LoginAttempts.RecordLoginAttempt(ctx, service.LoginAttemptRecord{
		Email:   email,
		IP:      ip,
		Success: success,
	})
}

func (h *AuthHandlers) setCookie(w http.ResponseWriter, name, value string, httpOnly bool, expires time.Time) {
	http.SetCookie(w, &http.Cookie{
		Name:     name,
		Value:    value,
		Path:     "/",
		Expires:  expires,
		HttpOnly: httpOnly,
		Secure:   h.config().secureCookies(),
		SameSite: http.SameSiteLaxMode,
	})
}

func (h *AuthHandlers) clearCookie(w http.ResponseWriter, name string, httpOnly bool) {
	http.SetCookie(w, &http.Cookie{
		Name:     name,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: httpOnly,
		Secure:   h.config().secureCookies(),
		SameSite: http.SameSiteLaxMode,
	})
}

func (h *AuthHandlers) config() AuthHandlerConfig {
	if h == nil {
		return defaultAuthHandlerConfig()
	}
	cfg := h.Config
	defaults := defaultAuthHandlerConfig()
	if cfg.SessionCookieName == "" {
		cfg.SessionCookieName = defaults.SessionCookieName
	}
	if cfg.CSRFCookieName == "" {
		cfg.CSRFCookieName = defaults.CSRFCookieName
	}
	if cfg.SecureCookies == nil {
		cfg.SecureCookies = defaults.SecureCookies
	}
	if cfg.SessionTTL <= 0 {
		cfg.SessionTTL = defaults.SessionTTL
	}
	if cfg.OAuthStateTTL <= 0 {
		cfg.OAuthStateTTL = defaults.OAuthStateTTL
	}
	cfg.BootstrapAdminEmail = normalizeEmail(cfg.BootstrapAdminEmail)
	return cfg
}

func defaultAuthHandlerConfig() AuthHandlerConfig {
	secure := true
	return AuthHandlerConfig{
		SessionCookieName: defaultSessionCookieName,
		CSRFCookieName:    defaultCSRFCookieName,
		SecureCookies:     &secure,
		SessionTTL:        defaultSessionTTL,
		OAuthStateTTL:     defaultOAuthStateTTL,
	}
}

func (cfg AuthHandlerConfig) secureCookies() bool {
	if cfg.SecureCookies == nil {
		return true
	}
	return *cfg.SecureCookies
}

func (h *AuthHandlers) now() time.Time {
	if h != nil && h.Now != nil {
		return h.Now()
	}
	return time.Now().UTC()
}

func currentUserResponse(user policy.User) *CurrentUserResponse {
	return &CurrentUserResponse{
		ID:    user.ID.String(),
		Email: user.Email,
		Name:  user.Name,
		Role:  string(user.Role),
	}
}

func loginUserToPolicyUser(user service.LoginUser) policy.User {
	return policy.User{
		ID:     user.ID,
		Email:  user.Email,
		Name:   user.Name,
		Role:   user.Role,
		Status: user.Status,
	}
}

func cookieValue(r *http.Request, name string) (string, bool) {
	cookie, err := r.Cookie(name)
	if err != nil || strings.TrimSpace(cookie.Value) == "" {
		return "", false
	}
	return cookie.Value, true
}

func normalizeEmail(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}

func remoteIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err == nil {
		return host
	}
	return strings.TrimSpace(r.RemoteAddr)
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}
