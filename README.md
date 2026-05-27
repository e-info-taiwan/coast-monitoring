# Coast Monitoring

Coast Monitoring is a single Go service backed by PostgreSQL. The server exposes admin APIs under `/api/admin`, app-facing APIs under `/api/app`, authentication endpoints, and the static admin UI from `web/admin`.

## Local Development

1. Copy the example environment file:

```bash
cp .env.example .env
```

2. Start PostgreSQL:

```bash
docker compose up -d db
```

3. Initialize the database schema.

The local Docker Compose database applies `migrations/000001_init.sql` automatically when Postgres creates a fresh `postgres_data` volume. If you are using an existing volume or an external database, apply the SQL in `migrations/000001_init.sql` manually before starting the Go server.

4. Run the service:

```bash
go run ./cmd/server
```

5. Open the admin UI:

```text
http://127.0.0.1:8090/
```

When the Docker daemon is available, you can also run the app through Docker Compose:

```bash
docker compose up --build
```

## Configuration

The common local and deployment settings are:

- `DATABASE_URL`: PostgreSQL connection string.
- `SESSION_SECRET`: random deployment secret required by service configuration; use at least 32 characters.
- `HTTP_ADDR`: listen address, usually `:8090`.
- `GOOGLE_CLIENT_ID`: Google OAuth client ID.
- `GOOGLE_CLIENT_SECRET`: Google OAuth client secret.
- `GOOGLE_REDIRECT_URL`: Google OAuth callback URL, for example `http://127.0.0.1:8090/api/auth/google/callback`.
- `BOOTSTRAP_ADMIN_EMAIL`: temporary first-admin bootstrap email.
- `ADMIN_ALLOWED_ORIGINS`: comma-separated browser origins allowed for admin API requests.
- `APP_ALLOWED_ORIGINS`: comma-separated browser origins allowed for app API requests.

## Authentication And Roles

The service supports Google login and email/password login. Users have one of two roles:

- `admin`: may use the admin UI and `/api/admin` endpoints.
- `volunteer`: may use app-facing `/api/app` endpoints.

For the first deployment or a new local database, set `BOOTSTRAP_ADMIN_EMAIL` to the email address that should become the first admin. Sign in once with that email, verify the account has the `admin` role, then remove `BOOTSTRAP_ADMIN_EMAIL` from the environment.

## API Boundaries

- `/api/admin` powers the admin UI and includes user management, catalog management, observation management, and audit log access. These routes require an active admin session.
- `/api/app` is for the app-facing frontend and volunteer workflows. It does not expose user management data.
- Unknown `/api/*` routes return `404` instead of falling back to the static admin UI.

## Testing

Run the Go test suite:

```bash
go test ./...
```

For quick frontend syntax checks, run `node --check` against changed JavaScript files, for example:

```bash
node --check web/admin/app.js
node --check web/public/species.js
node --check web/public/location.js
```

## Deployment

See [docs/gcp-deployment.md](docs/gcp-deployment.md) for the Cloud Run, Cloud SQL PostgreSQL, and Secret Manager deployment notes.
