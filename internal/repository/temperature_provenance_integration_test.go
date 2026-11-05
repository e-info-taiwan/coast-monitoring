package repository

import (
	"coast-monitoring/internal/service"
	"context"
	"os"
	"testing"
)

func TestTemperatureProvenanceAndManualStationPreservation(t *testing.T) {
	dsn := os.Getenv("REEF_DATA_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("isolated PostgreSQL required")
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
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	_, err = tx.Exec(ctx, `INSERT INTO cwa_marine_station(id,name_zh,latitude,longitude,is_active) VALUES('T-A','Temperature',25,121,true),('T-B','No temperature',25.001,121,true)`)
	must(err)
	_, err = tx.Exec(ctx, `INSERT INTO cwa_sea_temperature(station_id,observed_at,observed_date,sea_temp_c) VALUES('T-A','2026-01-01T09:00:00+08:00','2026-01-01',24.5),('T-A','2026-01-02T09:00:00+08:00','2026-01-02',25.5)`)
	must(err)
	repo := NewReefDataRepository(tx)
	station := "T-A"
	name := "Temperature fixture"
	site, err := repo.CreateSite(ctx, service.ReefDataSiteInput{NameZH: name, NameEN: "TempFixture", CWAStationID: &station})
	must(err)
	if site.CWAStationSource != "manual" || site.CWAStationUpdatedAt == nil {
		t.Fatal(site)
	}
	site, err = repo.UpdateSite(ctx, site.ID, service.ReefDataSiteInput{NameZH: "Rename"})
	must(err)
	if site.CWAStationID == nil || *site.CWAStationID != station || site.CWAStationSource != "manual" {
		t.Fatal("unrelated update replaced station", site)
	}
	auto, err := resolveEventTemperature(ctx, tx, &station, "2026-01-01", "09:30", nil, "")
	must(err)
	if auto.Value == nil || *auto.Value != 24.5 || auto.Source == nil || *auto.Source != "automatic" || auto.ObservedAt == nil || auto.FetchedAt == nil {
		t.Fatal(auto)
	}
	missing, err := resolveEventTemperature(ctx, tx, &station, "2026-01-03", "09:30", nil, "")
	must(err)
	if missing.Value != nil {
		t.Fatal("used other date")
	}
	value := 27.0
	manual, err := resolveEventTemperature(ctx, tx, &station, "2026-01-01", "09:30", &value, "manual")
	must(err)
	if manual.ObservedAt != nil || *manual.Value != 27 || *manual.Source != "manual" {
		t.Fatal(manual)
	}
	var survey, event int
	must(tx.QueryRow(ctx, `INSERT INTO survey(site_id,start_date,end_date) VALUES($1,'2026-01-01','2026-01-01') RETURNING id`, site.ID).Scan(&survey))
	must(tx.QueryRow(ctx, `INSERT INTO event(survey_id,event_id,survey_date,event_time,depth_m) VALUES($1,'temp-fixture','2026-01-01','09:30',5) RETURNING id`, survey).Scan(&event))
	must(saveEventTemperature(ctx, tx, event, manual))
	must(saveEventTemperature(ctx, tx, event, auto))
	var stored float64
	must(tx.QueryRow(ctx, `SELECT sea_temp_c::float8 FROM event WHERE id=$1`, event).Scan(&stored))
	if stored != 27 {
		t.Fatal("automatic overwrote manual")
	}
}
