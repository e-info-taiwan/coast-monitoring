package repository

import (
	"coast-monitoring/internal/service"
	"context"
	"encoding/json"
	"errors"
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

	var siteID int
	if err = tx.QueryRow(ctx, `INSERT INTO site(name_zh,name_en) VALUES('Submit fixture','SubmitFixture') RETURNING id`).Scan(&siteID); err != nil {
		t.Fatal(err)
	}
	sub := service.ReefCheckSurveySubmission{
		Event:         service.ReefCheckEventInput{SiteID: siteID, SurveyDate: "2025-06-01", EventTime: "09:30", DepthM: 6},
		Transects:     []service.ReefCheckTransectInput{{TransectKey: "line", Method: "line", Recorders: []string{"Fixture recorder"}}, {TransectKey: "fish", Method: "belt_fish", FishSizeMode: "split", Recorders: []string{"Fixture recorder"}}, {TransectKey: "invert", Method: "belt_invert", Recorders: []string{"Fixture recorder"}}},
		MissingReason: "Fixture: segment 2 not recorded",
	}
	fillSubmissionCatalog(t, ctx, tx, &sub)
	for segment := 1; segment <= 4; segment++ {
		for i := 0; i < 40; i++ {
			sub.SubstratePoints = append(sub.SubstratePoints, service.ReefCheckSubstratePointInput{TransectKey: "line", Segment: segment, PositionM: float64((segment-1)*25) + float64(i)/2, SubstrateCode: "0", SubstrateLayer: "surface"})
		}
	}

	for segment := 1; segment <= 4; segment++ {
		for _, code := range []string{"HC", "SC"} {
			zero := 0
			sub.SubstrateBleaching = append(sub.SubstrateBleaching, service.ReefCheckBleachingInput{TransectKey: "line", Segment: segment, SubstrateCode: code, BleachedPoints: &zero, RecordStatus: "recorded"})
		}
	}
	// The service accepts complete submitted rows, but the DB also requires every
	// active catalog row. Omitting one entire species must roll back the event.
	incomplete := sub
	incomplete.BeltObservations = nil
	for _, o := range sub.BeltObservations {
		if o.TaxonNameENLookup != "Snapper" {
			incomplete.BeltObservations = append(incomplete.BeltObservations, o)
		}
	}
	if _, err := repo.SubmitSurvey(ctx, incomplete, nil); !errors.Is(err, service.ErrValidation) {
		t.Fatalf("omitted species accepted: %v", err)
	}
	var leaked int
	if err = tx.QueryRow(ctx, `SELECT count(*) FROM event e JOIN survey s ON s.id=e.survey_id WHERE s.site_id=$1`, siteID).Scan(&leaked); err != nil || leaked != 0 {
		t.Fatalf("rejected submission leaked event: %d %v", leaked, err)
	}
	res, err := repo.SubmitSurvey(ctx, sub, nil)
	if err != nil {
		t.Fatalf("SubmitSurvey error: %v", err)
	}
	if res.Status != "saved" || res.ReceiptID == "" || res.EventID == "" {
		t.Fatalf("unexpected res: %+v", res)
	}

	if res.ReviewStatus != "draft" {
		t.Fatal("submission did not remain draft")
	}
	if _, err = repo.SubmitSurvey(ctx, sub, nil); !errors.Is(err, service.ErrConflict) {
		t.Fatalf("duplicate=%v", err)
	}
	var count, status string
	if err = tx.QueryRow(ctx, `SELECT count::text,record_status FROM belt_observation b JOIN transect t ON t.id=b.transect_id WHERE t.event_id=$1 AND b.segment=2`, res.EventID).Scan(&count, &status); err != nil || status != "not_recorded" {
		t.Fatalf("NA lost: %s %s %v", count, status, err)
	}

	var actor string
	if err = tx.QueryRow(ctx, `INSERT INTO users(email,name,role) VALUES('publication-fixture@example.invalid','Fixture','admin') RETURNING id::text`).Scan(&actor); err != nil {
		t.Fatal(err)
	}
	if _, err = tx.Exec(ctx, `UPDATE event SET missing_reason='' WHERE id=$1`, res.EventDBID); err != nil {
		t.Fatal(err)
	}
	if err = repo.SetPublication(ctx, res.EventDBID, "published", actor); !errors.Is(err, service.ErrValidation) {
		t.Fatalf("NA without reason published: %v", err)
	}
	if _, err = tx.Exec(ctx, `UPDATE event SET missing_reason=$2 WHERE id=$1`, res.EventDBID, sub.MissingReason); err != nil {
		t.Fatal(err)
	}
	var beltID, taxonID int
	if err = tx.QueryRow(ctx, `SELECT b.transect_id,b.taxon_id FROM belt_observation b JOIN transect t ON t.id=b.transect_id JOIN taxon x ON x.id=b.taxon_id WHERE t.event_id=$1 AND x.name_en='Butterflyfish' AND b.segment=1`, res.EventID).Scan(&beltID, &taxonID); err != nil {
		t.Fatal(err)
	}
	if _, err = tx.Exec(ctx, `DELETE FROM belt_observation WHERE transect_id=$1 AND taxon_id=$2 AND segment=4`, beltID, taxonID); err != nil {
		t.Fatal(err)
	}
	if err = repo.SetPublication(ctx, res.EventDBID, "published", actor); !errors.Is(err, service.ErrValidation) {
		t.Fatalf("incomplete board published: %v", err)
	}
	var auditCount int
	if err = tx.QueryRow(ctx, `SELECT publication_status,(SELECT count(*) FROM reef_publication_audit WHERE event_id=$1) FROM event WHERE id=$1`, res.EventDBID).Scan(&status, &auditCount); err != nil || status != "draft" || auditCount != 0 {
		t.Fatalf("rejected publication mutated data: %s %d %v", status, auditCount, err)
	}
	if err = upsertBeltObservation(ctx, tx, beltID, taxonID, 4, 0, "recorded"); err != nil {
		t.Fatal(err)
	}
	if err = repo.SetPublication(ctx, res.EventDBID, "published", actor); err != nil {
		t.Fatalf("complete explicit NA/0 rejected: %v", err)
	}
	filter, _ := service.ParsePublicReefFilter(nil)
	series, err := repo.PublicSeries(ctx, siteID, filter)
	if err != nil {
		t.Fatal(err)
	}
	foundDensity := false
	for _, row := range series {
		if row.Chart == "taxa" && row.Unit == "individuals_per_100m2" {
			foundDensity = true
			if row.DensityPer100M2 == nil || row.Mean == nil || *row.DensityPer100M2 != *row.Mean {
				t.Fatalf("density contract lost: %+v", row)
			}
		}
	}
	if !foundDensity {
		t.Fatal("no explicit density rows")
	}
	if err = upsertBeltObservation(ctx, tx, beltID, taxonID, 2, 7, "recorded"); err != nil {
		t.Fatal(err)
	}
	if err = tx.QueryRow(ctx, `SELECT count::text,record_status FROM belt_observation WHERE transect_id=$1 AND taxon_id=$2 AND segment=2`, beltID, taxonID).Scan(&count, &status); err != nil || count != "7" || status != "recorded" {
		t.Fatalf("NA to recorded upsert lost: %s %s %v", count, status, err)
	}
	if err = upsertBeltObservation(ctx, tx, beltID, taxonID, 2, 0, "not_recorded"); err != nil {
		t.Fatal(err)
	}
	var invertID, impactID int
	if err = tx.QueryRow(ctx, `SELECT o.transect_id,o.impact_type_id FROM impact_observation o JOIN transect t ON t.id=o.transect_id WHERE t.event_id=$1 LIMIT 1`, res.EventID).Scan(&invertID, &impactID); err != nil {
		t.Fatal(err)
	}
	for _, flag := range []string{"not_recorded", "recorded", "not_recorded"} {
		if err = upsertImpactObservation(ctx, tx, invertID, impactID, 1, 0, flag); err != nil {
			t.Fatal(err)
		}
		if err = tx.QueryRow(ctx, `SELECT record_status FROM impact_observation WHERE transect_id=$1 AND impact_type_id=$2 AND segment=1`, invertID, impactID).Scan(&status); err != nil || status != flag {
			t.Fatalf("impact upsert status=%s expected=%s %v", status, flag, err)
		}
	}

	cfg, err := repo.Config(ctx)
	if err != nil {
		t.Fatalf("Config error: %v", err)
	}
	if len(cfg.Sites) == 0 || len(cfg.Codes) == 0 {
		t.Fatalf("empty config sites or codes: %+v", cfg)
	}
}

// Real active catalog, never a fixture that omits a whole required board.
func fillSubmissionCatalog(t *testing.T, ctx context.Context, db DBTX, sub *service.ReefCheckSurveySubmission) {
	t.Helper()
	rows, err := db.Query(ctx, `SELECT taxon_group::text,name_en,COALESCE(size_class,'') FROM taxon WHERE is_active AND NOT is_aggregate AND NOT (name_en='Grouper' AND COALESCE(size_class,'')='') AND NOT(taxon_group='rare' AND name_en='Other')`)
	if err != nil {
		t.Fatal(err)
	}
	for rows.Next() {
		var group, name, size string
		if err := rows.Scan(&group, &name, &size); err != nil {
			t.Fatal(err)
		}
		key := "invert"
		if group == "fish" {
			key = "fish"
		}
		for seg := 1; seg <= 4; seg++ {
			zero := 0
			o := service.ReefCheckBeltObservationInput{TransectKey: key, TaxonGroup: group, TaxonNameENLookup: name, TaxonSizeClassLookup: size, Segment: seg, Count: &zero, RecordStatus: "recorded"}
			if name == "Butterflyfish" && seg == 2 {
				o.Count = nil
				o.RecordStatus = "not_recorded"
			}
			sub.BeltObservations = append(sub.BeltObservations, o)
		}
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	rows.Close()
	rows, err = db.Query(ctx, `SELECT impact_group::text,name_en,value_type::text,has_raw_count FROM impact_type WHERE is_active`)
	if err != nil {
		t.Fatal(err)
	}
	for rows.Next() {
		var group, name, kind string
		var raw bool
		if err := rows.Scan(&group, &name, &kind, &raw); err != nil {
			t.Fatal(err)
		}
		if raw {
			kind = "count"
		}
		for seg := 1; seg <= 4; seg++ {
			zero := 0.0
			sub.ImpactObservations = append(sub.ImpactObservations, service.ReefCheckImpactObservationInput{TransectKey: "invert", ImpactGroup: group, ImpactNameENLookup: name, ImpactValueType: kind, Segment: seg, RawValue: &zero, RecordStatus: "recorded"})
		}
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	rows.Close()
}
