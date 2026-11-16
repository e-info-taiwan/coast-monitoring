package repository

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"strings"
	"testing"

	"coast-monitoring/internal/service"
)

// Exercise app.js, not a fixture that fills every active catalog row. Optional
// source drafts are read locally only; the outer transaction always rolls back.
func TestWebBoardPayloadCanSubmitAndPublish(t *testing.T) {
	dsn := os.Getenv("REEF_DATA_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("set isolated test DB")
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
	var site, activity int
	if err = tx.QueryRow(ctx, `INSERT INTO site(name_zh,name_en) VALUES('Web board fixture','WebBoardFixture') RETURNING id`).Scan(&site); err != nil {
		t.Fatal(err)
	}
	code := "RC-WEB-BOARD-FIXTURE"
	if err = tx.QueryRow(ctx, `WITH parent AS (INSERT INTO public_content(kind,data) VALUES('activity','{"title":"Fixture"}') RETURNING id) INSERT INTO reef_check_activity(activity_content_id,name,code_hash,start_date,end_date) SELECT id,'Fixture',$1,'2026-11-01','2026-11-30' FROM parent RETURNING id`, activityHash(code)).Scan(&activity); err != nil {
		t.Fatal(err)
	}
	if _, err = tx.Exec(ctx, `INSERT INTO reef_check_activity_site(activity_id,site_id) VALUES($1,$2)`, activity, site); err != nil {
		t.Fatal(err)
	}
	// More historical/custom masters must not become hidden mandatory inputs.
	if _, err = tx.Exec(ctx, `INSERT INTO taxon(taxon_group,name_zh,name_en) VALUES('fish','額外歷史類群','Historical fixture'); INSERT INTO impact_type(impact_group,name_zh,name_en,value_type) VALUES('disease','額外歷史疾病','Historical fixture','percent')`); err != nil {
		t.Fatal(err)
	}
	repo := NewReefDataRepository(tx)
	cfg, err := repo.Config(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.EntryBoard == nil || len(cfg.EntryBoard.Taxa) != 31 || len(cfg.EntryBoard.Impacts) != 9 {
		t.Fatalf("wrong entry template: %+v", cfg.EntryBoard)
	}
	path := os.Getenv("REEF_ENTRY_DRAFT_FIXTURE")
	if path == "" {
		path = "testdata/volunteer_board_draft.json"
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var draft map[string]json.RawMessage
	if err = json.Unmarshal(raw, &draft); err != nil {
		t.Fatal(err)
	}
	if original, ok := draft["draft_state"]; ok {
		if err = json.Unmarshal(original, &draft); err != nil {
			t.Fatal(err)
		}
	}
	export := func(clock string) service.ReefCheckSurveySubmission {
		t.Helper()
		in, err := json.Marshal(map[string]any{"draft_state": draft, "config": cfg, "site": map[string]any{"id": site, "name_zh": "Web board fixture", "name_en": "WebBoardFixture"}, "activity": map[string]any{"code": code}, "event_time": clock})
		if err != nil {
			t.Fatal(err)
		}
		cmd := exec.Command("node", "../../scripts/prd-db/export-web-entry.mjs")
		cmd.Stdin = bytes.NewReader(in)
		var stderr bytes.Buffer
		cmd.Stderr = &stderr
		out, err := cmd.Output()
		if err != nil {
			t.Fatalf("browser export failed: %v %s", err, stderr.String())
		}
		var result struct {
			Issues  []string                          `json:"issues"`
			Payload service.ReefCheckSurveySubmission `json:"payload"`
		}
		if err = json.Unmarshal(out, &result); err != nil {
			t.Fatal(err)
		}
		if len(result.Issues) > 0 {
			t.Fatalf("browser validation: %v", result.Issues)
		}
		return result.Payload
	}
	sub := export("09:30")
	if len(sub.SubstratePoints) != 160 || len(sub.BeltObservations) != 120 || len(sub.ImpactObservations) != 36 {
		t.Fatalf("actual board counts: %d/%d/%d", len(sub.SubstratePoints), len(sub.BeltObservations), len(sub.ImpactObservations))
	}
	res, err := repo.SubmitSurvey(ctx, sub, nil)
	if err != nil {
		t.Fatalf("complete browser board rejected: %v", err)
	}
	var belts, impacts, extra int
	if err = tx.QueryRow(ctx, `SELECT (SELECT count(*) FROM belt_observation b JOIN transect t ON t.id=b.transect_id WHERE t.event_id=$1),(SELECT count(*) FROM impact_observation o JOIN transect t ON t.id=o.transect_id WHERE t.event_id=$1),(SELECT count(*) FROM belt_observation b JOIN transect t ON t.id=b.transect_id WHERE t.event_id=$1 AND NOT EXISTS(SELECT 1 FROM reef_entry_required_taxon r WHERE r.taxon_id=b.taxon_id))`, res.EventID).Scan(&belts, &impacts, &extra); err != nil {
		t.Fatal(err)
	}
	if belts != 120 || impacts != 36 || extra != 0 {
		t.Fatalf("unexpected synthesized rows: %d/%d/%d", belts, impacts, extra)
	}
	for _, p := range sub.SubstratePoints {
		var code string
		var health *string
		if err = tx.QueryRow(ctx, `SELECT p.substrate_code,p.health_status FROM substrate_point p JOIN transect t ON t.id=p.transect_id WHERE t.event_id=$1 AND p.segment=$2 AND p.position_m=$3 AND p.substrate_layer='surface'`, res.EventID, p.Segment, p.PositionM).Scan(&code, &health); err != nil {
			t.Fatal(err)
		}
		if code != p.SubstrateCode || (health == nil) != (p.HealthStatus == nil) || health != nil && *health != *p.HealthStatus {
			t.Fatalf("substrate/health changed at %g", p.PositionM)
		}
	}
	for _, b := range sub.BeltObservations {
		var value *int
		if err = tx.QueryRow(ctx, `SELECT o.count FROM belt_observation o JOIN transect t ON t.id=o.transect_id JOIN taxon x ON x.id=o.taxon_id WHERE t.event_id=$1 AND x.taxon_group::text=$2 AND x.name_en=$3 AND COALESCE(x.size_class,'')=$4 AND o.segment=$5`, res.EventID, b.TaxonGroup, b.TaxonNameENLookup, b.TaxonSizeClassLookup, b.Segment).Scan(&value); err != nil {
			t.Fatal(err)
		}
		if (value == nil) != (b.Count == nil) || value != nil && *value != *b.Count {
			t.Fatalf("Belt value changed: %s/%d", b.TaxonNameENLookup, b.Segment)
		}
	}
	for _, i := range sub.ImpactObservations {
		var value *float64
		if err = tx.QueryRow(ctx, `SELECT o.raw_value::float8 FROM impact_observation o JOIN transect t ON t.id=o.transect_id JOIN impact_type x ON x.id=o.impact_type_id WHERE t.event_id=$1 AND x.impact_group::text=$2 AND x.name_en=$3 AND o.segment=$4`, res.EventID, i.ImpactGroup, i.ImpactNameENLookup, i.Segment).Scan(&value); err != nil {
			t.Fatal(err)
		}
		if (value == nil) != (i.RawValue == nil) || value != nil && *value != *i.RawValue {
			t.Fatalf("impact value changed: %s/%d", i.ImpactNameENLookup, i.Segment)
		}
	}
	var actor string
	if err = tx.QueryRow(ctx, `INSERT INTO users(email,name,role) VALUES('web-board@example.invalid','Fixture','admin') RETURNING id::text`).Scan(&actor); err != nil {
		t.Fatal(err)
	}
	if err = repo.SetPublication(ctx, res.EventDBID, "published", actor); err != nil {
		t.Fatalf("complete web board cannot publish: %v", err)
	}
	// Removing real required rows still fails with an actionable name and no save.
	bad := export("10:30")
	badBelts := bad.BeltObservations
	bad.BeltObservations = nil
	for _, b := range badBelts {
		if b.TaxonNameENLookup != "Butterflyfish" {
			bad.BeltObservations = append(bad.BeltObservations, b)
		}
	}
	if _, err = repo.SubmitSurvey(ctx, bad, nil); !errors.Is(err, service.ErrValidation) || !strings.Contains(err.Error(), "蝴蝶魚") {
		t.Fatalf("missing required row accepted or unlocatable: %v", err)
	}
	var count int
	if err = tx.QueryRow(ctx, `SELECT count(*) FROM event e JOIN survey s ON s.id=e.survey_id WHERE s.site_id=$1`, site).Scan(&count); err != nil || count != 1 {
		t.Fatalf("failed entry was not rolled back: %d %v", count, err)
	}
	bad = export("11:30")
	badImpacts := bad.ImpactObservations
	bad.ImpactObservations = nil
	for _, i := range badImpacts {
		if i.ImpactNameENLookup != "Black Band (% colonies)" {
			bad.ImpactObservations = append(bad.ImpactObservations, i)
		}
	}
	if _, err = repo.SubmitSurvey(ctx, bad, nil); !errors.Is(err, service.ErrValidation) || !strings.Contains(err.Error(), "黑帶病") {
		t.Fatalf("missing impact row accepted: %v", err)
	}
	draft["fishLengthMode"] = json.RawMessage(`"combined"`)
	var fish map[string]any
	if err = json.Unmarshal(draft["fish"], &fish); err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"grouper-combined:1", "grouper-combined:2", "grouper-combined:3", "grouper-combined:4"} {
		fish[key] = "0"
	}
	draft["fish"], _ = json.Marshal(fish)
	combined := export("12:30")
	if len(combined.BeltObservations) != 104 {
		t.Fatalf("combined board has %d observations", len(combined.BeltObservations))
	}
	if _, err = repo.SubmitSurvey(ctx, combined, nil); err != nil {
		t.Fatalf("combined web board rejected: %v", err)
	}
	// Explicit unknowns remain NULL/NA, never a synthesized zero.
	fish["0:2"] = "NA"
	draft["fish"], _ = json.Marshal(fish)
	draft["missingReason"] = json.RawMessage(`"Fixture: not recorded"`)
	unknown := export("13:30")
	unknownRes, err := repo.SubmitSurvey(ctx, unknown, nil)
	if err != nil {
		t.Fatalf("explicit NA rejected: %v", err)
	}
	var value *int
	var status string
	if err = tx.QueryRow(ctx, `SELECT b.count,b.record_status FROM belt_observation b JOIN transect t ON t.id=b.transect_id JOIN taxon x ON x.id=b.taxon_id WHERE t.event_id=$1 AND x.name_en='Butterflyfish' AND COALESCE(x.size_class,'')='' AND b.segment=2`, unknownRes.EventID).Scan(&value, &status); err != nil || value != nil || status != "not_recorded" {
		t.Fatalf("NA was converted: %v/%s/%v", value, status, err)
	}
}
