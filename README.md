# Coast Monitoring

Coast Monitoring is a single Go service backed by PostgreSQL. The server exposes admin APIs under `/api/admin`, app-facing APIs under `/api/app`, authentication endpoints, and the static admin UI from `web/admin`.

## Reef Check v1.7 imported observations

The admin landing page reads imported observations from `survey → event → transect`.
It supports filtering, detail views, and audited edits of existing transect metadata and observations.
Migration `000006` removes the legacy Reef Check tables and links participants to users. See [the v1.7 admin guide](docs/reef-check-v17-admin.md)
for supported fields, calculation rules, local database verification, and API details.

## Local Development

The local base is `coast_v17` on native PostgreSQL at `127.0.0.1:5432`.
Keep its imported observations as well as its schema. The old local `coast_monitoring`
database has been retired; GCP database names and connection settings are unchanged.

1. Use the existing gitignored `.env`. On a new checkout only, copy `.env.example`
   to `.env`, set `DATABASE_URL` for the local PostgreSQL user, and configure a random
   `SESSION_SECRET` of at least 32 characters. An empty database can be created with
   `createdb -h 127.0.0.1 coast_v17`; migrations create schema and seeds, but do not
   restore imported observations. Restore a data backup when the full local base is needed.

2. Load the environment and apply pending migrations without starting the server:

```bash
set -a
source .env
set +a
go run ./cmd/migrate
```

3. Start the service when needed:

```bash
go run ./cmd/server
```

The server also applies pending migrations at startup. Open the address configured
by `HTTP_ADDR` (`127.0.0.1:8091` in the current local `.env`; the example uses `8090`).
Keep OAuth redirect URLs and allowed origins aligned with that address.

For future schema changes, add the next numbered SQL file under `migrations/`, apply
it to this same database, and verify locally before pushing for CI/CD. Do not edit
already-applied migrations or maintain a separate manual schema.

Docker Compose is an optional isolated sandbox, activated explicitly:

```bash
docker compose --profile isolated up --build
```

Its database uses host port `55440` and a separate Docker volume; it is not the daily
local base. The application applies all migrations on startup. A fresh volume contains
schema and seeds only. Changing `POSTGRES_DB` does not rename databases in an existing
Docker volume.

## Configuration

The common local and deployment settings are:

- `DATABASE_URL`: PostgreSQL connection string.
- `SESSION_SECRET`: random deployment secret required by service configuration; use at least 32 characters.
- `HTTP_ADDR`: local listen address, usually `:8090`. On Cloud Run, leave this unset and let the service use the injected `PORT`.
- `GOOGLE_CLIENT_ID`: Google OAuth client ID.
- `GOOGLE_CLIENT_SECRET`: Google OAuth client secret.
- `GOOGLE_REDIRECT_URL`: Google OAuth callback URL, for example `http://127.0.0.1:8090/api/auth/google/callback`.
- `BOOTSTRAP_ADMIN_EMAIL`: temporary first-admin bootstrap email.
- `ADMIN_ALLOWED_ORIGINS`: comma-separated browser origins allowed for admin API requests.
- `APP_ALLOWED_ORIGINS`: comma-separated browser origins allowed for app API requests.
- `CRON_SECRET`: shared secret for `/api/cron/*` webhook authentication (e.g. Cloud Scheduler).
- `ENABLE_CWA_CRON`: set `true` to enable the built-in background ticker for CWA marine observations.
- `CWA_SYNC_INTERVAL`: sync interval for CWA marine background ticker (e.g. `1h`).

## CWA Marine Temperature & Cronjob

Central Weather Administration (CWA) sea surface temperature data integration:
- When creating a Reef Check event in the admin UI, selecting a site automatically suggests the nearest CWA station and auto-fills the water temperature (`water_temp_c`) for the event date.
- Cronjobs for syncing CWA sea temperatures can be deployed via Cloud Scheduler (calling `/api/cron/sync-cwa-marine`), a standalone Cloud Run Job (`sync-cwa-marine`), or in-server background ticker (`ENABLE_CWA_CRON=true`).
- See [docs/cwa-marine-cronjob.md](docs/cwa-marine-cronjob.md) for full setup instructions and CLI documentation.

## Authentication And Roles

The service supports Google login and email/password login. Users have one of two roles:

- `admin`: may use the admin UI and `/api/admin` endpoints.
- `volunteer`: may use app-facing `/api/app` endpoints.

For the first deployment or a new local database, set `BOOTSTRAP_ADMIN_EMAIL` to the email address that should become the first admin. Sign in once with that email, verify the account has the `admin` role, then remove `BOOTSTRAP_ADMIN_EMAIL` from the environment.

## API Boundaries

- `/api/admin` powers the admin UI and includes user management, catalog management, observation management, and audit log access. These routes require an active admin session.
- `/api/app` is for the app-facing frontend and volunteer workflows. It does not expose user management data.
- Unknown `/api/*` routes return `404` instead of falling back to the static admin UI.

See [docs/api.md](docs/api.md) for request/response examples and the FE server integration pattern.

The Reef Check workflow supports complete survey creation, owner-aware listing and detail
access, atomic replacement, deletion, and computed survey reports. Report output includes
substrate coverage, live coral cover, and average/SD/SE values across the four transect segments.

## Testing

Run the Go test suite:

```bash
go test ./...
```

For quick frontend syntax checks, run `node --check` against changed JavaScript files, for example:

```bash
node --check web/admin/app.js
node --check web/admin/reef-data.js
node --test web/admin/tests/*.test.mjs
node --check web/public/species.js
node --check web/public/location.js
```

## Deployment

See [docs/gcp-deployment.md](docs/gcp-deployment.md) for the Cloud Run, Cloud Build, Cloud SQL PostgreSQL, and Secret Manager deployment notes.
