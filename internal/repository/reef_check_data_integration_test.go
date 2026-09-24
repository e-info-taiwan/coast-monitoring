package repository

import (
	"coast-monitoring/internal/service"
	"context"
	"encoding/json"
	"fmt"
	"github.com/jackc/pgx/v5/pgxpool"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

// Opt-in: run against a migrated local database. All fixture writes are rolled back.
func TestReefDataPostgresRoundTrip(t *testing.T) {
	dsn := os.Getenv("REEF_DATA_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("set REEF_DATA_TEST_DATABASE_URL to a migrated local PostgreSQL database")
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(ctx)
	var siteID, surveyID, eventID, lineID, invertID int
	mustScan := func(query string, dest any, args ...any) {
		t.Helper()
		if err := tx.QueryRow(ctx, query, args...).Scan(dest); err != nil {
			t.Fatal(err)
		}
	}
	mustExec := func(query string, args ...any) {
		t.Helper()
		if _, err := tx.Exec(ctx, query, args...); err != nil {
			t.Fatal(err)
		}
	}
	mustScan(`INSERT INTO site(name_zh) VALUES('Integration fixture') RETURNING id`, &siteID)
	mustScan(`INSERT INTO survey(site_id,start_date,end_date) VALUES($1,'2025-01-01','2025-01-02') RETURNING id`, &surveyID, siteID)
	eventKey := "test-event-" + t.Name()
	mustScan(`INSERT INTO event(survey_id,event_id,survey_date,event_time,depth_m) VALUES($1,$2,'2025-01-01','na',5.5) RETURNING id`, &eventID, surveyID, eventKey)
	mustScan(`INSERT INTO transect(event_id,method) VALUES($1,'line') RETURNING id`, &lineID, eventKey)
	mustScan(`INSERT INTO transect(event_id,method,water_temp_c) VALUES($1,'belt_invert',26.5) RETURNING id`, &invertID, eventKey)
	var pointID, bleachID, beltID, impactID int64
	var taxonID, impactTypeID int
	mustScan(`SELECT id FROM taxon WHERE taxon_group='invert' AND NOT is_aggregate ORDER BY id LIMIT 1`, &taxonID)
	mustScan(`SELECT id FROM impact_type WHERE has_raw_count ORDER BY id LIMIT 1`, &impactTypeID)
	mustScan(`INSERT INTO substrate_point(transect_id,segment,position_m,substrate_code,substrate_layer) VALUES($1,1,0,'NA','surface') RETURNING id`, &pointID, lineID)
	mustExec(`INSERT INTO substrate_point(transect_id,segment,position_m,substrate_code,substrate_layer) VALUES($1,1,0,'HC','down')`, lineID)
	mustScan(`INSERT INTO substrate_bleaching(transect_id,segment) VALUES($1,1) RETURNING id`, &bleachID, lineID)
	mustScan(`INSERT INTO belt_observation(transect_id,taxon_id,segment,count) VALUES($1,$2,1,0) RETURNING id`, &beltID, invertID, taxonID)
	mustScan(`INSERT INTO impact_observation(transect_id,impact_type_id,segment,raw_value) VALUES($1,$2,1,8) RETURNING id`, &impactID, invertID, impactTypeID)
	repo := NewReefDataRepository(tx)
	events, err := repo.ListEvents(ctx)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, e := range events {
		if e.ID == eventID {
			found = true
			if e.Latitude != nil || e.Longitude != nil || len(e.Methods) != 2 || e.EndDate != "2025-01-02" {
				t.Fatalf("event lost nulls or hierarchy: %+v", e)
			}
		}
	}
	if !found {
		t.Fatal("event not listed")
	}
	detail, err := repo.Event(ctx, eventID)
	if err != nil {
		t.Fatal(err)
	}
	if len(detail.Transects) != 2 {
		t.Fatal("missing method was fabricated")
	}
	codes, err := repo.Codes(ctx)
	if err != nil {
		t.Fatal(err)
	}
	line, err := repo.Transect(ctx, lineID, true)
	if err != nil {
		t.Fatal(err)
	}
	hc, sc := 1, 0
	u := service.ReefDataUpdate{Version: line.Version, Changes: []service.ReefDataChange{{Kind: "point", ID: pointID, Code: "SI(HC)"}, {Kind: "bleaching", ID: bleachID, HC: &hc, SC: &sc}}}
	if err = u.Validate(line, codes); err != nil {
		t.Fatal(err)
	}
	updated, err := repo.Update(ctx, lineID, u)
	if err != nil {
		t.Fatal(err)
	}
	if len(updated.Points) != 2 || updated.Points[0].Code != "SI(HC)" || updated.Points[1].Code != "HC" || updated.Bleaching[0].HC != 1 {
		t.Fatalf("line update lost layer/values: %+v", updated)
	}
	if u.Validate(updated, codes) != service.ErrConflict {
		t.Fatal("stale update accepted")
	}
	invert, err := repo.Transect(ctx, invertID, true)
	if err != nil {
		t.Fatal(err)
	}
	value, raw := 7.0, 12.0
	u = service.ReefDataUpdate{Version: invert.Version, Changes: []service.ReefDataChange{{Kind: "belt", ID: beltID, Value: &value}, {Kind: "impact", ID: impactID, Value: &raw}}}
	if err = u.Validate(invert, codes); err != nil {
		t.Fatal(err)
	}
	updated, err = repo.Update(ctx, invertID, u)
	if err != nil {
		t.Fatal(err)
	}
	if updated.Belt[0].Count != 7 || updated.Impacts[0].RawValue != 12 || updated.WaterTemp == nil || *updated.WaterTemp != 26.5 || updated.VisibilityMin != nil || len(updated.Participants) != 0 {
		t.Fatalf("unexpected round trip: %+v", updated)
	}
}

// The exact supplied confirmed-only package, for opt-in full-data verification.
func TestReefDataConfirmedPackage(t *testing.T) {
	dsn := os.Getenv("REEF_DATA_TEST_DATABASE_URL")
	if dsn == "" || os.Getenv("REEF_DATA_CONFIRMED_PACKAGE") != "1" {
		t.Skip("requires the local confirmed-only v1.7 import")
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	repo := NewReefDataRepository(pool)
	events, err := repo.ListEvents(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 726 {
		t.Fatalf("events=%d want 726", len(events))
	}
	codes, err := repo.Codes(ctx)
	if err != nil {
		t.Fatal(err)
	}
	previewDir := os.Getenv("REEF_DATA_PREVIEW_DIR")
	save := func(name string, value any) {
		if previewDir == "" {
			return
		}
		b, err := json.Marshal(value)
		if err != nil {
			t.Fatal(err)
		}
		if err = os.WriteFile(filepath.Join(previewDir, name), b, 0600); err != nil {
			t.Fatal(err)
		}
	}
	save("events.json", events)
	save("codes.json", codes)
	transects, points, bleaching, belt, impacts, participants := 0, 0, 0, 0, 0, 0
	for _, e := range events {
		detail, err := repo.Event(ctx, e.ID)
		if err != nil {
			t.Fatalf("event %d: %v", e.ID, err)
		}
		save(fmt.Sprintf("event-%d.json", e.ID), detail)
		for _, row := range detail.Transects {
			transects++
			points += len(row.Points)
			bleaching += len(row.Bleaching)
			belt += len(row.Belt)
			impacts += len(row.Impacts)
			participants += len(row.Participants)
			if row.Version == "" {
				t.Fatal("missing version")
			}
		}
	}
	got := []int{transects, points, bleaching, belt, impacts, participants}
	want := []int{1985, 104800, 2620, 74780, 22956, 279}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("detail totals=%v want %v", got, want)
	}
	t.Logf("verified 726 events: transects,points,bleaching,belt,impacts,participants=%v", got)
}

func TestReefDataSubmitSurvey(t *testing.T) {
	dsn := os.Getenv("REEF_DATA_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("set REEF_DATA_TEST_DATABASE_URL to a migrated local PostgreSQL database")
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(ctx)

	repo := NewReefDataRepository(tx)

	bleachHC := 5
	cnt := 3
	rawVal := 1.0
	sub := service.ReefCheckSurveySubmission{
		Format: "reefcheck-web-prototype-v2",
		Event: service.ReefCheckEventInput{
			SiteNameZH: "野柳",
			SurveyDate: "2025-06-01",
			EventTime:  "09:30",
			DepthM:     6.0,
		},
		Transects: []service.ReefCheckTransectInput{
			{Method: "line", StartTime: "09:30", Recorders: []string{"志工甲", "志工乙"}, TeamLeader: "隊長王"},
			{Method: "belt_fish", StartTime: "09:30", Recorders: []string{"志工丙"}},
			{Method: "belt_invert", StartTime: "09:30", Recorders: []string{"志工丁"}},
		},
		SubstratePoints: []service.ReefCheckSubstratePointInput{
			{Segment: 1, PositionM: 0.0, SubstrateCode: "HC", SubstrateLayer: "surface"},
			{Segment: 1, PositionM: 0.5, SubstrateCode: "SI", SubstrateLayer: "surface"},
		},
		MudAudit: []service.ReefCheckMudAuditInput{
			{Segment: 1, PositionM: 0.5, Surface: "SI", Down: "HC", Canonical: "SI(HC)"},
		},
		SubstrateBleaching: []service.ReefCheckBleachingInput{
			{Segment: 1, SubstrateCode: "HC", BleachedPoints: &bleachHC},
		},
		BeltObservations: []service.ReefCheckBeltObservationInput{
			{TaxonGroup: "fish", TaxonNameENLookup: "Butterflyfish", Segment: 1, Count: &cnt},
		},
		ImpactObservations: []service.ReefCheckImpactObservationInput{
			{ImpactGroup: "trash", ImpactNameENLookup: "Trash: general", Segment: 1, RawValue: &rawVal},
		},
	}

	res, err := repo.SubmitSurvey(ctx, sub, nil)
	if err != nil {
		t.Fatalf("SubmitSurvey error: %v", err)
	}
	if res.Status != "saved" || res.ReceiptID == "" || res.EventID == "" {
		t.Fatalf("unexpected res: %+v", res)
	}

	cfg, err := repo.Config(ctx)
	if err != nil {
		t.Fatalf("Config error: %v", err)
	}
	if len(cfg.Sites) == 0 || len(cfg.Codes) == 0 {
		t.Fatalf("empty config sites or codes: %+v", cfg)
	}
}
