package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"time"

	"coast-monitoring/internal/config"
	"coast-monitoring/internal/db"
	"coast-monitoring/internal/repository"
	"coast-monitoring/internal/service"
)

func main() {
	var (
		flagStations   = flag.Bool("stations", false, "Only sync CWA marine station list")
		flagLatest     = flag.Bool("latest", false, "Only sync latest observations from all sea areas")
		flagStationID  = flag.String("station", "", "Sync historical observations for a specific station ID (e.g. C4A02)")
		flagDays       = flag.Int("days", 30, "Number of days of history to sync when -station is specified (e.g. 2 or 30)")
		flagMigrate    = flag.Bool("migrate", true, "Run pending database migrations before sync")
		flagTimeout    = flag.Duration("timeout", 5*time.Minute, "Timeout for the sync operation")
	)
	flag.Parse()

	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("config error: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), *flagTimeout)
	defer cancel()

	pool, err := db.Open(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("database connection error: %v", err)
	}
	defer pool.Close()

	if *flagMigrate {
		if err := db.Migrate(ctx, pool, "migrations"); err != nil {
			log.Fatalf("database migration error: %v", err)
		}
	}

	repo := repository.NewCWAMarineRepository(pool)
	cwa := service.NewCWAMarineService(repo, nil)

	start := time.Now()

	switch {
	case *flagStationID != "":
		log.Printf("syncing station %s (%d days of history)...", *flagStationID, *flagDays)
		count, err := cwa.SyncStationHistory(ctx, *flagStationID, *flagDays)
		if err != nil {
			log.Fatalf("sync station %s failed: %v", *flagStationID, err)
		}
		log.Printf("successfully synced %d observations for station %s in %v", count, *flagStationID, time.Since(start))

	case *flagStations:
		log.Printf("syncing CWA marine station definitions...")
		count, err := cwa.SyncStations(ctx)
		if err != nil {
			log.Fatalf("sync stations failed: %v", err)
		}
		log.Printf("successfully synced %d stations in %v", count, time.Since(start))

	case *flagLatest:
		log.Printf("syncing latest CWA sea area observations...")
		count, err := cwa.SyncLatestObservations(ctx)
		if err != nil {
			log.Fatalf("sync observations failed: %v", err)
		}
		log.Printf("successfully synced %d observations in %v", count, time.Since(start))

	default:
		// Default behavior: sync everything (stations + latest observations + auto-link sites)
		log.Printf("running full CWA marine sync (stations, latest observations, auto-linking sites)...")
		report, err := cwa.SyncAll(ctx)
		if err != nil {
			log.Fatalf("sync all failed: %v", err)
		}
		fmt.Printf("\n=== CWA Marine Sync Completed in %dms ===\n", report.DurationMs)
		fmt.Printf("• Stations synced:        %d\n", report.StationsCount)
		fmt.Printf("• Observations saved:     %d\n", report.ObservationsSaved)
		fmt.Printf("• Sites linked:           %d\n", report.SitesLinked)
		if len(report.Errors) > 0 {
			fmt.Println("• Errors encountered:")
			for _, e := range report.Errors {
				fmt.Printf("  - %s\n", e)
			}
			os.Exit(1)
		}
	}
}
