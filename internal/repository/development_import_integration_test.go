package repository

import (
	"coast-monitoring/internal/service"
	"context"
	"encoding/json"
	"os"
	"testing"
)

func TestDevelopmentHandoffDraftImportIsIdempotent(t *testing.T) {
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
	migration, err := os.ReadFile("../../migrations/000019_development_handoff_drafts.sql")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = tx.Exec(ctx, string(migration)); err != nil {
		t.Fatal(err)
	}
	var cases, timeline, media, published int
	err = tx.QueryRow(ctx, `SELECT (SELECT count(*) FROM public_content WHERE kind='development' AND source_snapshot->>'source_format'='development-handoff-2026-11-04'),(SELECT count(*) FROM development_case_timeline),(SELECT count(*) FROM development_case_media),(SELECT count(*) FROM public_content WHERE kind='development' AND publication_status='published')`).Scan(&cases, &timeline, &media, &published)
	if err != nil {
		t.Fatal(err)
	}
	if cases != 25 || timeline != 171 || media != 4 || published != 0 {
		t.Fatalf("unexpected import counts %d %d %d %d", cases, timeline, media, published)
	}
	repo := NewReefDataRepository(tx)
	rows, err := repo.ListContent(ctx, "development", false)
	if err != nil {
		t.Fatal(err)
	}
	for _, row := range rows {
		var data service.ContentData
		if err := json.Unmarshal(row.Data, &data); err != nil {
			t.Fatal(err)
		}
		input := service.PublicContentInput{Kind: "development", Status: "draft", Data: data}
		if err := input.Validate(); err != nil {
			t.Fatalf("invalid imported case %d: %v", row.ID, err)
		}
	}
	_, err = tx.Exec(ctx, `UPDATE development_case SET data=jsonb_set(data,'{title}','"Editor correction"') WHERE id=(SELECT min(id) FROM development_case)`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = tx.Exec(ctx, string(migration)); err != nil {
		t.Fatal(err)
	}
	var title string
	if err = tx.QueryRow(ctx, `SELECT data->>'title' FROM development_case ORDER BY id LIMIT 1`).Scan(&title); err != nil {
		t.Fatal(err)
	}
	if title != "Editor correction" {
		t.Fatal("rerun overwrote manual edits")
	}
}
