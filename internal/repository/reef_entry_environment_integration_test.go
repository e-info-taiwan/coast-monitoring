package repository

import (
	"coast-monitoring/internal/policy"
	"coast-monitoring/internal/service"
	"context"
	"errors"
	"github.com/google/uuid"
	"os"
	"testing"
)

func TestEntryEnvironmentUsesScopedSameDayStationReference(t *testing.T) {
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
	_, err = tx.Exec(ctx, `INSERT INTO cwa_marine_station(id,name_zh,is_active) VALUES('ENTRY-REF','測試測站',true)`)
	must(err)
	_, err = tx.Exec(ctx, `INSERT INTO cwa_sea_temperature(station_id,observed_at,observed_date,sea_temp_c) VALUES('ENTRY-REF','2026-11-06T09:00:00+08:00','2026-11-06',24.5),('ENTRY-REF','2026-11-06T15:30:00+08:00','2026-11-06',27),('ENTRY-REF','2026-11-05T15:30:00+08:00','2026-11-05',26)`)
	must(err)
	var site int
	must(tx.QueryRow(ctx, `INSERT INTO site(name_zh,name_en,latitude,longitude,cwa_station_id) VALUES('美人洞測試','EnvironmentFixture',22.35,120.38,'ENTRY-REF') RETURNING id`).Scan(&site))
	var actor uuid.UUID
	must(tx.QueryRow(ctx, `INSERT INTO users(email,name,role) VALUES('environment@example.invalid','Fixture','admin') RETURNING id`).Scan(&actor))
	repo := NewReefDataRepository(tx)
	activity, err := repo.CreateActivity(ctx, service.ReefCheckActivityInput{Name: "環境測試", StartDate: "2026-11-06", EndDate: "2026-11-07", SiteIDs: []int{site}}, policy.User{ID: actor, Role: policy.RoleAdmin})
	must(err)
	in := service.ReefEntryEnvironmentInput{ActivityCode: activity.Code, SiteID: site, SurveyDate: "2026-11-06", EventTime: "15:40"}
	out, err := repo.EntryEnvironment(ctx, in)
	must(err)
	if out.SeaTempC == nil || *out.SeaTempC != 27 || out.ObservedAt == nil || out.StationName == nil || *out.StationName != "測試測站" {
		t.Fatalf("reference lost: %+v", out)
	}
	expected, err := resolveEventTemperature(ctx, tx, out.StationID, in.SurveyDate, in.EventTime, nil, "automatic")
	must(err)
	if expected.Value == nil || *expected.Value != *out.SeaTempC || !expected.ObservedAt.Equal(*out.ObservedAt) {
		t.Fatal("preview diverged from submission")
	}
	sites, err := repo.ListSites(ctx)
	must(err)
	found := false
	for _, s := range sites {
		if s.ID == site {
			found = true
			if s.Latitude == nil || *s.Latitude != 22.35 || s.Longitude == nil || *s.Longitude != 120.38 {
				t.Fatal("coordinates absent from entry sites")
			}
		}
	}
	if !found {
		t.Fatal("site absent")
	}
	in.SurveyDate = "2026-11-07"
	out, err = repo.EntryEnvironment(ctx, in)
	must(err)
	if out.SeaTempC != nil {
		t.Fatal("wrong day's temperature used")
	}
	in.SurveyDate = "2026-11-08"
	if _, err = repo.EntryEnvironment(ctx, in); !errors.Is(err, service.ErrValidation) {
		t.Fatal("out of activity date accepted")
	}
	in.SurveyDate = "2026-11-06"
	in.SiteID = site + 10000
	if _, err = repo.EntryEnvironment(ctx, in); !errors.Is(err, service.ErrValidation) {
		t.Fatal("out of activity site accepted")
	}
	in.SiteID = site
	_, err = repo.ChangeActivity(ctx, activity.ID, "disable", policy.User{ID: actor, Role: policy.RoleAdmin})
	must(err)
	if _, err = repo.EntryEnvironment(ctx, in); !errors.Is(err, service.ErrValidation) {
		t.Fatal("disabled code accepted")
	}
}
