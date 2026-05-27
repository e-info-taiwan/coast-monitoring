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
}
