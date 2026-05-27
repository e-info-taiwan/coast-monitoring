package db

import (
	"context"
	"os"
	"strings"
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

func TestInitialMigrationContainsReviewedSchemaDecisions(t *testing.T) {
	t.Parallel()

	migration, err := os.ReadFile("../../migrations/000001_init.sql")
	if err != nil {
		t.Fatal(err)
	}
	sql := string(migration)

	required := []string{
		"CREATE EXTENSION IF NOT EXISTS citext;",
		"email citext NOT NULL UNIQUE",
		"CREATE INDEX observations_location_id_idx ON observations(location_id);",
		"CREATE INDEX observations_species_id_idx ON observations(species_id);",
		"CREATE INDEX oauth_states_expires_at_idx ON oauth_states(expires_at);",
		"CREATE INDEX login_attempts_ip_time_idx ON login_attempts(ip, attempted_at DESC);",
	}
	for _, want := range required {
		if !strings.Contains(sql, want) {
			t.Fatalf("migration missing %q", want)
		}
	}
}

func TestDockerSetupRunsGoAppAndInitializesFreshDatabase(t *testing.T) {
	t.Parallel()

	compose, err := os.ReadFile("../../docker-compose.yml")
	if err != nil {
		t.Fatal(err)
	}
	composeText := string(compose)
	if !strings.Contains(composeText, "./migrations/000001_init.sql:/docker-entrypoint-initdb.d/000001_init.sql:ro") {
		t.Fatal("docker-compose.yml does not mount initial migration into Postgres init directory")
	}

	dockerfile, err := os.ReadFile("../../Dockerfile")
	if err != nil {
		t.Fatal(err)
	}
	dockerfileText := string(dockerfile)
	if strings.Contains(strings.ToLower(dockerfileText), "pocketbase") {
		t.Fatal("Dockerfile still references PocketBase")
	}
	for _, want := range []string{
		"RUN go test ./...",
		"RUN go build -o /out/coast-monitoring ./cmd/server",
		"COPY migrations /app/migrations",
		"COPY web /app/web",
		`CMD ["/app/coast-monitoring"]`,
	} {
		if !strings.Contains(dockerfileText, want) {
			t.Fatalf("Dockerfile missing %q", want)
		}
	}
}
