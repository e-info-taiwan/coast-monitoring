package db

import (
	"os"
	"path/filepath"
	"testing"
)

func TestMigrationFilesReturnsSortedSQLFiles(t *testing.T) {
	dir := t.TempDir()
	files := map[string]string{
		"000002_second.sql": "select 2;",
		"README.md":         "ignore",
		"000001_first.sql":  "select 1;",
	}
	for name, content := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	got, err := MigrationFiles(dir)
	if err != nil {
		t.Fatalf("MigrationFiles error = %v", err)
	}

	want := []string{
		filepath.Join(dir, "000001_first.sql"),
		filepath.Join(dir, "000002_second.sql"),
	}
	if len(got) != len(want) {
		t.Fatalf("MigrationFiles returned %d files, want %d: %v", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("MigrationFiles[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}
