package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"coast-monitoring/internal/config"
	httpx "coast-monitoring/internal/http"
)

func TestNewServerHandlerWiresAuthRoutes(t *testing.T) {
	secureCookies := false
	handler := newServerHandler(config.Config{
		SessionCookieName:   "coast_session",
		BootstrapAdminEmail: "admin@example.com",
		SecureCookies:       secureCookies,
	}, nil, &stubGoogleProvider{})

	req := httptest.NewRequest(http.MethodGet, "/api/session", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body %s", rec.Code, http.StatusOK, rec.Body.String())
	}
}

func TestNewServerHandlerAllowsMissingGoogleProvider(t *testing.T) {
	handler := newServerHandler(config.Config{
		SessionCookieName: "coast_session",
		SecureCookies:     false,
	}, nil, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/auth/google/start", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want %d; body %s", rec.Code, http.StatusServiceUnavailable, rec.Body.String())
	}
}

func TestGoogleConfigComplete(t *testing.T) {
	complete := config.Config{
		GoogleClientID:     "client-id",
		GoogleClientSecret: "client-secret",
		GoogleRedirectURL:  "https://app.example.com/api/auth/google/callback",
	}
	if !googleConfigComplete(complete) {
		t.Fatal("googleConfigComplete = false, want true")
	}
	if googleConfigComplete(config.Config{GoogleClientID: "client-id"}) {
		t.Fatal("googleConfigComplete = true, want false for incomplete config")
	}
}

func TestNewHTTPServerSetsDefensiveTimeouts(t *testing.T) {
	server := newHTTPServer(":0", http.NewServeMux())
	if server.ReadHeaderTimeout <= 0 {
		t.Fatal("ReadHeaderTimeout is not configured")
	}
	if server.ReadTimeout <= 0 {
		t.Fatal("ReadTimeout is not configured")
	}
	if server.WriteTimeout <= 0 {
		t.Fatal("WriteTimeout is not configured")
	}
	if server.IdleTimeout <= 0 {
		t.Fatal("IdleTimeout is not configured")
	}
}

type stubGoogleProvider struct{}

func (p *stubGoogleProvider) AuthCodeURL(state string) string {
	return "https://accounts.example/auth?state=" + state
}

func (p *stubGoogleProvider) Exchange(ctx context.Context, code string) (httpx.OAuthToken, error) {
	return httpx.OAuthToken{}, nil
}

func (p *stubGoogleProvider) VerifyIDToken(ctx context.Context, token httpx.OAuthToken) (httpx.GoogleIdentity, error) {
	return httpx.GoogleIdentity{}, nil
}
