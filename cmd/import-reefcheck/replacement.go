package main

import (
	"context"
	"fmt"
	"log"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"

	"github.com/jackc/pgx/v5"
)

func parseNullableImportCount(value, status string) (*int, string, error) {
	if value == "" || value == "NA" || value == "-" || status == "not_recorded" {
		return nil, "not_recorded", nil
	}
	n, s, err := parseImportCount(value, status)
	return &n, s, err
}

func parseImportImpact(value, status, unit, group string) (*float64, string, string, error) {
	if unit == "" { // Compatibility with older exports and the web-entry master convention.
		if group == "bleaching" || group == "disease" {
			unit = "percent"
		} else {
			unit = "count"
		}
	}
	if unit != "count" && unit != "level" && unit != "percent" {
		return nil, "", unit, fmt.Errorf("invalid impact unit %q", unit)
	}
	if (group == "bleaching" || group == "disease") != (unit == "percent") {
		return nil, "", unit, fmt.Errorf("impact group/unit mismatch: %s/%s", group, unit)
	}
	if value == "" || value == "NA" || value == "-" || status == "not_recorded" {
		return nil, "not_recorded", unit, nil
	}
	v, err := strconv.ParseFloat(value, 64)
	if err != nil || math.IsNaN(v) || math.IsInf(v, 0) || v < 0 || (unit != "percent" && math.Trunc(v) != v) || (unit == "level" && v > 3) || (unit == "percent" && v > 100) {
		return nil, "", unit, fmt.Errorf("invalid %s impact value %q", unit, value)
	}
	return &v, "recorded", unit, nil
}

func importIncidentalSightings(ctx context.Context, tx pgx.Tx, path string, transects map[string]int, taxa map[string]int) (int, error) {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return 0, nil
	}
	rows, err := readCSV(path)
	if err != nil {
		return 0, err
	}
	for _, r := range rows {
		tid, ok := transects[r["transect_key"]]
		if !ok {
			return 0, fmt.Errorf("unknown sighting transect")
		}
		var taxonID *int
		if name := r["taxon_name_en__lookup"]; name != "" {
			id, ok := taxa["rare|"+name+"|"]
			if !ok {
				return 0, fmt.Errorf("unknown sighting taxon %q", name)
			}
			taxonID = &id
		}
		count, _, err := parseNullableImportCount(r["count"], "")
		if err != nil {
			return 0, err
		}
		_, err = tx.Exec(ctx, `INSERT INTO incidental_sighting(transect_id,observation_scope,segment,taxon_id,count,raw_text,source_file,source_sheet,source_cell)
   VALUES($1,$2,NULLIF($3,'')::smallint,$4,$5,$6,$7,$8,$9)
   ON CONFLICT(transect_id,source_file,source_sheet,source_cell) DO NOTHING`, tid, r["observation_scope"], r["segment"], taxonID, count, r["raw_text"], r["source_file"], r["source_sheet"], r["source_cell"])
		if err != nil {
			return 0, err
		}
	}
	return len(rows), nil
}

func prepareReplacement(ctx context.Context, tx pgx.Tx, backup string, dryRun bool) error {
	// EXCLUSIVE blocks concurrent edits but permits reads and pg_dump's ACCESS SHARE.
	// Include children explicitly: locking only survey does not block child edits.
	_, err := tx.Exec(ctx, `SET LOCAL lock_timeout='15s'; LOCK TABLE survey,site_description,event,transect,transect_participant,substrate_point,substrate_bleaching,belt_observation,impact_observation,incidental_sighting,reef_check_entry_metadata,reef_publication_audit IN EXCLUSIVE MODE`)
	if err != nil {
		return fmt.Errorf("lock observation tables: %w", err)
	}
	if !dryRun {
		if backup == "" {
			return fmt.Errorf("replacement requires a new backup file")
		}
		f, err := os.OpenFile(backup, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
		if err != nil {
			return err
		}
		var snapshot string
		if err = tx.QueryRow(ctx, `SELECT pg_export_snapshot()`).Scan(&snapshot); err != nil {
			f.Close()
			return err
		}
		c := tx.Conn().Config()
		cmd := exec.CommandContext(ctx, "pg_dump", "--format=custom", "--no-owner", "--no-acl", "--snapshot="+snapshot)
		cmd.Env = append(os.Environ(), "PGHOST="+c.Host, "PGPORT="+strconv.Itoa(int(c.Port)), "PGUSER="+c.User, "PGPASSWORD="+c.Password, "PGDATABASE="+c.Database)
		if c.TLSConfig == nil {
			cmd.Env = append(cmd.Env, "PGSSLMODE=disable")
		}
		cmd.Stdout = f
		err = cmd.Run()
		syncErr := f.Sync()
		closeErr := f.Close()
		if err != nil {
			return fmt.Errorf("pg_dump failed; observations unchanged (incomplete archive %s): %w", backup, err)
		}
		if syncErr != nil {
			return syncErr
		}
		if closeErr != nil {
			return closeErr
		}
		if err = exec.CommandContext(ctx, "pg_restore", "--list", backup).Run(); err != nil {
			return fmt.Errorf("backup archive validation: %w", err)
		}
		log.Printf("Full backup saved under observation write locks: %s", backup)
	}
	// Explicitly delete orphan/legacy transects too. Never TRUNCATE CASCADE into masters.
	_, err = tx.Exec(ctx, `DELETE FROM transect; DELETE FROM survey`)
	return err
}

func verifyReplacementCounts(ctx context.Context, tx pgx.Tx, dir string) error {
	for _, table := range []string{"survey", "event", "transect", "transect_participant", "substrate_point", "substrate_bleaching", "belt_observation", "impact_observation", "incidental_sighting"} {
		rows, err := readCSV(filepath.Join(dir, table+".csv"))
		if err != nil {
			return err
		}
		if len(rows) == 0 {
			return fmt.Errorf("refusing empty replacement table %s", table)
		}
		var actual int
		if err = tx.QueryRow(ctx, "SELECT count(*) FROM "+pgx.Identifier{table}.Sanitize()).Scan(&actual); err != nil {
			return err
		}
		if actual != len(rows) {
			return fmt.Errorf("%s: imported %d, expected %d; rollback", table, actual, len(rows))
		}
		log.Printf("Verified %s: %d", table, actual)
		fields := map[string][]string{"belt_observation": {"count"}, "impact_observation": {"raw_value"}, "substrate_bleaching": {"hc_bleached_count", "sc_bleached_count"}, "incidental_sighting": {"count", "segment"}}[table]
		for _, field := range fields {
			missing := 0
			for _, r := range rows {
				if r[field] == "" || r[field] == "NA" || r[field] == "-" {
					missing++
				}
			}
			if err = tx.QueryRow(ctx, "SELECT count(*) FROM "+pgx.Identifier{table}.Sanitize()+" WHERE "+pgx.Identifier{field}.Sanitize()+" IS NULL").Scan(&actual); err != nil {
				return err
			}
			if actual != missing {
				return fmt.Errorf("%s.%s NULL mismatch: %d vs %d", table, field, actual, missing)
			}
		}
	}
	return nil
}
