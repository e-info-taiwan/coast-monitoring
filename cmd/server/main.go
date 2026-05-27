package main

import (
	"context"
	"log"
	"net/http"
	"strings"
	"time"

	"coast-monitoring/internal/config"
	"coast-monitoring/internal/db"
	httpx "coast-monitoring/internal/http"
	"coast-monitoring/internal/repository"
	"coast-monitoring/internal/service"

	"github.com/jackc/pgx/v5"
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

	var googleProvider httpx.GoogleOAuthProvider
	if googleConfigComplete(cfg) {
		googleCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
		googleProvider, err = httpx.NewGoogleOAuthProvider(googleCtx, httpx.GoogleOAuthConfig{
			ClientID:     cfg.GoogleClientID,
			ClientSecret: cfg.GoogleClientSecret,
			RedirectURL:  cfg.GoogleRedirectURL,
		})
		cancel()
		if err != nil {
			log.Fatal(err)
		}
	} else {
		log.Print("google auth disabled: GOOGLE_CLIENT_ID, GOOGLE_CLIENT_SECRET, or GOOGLE_REDIRECT_URL is missing")
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
	catalogRepo := repository.NewCatalogRepository(pool)
	observationRepo := repository.NewObservationRepository(pool)
	auditLogRepo := repository.NewAuditLogRepository(pool)
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
				CSRFHeaderName:      cfg.CSRFHeaderName,
				SecureCookies:       &secureCookies,
				BootstrapAdminEmail: cfg.BootstrapAdminEmail,
			},
		},
		AdminHandlers: &httpx.AdminHandlers{
			Users:        service.UserService{Users: userRepo},
			Catalog:      service.CatalogService{Catalog: catalogRepo},
			Observations: service.ObservationService{Observations: observationRepo},
			AuditLogs:    auditLogRepo,
			Mutations:    postgresAdminMutationRunner{pool: pool},
		},
	})
}

type postgresAdminMutationRunner struct {
	pool *pgxpool.Pool
}

func (r postgresAdminMutationRunner) RunAdminMutation(ctx context.Context, fn func(httpx.AdminMutationServices) error) error {
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return err
	}
	committed := false
	defer func() {
		if !committed {
			_ = tx.Rollback(ctx)
		}
	}()

	userRepo := repository.NewUserRepository(tx)
	catalogRepo := repository.NewCatalogRepository(tx)
	observationRepo := repository.NewObservationRepository(tx)
	auditLogRepo := repository.NewAuditLogRepository(tx)
	if err := fn(httpx.AdminMutationServices{
		Users:        service.UserService{Users: userRepo},
		Catalog:      service.CatalogService{Catalog: catalogRepo},
		Observations: service.ObservationService{Observations: observationRepo},
		AuditLogs:    auditLogRepo,
	}); err != nil {
		return err
	}
	if err := tx.Commit(ctx); err != nil {
		return err
	}
	committed = true
	return nil
}

func googleConfigComplete(cfg config.Config) bool {
	return strings.TrimSpace(cfg.GoogleClientID) != "" &&
		strings.TrimSpace(cfg.GoogleClientSecret) != "" &&
		strings.TrimSpace(cfg.GoogleRedirectURL) != ""
}
