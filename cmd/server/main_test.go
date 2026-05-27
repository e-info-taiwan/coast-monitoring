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
