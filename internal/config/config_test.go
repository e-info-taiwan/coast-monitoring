package config

import "testing"

func TestLoadRequiresDatabaseURL(t *testing.T) {
	t.Setenv("DATABASE_URL", "")
	_, err := Load()
	if err == nil {
		t.Fatal("expected DATABASE_URL validation error")
	}
}

func TestLoadDefaults(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://user:pass@localhost:5432/coast?sslmode=disable")
	t.Setenv("SESSION_SECRET", "01234567890123456789012345678901")
	t.Setenv("HTTP_ADDR", "")
	t.Setenv("SESSION_COOKIE_NAME", "")
	t.Setenv("CSRF_HEADER_NAME", "")
	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.HTTPAddr != ":8090" {
		t.Fatalf("HTTPAddr = %q, want :8090", cfg.HTTPAddr)
	}
	if cfg.SessionCookieName != "coast_session" {
		t.Fatalf("SessionCookieName = %q, want coast_session", cfg.SessionCookieName)
	}
	if cfg.CSRFHeaderName != "X-CSRF-Token" {
		t.Fatalf("CSRFHeaderName = %q, want X-CSRF-Token", cfg.CSRFHeaderName)
	}
}

func TestLoadNormalizesCSVAndBootstrapAdminEmail(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://user:pass@localhost:5432/coast?sslmode=disable")
	t.Setenv("SESSION_SECRET", "01234567890123456789012345678901")
	t.Setenv("BOOTSTRAP_ADMIN_EMAIL", " Admin@Example.COM ")
	t.Setenv("ADMIN_ALLOWED_ORIGINS", " https://admin.example.com, ,http://localhost:5173 ")
	t.Setenv("APP_ALLOWED_ORIGINS", " https://app.example.com, http://localhost:5174 ")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.BootstrapAdminEmail != "admin@example.com" {
		t.Fatalf("BootstrapAdminEmail = %q, want admin@example.com", cfg.BootstrapAdminEmail)
	}
	assertStrings(t, cfg.AdminAllowedOrigins, []string{"https://admin.example.com", "http://localhost:5173"})
	assertStrings(t, cfg.AppAllowedOrigins, []string{"https://app.example.com", "http://localhost:5174"})
}

func TestLoadSecureCookiesDefaultsTrueAndCanBeDisabled(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://user:pass@localhost:5432/coast?sslmode=disable")
	t.Setenv("SESSION_SECRET", "01234567890123456789012345678901")
	t.Setenv("SECURE_COOKIES", "")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if !cfg.SecureCookies {
		t.Fatal("SecureCookies = false, want true by default")
	}

	t.Setenv("SECURE_COOKIES", "false")
	cfg, err = Load()
	if err != nil {
		t.Fatalf("Load() with SECURE_COOKIES=false error = %v", err)
	}
	if cfg.SecureCookies {
		t.Fatal("SecureCookies = true, want false when SECURE_COOKIES=false")
	}
}

func assertStrings(t *testing.T, got, want []string) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("len = %d, want %d; got %#v", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("item %d = %q, want %q; got %#v", i, got[i], want[i], got)
		}
	}
}
