package httpx

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHealthz(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	rec := httptest.NewRecorder()
	NewRouter(Dependencies{}).ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	if got := rec.Header().Get("Content-Type"); got != "application/json" {
		t.Fatalf("Content-Type = %q, want application/json", got)
	}
	var body map[string]string
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("Decode body error = %v", err)
	}
	if body["status"] != "ok" {
		t.Fatalf("status body = %q, want ok", body["status"])
	}
}

func TestAdminCORSUsesAdminAllowedOrigins(t *testing.T) {
	req := httptest.NewRequest(http.MethodOptions, "/api/admin/users", nil)
	req.Header.Set("Origin", "https://admin.example.com")
	req.Header.Set("Access-Control-Request-Method", http.MethodGet)
	rec := httptest.NewRecorder()

	NewRouter(Dependencies{
		AdminAllowedOrigins: []string{"https://admin.example.com"},
		AppAllowedOrigins:   []string{"https://app.example.com"},
	}).ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusNoContent)
	}
	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "https://admin.example.com" {
		t.Fatalf("Access-Control-Allow-Origin = %q, want admin origin", got)
	}
	if got := rec.Header().Get("Access-Control-Allow-Credentials"); got != "true" {
		t.Fatalf("Access-Control-Allow-Credentials = %q, want true", got)
	}
	if got := rec.Header().Get("Access-Control-Allow-Headers"); got != "Accept, Content-Type, X-CSRF-Token" {
		t.Fatalf("Access-Control-Allow-Headers = %q", got)
	}
}

func TestAdminCORSRejectsAppOrigin(t *testing.T) {
	req := httptest.NewRequest(http.MethodOptions, "/api/admin/users", nil)
	req.Header.Set("Origin", "https://app.example.com")
	req.Header.Set("Access-Control-Request-Method", http.MethodGet)
	rec := httptest.NewRecorder()

	NewRouter(Dependencies{
		AdminAllowedOrigins: []string{"https://admin.example.com"},
		AppAllowedOrigins:   []string{"https://app.example.com"},
	}).ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusForbidden)
	}
	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "" {
		t.Fatalf("Access-Control-Allow-Origin = %q, want empty", got)
	}
}

func TestAdminCORSDoesNotBypassSession(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/admin/users", nil)
	req.Header.Set("Origin", "https://admin.example.com")
	rec := httptest.NewRecorder()

	NewRouter(Dependencies{
		AdminAllowedOrigins: []string{"https://admin.example.com"},
		AppAllowedOrigins:   []string{"https://app.example.com"},
		AuthHandlers:        testAuthHandlers(),
	}).ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "https://admin.example.com" {
		t.Fatalf("Access-Control-Allow-Origin = %q, want admin origin", got)
	}
}

func TestAdminCORSActualRequestRejectsAppOriginHeaders(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/admin/users", nil)
	req.Header.Set("Origin", "https://app.example.com")
	rec := httptest.NewRecorder()

	NewRouter(Dependencies{
		AdminAllowedOrigins: []string{"https://admin.example.com"},
		AppAllowedOrigins:   []string{"https://app.example.com"},
		AuthHandlers:        testAuthHandlers(),
	}).ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "" {
		t.Fatalf("Access-Control-Allow-Origin = %q, want empty", got)
	}
}

func TestAppCORSUsesAppAllowedOrigins(t *testing.T) {
	req := httptest.NewRequest(http.MethodOptions, "/api/app/observations", nil)
	req.Header.Set("Origin", "https://app.example.com")
	req.Header.Set("Access-Control-Request-Method", http.MethodPost)
	rec := httptest.NewRecorder()

	NewRouter(Dependencies{
		AdminAllowedOrigins: []string{"https://admin.example.com"},
		AppAllowedOrigins:   []string{"https://app.example.com"},
	}).ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusNoContent)
	}
	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "https://app.example.com" {
		t.Fatalf("Access-Control-Allow-Origin = %q, want app origin", got)
	}
}

func TestAdminOPTIONSWithoutOriginRequiresSession(t *testing.T) {
	req := httptest.NewRequest(http.MethodOptions, "/api/admin/users", nil)
	rec := httptest.NewRecorder()

	NewRouter(Dependencies{
		AdminAllowedOrigins: []string{"https://admin.example.com"},
		AuthHandlers:        testAuthHandlers(),
	}).ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}

func TestAppOPTIONSWithoutOriginRequiresSession(t *testing.T) {
	req := httptest.NewRequest(http.MethodOptions, "/api/app/observations", nil)
	rec := httptest.NewRecorder()

	NewRouter(Dependencies{
		AppAllowedOrigins: []string{"https://app.example.com"},
		AuthHandlers:      testAuthHandlers(),
	}).ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}
