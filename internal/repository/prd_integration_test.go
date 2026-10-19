package repository

import (
	"coast-monitoring/internal/service"
	"context"
	"encoding/json"
	"github.com/jackc/pgx/v5/pgxpool"
	"math"
	"os"
	"testing"
)

func TestPRDSummariesAndPublication(t *testing.T) {
	dsn := os.Getenv("REEF_DATA_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("local PostgreSQL required")
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
	scan := func(q string, dst any, args ...any) {
		t.Helper()
		if err := tx.QueryRow(ctx, q, args...).Scan(dst); err != nil {
			t.Fatal(err)
		}
	}
	exec := func(q string, args ...any) {
		t.Helper()
		if _, err := tx.Exec(ctx, q, args...); err != nil {
			t.Fatal(err)
		}
	}
	var site, survey, line, belt, invert, taxon, impact int
	scan(`INSERT INTO site(name_zh,name_en,latitude,longitude) VALUES('PRD Fixture','PRDFixture',25,121) RETURNING id`, &site)
	scan(`INSERT INTO survey(site_id,start_date,end_date) VALUES($1,'2026-10-02','2026-10-02') RETURNING id`, &survey, site)
	exec(`INSERT INTO event(survey_id,event_id,survey_date,event_time,depth_m) VALUES($1,'prd-fixture-1','2026-10-02','09:30',5),($1,'prd-fixture-2','2026-10-02','10:30',5)`, survey)
	scan(`INSERT INTO transect(event_id,method) VALUES('prd-fixture-1','line') RETURNING id`, &line)
	scan(`INSERT INTO transect(event_id,method,fish_size_mode) VALUES('prd-fixture-1','belt_fish','split') RETURNING id`, &belt)
	scan(`INSERT INTO transect(event_id,method) VALUES('prd-fixture-1','belt_invert') RETURNING id`, &invert)
	scan(`SELECT id FROM taxon WHERE name_en='Butterflyfish' AND NOT is_aggregate LIMIT 1`, &taxon)
	scan(`SELECT id FROM impact_type WHERE impact_group='trash' ORDER BY id LIMIT 1`, &impact)
	for i, v := range []int{0, 1, 4, 5} {
		exec(`INSERT INTO impact_observation(transect_id,impact_type_id,segment,raw_value) VALUES($1,$2,$3,$4)`, invert, impact, i+1, v)
	}
	for i, v := range []int{0, 2, 0, 4} {
		status := "recorded"
		if i == 2 {
			status = "not_recorded"
		}
		exec(`INSERT INTO belt_observation(transect_id,taxon_id,segment,count,record_status) VALUES($1,$2,$3,$4,$5)`, belt, taxon, i+1, v, status)
	}
	for i, code := range []string{"HC-a", "SC", "NA", "OT"} {
		exec(`INSERT INTO substrate_point(transect_id,segment,position_m,substrate_code) VALUES($1,$2,$3,$4)`, line, i+1, i*25, code)
	}
	var mean, se, level float64
	var n int
	if err = tx.QueryRow(ctx, `SELECT mean,se,n FROM belt_summary WHERE transect_id=$1 AND taxon_id=$2`, belt, taxon).Scan(&mean, &se, &n); err != nil {
		t.Fatal(err)
	}
	if mean != 2 || n != 3 || math.Abs(se-2/math.Sqrt(3)) > 1e-10 {
		t.Fatalf("belt mean=%g se=%g n=%d", mean, se, n)
	}
	if err = tx.QueryRow(ctx, `SELECT mean,level_mean,n FROM impact_summary WHERE transect_id=$1`, invert).Scan(&mean, &level, &n); err != nil {
		t.Fatal(err)
	}
	if mean != 2.5 || level != 1.5 || n != 4 {
		t.Fatalf("impact mean=%g level=%g n=%d", mean, level, n)
	}
	scan(`SELECT derived_level::float8 FROM impact_observation_with_level WHERE transect_id=$1 AND segment=3`, &level, invert)
	if level != 2 {
		t.Fatal("4 pieces must produce grade 2")
	}
	var cover float64
	scan(`SELECT coverage_percent::float8 FROM substrate_summary WHERE transect_id=$1 AND code='live_coral'`, &cover, line)
	if math.Abs(cover-200.0/3) > 1e-10 {
		t.Fatal(cover)
	}
	repo := NewReefDataRepository(tx)
	f, _ := service.ParsePublicReefFilter(nil)
	rows, err := repo.PublicSeries(ctx, site, f)
	if err != nil || len(rows) != 0 {
		t.Fatalf("draft exposed: %v %v", rows, err)
	}
	exec(`UPDATE event SET publication_status='published' WHERE survey_id=$1`, survey)
	var line2 int
	scan(`INSERT INTO transect(event_id,method) VALUES('prd-fixture-2','line') RETURNING id`, &line2)
	exec(`INSERT INTO substrate_point(transect_id,segment,position_m,substrate_code) VALUES($1,1,0,'NA')`, line2)
	rows, err = repo.PublicSeries(ctx, site, f)
	if err != nil {
		t.Fatal(err)
	}
	found := map[string]bool{}
	for _, r := range rows {
		found[r.EventID] = true
		if r.EventID == "prd-fixture-2" && (r.Value != nil || r.N != 0 || r.SE != nil) {
			t.Fatalf("all NA became zero: %+v", r)
		}
	}
	if len(found) != 2 {
		t.Fatalf("distinct events merged: %v", found)
	}
	detail, err := repo.Transect(ctx, line, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(detail.Summaries) == 0 || !json.Valid(detail.Summaries) {
		t.Fatal("admin summaries missing")
	}
	exec(`UPDATE impact_observation SET raw_value=1 WHERE transect_id=$1 AND segment=3`, invert)
	scan(`SELECT derived_level::float8 FROM impact_observation_with_level WHERE transect_id=$1 AND segment=3`, &level, invert)
	if level != 1 {
		t.Fatal("summary stale after raw edit")
	}
}

func TestPRDContentWorkflow(t *testing.T) {
	dsn := os.Getenv("REEF_DATA_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("local PostgreSQL required")
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
	var actor string
	if err = tx.QueryRow(ctx, `INSERT INTO users(email,name,role) VALUES('prd-fixture@example.invalid','Fixture','admin') RETURNING id::text`).Scan(&actor); err != nil {
		t.Fatal(err)
	}

	// Settings is supported by migration 000011, not by editing applied 000009.
	if _, err = tx.Exec(ctx, `DELETE FROM public_content_audit WHERE content_id IN(SELECT id FROM public_content WHERE kind='settings'); DELETE FROM public_content WHERE kind='settings'`); err != nil {
		t.Fatal(err)
	}
	limit := 7
	settings, err := repo.SaveContent(ctx, 0, service.PublicContentInput{Kind: "settings", Data: service.ContentData{Title: "首頁設定", ArticleLimit: &limit}}, actor)
	if err != nil {
		t.Fatalf("settings create: %v", err)
	}
	limit = 9
	settings, err = repo.SaveContent(ctx, settings.ID, service.PublicContentInput{Kind: "settings", Version: settings.UpdatedAt, Status: "published", Data: service.ContentData{Title: "首頁設定", ArticleLimit: &limit}}, actor)
	if err != nil || settings.Status != "published" {
		t.Fatalf("settings save/publication: %v", err)
	}
	publicSettings, err := repo.ListContent(ctx, "settings", true)
	if err != nil || len(publicSettings) != 1 {
		t.Fatalf("settings read: %v %v", publicSettings, err)
	}
	c := service.PublicContentInput{Kind: "development", Status: "published", Data: service.ContentData{Title: "Draft fixture", Geometry: json.RawMessage(`{"type":"Point","coordinates":[121,25]}`), Timeline: []service.ContentTimeline{{Title: "Draft timeline", Precision: "unknown", SortOrder: 1}, {Title: "Published timeline", Date: "2026-10", Precision: "month", Published: true, SourceURL: "https://example.com/source"}}, Media: []service.ContentMedia{{URL: "https://example.com/hidden.png", Alt: "hidden"}, {URL: "https://example.com/public.png", Alt: "public", Published: true, Valid: true, Licensed: true}}}}
	saved, err := repo.SaveContent(ctx, 0, c, actor)
	if err != nil {
		t.Fatal(err)
	}
	if saved.Status != "draft" {
		t.Fatal("new content not draft")
	}
	rows, err := repo.ListContent(ctx, "development", true)
	if err != nil {
		t.Fatal(err)
	}
	for _, r := range rows {
		if r.ID == saved.ID {
			t.Fatal("draft exposed")
		}
	}
	if _, err = tx.Exec(ctx, `UPDATE public_content SET source_snapshot='{"original":"protected"}' WHERE id=$1`, saved.ID); err != nil {
		t.Fatal(err)
	}
	c.Version = saved.UpdatedAt
	c.Status = "published"
	updated, err := repo.SaveContent(ctx, saved.ID, c, actor)
	if err != nil {
		t.Fatal(err)
	}
	rows, err = repo.ListContent(ctx, "development", true)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, r := range rows {
		if r.ID == saved.ID {
			found = true
			var d service.ContentData
			if err = json.Unmarshal(r.Data, &d); err != nil {
				t.Fatal(err)
			}
			if len(d.Timeline) != 1 || len(d.Media) != 1 {
				t.Fatal("unpublished children exposed")
			}
		}
	}
	if !found {
		t.Fatal("published content missing")
	}
	if _, err = repo.SaveContent(ctx, saved.ID, c, actor); err != service.ErrConflict {
		t.Fatalf("stale content overwritten: %v", err)
	}
	var snapshot string
	if err = tx.QueryRow(ctx, `SELECT source_snapshot->>'original' FROM public_content WHERE id=$1`, saved.ID).Scan(&snapshot); err != nil || snapshot != "protected" {
		t.Fatal("source overwritten")
	}
	var auditCount int
	if err = tx.QueryRow(ctx, `SELECT count(*) FROM public_content_audit WHERE content_id=$1 AND actor_id=$2::uuid`, updated.ID, actor).Scan(&auditCount); err != nil || auditCount != 2 {
		t.Fatalf("audit missing: %d %v", auditCount, err)
	}
}
