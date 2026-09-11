package main

import (
	"context"
	"encoding/csv"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

func main() {
	var (
		csvDir = flag.String("csv-dir", "", "Path to the csv directory containing the import files")
		dbURL  = flag.String("db-url", "", "PostgreSQL database URL or connection string")
	)
	flag.Parse()

	if *csvDir == "" {
		log.Fatal("missing --csv-dir flag")
	}
	if *dbURL == "" {
		*dbURL = os.Getenv("DATABASE_URL")
	}
	if *dbURL == "" {
		log.Fatal("missing --db-url or DATABASE_URL; choose the target database explicitly")
	}

	ctx := context.Background()
	config, err := pgxpool.ParseConfig(*dbURL)
	if err != nil {
		log.Fatalf("parse db config: %v", err)
	}
	pool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		log.Fatalf("connect to db: %v", err)
	}
	defer pool.Close()

	if err := pool.Ping(ctx); err != nil {
		log.Fatalf("ping db: %v", err)
	}
	log.Printf("connected to database successfully")

	start := time.Now()
	if err := runImport(ctx, pool, *csvDir); err != nil {
		log.Fatalf("import failed: %v", err)
	}
	log.Printf("=== Import finished successfully in %v ===", time.Since(start))
}

func runImport(ctx context.Context, pool *pgxpool.Pool, csvDir string) error {
	tx, err := pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	defer func() {
		_ = tx.Rollback(ctx)
	}()

	// 1. site_official_lookup.csv (61 rows)
	log.Printf("--> 1/8: Importing site_official_lookup.csv ...")
	siteMap, err := importSites(ctx, tx, filepath.Join(csvDir, "site_official_lookup.csv"))
	if err != nil {
		return fmt.Errorf("import sites: %w", err)
	}
	log.Printf("    Sites loaded/cached: %d", len(siteMap))

	// 2. Masters validation & cache
	log.Printf("--> 2/8: Validating & caching masters (substrate_type, taxon, impact_type) ...")
	substrateCodes, err := loadSubstrateCodes(ctx, tx)
	if err != nil {
		return fmt.Errorf("load substrate codes: %w", err)
	}
	log.Printf("    Substrate codes: %d", len(substrateCodes))

	taxonMap, err := loadTaxonMap(ctx, tx)
	if err != nil {
		return fmt.Errorf("load taxon map: %w", err)
	}
	log.Printf("    Taxon map entries: %d", len(taxonMap))

	impactMap, err := loadImpactMap(ctx, tx)
	if err != nil {
		return fmt.Errorf("load impact map: %w", err)
	}
	log.Printf("    Impact map entries: %d", len(impactMap))

	// 3. diver.csv (65 rows)
	log.Printf("--> 3/8: Importing diver.csv ...")
	diverMap, err := importDivers(ctx, tx, filepath.Join(csvDir, "diver.csv"))
	if err != nil {
		return fmt.Errorf("import divers: %w", err)
	}
	log.Printf("    Divers loaded/cached: %d", len(diverMap))

	// 4. survey.csv (392 rows)
	log.Printf("--> 4/8: Importing survey.csv ...")
	surveyMap, err := importSurveys(ctx, tx, filepath.Join(csvDir, "survey.csv"), siteMap)
	if err != nil {
		return fmt.Errorf("import surveys: %w", err)
	}
	log.Printf("    Surveys loaded/cached: %d", len(surveyMap))

	// 5. event.csv (726 rows)
	log.Printf("--> 5/8: Importing event.csv ...")
	eventMap, err := importEvents(ctx, tx, filepath.Join(csvDir, "event.csv"), surveyMap)
	if err != nil {
		return fmt.Errorf("import events: %w", err)
	}
	log.Printf("    Events loaded/cached: %d", len(eventMap))

	// 6. transect.csv (1,985 rows)
	log.Printf("--> 6/8: Importing transect.csv ...")
	transectMap, err := importTransects(ctx, tx, filepath.Join(csvDir, "transect.csv"))
	if err != nil {
		return fmt.Errorf("import transects: %w", err)
	}
	log.Printf("    Transects loaded/cached: %d", len(transectMap))

	// 7. transect_participant.csv (279 rows)
	log.Printf("--> 7/8: Importing transect_participant.csv ...")
	participantCount, err := importParticipants(ctx, tx, filepath.Join(csvDir, "transect_participant.csv"), transectMap, diverMap)
	if err != nil {
		return fmt.Errorf("import participants: %w", err)
	}
	log.Printf("    Participants upserted: %d", participantCount)

	// 8. Observations
	log.Printf("--> 8/8: Importing observation tables ...")
	spCount, err := importSubstratePoints(ctx, tx, filepath.Join(csvDir, "substrate_point.csv"), transectMap, substrateCodes)
	if err != nil {
		return fmt.Errorf("import substrate points: %w", err)
	}
	log.Printf("    Substrate points upserted: %d", spCount)

	sbCount, err := importSubstrateBleaching(ctx, tx, filepath.Join(csvDir, "substrate_bleaching.csv"), transectMap)
	if err != nil {
		return fmt.Errorf("import substrate bleaching: %w", err)
	}
	log.Printf("    Substrate bleaching upserted: %d", sbCount)

	boCount, err := importBeltObservations(ctx, tx, filepath.Join(csvDir, "belt_observation.csv"), transectMap, taxonMap)
	if err != nil {
		return fmt.Errorf("import belt observations: %w", err)
	}
	log.Printf("    Belt observations upserted: %d", boCount)

	ioCount, err := importImpactObservations(ctx, tx, filepath.Join(csvDir, "impact_observation.csv"), transectMap, impactMap)
	if err != nil {
		return fmt.Errorf("import impact observations: %w", err)
	}
	log.Printf("    Impact observations upserted: %d", ioCount)

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit transaction: %w", err)
	}
	log.Printf("Transaction committed successfully.")
	return nil
}

func cleanKey(s string) string {
	s = strings.TrimPrefix(s, "\ufeff")
	return strings.TrimSpace(s)
}

func readCSV(path string) ([]map[string]string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	reader := csv.NewReader(f)
	header, err := reader.Read()
	if err != nil {
		return nil, fmt.Errorf("read header: %w", err)
	}
	for i := range header {
		header[i] = cleanKey(header[i])
	}

	var rows []map[string]string
	for {
		record, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("read row: %w", err)
		}
		row := make(map[string]string, len(header))
		for i, val := range record {
			if i < len(header) {
				row[header[i]] = strings.TrimSpace(val)
			}
		}
		rows = append(rows, row)
	}
	return rows, nil
}

func importSites(ctx context.Context, tx pgx.Tx, path string) (map[string]int, error) {
	rows, err := readCSV(path)
	if err != nil {
		return nil, err
	}

	for _, r := range rows {
		nameEn := r["dive_site__lookup"]
		if nameEn == "" {
			continue
		}
		nameZh := r["name_zh"]
		region := r["region"]
		county := r["county"]
		location := r["location"]

		var lat, lon *float64
		if r["latitude"] != "" {
			if v, err := strconv.ParseFloat(r["latitude"], 64); err == nil {
				lat = &v
			}
		}
		if r["longitude"] != "" {
			if v, err := strconv.ParseFloat(r["longitude"], 64); err == nil {
				lon = &v
			}
		}

		// Check if exists
		var existingID int
		err := tx.QueryRow(ctx, `SELECT id FROM site WHERE name_en = $1`, nameEn).Scan(&existingID)
		if err == pgx.ErrNoRows {
			// Insert new site
			_, err = tx.Exec(ctx, `
				INSERT INTO site (name_en, name_zh, region, county, location, latitude, longitude)
				VALUES ($1, $2, NULLIF($3, ''), NULLIF($4, ''), NULLIF($5, ''), $6, $7)
			`, nameEn, nameZh, region, county, location, lat, lon)
			if err != nil {
				return nil, fmt.Errorf("insert site %s: %w", nameEn, err)
			}
		} else if err != nil {
			return nil, fmt.Errorf("query site %s: %w", nameEn, err)
		} else {
			// Update without overwriting non-empty values with empty
			_, err = tx.Exec(ctx, `
				UPDATE site SET
					name_zh = COALESCE(NULLIF($1, ''), name_zh),
					region = COALESCE(NULLIF($2, ''), region),
					county = COALESCE(NULLIF($3, ''), county),
					location = COALESCE(NULLIF($4, ''), location),
					latitude = COALESCE($5, latitude),
					longitude = COALESCE($6, longitude)
				WHERE name_en = $7
			`, nameZh, region, county, location, lat, lon, nameEn)
			if err != nil {
				return nil, fmt.Errorf("update site %s: %w", nameEn, err)
			}
		}
	}

	siteMap := make(map[string]int)
	dbRows, err := tx.Query(ctx, `SELECT id, name_en FROM site WHERE name_en IS NOT NULL`)
	if err != nil {
		return nil, err
	}
	defer dbRows.Close()
	for dbRows.Next() {
		var id int
		var nameEn string
		if err := dbRows.Scan(&id, &nameEn); err != nil {
			return nil, err
		}
		siteMap[nameEn] = id
	}
	return siteMap, dbRows.Err()
}

func loadSubstrateCodes(ctx context.Context, tx pgx.Tx) (map[string]bool, error) {
	rows, err := tx.Query(ctx, `SELECT code FROM substrate_type`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	codes := make(map[string]bool)
	for rows.Next() {
		var code string
		if err := rows.Scan(&code); err != nil {
			return nil, err
		}
		codes[code] = true
	}
	return codes, rows.Err()
}

func loadTaxonMap(ctx context.Context, tx pgx.Tx) (map[string]int, error) {
	rows, err := tx.Query(ctx, `SELECT id, taxon_group, name_en, COALESCE(size_class, '') FROM taxon`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	m := make(map[string]int)
	for rows.Next() {
		var id int
		var group, nameEn, sizeClass string
		if err := rows.Scan(&id, &group, &nameEn, &sizeClass); err != nil {
			return nil, err
		}
		key := fmt.Sprintf("%s|%s|%s", group, nameEn, sizeClass)
		m[key] = id
	}
	return m, rows.Err()
}

func loadImpactMap(ctx context.Context, tx pgx.Tx) (map[string]int, error) {
	rows, err := tx.Query(ctx, `SELECT id, impact_group, name_en FROM impact_type`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	m := make(map[string]int)
	for rows.Next() {
		var id int
		var group, nameEn string
		if err := rows.Scan(&id, &group, &nameEn); err != nil {
			return nil, err
		}
		key := fmt.Sprintf("%s|%s", group, nameEn)
		m[key] = id
	}
	return m, rows.Err()
}

func importDivers(ctx context.Context, tx pgx.Tx, path string) (map[string]int, error) {
	rows, err := readCSV(path)
	if err != nil {
		return nil, err
	}

	for _, r := range rows {
		diverKey := r["diver_key"]
		if diverKey == "" {
			continue
		}
		nameZh := r["name_zh"]
		nameEn := r["name_en"]
		code := r["reef_check_code"]
		isActive := r["is_active"] != "false"

		_, err := tx.Exec(ctx, `
			INSERT INTO diver (diver_key, name_zh, name_en, reef_check_code, is_active)
			VALUES ($1, NULLIF($2, ''), NULLIF($3, ''), NULLIF($4, ''), $5)
			ON CONFLICT (diver_key) DO UPDATE SET
				name_zh = COALESCE(EXCLUDED.name_zh, diver.name_zh),
				name_en = COALESCE(EXCLUDED.name_en, diver.name_en),
				reef_check_code = COALESCE(EXCLUDED.reef_check_code, diver.reef_check_code),
				is_active = EXCLUDED.is_active
		`, diverKey, nameZh, nameEn, code, isActive)
		if err != nil {
			return nil, fmt.Errorf("upsert diver %s: %w", diverKey, err)
		}
	}

	diverMap := make(map[string]int)
	dbRows, err := tx.Query(ctx, `SELECT id, diver_key FROM diver WHERE diver_key IS NOT NULL`)
	if err != nil {
		return nil, err
	}
	defer dbRows.Close()
	for dbRows.Next() {
		var id int
		var k string
		if err := dbRows.Scan(&id, &k); err != nil {
			return nil, err
		}
		diverMap[k] = id
	}
	return diverMap, dbRows.Err()
}

func importSurveys(ctx context.Context, tx pgx.Tx, path string, siteMap map[string]int) (map[string]int, error) {
	rows, err := readCSV(path)
	if err != nil {
		return nil, err
	}

	surveyMap := make(map[string]int)
	for _, r := range rows {
		surveyKey := r["survey_key"]
		siteName := r["site_name_en__lookup"]
		siteID, ok := siteMap[siteName]
		if !ok {
			return nil, fmt.Errorf("site %q not found for survey %s", siteName, surveyKey)
		}
		startDate := r["start_date"]
		endDate := r["end_date"]
		label := r["label"]

		var surveyID int
		err := tx.QueryRow(ctx, `
			INSERT INTO survey (site_id, start_date, end_date, label)
			VALUES ($1, $2, $3, NULLIF($4, ''))
			ON CONFLICT (site_id, start_date) DO UPDATE SET
				end_date = EXCLUDED.end_date,
				label = COALESCE(EXCLUDED.label, survey.label)
			RETURNING id
		`, siteID, startDate, endDate, label).Scan(&surveyID)
		if err != nil {
			return nil, fmt.Errorf("upsert survey %s: %w", surveyKey, err)
		}
		surveyMap[surveyKey] = surveyID
	}
	return surveyMap, nil
}

func importEvents(ctx context.Context, tx pgx.Tx, path string, surveyMap map[string]int) (map[string]string, error) {
	rows, err := readCSV(path)
	if err != nil {
		return nil, err
	}

	eventMap := make(map[string]string)
	for _, r := range rows {
		eventKey := r["event_key"]
		eventID := r["event_id"]
		surveyKey := r["survey_key"]
		surveyID, ok := surveyMap[surveyKey]
		if !ok {
			return nil, fmt.Errorf("survey %q not found for event %s", surveyKey, eventID)
		}
		surveyDate := r["survey_date"]
		eventTime := r["event_time"]
		if eventTime == "" {
			eventTime = "na"
		}
		depthM, err := strconv.ParseFloat(r["depth_m"], 64)
		if err != nil {
			return nil, fmt.Errorf("parse depth %q for event %s: %w", r["depth_m"], eventID, err)
		}

		_, err = tx.Exec(ctx, `
			INSERT INTO event (survey_id, event_id, survey_date, event_time, depth_m)
			VALUES ($1, $2, $3, $4, $5)
			ON CONFLICT (event_id) DO UPDATE SET
				survey_id = EXCLUDED.survey_id,
				survey_date = EXCLUDED.survey_date,
				event_time = EXCLUDED.event_time,
				depth_m = EXCLUDED.depth_m
		`, surveyID, eventID, surveyDate, eventTime, depthM)
		if err != nil {
			return nil, fmt.Errorf("upsert event %s: %w", eventID, err)
		}
		eventMap[eventKey] = eventID
	}
	return eventMap, nil
}

func importTransects(ctx context.Context, tx pgx.Tx, path string) (map[string]int, error) {
	rows, err := readCSV(path)
	if err != nil {
		return nil, err
	}

	transectMap := make(map[string]int)
	for _, r := range rows {
		transectKey := r["transect_key"]
		eventID := r["event_id"]
		method := r["method"]

		var startTime *string
		if r["start_time"] != "" {
			v := r["start_time"]
			startTime = &v
		}
		var waterTemp *float64
		if r["water_temp_c"] != "" {
			if v, err := strconv.ParseFloat(r["water_temp_c"], 64); err == nil {
				waterTemp = &v
			}
		}
		var visMin *float64
		if r["visibility_min_m"] != "" {
			if v, err := strconv.ParseFloat(r["visibility_min_m"], 64); err == nil {
				visMin = &v
			}
		}
		var visMax *float64
		if r["visibility_max_m"] != "" {
			if v, err := strconv.ParseFloat(r["visibility_max_m"], 64); err == nil {
				visMax = &v
			}
		}
		var comments *string
		if r["comments"] != "" {
			v := r["comments"]
			comments = &v
		}
		var rkcNote *string
		if r["rkc_bleaching_note"] != "" {
			v := r["rkc_bleaching_note"]
			rkcNote = &v
		}

		var transectID int
		err := tx.QueryRow(ctx, `
			INSERT INTO transect (event_id, method, start_time, water_temp_c, visibility_min_m, visibility_max_m, comments, rkc_bleaching_note)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
			ON CONFLICT (event_id, method) DO UPDATE SET
				start_time = EXCLUDED.start_time,
				water_temp_c = EXCLUDED.water_temp_c,
				visibility_min_m = EXCLUDED.visibility_min_m,
				visibility_max_m = EXCLUDED.visibility_max_m,
				comments = EXCLUDED.comments,
				rkc_bleaching_note = EXCLUDED.rkc_bleaching_note
			RETURNING id
		`, eventID, method, startTime, waterTemp, visMin, visMax, comments, rkcNote).Scan(&transectID)
		if err != nil {
			return nil, fmt.Errorf("upsert transect %s: %w", transectKey, err)
		}
		transectMap[transectKey] = transectID
	}
	return transectMap, nil
}

func importParticipants(ctx context.Context, tx pgx.Tx, path string, transectMap map[string]int, diverMap map[string]int) (int, error) {
	rows, err := readCSV(path)
	if err != nil {
		return 0, err
	}

	count := 0
	for _, r := range rows {
		transectKey := r["transect_key"]
		transectID, ok := transectMap[transectKey]
		if !ok {
			return 0, fmt.Errorf("transect %q not found for participant", transectKey)
		}
		diverKey := r["diver_key"]
		diverID, ok := diverMap[diverKey]
		if !ok {
			return 0, fmt.Errorf("diver %q not found for participant", diverKey)
		}
		role := r["role"]

		_, err := tx.Exec(ctx, `
			INSERT INTO transect_participant (transect_id, diver_id, role)
			VALUES ($1, $2, $3)
			ON CONFLICT (transect_id, diver_id, role) DO NOTHING
		`, transectID, diverID, role)
		if err != nil {
			return 0, fmt.Errorf("insert participant: %w", err)
		}
		count++
	}
	return count, nil
}

func importSubstratePoints(ctx context.Context, tx pgx.Tx, path string, transectMap map[string]int, substrateCodes map[string]bool) (int, error) {
	rows, err := readCSV(path)
	if err != nil {
		return 0, err
	}

	const batchSize = 2000
	total := len(rows)
	for i := 0; i < total; i += batchSize {
		end := i + batchSize
		if end > total {
			end = total
		}
		batch := &pgx.Batch{}
		for _, r := range rows[i:end] {
			transectKey := r["transect_key"]
			transectID, ok := transectMap[transectKey]
			if !ok {
				return 0, fmt.Errorf("transect %q not found in substrate_point", transectKey)
			}
			segment, _ := strconv.Atoi(r["segment"])
			pos, _ := strconv.ParseFloat(r["position_m"], 64)
			code := r["substrate_code"]
			if !substrateCodes[code] {
				return 0, fmt.Errorf("invalid substrate_code %q", code)
			}
			layer := r["substrate_layer"]
			if layer == "" {
				layer = "surface"
			}

			batch.Queue(`
				INSERT INTO substrate_point (transect_id, segment, position_m, substrate_layer, substrate_code)
				VALUES ($1, $2, $3, $4, $5)
				ON CONFLICT (transect_id, position_m, substrate_layer) DO UPDATE SET
					segment = EXCLUDED.segment,
					substrate_code = EXCLUDED.substrate_code
			`, transectID, segment, pos, layer, code)
		}

		br := tx.SendBatch(ctx, batch)
		for j := i; j < end; j++ {
			if _, err := br.Exec(); err != nil {
				br.Close()
				return 0, fmt.Errorf("batch exec substrate point %d: %w", j, err)
			}
		}
		if err := br.Close(); err != nil {
			return 0, fmt.Errorf("batch close substrate point: %w", err)
		}
	}
	return total, nil
}

func importSubstrateBleaching(ctx context.Context, tx pgx.Tx, path string, transectMap map[string]int) (int, error) {
	rows, err := readCSV(path)
	if err != nil {
		return 0, err
	}

	batch := &pgx.Batch{}
	for _, r := range rows {
		transectKey := r["transect_key"]
		transectID, ok := transectMap[transectKey]
		if !ok {
			return 0, fmt.Errorf("transect %q not found in substrate_bleaching", transectKey)
		}
		segment, _ := strconv.Atoi(r["segment"])
		hcBleached, _ := strconv.Atoi(r["hc_bleached_count"])
		scBleached, _ := strconv.Atoi(r["sc_bleached_count"])

		batch.Queue(`
			INSERT INTO substrate_bleaching (transect_id, segment, hc_bleached_count, sc_bleached_count)
			VALUES ($1, $2, $3, $4)
			ON CONFLICT (transect_id, segment) DO UPDATE SET
				hc_bleached_count = EXCLUDED.hc_bleached_count,
				sc_bleached_count = EXCLUDED.sc_bleached_count
		`, transectID, segment, hcBleached, scBleached)
	}

	br := tx.SendBatch(ctx, batch)
	for i := range rows {
		if _, err := br.Exec(); err != nil {
			br.Close()
			return 0, fmt.Errorf("batch exec substrate bleaching %d: %w", i, err)
		}
	}
	if err := br.Close(); err != nil {
		return 0, fmt.Errorf("batch close substrate bleaching: %w", err)
	}
	return len(rows), nil
}

func importBeltObservations(ctx context.Context, tx pgx.Tx, path string, transectMap map[string]int, taxonMap map[string]int) (int, error) {
	rows, err := readCSV(path)
	if err != nil {
		return 0, err
	}

	const batchSize = 2000
	total := len(rows)
	for i := 0; i < total; i += batchSize {
		end := i + batchSize
		if end > total {
			end = total
		}
		batch := &pgx.Batch{}
		for _, r := range rows[i:end] {
			transectKey := r["transect_key"]
			transectID, ok := transectMap[transectKey]
			if !ok {
				return 0, fmt.Errorf("transect %q not found in belt_observation", transectKey)
			}
			group := r["taxon_group"]
			nameEn := r["taxon_name_en__lookup"]
			sizeClass := r["taxon_size_class__lookup"]
			key := fmt.Sprintf("%s|%s|%s", group, nameEn, sizeClass)
			taxonID, ok := taxonMap[key]
			if !ok {
				return 0, fmt.Errorf("taxon %q not found in belt_observation", key)
			}
			segment, _ := strconv.Atoi(r["segment"])
			count, _ := strconv.Atoi(r["count"])

			batch.Queue(`
				INSERT INTO belt_observation (transect_id, taxon_id, segment, count)
				VALUES ($1, $2, $3, $4)
				ON CONFLICT (transect_id, taxon_id, segment) DO UPDATE SET
					count = EXCLUDED.count
			`, transectID, taxonID, segment, count)
		}

		br := tx.SendBatch(ctx, batch)
		for j := i; j < end; j++ {
			if _, err := br.Exec(); err != nil {
				br.Close()
				return 0, fmt.Errorf("batch exec belt observation %d: %w", j, err)
			}
		}
		if err := br.Close(); err != nil {
			return 0, fmt.Errorf("batch close belt observation: %w", err)
		}
	}
	return total, nil
}

func importImpactObservations(ctx context.Context, tx pgx.Tx, path string, transectMap map[string]int, impactMap map[string]int) (int, error) {
	rows, err := readCSV(path)
	if err != nil {
		return 0, err
	}

	const batchSize = 2000
	total := len(rows)
	for i := 0; i < total; i += batchSize {
		end := i + batchSize
		if end > total {
			end = total
		}
		batch := &pgx.Batch{}
		for _, r := range rows[i:end] {
			transectKey := r["transect_key"]
			transectID, ok := transectMap[transectKey]
			if !ok {
				return 0, fmt.Errorf("transect %q not found in impact_observation", transectKey)
			}
			group := r["impact_group"]
			nameEn := r["impact_name_en__lookup"]
			key := fmt.Sprintf("%s|%s", group, nameEn)
			impactID, ok := impactMap[key]
			if !ok {
				return 0, fmt.Errorf("impact %q not found in impact_observation", key)
			}
			segment, _ := strconv.Atoi(r["segment"])
			rawValue, err := strconv.ParseFloat(r["raw_value"], 64)
			if err != nil {
				return 0, fmt.Errorf("parse raw_value %q: %w", r["raw_value"], err)
			}

			batch.Queue(`
				INSERT INTO impact_observation (transect_id, impact_type_id, segment, raw_value)
				VALUES ($1, $2, $3, $4)
				ON CONFLICT (transect_id, impact_type_id, segment) DO UPDATE SET
					raw_value = EXCLUDED.raw_value
			`, transectID, impactID, segment, rawValue)
		}

		br := tx.SendBatch(ctx, batch)
		for j := i; j < end; j++ {
			if _, err := br.Exec(); err != nil {
				br.Close()
				return 0, fmt.Errorf("batch exec impact observation %d: %w", j, err)
			}
		}
		if err := br.Close(); err != nil {
			return 0, fmt.Errorf("batch close impact observation: %w", err)
		}
	}
	return total, nil
}
