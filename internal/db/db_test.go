package db

import (
	"context"
	"testing"
)

func TestOpenRejectsInvalidDatabaseURL(t *testing.T) {
	t.Parallel()

	pool, err := Open(context.Background(), "://not-a-valid-database-url")
	if err == nil {
		t.Fatal("Open() error = nil, want parse error")
	}
	if pool != nil {
		t.Fatal("Open() returned a pool for an invalid database URL")
	}
}
