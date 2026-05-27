# Go/PostgreSQL Rewrite Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace the PocketBase/SQLite backend with a single Go Cloud Run service backed by PostgreSQL while preserving the existing admin UI design and adding a restricted app-facing API.

**Architecture:** The service serves static frontend assets and REST APIs from one Go binary. Admin and app HTTP routes are separate, but both call shared services and repositories. PostgreSQL is the single source of truth for users, roles, sessions, domain data, and audit logs.

**Tech Stack:** Go 1.23+, `net/http`, `github.com/go-chi/chi/v5`, `github.com/jackc/pgx/v5/pgxpool`, `github.com/google/uuid`, `golang.org/x/crypto/bcrypt`, `golang.org/x/oauth2`, `github.com/coreos/go-oidc/v3/oidc`, PostgreSQL 16, Cloud Run, Cloud SQL.

---

## Scope Check

This is a backend replacement plus frontend API migration. It stays as one plan because the output must be a single deployable service. The plan is organized into incremental tasks so each task leaves the repository buildable or moves one bounded subsystem forward.

## Target File Structure

- Create `go.mod`: module definition and Go dependencies.
- Create `cmd/server/main.go`: process entrypoint.
- Create `internal/config/config.go`: environment parsing and validation.
- Create `internal/db/db.go`: PostgreSQL pool setup and health checks.
- Create `internal/db/migrate.go`: migration runner used by the server and tests.
- Create `migrations/000001_init.sql`: PostgreSQL schema.
- Create `internal/auth/password.go`: bcrypt password hashing and verification.
- Create `internal/auth/session.go`: session token generation, hashing, cookie helpers.
- Create `internal/auth/oauth.go`: Google OAuth start/callback helpers.
- Create `internal/policy/policy.go`: role and ownership authorization checks.
- Create `internal/repository/*.go`: database access for users, sessions, locations, species, observations, audit logs, and login attempts.
- Create `internal/service/*.go`: validation and use-case logic shared by admin and app routes.
- Create `internal/http/router.go`: top-level router, middleware, static file serving.
- Create `internal/http/auth_handlers.go`: login, logout, current session.
- Create `internal/http/admin_handlers.go`: admin API handlers.
- Create `internal/http/app_handlers.go`: app-facing API handlers.
- Create `internal/http/dto.go`: response/request DTOs with separated admin/app shapes.
- Create `internal/audit/audit.go`: audit log helper.
- Move `pb_public/*` to `web/admin/*`: preserve current UI assets.
- Modify `web/admin/app.js`: replace PocketBase client calls with first-party API calls.
- Modify `Dockerfile`: build Go binary and copy `web/` and `migrations/`.
- Modify `docker-compose.yml`: replace PocketBase with app and PostgreSQL services.
- Modify `.env.example`: document local and production env vars.
- Modify `README.md`: replace PocketBase runbook with Go/PostgreSQL runbook.
- Delete `pb_hooks/` and `pb_migrations/` after the Go routes and frontend pass verification.

---

### Task 1: Scaffold Go Module And HTTP Server

**Files:**
- Create: `go.mod`
- Create: `cmd/server/main.go`
- Create: `internal/config/config.go`
- Create: `internal/http/router.go`
- Test: `internal/config/config_test.go`
- Test: `internal/http/router_test.go`

- [ ] **Step 1: Initialize Go module**

Run:

```bash
go mod init coast-monitoring
go get github.com/go-chi/chi/v5 github.com/jackc/pgx/v5/pgxpool github.com/google/uuid golang.org/x/crypto/bcrypt golang.org/x/oauth2 github.com/coreos/go-oidc/v3/oidc
```

Expected: `go.mod` and `go.sum` are created.

- [ ] **Step 2: Write config tests**

Create `internal/config/config_test.go`:

```go
package config

import "testing"

func TestLoadRequiresDatabaseURL(t *testing.T) {
	t.Setenv("DATABASE_URL", "")
	_, err := Load()
	if err == nil {
		t.Fatal("expected DATABASE_URL validation error")
	}
}

func TestLoadDefaults(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://user:pass@localhost:5432/coast?sslmode=disable")
	t.Setenv("SESSION_SECRET", "01234567890123456789012345678901")
	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.HTTPAddr != ":8090" {
		t.Fatalf("HTTPAddr = %q, want :8090", cfg.HTTPAddr)
	}
	if cfg.SessionCookieName != "coast_session" {
		t.Fatalf("SessionCookieName = %q", cfg.SessionCookieName)
	}
}
```

- [ ] **Step 3: Implement config loader**

Create `internal/config/config.go`:

```go
package config

import (
	"errors"
	"os"
	"strings"
)

type Config struct {
	HTTPAddr            string
	DatabaseURL         string
	SessionSecret       string
	SessionCookieName   string
	CSRFHeaderName      string
	GoogleClientID      string
	GoogleClientSecret  string
	GoogleRedirectURL   string
	BootstrapAdminEmail string
	AdminAllowedOrigins []string
	AppAllowedOrigins   []string
}

func Load() (Config, error) {
	cfg := Config{
		HTTPAddr:          env("HTTP_ADDR", ":8090"),
		DatabaseURL:       strings.TrimSpace(os.Getenv("DATABASE_URL")),
		SessionSecret:     strings.TrimSpace(os.Getenv("SESSION_SECRET")),
		SessionCookieName: env("SESSION_COOKIE_NAME", "coast_session"),
		CSRFHeaderName:    env("CSRF_HEADER_NAME", "X-CSRF-Token"),
		GoogleClientID:    strings.TrimSpace(os.Getenv("GOOGLE_CLIENT_ID")),
		GoogleClientSecret: strings.TrimSpace(
			os.Getenv("GOOGLE_CLIENT_SECRET"),
		),
		GoogleRedirectURL: strings.TrimSpace(os.Getenv("GOOGLE_REDIRECT_URL")),
		BootstrapAdminEmail: strings.ToLower(strings.TrimSpace(
			os.Getenv("BOOTSTRAP_ADMIN_EMAIL"),
		)),
		AdminAllowedOrigins: splitCSV(os.Getenv("ADMIN_ALLOWED_ORIGINS")),
		AppAllowedOrigins:   splitCSV(os.Getenv("APP_ALLOWED_ORIGINS")),
	}

	if cfg.DatabaseURL == "" {
		return Config{}, errors.New("DATABASE_URL is required")
	}
	if len(cfg.SessionSecret) < 32 {
		return Config{}, errors.New("SESSION_SECRET must be at least 32 characters")
	}
	return cfg, nil
}

func env(key, fallback string) string {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	return value
}

func splitCSV(value string) []string {
	parts := strings.Split(value, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			out = append(out, part)
		}
	}
	return out
}
```

- [ ] **Step 4: Implement router health route**

Create `internal/http/router.go`:

```go
package httpx

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
)

type Dependencies struct{}

func NewRouter(deps Dependencies) http.Handler {
	r := chi.NewRouter()
	r.Get("/healthz", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})
	return r
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}
```

- [ ] **Step 5: Write router test**

Create `internal/http/router_test.go`:

```go
package httpx

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHealthz(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	rec := httptest.NewRecorder()
	NewRouter(Dependencies{}).ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
}
```

- [ ] **Step 6: Implement entrypoint**

Create `cmd/server/main.go`:

```go
package main

import (
	"log"
	"net/http"

	"coast-monitoring/internal/config"
	httpx "coast-monitoring/internal/http"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatal(err)
	}

	server := &http.Server{
		Addr:    cfg.HTTPAddr,
		Handler: httpx.NewRouter(httpx.Dependencies{}),
	}

	log.Printf("listening on %s", cfg.HTTPAddr)
	log.Fatal(server.ListenAndServe())
}
```

- [ ] **Step 7: Verify scaffold**

Run:

```bash
go test ./...
```

Expected: all tests pass.

- [ ] **Step 8: Commit**

Run:

```bash
git add go.mod go.sum cmd/server internal/config internal/http
git commit -m "feat: scaffold go service"
```

---

### Task 2: Add PostgreSQL Schema And Local Database

**Files:**
- Create: `migrations/000001_init.sql`
- Create: `internal/db/db.go`
- Modify: `docker-compose.yml`
- Modify: `.env.example`
- Test: `internal/db/db_test.go`

- [ ] **Step 1: Replace Docker Compose with app database services**

Modify `docker-compose.yml`:

```yaml
services:
  db:
    image: postgres:16-alpine
    environment:
      POSTGRES_DB: coast_monitoring
      POSTGRES_USER: coast
      POSTGRES_PASSWORD: coast
    ports:
      - "5432:5432"
    volumes:
      - postgres_data:/var/lib/postgresql/data
    healthcheck:
      test: ["CMD-SHELL", "pg_isready -U coast -d coast_monitoring"]
      interval: 5s
      timeout: 5s
      retries: 10

  app:
    build:
      context: .
    environment:
      DATABASE_URL: postgres://coast:coast@db:5432/coast_monitoring?sslmode=disable
      SESSION_SECRET: local-development-session-secret-32
      HTTP_ADDR: :8090
    ports:
      - "8090:8090"
    depends_on:
      db:
        condition: service_healthy

volumes:
  postgres_data:
```

- [ ] **Step 2: Write initial migration**

Create `migrations/000001_init.sql`:

```sql
CREATE EXTENSION IF NOT EXISTS pgcrypto;

CREATE TYPE user_role AS ENUM ('admin', 'volunteer');
CREATE TYPE user_status AS ENUM ('active', 'disabled');
CREATE TYPE audit_action AS ENUM ('create', 'update', 'delete');

CREATE TABLE users (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  email text NOT NULL UNIQUE,
  name text NOT NULL,
  role user_role NOT NULL DEFAULT 'volunteer',
  status user_status NOT NULL DEFAULT 'active',
  google_sub text UNIQUE,
  password_hash text,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE sessions (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  token_hash bytea NOT NULL UNIQUE,
  csrf_token_hash bytea NOT NULL UNIQUE,
  user_agent text NOT NULL DEFAULT '',
  ip inet,
  expires_at timestamptz NOT NULL,
  created_at timestamptz NOT NULL DEFAULT now(),
  revoked_at timestamptz
);

CREATE TABLE oauth_states (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  state_hash bytea NOT NULL UNIQUE,
  redirect_path text NOT NULL DEFAULT '/',
  expires_at timestamptz NOT NULL,
  created_at timestamptz NOT NULL DEFAULT now(),
  consumed_at timestamptz
);

CREATE TABLE locations (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  chinese_name text NOT NULL,
  english_name text NOT NULL,
  created_by uuid REFERENCES users(id) ON DELETE SET NULL,
  updated_by uuid REFERENCES users(id) ON DELETE SET NULL,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE species (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  chinese_name text NOT NULL,
  english_name text NOT NULL,
  created_by uuid REFERENCES users(id) ON DELETE SET NULL,
  updated_by uuid REFERENCES users(id) ON DELETE SET NULL,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE observations (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  observed_on date NOT NULL,
  location_id uuid NOT NULL REFERENCES locations(id) ON DELETE RESTRICT,
  species_id uuid NOT NULL REFERENCES species(id) ON DELETE RESTRICT,
  observer_id uuid NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
  count integer NOT NULL CHECK (count >= 0),
  notes text NOT NULL DEFAULT '',
  created_by uuid REFERENCES users(id) ON DELETE SET NULL,
  updated_by uuid REFERENCES users(id) ON DELETE SET NULL,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE audit_logs (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  action audit_action NOT NULL,
  target_table text NOT NULL,
  target_id uuid NOT NULL,
  actor_user_id uuid REFERENCES users(id) ON DELETE SET NULL,
  actor_email text NOT NULL DEFAULT '',
  before_data jsonb,
  after_data jsonb,
  method text NOT NULL DEFAULT '',
  path text NOT NULL DEFAULT '',
  ip inet,
  user_agent text NOT NULL DEFAULT '',
  logged_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE login_attempts (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  email text NOT NULL DEFAULT '',
  ip inet,
  success boolean NOT NULL,
  attempted_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX sessions_user_id_idx ON sessions(user_id);
CREATE INDEX sessions_expires_at_idx ON sessions(expires_at);
CREATE INDEX observations_observer_id_idx ON observations(observer_id);
CREATE INDEX observations_observed_on_idx ON observations(observed_on);
CREATE INDEX audit_logs_target_idx ON audit_logs(target_table, target_id);
CREATE INDEX audit_logs_logged_at_idx ON audit_logs(logged_at DESC);
CREATE INDEX login_attempts_email_time_idx ON login_attempts(email, attempted_at DESC);
```

- [ ] **Step 3: Implement database pool**

Create `internal/db/db.go`:

```go
package db

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

func Open(ctx context.Context, databaseURL string) (*pgxpool.Pool, error) {
	cfg, err := pgxpool.ParseConfig(databaseURL)
	if err != nil {
		return nil, err
	}
	cfg.MaxConns = 10
	cfg.MinConns = 1
	cfg.MaxConnLifetime = time.Hour
	cfg.MaxConnIdleTime = 30 * time.Minute
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, err
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, err
	}
	return pool, nil
}
```

- [ ] **Step 4: Document env vars**

Modify `.env.example`:

```text
DATABASE_URL=postgres://coast:coast@localhost:5432/coast_monitoring?sslmode=disable
SESSION_SECRET=replace-with-at-least-32-random-characters
HTTP_ADDR=:8090
BOOTSTRAP_ADMIN_EMAIL=admin@example.com
GOOGLE_CLIENT_ID=
GOOGLE_CLIENT_SECRET=
GOOGLE_REDIRECT_URL=http://127.0.0.1:8090/api/auth/google/callback
ADMIN_ALLOWED_ORIGINS=http://127.0.0.1:8090
APP_ALLOWED_ORIGINS=http://127.0.0.1:8090
```

- [ ] **Step 5: Add migration verification command**

Run:

```bash
docker compose up -d db
psql "postgres://coast:coast@localhost:5432/coast_monitoring?sslmode=disable" -f migrations/000001_init.sql
```

Expected: migration applies without SQL errors.

- [ ] **Step 6: Commit**

Run:

```bash
git add migrations internal/db docker-compose.yml .env.example
git commit -m "feat: add postgres schema"
```

---

### Task 3: Implement Auth Primitives

**Files:**
- Create: `internal/auth/password.go`
- Create: `internal/auth/session.go`
- Test: `internal/auth/password_test.go`
- Test: `internal/auth/session_test.go`

- [ ] **Step 1: Write password tests**

Create `internal/auth/password_test.go`:

```go
package auth

import "testing"

func TestHashAndVerifyPassword(t *testing.T) {
	hash, err := HashPassword("CorrectHorseBatteryStaple1!")
	if err != nil {
		t.Fatalf("HashPassword() error = %v", err)
	}
	if !VerifyPassword(hash, "CorrectHorseBatteryStaple1!") {
		t.Fatal("expected password to verify")
	}
	if VerifyPassword(hash, "wrong-password") {
		t.Fatal("wrong password verified")
	}
}
```

- [ ] **Step 2: Implement password hashing**

Create `internal/auth/password.go`:

```go
package auth

import "golang.org/x/crypto/bcrypt"

func HashPassword(password string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}
	return string(hash), nil
}

func VerifyPassword(hash string, password string) bool {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)) == nil
}
```

- [ ] **Step 3: Write session tests**

Create `internal/auth/session_test.go`:

```go
package auth

import "testing"

func TestTokenHashIsStableAndNotRawToken(t *testing.T) {
	token := "session-token"
	hash1 := HashToken(token)
	hash2 := HashToken(token)
	if string(hash1) != string(hash2) {
		t.Fatal("token hash should be stable")
	}
	if string(hash1) == token {
		t.Fatal("token hash must not equal raw token")
	}
}

func TestGenerateToken(t *testing.T) {
	token, err := GenerateToken(32)
	if err != nil {
		t.Fatalf("GenerateToken() error = %v", err)
	}
	if len(token) < 40 {
		t.Fatalf("token length = %d, want at least 40", len(token))
	}
}
```

- [ ] **Step 4: Implement session token helpers**

Create `internal/auth/session.go`:

```go
package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
)

func GenerateToken(byteCount int) (string, error) {
	raw := make([]byte, byteCount)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(raw), nil
}

func HashToken(token string) []byte {
	sum := sha256.Sum256([]byte(token))
	return sum[:]
}
```

- [ ] **Step 5: Verify auth primitives**

Run:

```bash
go test ./internal/auth
```

Expected: all auth tests pass.

- [ ] **Step 6: Commit**

Run:

```bash
git add internal/auth
git commit -m "feat: add auth primitives"
```

---

### Task 4: Add Policy Layer

**Files:**
- Create: `internal/policy/policy.go`
- Test: `internal/policy/policy_test.go`

- [ ] **Step 1: Write policy tests**

Create `internal/policy/policy_test.go`:

```go
package policy

import (
	"testing"

	"github.com/google/uuid"
)

func TestAdminCanUseAdminAPI(t *testing.T) {
	user := User{ID: uuid.New(), Role: RoleAdmin, Status: StatusActive}
	if !CanUseAdminAPI(user) {
		t.Fatal("admin should use admin API")
	}
}

func TestVolunteerCannotUseAdminAPI(t *testing.T) {
	user := User{ID: uuid.New(), Role: RoleVolunteer, Status: StatusActive}
	if CanUseAdminAPI(user) {
		t.Fatal("volunteer should not use admin API")
	}
}

func TestDisabledUserCannotUseAPIs(t *testing.T) {
	user := User{ID: uuid.New(), Role: RoleAdmin, Status: StatusDisabled}
	if CanUseAdminAPI(user) || CanUseAppAPI(user) {
		t.Fatal("disabled user should not use APIs")
	}
}

func TestObservationOwnership(t *testing.T) {
	ownerID := uuid.New()
	otherID := uuid.New()
	owner := User{ID: ownerID, Role: RoleVolunteer, Status: StatusActive}
	if !CanManageObservation(owner, ownerID) {
		t.Fatal("volunteer should manage own observation")
	}
	if CanManageObservation(owner, otherID) {
		t.Fatal("volunteer should not manage another user's observation")
	}
	admin := User{ID: otherID, Role: RoleAdmin, Status: StatusActive}
	if !CanManageObservation(admin, ownerID) {
		t.Fatal("admin should manage any observation")
	}
}
```

- [ ] **Step 2: Implement policies**

Create `internal/policy/policy.go`:

```go
package policy

import "github.com/google/uuid"

type Role string
type Status string

const (
	RoleAdmin     Role = "admin"
	RoleVolunteer Role = "volunteer"

	StatusActive   Status = "active"
	StatusDisabled Status = "disabled"
)

type User struct {
	ID     uuid.UUID
	Email  string
	Name   string
	Role   Role
	Status Status
}

func CanUseAdminAPI(user User) bool {
	return user.Status == StatusActive && user.Role == RoleAdmin
}

func CanUseAppAPI(user User) bool {
	return user.Status == StatusActive && (user.Role == RoleAdmin || user.Role == RoleVolunteer)
}

func CanManageObservation(user User, observerID uuid.UUID) bool {
	if user.Status != StatusActive {
		return false
	}
	if user.Role == RoleAdmin {
		return true
	}
	return user.Role == RoleVolunteer && user.ID == observerID
}
```

- [ ] **Step 3: Verify policies**

Run:

```bash
go test ./internal/policy
```

Expected: all policy tests pass.

- [ ] **Step 4: Commit**

Run:

```bash
git add internal/policy
git commit -m "feat: add authorization policies"
```

---

### Task 5: Implement Repositories And Services

**Files:**
- Create: `internal/repository/users.go`
- Create: `internal/repository/sessions.go`
- Create: `internal/repository/catalog.go`
- Create: `internal/repository/observations.go`
- Create: `internal/repository/audit_logs.go`
- Create: `internal/service/users.go`
- Create: `internal/service/catalog.go`
- Create: `internal/service/observations.go`
- Create: `internal/service/auth.go`
- Test: `internal/service/users_test.go`
- Test: `internal/service/observations_test.go`

- [ ] **Step 1: Define shared service models**

Create request/response model structs in service files using these field names:

```go
type User struct {
	ID          uuid.UUID
	Email       string
	Name        string
	Role        policy.Role
	Status      policy.Status
	GoogleSub   *string
	HasPassword bool
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

type Location struct {
	ID          uuid.UUID
	ChineseName string
	EnglishName string
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

type Species struct {
	ID          uuid.UUID
	ChineseName string
	EnglishName string
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

type Observation struct {
	ID          uuid.UUID
	ObservedOn  time.Time
	LocationID  uuid.UUID
	SpeciesID   uuid.UUID
	ObserverID  uuid.UUID
	Count       int
	Notes       string
	CreatedAt   time.Time
	UpdatedAt   time.Time
}
```

- [ ] **Step 2: Implement user service validation**

Create `internal/service/users.go` with these exported methods:

```go
type UserService struct {
	Users UserRepository
}

func (s UserService) CreateUser(ctx context.Context, actor policy.User, input CreateUserInput) (User, error)
func (s UserService) UpdateUser(ctx context.Context, actor policy.User, id uuid.UUID, input UpdateUserInput) (User, error)
func (s UserService) DisableUser(ctx context.Context, actor policy.User, id uuid.UUID) error
func (s UserService) ListUsers(ctx context.Context, actor policy.User) ([]User, error)
```

Validation rules:

- actor must pass `policy.CanUseAdminAPI`.
- email is lowercase and non-empty.
- name is non-empty.
- role is exactly `admin` or `volunteer`.
- status is exactly `active` or `disabled`.
- password is optional; when present it is hashed before storage.

- [ ] **Step 3: Implement catalog service validation**

Create `internal/service/catalog.go` with admin write methods and app read methods:

```go
func (s CatalogService) ListLocations(ctx context.Context, actor policy.User) ([]Location, error)
func (s CatalogService) CreateLocation(ctx context.Context, actor policy.User, input CatalogInput) (Location, error)
func (s CatalogService) UpdateLocation(ctx context.Context, actor policy.User, id uuid.UUID, input CatalogInput) (Location, error)
func (s CatalogService) DeleteLocation(ctx context.Context, actor policy.User, id uuid.UUID) error
func (s CatalogService) ListSpecies(ctx context.Context, actor policy.User) ([]Species, error)
func (s CatalogService) CreateSpecies(ctx context.Context, actor policy.User, input CatalogInput) (Species, error)
func (s CatalogService) UpdateSpecies(ctx context.Context, actor policy.User, id uuid.UUID, input CatalogInput) (Species, error)
func (s CatalogService) DeleteSpecies(ctx context.Context, actor policy.User, id uuid.UUID) error
```

Validation rules:

- list methods require `policy.CanUseAppAPI`.
- create/update/delete require `policy.CanUseAdminAPI`.
- Chinese and English names are non-empty.

- [ ] **Step 4: Implement observation service validation**

Create `internal/service/observations.go`:

```go
func (s ObservationService) ListForAdmin(ctx context.Context, actor policy.User) ([]Observation, error)
func (s ObservationService) ListForApp(ctx context.Context, actor policy.User) ([]Observation, error)
func (s ObservationService) Create(ctx context.Context, actor policy.User, input ObservationInput) (Observation, error)
func (s ObservationService) Update(ctx context.Context, actor policy.User, id uuid.UUID, input ObservationInput) (Observation, error)
func (s ObservationService) Delete(ctx context.Context, actor policy.User, id uuid.UUID) error
```

Validation rules:

- admin list requires `policy.CanUseAdminAPI`.
- app list requires `policy.CanUseAppAPI` and returns only the actor's observations for volunteers.
- create requires `policy.CanUseAppAPI`; volunteers can only create observations for themselves.
- update/delete require `policy.CanManageObservation`.
- count must be zero or greater.
- notes are trimmed and limited to 1000 characters.

- [ ] **Step 5: Write service tests for boundary behavior**

Create tests that use in-memory fake repositories:

```go
func TestVolunteerCannotListUsers(t *testing.T)
func TestAdminCanCreateVolunteer(t *testing.T)
func TestVolunteerAppObservationListOnlyReturnsOwnRows(t *testing.T)
func TestAdminCanUpdateAnyObservation(t *testing.T)
func TestVolunteerCannotUpdateOtherUsersObservation(t *testing.T)
```

- [ ] **Step 6: Verify services**

Run:

```bash
go test ./internal/service ./internal/policy ./internal/auth
```

Expected: all tests pass.

- [ ] **Step 7: Commit**

Run:

```bash
git add internal/repository internal/service
git commit -m "feat: add data services"
```

---

### Task 6: Implement Auth And Session HTTP Routes

**Files:**
- Create: `internal/http/auth_handlers.go`
- Create: `internal/http/middleware.go`
- Modify: `internal/http/router.go`
- Modify: `internal/http/dto.go`
- Test: `internal/http/auth_handlers_test.go`

- [ ] **Step 1: Define auth routes**

Add these routes in `internal/http/router.go`:

```go
r.Get("/api/session", deps.AuthHandlers.Session)
r.Post("/api/auth/password", deps.AuthHandlers.PasswordLogin)
r.Get("/api/auth/google/start", deps.AuthHandlers.GoogleStart)
r.Get("/api/auth/google/callback", deps.AuthHandlers.GoogleCallback)
r.Post("/api/auth/logout", deps.AuthHandlers.Logout)
```

- [ ] **Step 2: Implement session DTOs**

Create `internal/http/dto.go` with:

```go
type CurrentUserResponse struct {
	ID    string `json:"id"`
	Email string `json:"email"`
	Name  string `json:"name"`
	Role  string `json:"role"`
}

type SessionResponse struct {
	Authenticated bool                 `json:"authenticated"`
	User          *CurrentUserResponse `json:"user,omitempty"`
	CSRFToken     string               `json:"csrfToken,omitempty"`
}
```

- [ ] **Step 3: Implement middleware contract**

Create `internal/http/middleware.go`:

```go
type contextKey string

const currentUserKey contextKey = "currentUser"

func withCurrentUser(ctx context.Context, user policy.User) context.Context {
	return context.WithValue(ctx, currentUserKey, user)
}

func currentUser(r *http.Request) (policy.User, bool) {
	user, ok := r.Context().Value(currentUserKey).(policy.User)
	return user, ok
}
```

- [ ] **Step 4: Implement password login**

`PasswordLogin` must:

- parse `email` and `password`.
- record failed and successful login attempts.
- reject disabled users.
- verify bcrypt hash.
- create a session row with hashed session token and hashed CSRF token.
- set `coast_session` as `HttpOnly`, `Secure` in production, `SameSite=Lax`, path `/`.
- set `coast_csrf` as non-HttpOnly, `Secure` in production, `SameSite=Lax`, path `/`.
- return `SessionResponse`.

- [ ] **Step 5: Implement Google Login**

`GoogleStart` must:

- create an OAuth state token.
- store the hashed state token with expiry in `oauth_states`.
- redirect to Google.

`GoogleCallback` must:

- verify the state.
- exchange the code.
- verify ID token through OIDC.
- find user by `google_sub`; if not found, find active user by email and attach `google_sub`.
- if no admin exists and email equals `BOOTSTRAP_ADMIN_EMAIL`, create the first admin.
- reject unknown non-bootstrap emails.
- create the same session cookies used by password login.

- [ ] **Step 6: Test app route cannot infer admin user data**

In `auth_handlers_test.go`, verify `/api/session` returns only `id`, `email`, `name`, and `role`, not `password_hash`, `google_sub`, `status`, or audit fields.

- [ ] **Step 7: Verify auth HTTP**

Run:

```bash
go test ./internal/http ./internal/auth ./internal/service
```

Expected: all tests pass.

- [ ] **Step 8: Commit**

Run:

```bash
git add internal/http internal/auth internal/service
git commit -m "feat: add session auth routes"
```

---

### Task 7: Implement Admin API

**Files:**
- Create: `internal/http/admin_handlers.go`
- Modify: `internal/http/router.go`
- Modify: `internal/http/dto.go`
- Test: `internal/http/admin_handlers_test.go`

- [ ] **Step 1: Add admin route group**

In `internal/http/router.go`:

```go
r.Route("/api/admin", func(r chi.Router) {
	r.Use(deps.RequireSession)
	r.Use(deps.RequireAdmin)
	r.Get("/users", deps.AdminHandlers.ListUsers)
	r.Post("/users", deps.AdminHandlers.CreateUser)
	r.Patch("/users/{id}", deps.AdminHandlers.UpdateUser)
	r.Delete("/users/{id}", deps.AdminHandlers.DisableUser)
	r.Get("/locations", deps.AdminHandlers.ListLocations)
	r.Post("/locations", deps.AdminHandlers.CreateLocation)
	r.Patch("/locations/{id}", deps.AdminHandlers.UpdateLocation)
	r.Delete("/locations/{id}", deps.AdminHandlers.DeleteLocation)
	r.Get("/species", deps.AdminHandlers.ListSpecies)
	r.Post("/species", deps.AdminHandlers.CreateSpecies)
	r.Patch("/species/{id}", deps.AdminHandlers.UpdateSpecies)
	r.Delete("/species/{id}", deps.AdminHandlers.DeleteSpecies)
	r.Get("/observations", deps.AdminHandlers.ListObservations)
	r.Patch("/observations/{id}", deps.AdminHandlers.UpdateObservation)
	r.Delete("/observations/{id}", deps.AdminHandlers.DeleteObservation)
	r.Get("/audit-logs", deps.AdminHandlers.ListAuditLogs)
})
```

- [ ] **Step 2: Define admin DTOs**

Add to `internal/http/dto.go`:

```go
type AdminUserResponse struct {
	ID          string `json:"id"`
	Email       string `json:"email"`
	Name        string `json:"name"`
	Role        string `json:"role"`
	Status      string `json:"status"`
	HasGoogle   bool   `json:"hasGoogle"`
	HasPassword bool   `json:"hasPassword"`
	CreatedAt   string `json:"createdAt"`
	UpdatedAt   string `json:"updatedAt"`
}

type SaveUserRequest struct {
	Email    string `json:"email"`
	Name     string `json:"name"`
	Role     string `json:"role"`
	Status   string `json:"status"`
	Password string `json:"password"`
}

type CatalogResponse struct {
	ID          string `json:"id"`
	ChineseName string `json:"chineseName"`
	EnglishName string `json:"englishName"`
	CreatedAt   string `json:"createdAt"`
	UpdatedAt   string `json:"updatedAt"`
}

type SaveCatalogRequest struct {
	ChineseName string `json:"chineseName"`
	EnglishName string `json:"englishName"`
}
```

- [ ] **Step 3: Implement admin handlers**

Each admin handler must:

- parse and validate UUID path params.
- decode JSON request bodies with `http.MaxBytesReader` limit `1<<20`.
- call the matching service method.
- return field-specific DTOs.
- never return `password_hash` or raw `google_sub`.
- create audit logs for create/update/delete service calls.

- [ ] **Step 4: Add admin boundary tests**

Create tests:

```go
func TestAdminUsersRequiresAdmin(t *testing.T)
func TestAdminUserResponseDoesNotExposeSecrets(t *testing.T)
func TestVolunteerCannotCallAdminLocations(t *testing.T)
func TestAdminCanCreateLocation(t *testing.T)
```

- [ ] **Step 5: Verify admin API**

Run:

```bash
go test ./internal/http ./internal/service
```

Expected: all tests pass.

- [ ] **Step 6: Commit**

Run:

```bash
git add internal/http internal/service internal/audit
git commit -m "feat: add admin api"
```

---

### Task 8: Implement App-Facing API

**Files:**
- Create: `internal/http/app_handlers.go`
- Modify: `internal/http/router.go`
- Modify: `internal/http/dto.go`
- Test: `internal/http/app_handlers_test.go`

- [ ] **Step 1: Add app route group**

In `internal/http/router.go`:

```go
r.Route("/api/app", func(r chi.Router) {
	r.Use(deps.RequireSession)
	r.Use(deps.RequireAppUser)
	r.Get("/session", deps.AppHandlers.Session)
	r.Get("/locations", deps.AppHandlers.ListLocations)
	r.Get("/species", deps.AppHandlers.ListSpecies)
	r.Get("/observations", deps.AppHandlers.ListObservations)
	r.Post("/observations", deps.AppHandlers.CreateObservation)
	r.Patch("/observations/{id}", deps.AppHandlers.UpdateObservation)
	r.Delete("/observations/{id}", deps.AppHandlers.DeleteObservation)
})
```

- [ ] **Step 2: Define app DTOs**

Add to `internal/http/dto.go`:

```go
type AppObservationResponse struct {
	ID          string `json:"id"`
	ObservedOn  string `json:"observedOn"`
	LocationID  string `json:"locationId"`
	SpeciesID   string `json:"speciesId"`
	Count       int    `json:"count"`
	Notes       string `json:"notes"`
	CreatedAt   string `json:"createdAt"`
	UpdatedAt   string `json:"updatedAt"`
}

type SaveObservationRequest struct {
	ObservedOn string `json:"observedOn"`
	LocationID string `json:"locationId"`
	SpeciesID  string `json:"speciesId"`
	Count      int    `json:"count"`
	Notes      string `json:"notes"`
}
```

- [ ] **Step 3: Implement app handlers**

Each app handler must:

- require an active `admin` or `volunteer` session.
- return only app DTOs.
- let volunteers see only their observations.
- let admins use the app API without exposing admin-only user data.
- write audit logs for observation create/update/delete.

- [ ] **Step 4: Add app boundary tests**

Create tests:

```go
func TestAppRoutesRequireSession(t *testing.T)
func TestAppRoutesDoNotExposeUsersEndpoint(t *testing.T)
func TestVolunteerOnlySeesOwnObservations(t *testing.T)
func TestVolunteerCannotUpdateOtherUsersObservation(t *testing.T)
func TestAdminCanUseAppObservationRoute(t *testing.T)
```

`TestAppRoutesDoNotExposeUsersEndpoint` must request `/api/app/users` and expect `404 Not Found`.

- [ ] **Step 5: Verify app API**

Run:

```bash
go test ./internal/http ./internal/service ./internal/policy
```

Expected: all tests pass.

- [ ] **Step 6: Commit**

Run:

```bash
git add internal/http internal/service
git commit -m "feat: add app api"
```

---

### Task 9: Move And Migrate Admin Frontend

**Files:**
- Move: `pb_public/index.html` to `web/admin/index.html`
- Move: `pb_public/app.js` to `web/admin/app.js`
- Move: `pb_public/styles.css` to `web/admin/styles.css`
- Move: `pb_public/oauth-callback.html` to `web/admin/oauth-callback.html`
- Move: `pb_public/location.html` to `web/public/location.html`
- Move: `pb_public/location.js` to `web/public/location.js`
- Move: `pb_public/species.html` to `web/public/species.html`
- Move: `pb_public/species.js` to `web/public/species.js`
- Modify: `internal/http/router.go`
- Modify: `web/admin/app.js`
- Test: `internal/http/static_test.go`

- [ ] **Step 1: Move static files**

Run:

```bash
mkdir -p web/admin web/public
git mv pb_public/index.html web/admin/index.html
git mv pb_public/app.js web/admin/app.js
git mv pb_public/styles.css web/admin/styles.css
git mv pb_public/oauth-callback.html web/admin/oauth-callback.html
git mv pb_public/location.html web/public/location.html
git mv pb_public/location.js web/public/location.js
git mv pb_public/species.html web/public/species.html
git mv pb_public/species.js web/public/species.js
```

- [ ] **Step 2: Serve static files**

In `internal/http/router.go`, add:

```go
r.Handle("/*", http.FileServer(http.Dir("web/admin")))
r.Handle("/public/*", http.StripPrefix("/public/", http.FileServer(http.Dir("web/public"))))
```

- [ ] **Step 3: Replace PocketBase API client base**

In `web/admin/app.js`, replace:

```js
const API_BASE = `${window.location.origin}/api`
```

with:

```js
const API_BASE = `${window.location.origin}/api`
const ADMIN_API_BASE = `${API_BASE}/admin`
const APP_API_BASE = `${API_BASE}/app`
```

- [ ] **Step 4: Replace auth flows**

Change admin login calls:

```js
await apiFetch("/collections/users/auth-methods", { auth: false })
await apiFetch(`/collections/${targetCollection}/auth-with-password`, ...)
await apiFetch(`/collections/${targetCollection}/auth-refresh`, ...)
```

to:

```js
await apiFetch("/session", { auth: false })
await apiFetch("/auth/password", { method: "POST", body: { email, password }, auth: false })
await apiFetch("/session", { auth: false })
```

Change Google login start to:

```js
window.location.href = "/api/auth/google/start?redirect=/"
```

- [ ] **Step 5: Replace collection calls with admin/app calls**

Use these mappings in `web/admin/app.js`:

```text
/collections/users/records                 -> /admin/users
/collections/location/records              -> /admin/locations
/collections/species/records               -> /admin/species
/collections/observation/records           -> /app/observations for volunteer entry
/collections/audit_logs/records            -> /admin/audit-logs
```

The admin UI must use `/admin/observations` for admin-wide observation management and `/app/observations` for the volunteer-oriented observation entry workflow.

- [ ] **Step 6: Stop storing auth tokens in localStorage**

Remove writes to `coast-monitoring-auth`. Store only transient UI state in memory. Session state comes from `/api/session` and cookies.

- [ ] **Step 7: Verify static serving**

Create `internal/http/static_test.go`:

```go
package httpx

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestRootServesAdminHTML(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	NewRouter(Dependencies{}).ServeHTTP(rec, req)
	if rec.Code != http.StatusOK && rec.Code != http.StatusNotFound {
		t.Fatalf("unexpected status = %d", rec.Code)
	}
}
```

- [ ] **Step 8: Verify frontend migration**

Run:

```bash
rg "/collections/|auth-with-password|auth-refresh|coast-monitoring-auth" web/admin
```

Expected: no matches except comments describing removed PocketBase behavior.

- [ ] **Step 9: Commit**

Run:

```bash
git add web internal/http
git commit -m "feat: migrate admin frontend to first-party api"
```

---

### Task 10: Docker, Cloud Run, And Documentation

**Files:**
- Modify: `Dockerfile`
- Modify: `README.md`
- Create: `docs/gcp-deployment.md`

- [ ] **Step 1: Replace Dockerfile**

Modify `Dockerfile`:

```dockerfile
FROM golang:1.23-alpine AS build

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN go test ./...
RUN go build -o /out/coast-monitoring ./cmd/server

FROM alpine:3.20

RUN apk add --no-cache ca-certificates
WORKDIR /app
COPY --from=build /out/coast-monitoring /app/coast-monitoring
COPY migrations /app/migrations
COPY web /app/web

EXPOSE 8090
CMD ["/app/coast-monitoring"]
```

- [ ] **Step 2: Add GCP deployment guide**

Create `docs/gcp-deployment.md` with:

```markdown
# GCP Deployment

## Services

- Cloud Run: one service for static frontend and API.
- Cloud SQL PostgreSQL: production database.
- Secret Manager: `DATABASE_URL`, `SESSION_SECRET`, Google OAuth secrets.

## Required Environment Variables

- `DATABASE_URL`
- `SESSION_SECRET`
- `GOOGLE_CLIENT_ID`
- `GOOGLE_CLIENT_SECRET`
- `GOOGLE_REDIRECT_URL`
- `BOOTSTRAP_ADMIN_EMAIL`
- `ADMIN_ALLOWED_ORIGINS`
- `APP_ALLOWED_ORIGINS`

## First Admin Bootstrap

Set `BOOTSTRAP_ADMIN_EMAIL` only for the first deployment. Sign in once with that Google account, verify the user has role `admin`, then remove the variable from production.

## Scaling Defaults

Start with Cloud Run min instances `0`, max instances `2`, and concurrency `20`. Increase max instances only after database connection pool limits are reviewed.
```

- [ ] **Step 3: Update README**

Replace PocketBase-specific run instructions with:

```markdown
## Local Development

1. Copy `.env.example` to `.env`.
2. Run `docker compose up -d db`.
3. Run database migrations.
4. Run `go run ./cmd/server`.
5. Open `http://127.0.0.1:8090/`.
```

- [ ] **Step 4: Verify container build**

Run:

```bash
docker build -t coast-monitoring:local .
```

Expected: image builds and `go test ./...` passes inside the build stage.

- [ ] **Step 5: Commit**

Run:

```bash
git add Dockerfile README.md docs/gcp-deployment.md
git commit -m "docs: add go cloud run deployment guide"
```

---

### Task 11: Remove PocketBase Artifacts

**Files:**
- Delete: `pb_hooks/`
- Delete: `pb_migrations/`
- Delete: remaining `pb_public/` directory after all assets are moved
- Modify: `README.md`

- [ ] **Step 1: Verify there are no PocketBase references in runtime code**

Run:

```bash
rg "PocketBase|pb_public|pb_hooks|pb_migrations|/collections/" .
```

Expected: matches only appear in historical docs or migration notes.

- [ ] **Step 2: Remove PocketBase directories**

Run:

```bash
git rm -r pb_hooks pb_migrations
test ! -d pb_public || git rm -r pb_public
```

- [ ] **Step 3: Verify app still builds**

Run:

```bash
go test ./...
docker build -t coast-monitoring:local .
```

Expected: tests pass and image builds.

- [ ] **Step 4: Commit**

Run:

```bash
git add README.md
git commit -m "chore: remove pocketbase runtime"
```

---

## Final Verification

- [ ] Run `go test ./...`.
- [ ] Run `docker compose up --build`.
- [ ] Open `http://127.0.0.1:8090/`.
- [ ] Bootstrap the first admin.
- [ ] Log in with email/password.
- [ ] Log in with Google.
- [ ] Confirm admin can manage users, locations, species, observations, and audit logs.
- [ ] Confirm volunteer cannot access `/api/admin/users`.
- [ ] Confirm `/api/app/users` returns `404`.
- [ ] Confirm volunteer can create an observation.
- [ ] Confirm volunteer cannot update another user's observation.
- [ ] Confirm admin and app routes both write audit logs for mutations.

## Self-Review

- Spec coverage: The plan covers single Cloud Run deployment, PostgreSQL schema, Google and email/password auth, two-role authorization, separated admin/app routes, shared services, frontend migration, CORS-ready API boundaries, audit logging, Docker, and PocketBase removal.
- Placeholder scan: The plan does not leave open named placeholder markers.
- Type consistency: Role names are consistently `admin` and `volunteer`; status names are consistently `active` and `disabled`; app route names use `/api/app/*`; admin route names use `/api/admin/*`.
