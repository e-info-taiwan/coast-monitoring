package main

import (
	"context"
	"log"
	"net/http"

	"coast-monitoring/internal/config"
	"coast-monitoring/internal/db"
	httpx "coast-monitoring/internal/http"
	"coast-monitoring/internal/repository"
	"coast-monitoring/internal/service"

	"github.com/jackc/pgx/v5/pgxpool"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatal(err)
	}

	ctx := context.Background()
	pool, err := db.Open(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Fatal(err)
	}
	defer pool.Close()

	googleProvider, err := httpx.NewGoogleOAuthProvider(ctx, httpx.GoogleOAuthConfig{
		ClientID:     cfg.GoogleClientID,
		ClientSecret: cfg.GoogleClientSecret,
		RedirectURL:  cfg.GoogleRedirectURL,
	})
	if err != nil {
		log.Fatal(err)
	}

	server := &http.Server{
		Addr:    cfg.HTTPAddr,
		Handler: newServerHandler(cfg, pool, googleProvider),
	}

	log.Printf("listening on %s", cfg.HTTPAddr)
	log.Fatal(server.ListenAndServe())
}

func newServerHandler(cfg config.Config, pool *pgxpool.Pool, googleProvider httpx.GoogleOAuthProvider) http.Handler {
	userRepo := repository.NewUserRepository(pool)
	sessionRepo := repository.NewSessionRepository(pool)
	authService := service.AuthService{
		Sessions: sessionRepo,
		Users:    userRepo,
	}
	secureCookies := cfg.SecureCookies
	return httpx.NewRouter(httpx.Dependencies{
		AuthHandlers: &httpx.AuthHandlers{
			Auth:          authService,
			Sessions:      sessionRepo,
			LoginAttempts: repository.NewLoginAttemptRepository(pool),
			OAuthStates:   repository.NewOAuthStateRepository(pool),
			Google:        googleProvider,
			Config: httpx.AuthHandlerConfig{
				SessionCookieName:   cfg.SessionCookieName,
				SecureCookies:       &secureCookies,
				BootstrapAdminEmail: cfg.BootstrapAdminEmail,
			},
		},
	})
}
