package repository

import (
	"context"
	"errors"
	"os"
	"testing"

	"coast-monitoring/internal/service"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Uses a migrated disposable/local DB. The outer transaction rolls back all data.
func TestPointEntryPreservesPublicationAndHistoricalTotals(t *testing.T) {
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
	var site, activity int
	if err = tx.QueryRow(ctx, `INSERT INTO site(name_zh,name_en) VALUES('Point entry fixture','PointEntryFixture') RETURNING id`).Scan(&site); err != nil {
		t.Fatal(err)
	}
	code := "RC-POINT-ENTRY-FIXTURE"
	if err = tx.QueryRow(ctx, `INSERT INTO reef_check_activity(name,code_hash,start_date,end_date) VALUES('Fixture',$1,'2026-10-01','2026-10-31') RETURNING id`, activityHash(code)).Scan(&activity); err != nil {
		t.Fatal(err)
	}
	if _, err = tx.Exec(ctx, `INSERT INTO reef_check_activity_site(activity_id,site_id) VALUES($1,$2)`, activity, site); err != nil {
		t.Fatal(err)
	}
	sub := service.ReefCheckSurveySubmission{Format: "reefcheck-web-v4", ActivityCode: code, BleachingMode: "point_marks", Event: service.ReefCheckEventInput{SiteID: site, SurveyDate: "2026-10-19", EventTime: "09:30", DepthM: 5}, Transects: []service.ReefCheckTransectInput{{TransectKey: "line", Method: "line", Recorders: []string{"Fixture recorder"}, CoralMorphology: "general"}}, EntryMetadata: service.ReefCheckEntryMetadata{Weather: "晴"}}
	for seg := 1; seg <= 4; seg++ {
		for i := 0; i < 40; i++ {
			marked := seg == 1 && i == 0
			sub.SubstratePoints = append(sub.SubstratePoints, service.ReefCheckSubstratePointInput{TransectKey: "line", Segment: seg, PositionM: float64((seg-1)*25) + float64(i)/2, SubstrateLayer: "surface", SubstrateCode: "HC", IsBleached: &marked})
		}
	}
	repo := NewReefDataRepository(tx)
	res, err := repo.SubmitSurvey(ctx, sub, nil)
	if err != nil {
		t.Fatal(err)
	}
	if res.PublicationStatus != "draft" {
		t.Fatalf("unexpected publication: %+v", res)
	}
	var rawRows int
	if err = tx.QueryRow(ctx, `SELECT count(*) FROM substrate_bleaching b JOIN transect t ON t.id=b.transect_id WHERE t.event_id=$1`, res.EventID).Scan(&rawRows); err != nil || rawRows != 0 {
		t.Fatalf("manual totals were synthesized: %d %v", rawRows, err)
	}
	detail, err := repo.Event(ctx, res.EventDBID)
	if err != nil {
		t.Fatal(err)
	}
	if detail.Event.EntryMetadata == nil || detail.Event.EntryMetadata.Weather != "晴" || detail.Event.PublicationStatus != "draft" {
		t.Fatalf("metadata/publication lost: %+v", detail.Event)
	}
	var total int
	if err = tx.QueryRow(ctx, `SELECT total FROM substrate_bleaching_summary WHERE transect_id=$1 AND code='HC'`, detail.Transects[0].ID).Scan(&total); err != nil || total != 1 {
		t.Fatalf("point summary=%d %v", total, err)
	}
	var actor string
	if err = tx.QueryRow(ctx, `INSERT INTO users(email,name,role) VALUES('point-entry-fixture@example.invalid','Fixture','admin') RETURNING id::text`).Scan(&actor); err != nil {
		t.Fatal(err)
	}
	if err = repo.SetPublication(ctx, res.EventDBID, "published", actor); err != nil {
		t.Fatalf("complete point entry could not publish: %v", err)
	}
	if _, err = tx.Exec(ctx, `UPDATE substrate_point SET is_bleached=NULL WHERE transect_id=$1 AND position_m=0`, detail.Transects[0].ID); err != nil {
		t.Fatal(err)
	}
	if err = repo.SetPublication(ctx, res.EventDBID, "published", actor); !errors.Is(err, service.ErrValidation) {
		t.Fatalf("unreviewed coral point accepted: %v", err)
	}
	// Historical/manual totals and point summaries remain independent, even when
	// both are present: reading/recomputing the new view must not rewrite raw data.
	if _, err = tx.Exec(ctx, `UPDATE substrate_point SET is_bleached=true WHERE transect_id=$1 AND position_m=0`, detail.Transects[0].ID); err != nil {
		t.Fatal(err)
	}
	if _, err = tx.Exec(ctx, `INSERT INTO substrate_bleaching(transect_id,segment,hc_bleached_count) VALUES($1,1,5)`, detail.Transects[0].ID); err != nil {
		t.Fatal(err)
	}
	var raw, derived int
	if err = tx.QueryRow(ctx, `SELECT b.hc_bleached_count,p.derived_hc_bleached_point_count FROM substrate_bleaching b JOIN substrate_bleaching_point_summary p USING(transect_id,segment) WHERE b.transect_id=$1 AND b.segment=1`, detail.Transects[0].ID).Scan(&raw, &derived); err != nil || raw != 5 || derived != 1 {
		t.Fatalf("raw/derived changed: %d/%d %v", raw, derived, err)
	}
}
