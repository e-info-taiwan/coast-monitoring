package db

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
)

func MigrationFiles(dir string) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	files := make([]string, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".sql" {
			continue
		}
		files = append(files, filepath.Join(dir, entry.Name()))
	}
	sort.Strings(files)
	return files, nil
}

func Migrate(ctx context.Context, pool *pgxpool.Pool, dir string) error {
	files, err := MigrationFiles(dir)
	if err != nil {
		return fmt.Errorf("list migrations: %w", err)
	}
	if len(files) == 0 {
		return nil
	}

	if _, err := pool.Exec(ctx, `
		CREATE TABLE IF NOT EXISTS schema_migrations (
			version text PRIMARY KEY,
			applied_at timestamptz NOT NULL DEFAULT now()
		)
	`); err != nil {
		return fmt.Errorf("create schema_migrations: %w", err)
	}

	for _, file := range files {
		version := migrationVersion(file)
		applied, err := migrationApplied(ctx, pool, version)
		if err != nil {
			return err
		}
		if applied {
			continue
		}
		if version == "000001_init" {
			present, err := initialSchemaAlreadyPresent(ctx, pool)
			if err != nil {
				return err
			}
			if present {
				if err := recordMigration(ctx, pool, version); err != nil {
					return err
				}
				continue
			}
		}
		if err := applyMigrationFile(ctx, pool, file, version); err != nil {
			return err
		}
	}
	return nil
}

func migrationVersion(file string) string {
	return strings.TrimSuffix(filepath.Base(file), filepath.Ext(file))
}

func migrationApplied(ctx context.Context, pool *pgxpool.Pool, version string) (bool, error) {
	var applied bool
	err := pool.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT 1 FROM schema_migrations WHERE version = $1
		)
	`, version).Scan(&applied)
	if err != nil {
		return false, fmt.Errorf("check migration %s: %w", version, err)
	}
	return applied, nil
}

func initialSchemaAlreadyPresent(ctx context.Context, pool *pgxpool.Pool) (bool, error) {
	var present bool
	err := pool.QueryRow(ctx, `
		SELECT to_regclass('public.users') IS NOT NULL
			AND to_regclass('public.reef_check_surveys') IS NOT NULL
	`).Scan(&present)
	if err != nil {
		return false, fmt.Errorf("check existing initial schema: %w", err)
	}
	return present, nil
}

func recordMigration(ctx context.Context, pool *pgxpool.Pool, version string) error {
	_, err := pool.Exec(ctx, `
		INSERT INTO schema_migrations (version)
		VALUES ($1)
		ON CONFLICT (version) DO NOTHING
	`, version)
	if err != nil {
		return fmt.Errorf("record migration %s: %w", version, err)
	}
	return nil
}

func applyMigrationFile(ctx context.Context, pool *pgxpool.Pool, file string, version string) error {
	sql, err := os.ReadFile(file)
	if err != nil {
		return fmt.Errorf("read migration %s: %w", version, err)
	}
	tx, err := pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin migration %s: %w", version, err)
	}
	committed := false
	defer func() {
		if !committed {
			_ = tx.Rollback(ctx)
		}
	}()
	if _, err := tx.Exec(ctx, string(sql)); err != nil {
		return fmt.Errorf("apply migration %s: %w", version, err)
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO schema_migrations (version)
		VALUES ($1)
	`, version); err != nil {
		return fmt.Errorf("record migration %s: %w", version, err)
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit migration %s: %w", version, err)
	}
	committed = true
	return nil
}
