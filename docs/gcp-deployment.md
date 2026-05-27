# GCP Deployment

This service is intended to run as one Cloud Run service backed by Cloud SQL PostgreSQL, with secrets stored in Secret Manager.

## Services

- Cloud Run: runs the Go HTTP service and serves the static admin UI.
- Cloud SQL for PostgreSQL: stores users, sessions, catalog data, observations, and audit logs.
- Secret Manager: stores database credentials, session secret, and OAuth client secrets.

## Required Environment Variables

- `DATABASE_URL`: PostgreSQL connection string for the Cloud SQL database.
- `SESSION_SECRET`: random session signing secret; use at least 32 characters.
- `GOOGLE_CLIENT_ID`: Google OAuth client ID.
- `GOOGLE_CLIENT_SECRET`: Google OAuth client secret.
- `GOOGLE_REDIRECT_URL`: production OAuth callback URL, usually `https://SERVICE_URL/api/auth/google/callback`.
- `BOOTSTRAP_ADMIN_EMAIL`: first-admin email; set only for the first deployment.
- `ADMIN_ALLOWED_ORIGINS`: comma-separated allowed origins for the admin UI.
- `APP_ALLOWED_ORIGINS`: comma-separated allowed origins for the app-facing frontend.

Store sensitive values such as `DATABASE_URL`, `SESSION_SECRET`, and `GOOGLE_CLIENT_SECRET` in Secret Manager, then mount them into Cloud Run as environment variables or secret references.

## Database Setup

Create a Cloud SQL PostgreSQL instance and database, then apply the SQL migration before routing production traffic:

```bash
psql "$DATABASE_URL" -f migrations/000001_init.sql
```

For Cloud Run, configure the service with the Cloud SQL instance connection and use a `DATABASE_URL` that matches the chosen connection method. Keep the application database user limited to the app database.

## First Admin Bootstrap

1. Set `BOOTSTRAP_ADMIN_EMAIL` to the Google account that should become the first admin.
2. Deploy the service.
3. Sign in once with that exact email address.
4. Verify the user has the `admin` role in the admin UI or database.
5. Remove `BOOTSTRAP_ADMIN_EMAIL` from the Cloud Run service configuration and redeploy.

Do not keep `BOOTSTRAP_ADMIN_EMAIL` in the long-term production environment.

## Scaling Defaults

Use conservative Cloud Run scaling until database pool limits have been reviewed:

- Minimum instances: `0`
- Maximum instances: `2`
- Concurrency: `20`

Increase maximum instances only after reviewing Cloud SQL connection limits and the Go service database pool settings.

Example Cloud Run flags:

```bash
gcloud run deploy coast-monitoring \
  --image REGION-docker.pkg.dev/PROJECT/REPOSITORY/coast-monitoring:TAG \
  --region REGION \
  --allow-unauthenticated \
  --port 8090 \
  --min-instances 0 \
  --max-instances 2 \
  --concurrency 20
```

Set environment variables and secret references in the same deployment command or through the Cloud Run console. The container currently listens on `8090`; if you change `HTTP_ADDR`, keep the Cloud Run container port and the application listen port aligned.
