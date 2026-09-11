// Command migrate applies pending checked-in migrations without starting an HTTP server.
package main

import (
	"context"
	"log"
	"os"
	"strings"

	"coast-monitoring/internal/db"
)

func main() {
	databaseURL := strings.TrimSpace(os.Getenv("DATABASE_URL"))
	if databaseURL == "" {
		log.Fatal("DATABASE_URL is required; choose the target database explicitly")
	}
	ctx := context.Background()
	pool, err := db.Open(ctx, databaseURL)
	if err != nil {
		log.Fatal(err)
	}
	defer pool.Close()
	if err := db.Migrate(ctx, pool, "migrations"); err != nil {
		log.Fatal(err)
	}
	log.Print("database migrations are up to date")
}
