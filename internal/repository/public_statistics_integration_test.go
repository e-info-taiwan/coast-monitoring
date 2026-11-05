package repository

import (
	"context"
	"math"
	"net/url"
	"os"
	"strconv"
	"testing"
	"time"

	"coast-monitoring/internal/service"
)

func TestScopedPublicStatisticsContract(t *testing.T) {
	dsn := os.Getenv("REEF_DATA_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("PostgreSQL required")
	}
	ctx := context.Background()
	pool, err := integrationPool(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(ctx)
	exec := func(q string, args ...any) {
		t.Helper()
		if _, err := tx.Exec(ctx, q, args...); err != nil {
			t.Fatal(err)
		}
	}
	scan := func(q string, dst any, args ...any) {
		t.Helper()
		if err := tx.QueryRow(ctx, q, args...).Scan(dst); err != nil {
			t.Fatal(err)
		}
	}
	var site, survey, line, belt, taxon int
	scan(`INSERT INTO site(name_zh,name_en) VALUES('統計測試','StatsFixture') RETURNING id`, &site)
	scan(`INSERT INTO survey(site_id,start_date,end_date) VALUES($1,'2024-04-13','2024-04-13') RETURNING id`, &survey, site)
	exec(`INSERT INTO event(survey_id,event_id,survey_date,event_time,depth_m,publication_status) VALUES($1,'stats-morning','2024-04-13','na',5,'published'),($1,'stats-afternoon','2024-04-13','14-30',5,'published')`, survey)
	scan(`INSERT INTO transect(event_id,method) VALUES('stats-morning','line') RETURNING id`, &line)
	scan(`INSERT INTO transect(event_id,method,fish_size_mode) VALUES('stats-morning','belt_fish','combined') RETURNING id`, &belt)
	scan(`SELECT id FROM taxon WHERE name_en='Butterflyfish' AND NOT is_aggregate LIMIT 1`, &taxon)
	exec(`INSERT INTO substrate_point(transect_id,segment,position_m,substrate_code) VALUES($1,1,0,'HC-a'),($1,2,25,'SC'),($1,3,50,'NA'),($1,4,75,'OT')`, line)
	exec(`INSERT INTO belt_observation(transect_id,taxon_id,segment,count,record_status) VALUES($1,$2,1,0,'recorded'),($1,$2,2,2,'recorded'),($1,$2,3,NULL,'recorded'),($1,$2,4,4,'recorded')`, belt, taxon)
	exec(`INSERT INTO substrate_bleaching(transect_id,segment,hc_bleached_count,sc_bleached_count) VALUES($1,1,0,NULL),($1,2,NULL,NULL)`, line)
	var line2 int
	scan(`INSERT INTO transect(event_id,method) VALUES('stats-afternoon','line') RETURNING id`, &line2)
	exec(`INSERT INTO substrate_point(transect_id,segment,position_m,substrate_code) VALUES($1,1,0,'NA')`, line2)
	repo := NewReefDataRepository(tx)
	query := func(q string) []service.PublicReefSeries {
		t.Helper()
		v, e := url.ParseQuery(q)
		if e != nil {
			t.Fatal(e)
		}
		f, e := service.ParsePublicReefFilter(v)
		if e != nil {
			t.Fatal(e)
		}
		rows, e := repo.PublicSeries(ctx, site, f)
		if e != nil {
			t.Fatal(e)
		}
		return rows
	}
	rows := query("")
	events := map[string]bool{}
	foundTaxa, foundBleaching := false, false
	for _, r := range rows {
		events[r.EventID] = true
		if _, err := time.Parse(time.RFC3339Nano, r.UpdatedAt); err != nil {
			t.Fatalf("non ISO timestamp: %q", r.UpdatedAt)
		}
		if r.EventID == "stats-morning" && r.Time != nil {
			t.Fatalf("unknown time must be null: %+v", r)
		}
		if r.EventID == "stats-afternoon" && (r.Time == nil || *r.Time != "14:30") {
			t.Fatalf("time not normalized: %+v", r)
		}
		if r.Chart != "taxa" && (r.TaxonID != nil || r.Size != nil || r.Mode != nil || r.TaxonGroup != "") {
			t.Fatalf("non-taxa dimensions: %+v", r)
		}
		if r.N == 0 && (r.Value != nil || r.Total != nil || r.Mean != nil || r.SD != nil || r.SE != nil || r.Status != "not_calculable" || r.MissingReason == "") {
			t.Fatalf("invalid all-missing row: %+v", r)
		}
		if r.Chart == "taxa" {
			foundTaxa = true
			if r.N != 3 || r.Mean == nil || *r.Mean != 2 || r.Total == nil || *r.Total != 6 || r.SD == nil || *r.SD != 2 || r.SE == nil || math.Abs(*r.SE-2/math.Sqrt(3)) > 1e-10 {
				t.Fatalf("NULL counted as observation: %+v", r)
			}
			if r.DensityPer100M2 == nil || *r.DensityPer100M2 != 2 || r.Unit != "individuals_per_100m2" || r.Name == "Butterflyfish" || r.Mode == nil || *r.Mode != "combined" {
				t.Fatalf("density metadata: %+v", r)
			}
		}
		if r.Chart == "bleaching" && r.Key == "HC" {
			foundBleaching = true
			if r.N != 1 || r.Value == nil || *r.Value != 0 || r.SD != nil || r.SE != nil || r.Unit != "bleached_points" {
				t.Fatalf("one valid zero: %+v", r)
			}
		}
	}
	if len(events) != 2 || !foundTaxa || !foundBleaching {
		t.Fatalf("missing series: %v taxa=%v bleaching=%v", events, foundTaxa, foundBleaching)
	}
	// All filters intersect, including taxon filters on a mixed-chart request.
	filtered := query("taxon_ids=" + strconv.Itoa(taxon) + "&charts=taxa,substrate&survey_date=2024-04-13&event_ids=stats-morning,stats-afternoon&depth_m=5&unit=individuals_per_100m2&fish_size_mode=combined")
	if len(filtered) != 1 || filtered[0].Chart != "taxa" {
		t.Fatalf("filter intersection: %+v", filtered)
	}
	for _, q := range []string{"event_id=stats-morning&event_ids=stats-afternoon", "from_year=2025", "depth_m=10", "taxon_group=invert", "charts=taxa&unit=percent"} {
		if got := query(q); len(got) != 0 {
			t.Fatalf("non-intersection %s: %+v", q, got)
		}
	}
	stamp := filtered[0].UpdatedAt
	if query("charts=taxa")[0].UpdatedAt != stamp {
		t.Fatal("request time used as update time")
	}
	exec(`UPDATE belt_observation SET count=8 WHERE transect_id=$1 AND segment=4`, belt)
	changed := query("charts=taxa")[0]
	if changed.UpdatedAt == stamp || *changed.Total != 10 {
		t.Fatalf("raw edit not reflected: %+v", changed)
	}
	exec(`UPDATE event SET publication_status='draft' WHERE event_id='stats-morning'`)
	if len(query("charts=taxa")) != 0 {
		t.Fatal("withdrawn event still public")
	}
	exec(`UPDATE event SET publication_status='archived' WHERE event_id='stats-afternoon'`)
	if len(query("")) != 0 {
		t.Fatal("archived event still public")
	}
}
