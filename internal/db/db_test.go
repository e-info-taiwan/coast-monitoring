package db

import (
	"context"
	"os"
	"regexp"
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
		"CREATE TABLE sites (",
		"county text NOT NULL",
		"latitude numeric(9,6) NOT NULL",
		"longitude numeric(9,6) NOT NULL",
		"CREATE TABLE reef_check_surveys (",
		"depth_m integer NOT NULL CHECK (depth_m > 0)",
		"rkc_reason text NOT NULL DEFAULT ''",
		"CREATE TABLE reef_check_survey_recorders (",
		"role text NOT NULL CHECK (role IN ('benthos', 'fish', 'invertebrate'))",
		"CREATE TABLE reef_check_segments (",
		"segment_index integer NOT NULL CHECK (segment_index BETWEEN 1 AND 4)",
		"CREATE TABLE substrate_codes (",
		"normalized_category text NOT NULL CHECK (normalized_category IN ('HC', 'SC', 'RKC', 'NIA', 'SP', 'RC', 'RB', 'SD', 'SI', 'OT'))",
		"CREATE TABLE substrate_points (",
		"point_index integer NOT NULL CHECK (point_index BETWEEN 1 AND 40)",
		"transect_m numeric(4,1) NOT NULL",
		"UNIQUE (survey_id, segment_index, point_index)",
		"UNIQUE (survey_id, transect_m)",
		"CREATE TABLE substrate_bleaching_counts (",
		"hc_bleached_count integer NOT NULL CHECK (hc_bleached_count >= 0)",
		"sc_bleached_count integer NOT NULL CHECK (sc_bleached_count >= 0)",
		"CREATE TABLE reef_check_metrics (",
		"module text NOT NULL CHECK (module IN ('fish', 'invertebrate', 'impact', 'rare_organism'))",
		"CREATE TABLE reef_check_metric_counts (",
		"count integer NOT NULL CHECK (count >= 0)",
		"INSERT INTO species",
		"('蝶魚', 'Butterflyfish')",
		"('石斑魚 30-40', 'Grouper 30-40')",
		"('珊瑚蝦', 'Banded coral shrimp')",
		"('硨磲貝 40-50', 'Giant clam 40-50')",
		"('海龜', 'Turtles')",
		"INSERT INTO substrate_codes",
		"('1', '硬珊瑚', 'HC')",
		"('98', '泥沙 (沙)', 'SD')",
		"INSERT INTO reef_check_metrics",
		"('fish', 'butterflyfish', '蝶魚'",
		"('fish', 'grouper_30_40', '石斑魚 30-40'",
		"('invertebrate', 'lobster', '龍蝦'",
		"('impact', 'trash', '垃圾'",
		"('rare_organism', 'turtles', '海龜'",
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

func TestReefCheckGPSSiteSeedContainsCSVLocationsAndSites(t *testing.T) {
	t.Parallel()

	seed, err := os.ReadFile("../../migrations/000002_seed_reef_check_sites.sql")
	if err != nil {
		t.Fatal(err)
	}
	sql := string(seed)

	required := []string{
		"INSERT INTO locations",
		"INSERT INTO sites",
		"('萬里區', '萬里區')",
		"('恆春鎮', '恆春鎮')",
		"('蘭嶼鄉', '蘭嶼鄉')",
		"('萬里區', '新北市', '野柳', 'Yeliu', 25.211511, 121.698828)",
		"('恆春鎮', '屏東縣', '合界', 'Hejie', 21.955583, 120.711889)",
		"('蘭嶼鄉', '臺東縣', '曙光礁', 'Shuguang Reef', 22.067078, 121.569790)",
	}
	for _, want := range required {
		if !strings.Contains(sql, want) {
			t.Fatalf("Reef Check GPS seed missing %q", want)
		}
	}
	locationRows := regexp.MustCompile(`\('[^']+', '[^']+'\)`).FindAllString(sql, -1)
	if len(locationRows) < 14 {
		t.Fatalf("location seed rows = %d, want at least 14", len(locationRows))
	}
	siteRows := regexp.MustCompile(`\('[^']+', '[^']+', '[^']+', '[^']+', [0-9]+\.[0-9]+, [0-9]+\.[0-9]+\)`).FindAllString(sql, -1)
	if len(siteRows) != 36 {
		t.Fatalf("site seed rows = %d, want 36", len(siteRows))
	}
}

func TestReefCheckFieldSheetMetadataMigrationMatchesFieldWorkbook(t *testing.T) {
	t.Parallel()

	migration, err := os.ReadFile("../../migrations/000003_add_reef_check_field_sheet_metadata.sql")
	if err != nil {
		t.Fatal(err)
	}
	sql := string(migration)

	required := []string{
		"ALTER TABLE reef_check_surveys",
		"ADD COLUMN IF NOT EXISTS country_island text NOT NULL DEFAULT ''",
		"ADD COLUMN IF NOT EXISTS team_leader text NOT NULL DEFAULT ''",
		"ADD COLUMN IF NOT EXISTS survey_time text NOT NULL DEFAULT ''",
		"ADD COLUMN IF NOT EXISTS visibility text NOT NULL DEFAULT ''",
		"ADD COLUMN IF NOT EXISTS temperature text NOT NULL DEFAULT ''",
		"ADD COLUMN IF NOT EXISTS substrate_comments text NOT NULL DEFAULT ''",
		"ADD COLUMN IF NOT EXISTS rkc_bleaching_percent numeric(5,2)",
		"CHECK (rkc_bleaching_percent IS NULL OR (rkc_bleaching_percent >= 0 AND rkc_bleaching_percent <= 100))",
	}
	for _, want := range required {
		if !strings.Contains(sql, want) {
			t.Fatalf("field sheet metadata migration missing %q", want)
		}
	}
}

func TestReefCheckV12SchemaMigrationMatchesSpec(t *testing.T) {
	t.Parallel()

	migration, err := os.ReadFile("../../migrations/000004_reef_check_v12_schema.sql")
	if err != nil {
		t.Fatal(err)
	}
	sql := string(migration)

	required := []string{
		"CREATE TYPE taxon_group AS ENUM ('fish', 'invert', 'rare')",
		"CREATE TYPE impact_group AS ENUM ('coral_damage', 'trash', 'bleaching', 'disease')",
		"CREATE TYPE impact_value_type AS ENUM ('level', 'count', 'percent')",
		"CREATE TYPE transect_method AS ENUM ('line', 'belt_fish', 'belt_invert')",
		"CREATE TYPE participant_role AS ENUM ('member', 'team_leader', 'team_scientist')",
		"CREATE TABLE IF NOT EXISTS substrate_type (",
		"code text PRIMARY KEY",
		"numeric_code smallint NOT NULL",
		"CREATE TABLE IF NOT EXISTS taxon (",
		"taxon_group taxon_group NOT NULL",
		"is_aggregate boolean NOT NULL DEFAULT false",
		"aggregate_of text",
		"CREATE TABLE IF NOT EXISTS impact_type (",
		"impact_group impact_group NOT NULL",
		"value_type impact_value_type NOT NULL",
		"has_raw_count boolean NOT NULL DEFAULT false",
		"CREATE TABLE IF NOT EXISTS diver (",
		"CHECK (name_zh IS NOT NULL OR name_en IS NOT NULL)",
		"CREATE TABLE IF NOT EXISTS site (",
		"region text",
		"UNIQUE (name_en)",
		"CREATE TABLE IF NOT EXISTS survey (",
		"start_date date NOT NULL",
		"end_date date NOT NULL",
		"UNIQUE (site_id, start_date)",
		"CREATE TABLE IF NOT EXISTS site_description (",
		"survey_id integer PRIMARY KEY REFERENCES survey(id) ON DELETE CASCADE",
		"temp_surface_c numeric",
		"anthropogenic_impact text",
		"submitted_by text",
		"CREATE TABLE IF NOT EXISTS transect (",
		"method transect_method NOT NULL",
		"survey_date date",
		"rkc_bleaching_note text",
		"UNIQUE (survey_id, method, depth_m)",
		"CREATE TABLE IF NOT EXISTS transect_participant (",
		"role participant_role NOT NULL",
		"UNIQUE (transect_id, diver_id, role)",
		"CREATE TABLE IF NOT EXISTS substrate_point (",
		"substrate_code text NOT NULL REFERENCES substrate_type(code)",
		"UNIQUE (transect_id, position_m)",
		"CREATE TABLE IF NOT EXISTS substrate_bleaching (",
		"UNIQUE (transect_id, segment)",
		"CREATE TABLE IF NOT EXISTS belt_observation (",
		"taxon_id integer NOT NULL REFERENCES taxon(id)",
		"UNIQUE (transect_id, taxon_id, segment)",
		"CREATE TABLE IF NOT EXISTS impact_observation (",
		"impact_type_id integer NOT NULL REFERENCES impact_type(id)",
		"raw_value numeric NOT NULL DEFAULT 0",
		"UNIQUE (transect_id, impact_type_id, segment)",
		"('HC', 1, '硬珊瑚', 'hard coral'",
		"('OT', 10, '其他', 'other'",
		"('fish', '石斑魚總數', 'Grouper total', NULL, true, 'grouper'",
		"('invert', '硨磲貝總數', 'Giant clam total', NULL, true, 'giant_clam'",
		"('trash', '漁業垃圾（漁網）', 'Trash: fish nets', 'level', true",
		"('bleaching', '珊瑚白化佔總體百分比', 'Bleaching (% of population)', 'percent', false",
		"('北海岸與東北角', '新北市', '萬里區', '野柳', 'Yeliu', 25.21151111, 121.6988278)",
		"('蘭嶼', '臺東縣', '蘭嶼鄉', '曙光礁', 'Shuguang Reef', 22.067078, 121.56979)",
	}
	for _, want := range required {
		if !strings.Contains(sql, want) {
			t.Fatalf("v1.2 schema migration missing %q", want)
		}
	}
	siteRows := regexp.MustCompile(`\('[^']+', '[^']+', '[^']+', '[^']+', '[^']+', [0-9]+\.[0-9]+, [0-9]+\.[0-9]+\)`).FindAllString(sql, -1)
	if len(siteRows) != 36 {
		t.Fatalf("v1.2 site seed rows = %d, want 36", len(siteRows))
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
	if strings.Contains(strings.ToLower(dockerfileText), "pocket"+"base") {
		t.Fatal("Dockerfile still references the legacy backend")
	}
	for _, want := range []string{
		"RUN go test ./...",
		"RUN go build -o /out/coast-monitoring ./cmd/server",
		"COPY --chown=app:app migrations /app/migrations",
		"COPY --chown=app:app web /app/web",
		"EXPOSE 8080",
		"USER app",
		`CMD ["/app/coast-monitoring"]`,
	} {
		if !strings.Contains(dockerfileText, want) {
			t.Fatalf("Dockerfile missing %q", want)
		}
	}
}

func TestCloudBuildConfigBuildsPushesAndDeploysToCloudRun(t *testing.T) {
	t.Parallel()

	config, err := os.ReadFile("../../cloudbuild.yaml")
	if err != nil {
		t.Fatal(err)
	}
	text := string(config)
	required := []string{
		"name: gcr.io/cloud-builders/docker",
		"name: gcr.io/google.com/cloudsdktool/cloud-sdk:slim",
		"golang:1.23-alpine",
		"go test -count=1 ./...",
		"${_REGION}-docker.pkg.dev/${PROJECT_ID}/${_ARTIFACT_REPOSITORY}/${_IMAGE_NAME}:${SHORT_SHA}",
		"--add-cloudsql-instances=${_CLOUDSQL_INSTANCE}",
		"--update-secrets=DATABASE_URL=${_DATABASE_URL_SECRET},SESSION_SECRET=${_SESSION_SECRET_SECRET},GOOGLE_CLIENT_SECRET=${_GOOGLE_CLIENT_SECRET_SECRET}",
		"--min-instances=${_MIN_INSTANCES}",
		"--max-instances=${_MAX_INSTANCES}",
		"--concurrency=${_CONCURRENCY}",
		"REGIONAL_USER_OWNED_BUCKET",
	}
	for _, want := range required {
		if !strings.Contains(text, want) {
			t.Fatalf("cloudbuild.yaml missing %q", want)
		}
	}
}
