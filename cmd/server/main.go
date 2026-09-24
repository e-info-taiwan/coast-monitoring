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
	if err := db.Migrate(ctx, pool, "migrations"); err != nil {
		log.Fatal(err)
	}

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

	cwaRepo := repository.NewCWAMarineRepository(pool)
	cwaService := service.NewCWAMarineService(cwaRepo, nil)

	if cfg.EnableCWACron {
		cronCtx, cronCancel := context.WithCancel(ctx)
		defer cronCancel()
		go runCWACron(cronCtx, cwaService, cfg.CWASyncInterval)
	}

	server := newHTTPServer(cfg.HTTPAddr, newServerHandler(cfg, pool, googleProvider, cwaService))

	log.Printf("listening on %s", cfg.HTTPAddr)
	log.Fatal(server.ListenAndServe())
}

func newHTTPServer(addr string, handler http.Handler) *http.Server {
	return &http.Server{
		Addr:              addr,
		Handler:           handler,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
	}
}

func newServerHandler(cfg config.Config, pool *pgxpool.Pool, googleProvider httpx.GoogleOAuthProvider, cwaService *service.CWAMarineService) http.Handler {
	userRepo := repository.NewUserRepository(pool)
	sessionRepo := repository.NewSessionRepository(pool)
	catalogRepo := repository.NewCatalogRepository(pool)
	observationRepo := repository.NewObservationRepository(pool)
	auditLogRepo := repository.NewAuditLogRepository(pool)
	authService := service.AuthService{
		Sessions: sessionRepo,
		Users:    userRepo,
	}
	var reefDataRepo httpx.AppReefDataService
	if pool != nil {
		reefDataRepo = repository.NewReefDataRepository(pool)
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
			ReefData:     repository.NewReefDataRepository(pool),
			AuditLogs:    auditLogRepo,
			Mutations:    postgresAdminMutationRunner{pool: pool},
			CWAMarine:    cwaService,
		},
		AppHandlers: &httpx.AppHandlers{
			Catalog:      service.CatalogService{Catalog: catalogRepo},
			Observations: service.ObservationService{Observations: observationRepo},
			Mutations:    postgresAdminMutationRunner{pool: pool},
			ReefData:     reefDataRepo,
		},
		CronHandlers: &httpx.CronHandlers{
			CWAMarine:  cwaService,
			CronSecret: cfg.CronSecret,
		},
		AdminAllowedOrigins: cfg.AdminAllowedOrigins,
		AppAllowedOrigins:   cfg.AppAllowedOrigins,
	})
}

func runCWACron(ctx context.Context, cwa *service.CWAMarineService, interval time.Duration) {
	if interval <= 0 {
		interval = 1 * time.Hour
	}
	log.Printf("cwa marine cron enabled, interval: %v", interval)

	// Run initial sync after a short delay on startup (5 seconds)
	select {
	case <-time.After(5 * time.Second):
		report, err := cwa.SyncAll(ctx)
		if err != nil {
			log.Printf("cwa marine initial sync error: %v", err)
		} else {
			log.Printf("cwa marine initial sync finished in %dms: %d stations, %d obs, %d sites linked",
				report.DurationMs, report.StationsCount, report.ObservationsSaved, report.SitesLinked)
		}
	case <-ctx.Done():
		return
	}

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			report, err := cwa.SyncAll(ctx)
			if err != nil {
				log.Printf("cwa marine cron sync error: %v", err)
			} else {
				log.Printf("cwa marine cron sync finished in %dms: %d stations, %d obs, %d sites linked",
					report.DurationMs, report.StationsCount, report.ObservationsSaved, report.SitesLinked)
			}
		case <-ctx.Done():
			return
		}
	}
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
		ReefData:     repository.NewReefDataRepository(tx),
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
