package config

import (
	"errors"
	"os"
	"strings"
)

type Config struct {
	HTTPAddr            string
	DatabaseURL         string
	SessionSecret       string
	SessionCookieName   string
	CSRFHeaderName      string
	GoogleClientID      string
	GoogleClientSecret  string
	GoogleRedirectURL   string
	BootstrapAdminEmail string
	SecureCookies       bool
	AdminAllowedOrigins []string
	AppAllowedOrigins   []string
}

func Load() (Config, error) {
	secureCookies, err := boolEnv("SECURE_COOKIES", true)
	if err != nil {
		return Config{}, err
	}
	cfg := Config{
		HTTPAddr:          env("HTTP_ADDR", ":8090"),
		DatabaseURL:       strings.TrimSpace(os.Getenv("DATABASE_URL")),
		SessionSecret:     strings.TrimSpace(os.Getenv("SESSION_SECRET")),
		SessionCookieName: env("SESSION_COOKIE_NAME", "coast_session"),
		CSRFHeaderName:    env("CSRF_HEADER_NAME", "X-CSRF-Token"),
		GoogleClientID:    strings.TrimSpace(os.Getenv("GOOGLE_CLIENT_ID")),
		GoogleClientSecret: strings.TrimSpace(
			os.Getenv("GOOGLE_CLIENT_SECRET"),
		),
		GoogleRedirectURL: strings.TrimSpace(os.Getenv("GOOGLE_REDIRECT_URL")),
		BootstrapAdminEmail: strings.ToLower(strings.TrimSpace(
			os.Getenv("BOOTSTRAP_ADMIN_EMAIL"),
		)),
		SecureCookies:       secureCookies,
		AdminAllowedOrigins: splitCSV(os.Getenv("ADMIN_ALLOWED_ORIGINS")),
		AppAllowedOrigins:   splitCSV(os.Getenv("APP_ALLOWED_ORIGINS")),
	}

	if cfg.DatabaseURL == "" {
		return Config{}, errors.New("DATABASE_URL is required")
	}
	if len(cfg.SessionSecret) < 32 {
		return Config{}, errors.New("SESSION_SECRET must be at least 32 characters")
	}
	return cfg, nil
}

func env(key, fallback string) string {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	return value
}

func boolEnv(key string, fallback bool) (bool, error) {
	value := strings.ToLower(strings.TrimSpace(os.Getenv(key)))
	if value == "" {
		return fallback, nil
	}
	switch value {
	case "1", "true", "yes", "y", "on":
		return true, nil
	case "0", "false", "no", "n", "off":
		return false, nil
	default:
		return false, errors.New(key + " must be a boolean")
	}
}

func splitCSV(value string) []string {
	parts := strings.Split(value, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			out = append(out, part)
		}
	}
	return out
}
