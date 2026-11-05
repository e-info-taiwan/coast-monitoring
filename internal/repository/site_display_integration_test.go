package repository

import (
	"coast-monitoring/internal/policy"
	"coast-monitoring/internal/service"
	"context"
	"encoding/json"
	"errors"
	"github.com/google/uuid"
	"os"
	"testing"
)

func TestSiteDisplayActivityReportAndChartTransactions(t *testing.T) {
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
	repo := NewReefDataRepository(tx)
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	var actor string
	must(tx.QueryRow(ctx, `INSERT INTO users(email,name,role) VALUES('display@example.invalid','Test','admin') RETURNING id::text`).Scan(&actor))
	ids := []int{}
	for _, name := range []string{"A", "B", "C"} {
		var id int
		must(tx.QueryRow(ctx, `INSERT INTO site(name_zh,name_en) VALUES($1,$1) RETURNING id`, "Display"+name).Scan(&id))
		ids = append(ids, id)
	}
	display, err := repo.SiteDisplay(ctx, ids[0], false)
	must(err)
	if len(display.EffectiveSettings) != 6 {
		t.Fatal(display)
	}
	profileInput := service.PublicContentInput{Kind: "profile", SiteID: &ids[0], Data: service.ContentData{Title: "Original profile"}}
	profile, err := repo.SaveContent(ctx, 0, profileInput, actor)
	must(err)
	profileInput.Version = profile.UpdatedAt
	profileInput.Status = "published"
	profile, err = repo.SaveContent(ctx, profile.ID, profileInput, actor)
	must(err)
	newProfile, err := repo.SaveContent(ctx, 0, profileInput, actor)
	must(err)
	display, err = repo.SiteDisplay(ctx, ids[0], false)
	must(err)
	if newProfile.Status != "draft" || display.Profile == nil || display.Profile.ID != profile.ID {
		t.Fatal("new draft displaced published profile")
	}
	hidden := false
	draft, err := repo.SaveChartSetting(ctx, service.ChartSetting{SiteID: ids[0], ChartKey: "taxa_density", Visible: &hidden}, actor)
	must(err)
	display, err = repo.SiteDisplay(ctx, ids[0], false)
	must(err)
	if !*display.EffectiveSettings[1].Visible {
		t.Fatal("draft replaced live")
	}
	draft.Status = "published"
	live, err := repo.SaveChartSetting(ctx, draft, actor)
	must(err)
	_, err = repo.SaveChartSetting(ctx, draft, actor)
	if !errors.Is(err, service.ErrConflict) {
		t.Fatalf("stale revision accepted: %v", err)
	}
	display, err = repo.SiteDisplay(ctx, ids[0], false)
	must(err)
	if *display.EffectiveSettings[1].Visible {
		t.Fatal("publication absent")
	}
	live.Restore = true
	live.Confirm = true
	_, err = repo.SaveChartSetting(ctx, live, actor)
	must(err)
	display, err = repo.SiteDisplay(ctx, ids[0], false)
	must(err)
	if display.EffectiveSettings[1].Source != "default" {
		t.Fatal("restore not inherited")
	}
	group, err := repo.SaveSharedGroup(ctx, service.SharedChartGroup{Title: "G", Status: "published", Members: []service.SharedChartMember{{SiteID: ids[0], Depth: 5}, {SiteID: ids[0], Depth: 10}, {SiteID: ids[1], Depth: 10}}}, actor)
	must(err)
	display, err = repo.SiteDisplay(ctx, ids[0], false)
	must(err)
	if len(display.Groups) != 1 || len(display.Groups[0].Members) != 3 {
		t.Fatal("depths collapsed", display.Groups)
	}
	group.Members = []service.SharedChartMember{{SiteID: ids[0], Depth: 5}, {SiteID: ids[2], Depth: 10}}
	group.Confirm = true
	group, err = repo.SaveSharedGroup(ctx, group, actor)
	must(err)
	if len(group.AffectedSiteIDs) != 3 {
		t.Fatal("missing removed site", group)
	}
	var survey, transect int
	must(tx.QueryRow(ctx, `INSERT INTO survey(site_id,start_date,end_date) VALUES($1,'2026-01-01','2026-01-01') RETURNING id`, ids[0]).Scan(&survey))
	_, err = tx.Exec(ctx, `INSERT INTO event(survey_id,event_id,survey_date,event_time,depth_m,publication_status) VALUES($1,'shared-A','2026-01-01','09:00',5,'published')`, survey)
	must(err)
	must(tx.QueryRow(ctx, `INSERT INTO transect(event_id,method) VALUES('shared-A','line') RETURNING id`).Scan(&transect))
	_, err = tx.Exec(ctx, `INSERT INTO substrate_point(transect_id,segment,position_m,substrate_code) VALUES($1,1,0,'HC')`, transect)
	must(err)
	_, err = repo.SaveChartSetting(ctx, service.ChartSetting{SiteID: ids[0], ChartKey: "live_coral_history", Visible: &hidden, Status: "published"}, actor)
	must(err)
	f, _ := service.ParsePublicReefFilter(nil)
	f.Charts = []string{"live_coral"}
	series, err := repo.PublicSeries(ctx, ids[0], f)
	must(err)
	if len(series) != 0 {
		t.Fatal("site hidden chart exposed")
	}
	depth := 5.0
	f.Depth = &depth
	f.GroupID = group.ID
	series, err = repo.PublicSeries(ctx, ids[0], f)
	must(err)
	if len(series) != 1 {
		t.Fatal("local visibility removed shared member", series)
	}
	report, err := repo.SaveContent(ctx, 0, service.PublicContentInput{Kind: "article", Data: service.ContentData{Title: "Shared report", Category: "observation_report", ExternalURL: "https://example.org/report"}}, actor)
	must(err)
	var data service.ContentData
	must(json.Unmarshal(report.Data, &data))
	report, err = repo.SaveContent(ctx, report.ID, service.PublicContentInput{Kind: "article", Status: "published", Version: report.UpdatedAt, Data: data}, actor)
	must(err)
	for _, id := range ids[:2] {
		must(repo.SaveReportLink(ctx, service.ReportLink{SiteID: id, ArticleID: report.ID, Visible: true}, actor))
	}
	display, err = repo.SiteDisplay(ctx, ids[0], true)
	must(err)
	link := display.ReportLinks[0]
	link.Visible = false
	must(repo.SaveReportLink(ctx, link, actor))
	display, err = repo.SiteDisplay(ctx, ids[0], false)
	must(err)
	if len(display.Reports) != 0 || display.LatestReport != nil {
		t.Fatal("hidden report exposed")
	}
	display, err = repo.SiteDisplay(ctx, ids[1], false)
	must(err)
	if len(display.Reports) != 1 || display.LatestReport.ID != report.ID {
		t.Fatal("other site changed")
	}
	_, err = repo.SaveContent(ctx, report.ID, service.PublicContentInput{Kind: "article", Status: "archived", Version: report.UpdatedAt, Data: data}, actor)
	if !errors.Is(err, service.ErrValidation) {
		t.Fatal("shared confirmation absent", err)
	}
	activityData := service.ContentData{Title: "Ordinary activity", StartAt: "2026-01-01T00:00:00+08:00", EndAt: "2026-01-02T23:59:59+08:00", Recruitment: "internal", Status: "scheduled"}
	activity, err := repo.SaveContent(ctx, 0, service.PublicContentInput{Kind: "activity", Data: activityData}, actor)
	must(err)
	if activity.EntryConfigID != nil || activity.EntryCode != "" {
		t.Fatal("ordinary activity created code")
	}
	activityData.EntryEnabled = true
	activityData.SiteIDs = ids[:2]
	activity, err = repo.SaveContent(ctx, activity.ID, service.PublicContentInput{Kind: "activity", Status: "draft", Version: activity.UpdatedAt, Data: activityData}, actor)
	must(err)
	verified, err := repo.VerifyActivity(ctx, activity.EntryCode)
	must(err)
	if verified.ActivityID != activity.ID || len(verified.SiteIDs) != 2 {
		t.Fatal(verified)
	}
	created, err := repo.CreateEvent(ctx, service.ReefDataCreateInput{ActivityID: &activity.ID, SiteID: ids[0], StartDate: "2026-01-01", EndDate: "2026-01-01", SurveyDate: "2026-01-01", EventTime: "10:30", DepthM: 5, Methods: []string{"line"}})
	must(err)
	if created.Event.ActivityID == nil || *created.Event.ActivityID != activity.ID {
		t.Fatal("admin event lost parent activity")
	}
	rotated, err := repo.ChangeActivity(ctx, *activity.EntryConfigID, "rotate", policy.User{ID: uuid.MustParse(actor)})
	must(err)
	if rotated.ActivityID != activity.ID {
		t.Fatal("stable activity changed")
	}
	if _, err = repo.VerifyActivity(ctx, activity.EntryCode); err == nil {
		t.Fatal("old code still valid")
	}
	verified, err = repo.VerifyActivity(ctx, rotated.Code)
	must(err)
	if verified.ActivityID != activity.ID {
		t.Fatal(verified)
	}
	_, err = repo.ChangeActivity(ctx, *activity.EntryConfigID, "disable", policy.User{ID: uuid.MustParse(actor)})
	must(err)
	if _, err = repo.VerifyActivity(ctx, rotated.Code); err == nil {
		t.Fatal("disabled code accepted")
	}
	auditRows, err := repo.PublicAudit(ctx)
	must(err)
	if len(auditRows) == 0 {
		t.Fatal("missing audit timeline")
	}
	var audits int
	must(tx.QueryRow(ctx, `SELECT count(*) FROM reef_check_activity_audit WHERE activity_id=$1 AND after_data IS NOT NULL AND NOT after_data ? 'code_hash'`, *activity.EntryConfigID).Scan(&audits))
	if audits != 3 {
		t.Fatal("activity audit missing", audits)
	}
}
