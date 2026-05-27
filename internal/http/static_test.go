package httpx

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestRootServesAdminHTML(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()

	NewRouter(Dependencies{}).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	if got := rec.Header().Get("Content-Type"); !strings.Contains(got, "text/html") {
		t.Fatalf("Content-Type = %q, want text/html", got)
	}
	if body := rec.Body.String(); !strings.Contains(body, `src="/app.js"`) {
		t.Fatalf("body does not look like admin HTML")
	}
}

func TestPublicStaticRouteServesPublicHTML(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/public/species.html", nil)
	rec := httptest.NewRecorder()

	NewRouter(Dependencies{}).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	if got := rec.Header().Get("Content-Type"); !strings.Contains(got, "text/html") {
		t.Fatalf("Content-Type = %q, want text/html", got)
	}
	if body := rec.Body.String(); !strings.Contains(body, `src="/public/species.js"`) {
		t.Fatalf("body does not look like public species HTML")
	}
}

func TestUnknownAPIRouteDoesNotServeAdminHTML(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/unknown", nil)
	rec := httptest.NewRecorder()

	NewRouter(Dependencies{}).ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusNotFound)
	}
	if body := rec.Body.String(); strings.Contains(body, `src="/app.js"`) {
		t.Fatalf("unknown API route served admin HTML")
	}
}
