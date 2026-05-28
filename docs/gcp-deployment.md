# GCP Deployment

This service is designed to run as one Cloud Run service backed by Cloud SQL for PostgreSQL. The same Go process serves:

- the admin UI from `web/admin`
- public static pages from `web/public`
- auth endpoints under `/api/auth` and `/api/session`
- admin APIs under `/api/admin`
- app-facing APIs under `/api/app`

Primary Google references:

- Cloud Run container contract: https://cloud.google.com/run/docs/container-contract
- Cloud Run source deploy: https://cloud.google.com/run/docs/deploying-source-code
- Cloud Run secrets: https://cloud.google.com/run/docs/configuring/services/secrets
- Cloud Build config schema: https://cloud.google.com/build/docs/build-config-file-schema
- Cloud Build deploy to Cloud Run: https://cloud.google.com/build/docs/deploying-builds/deploy-cloud-run
- Cloud SQL PostgreSQL from Cloud Run: https://cloud.google.com/sql/docs/postgres/connect-run
- `.gcloudignore`: https://cloud.google.com/sdk/gcloud/reference/topic/gcloudignore

## Deployment Files In This Repo

Required for Cloud Run:

- `Dockerfile`: builds and runs the Go service, includes `migrations/` and `web/`, and runs as the non-root `app` user.
- `cloudbuild.yaml`: Cloud Build pipeline for test, Docker build, Artifact Registry push, and Cloud Run deploy.
- `.gcloudignore`: prevents local-only files such as `.env`, `pb_data/`, `.claude/`, and worktrees from being uploaded by `gcloud run deploy --source .`.
- `.env.example`: documents runtime configuration names and local defaults.
- `deploy/cloud-run.env.yaml.example`: non-secret Cloud Run environment variable template.
- `migrations/000001_init.sql`: initial PostgreSQL schema. Apply this before routing production traffic.
- `docs/gcp-deployment.md`: this deployment runbook.
- `docs/api.md`: API usage for the admin UI and app-facing frontend.

Optional for local development only:

- `docker-compose.yml`: local PostgreSQL and local app container wiring. Do not deploy Compose directly to Cloud Run.

## Runtime Configuration

The service reads these environment variables:

| Variable | Required | Secret | Notes |
| --- | --- | --- | --- |
| `DATABASE_URL` | yes | yes | PostgreSQL connection string. For Cloud SQL Unix sockets, use the keyword format shown below. |
| `SESSION_SECRET` | yes | yes | At least 32 characters. Keep stable across revisions unless all sessions can be invalidated. |
| `PORT` | Cloud Run sets it | no | Used when `HTTP_ADDR` is unset. Cloud Run defaults to `8080`. |
| `HTTP_ADDR` | no | no | Local override such as `:8090`. Leave unset on Cloud Run unless you intentionally override the container port. |
| `GOOGLE_CLIENT_ID` | for Google login | no | OAuth client ID. |
| `GOOGLE_CLIENT_SECRET` | for Google login | yes | OAuth client secret. |
| `GOOGLE_REDIRECT_URL` | for Google login | no | `https://SERVICE_URL/api/auth/google/callback`. |
| `BOOTSTRAP_ADMIN_EMAIL` | first deploy only | no | Exact email allowed to create the first admin through Google login. Remove after bootstrap. |
| `ADMIN_ALLOWED_ORIGINS` | when admin UI calls cross-origin | no | Comma-separated origins allowed for `/api/admin/*`. Same-origin admin UI does not need this. |
| `APP_ALLOWED_ORIGINS` | when app FE calls cross-origin | no | Comma-separated origins allowed for `/api/app/*`. |
| `SECURE_COOKIES` | no | no | Defaults to `true`. Keep `true` in production. |
| `SESSION_COOKIE_NAME` | no | no | Defaults to `coast_session`. |
| `CSRF_HEADER_NAME` | no | no | Defaults to `X-CSRF-Token`. |

Cloud Run injects `PORT`; the app now uses it when `HTTP_ADDR` is unset. Local development can keep `HTTP_ADDR=:8090`.

## Cloud SQL Setup

Create the PostgreSQL instance, database, and user:

```bash
PROJECT_ID="your-gcp-project"
REGION="asia-east1"
INSTANCE="coast-monitoring-db"
DB_NAME="coast_monitoring"
DB_USER="coast_app"
AR_REPOSITORY="coast-monitoring"

gcloud config set project "$PROJECT_ID"
gcloud services enable run.googleapis.com cloudbuild.googleapis.com sqladmin.googleapis.com secretmanager.googleapis.com artifactregistry.googleapis.com

gcloud artifacts repositories create "$AR_REPOSITORY" \
  --repository-format=docker \
  --location="$REGION" \
  --description="Coast Monitoring Docker images"

gcloud sql instances create "$INSTANCE" \
  --database-version=POSTGRES_16 \
  --region="$REGION" \
  --tier=db-f1-micro \
  --storage-size=10GB

gcloud sql databases create "$DB_NAME" --instance="$INSTANCE"
gcloud sql users create "$DB_USER" --instance="$INSTANCE" --password="replace-with-db-password"
```

For a very small cost-sensitive deployment, start with the smallest acceptable Cloud SQL tier and conservative Cloud Run scaling. Review Cloud SQL connection limits before increasing Cloud Run maximum instances or concurrency.

## Apply The Initial Migration

Apply `migrations/000001_init.sql` once for a new database before sending production traffic to the service.

Using Cloud SQL Auth Proxy locally or in Cloud Shell:

```bash
INSTANCE_CONNECTION_NAME="$PROJECT_ID:$REGION:$INSTANCE"

cloud-sql-proxy "$INSTANCE_CONNECTION_NAME" --port 5432
```

In another terminal:

```bash
PGPASSWORD="replace-with-db-password" psql \
  "host=127.0.0.1 port=5432 user=$DB_USER dbname=$DB_NAME sslmode=disable" \
  -f migrations/000001_init.sql
```

If you prefer `gcloud sql connect`, connect to the database and run the same SQL file from Cloud Shell.

## Secret Manager

Use Secret Manager for sensitive values. For Cloud SQL through the Cloud Run Cloud SQL connection, use a Unix socket host in the database URL:

```bash
INSTANCE_CONNECTION_NAME="$PROJECT_ID:$REGION:$INSTANCE"
DATABASE_URL_VALUE="user=$DB_USER password=replace-with-db-password dbname=$DB_NAME host=/cloudsql/$INSTANCE_CONNECTION_NAME sslmode=disable"

printf "%s" "$DATABASE_URL_VALUE" | gcloud secrets create coast-database-url --data-file=-
openssl rand -base64 32 | gcloud secrets create coast-session-secret --data-file=-
printf "%s" "replace-with-google-client-secret" | gcloud secrets create coast-google-client-secret --data-file=-
```

Grant the Cloud Run runtime service account access to the secrets and Cloud SQL. A dedicated service account is recommended:

```bash
SERVICE_ACCOUNT="coast-monitoring-run"

gcloud iam service-accounts create "$SERVICE_ACCOUNT" \
  --display-name="Coast Monitoring Cloud Run"

RUNTIME_SA="$SERVICE_ACCOUNT@$PROJECT_ID.iam.gserviceaccount.com"

gcloud projects add-iam-policy-binding "$PROJECT_ID" \
  --member="serviceAccount:$RUNTIME_SA" \
  --role="roles/cloudsql.client"

for SECRET in coast-database-url coast-session-secret coast-google-client-secret; do
  gcloud secrets add-iam-policy-binding "$SECRET" \
    --member="serviceAccount:$RUNTIME_SA" \
    --role="roles/secretmanager.secretAccessor"
done
```

## Google OAuth Setup

Create an OAuth web client in Google Cloud Console.

Authorized redirect URI:

```text
https://SERVICE_URL/api/auth/google/callback
```

After the first deploy, update `GOOGLE_REDIRECT_URL` to the final Cloud Run or custom-domain URL. If you use a custom domain, update the OAuth client and redeploy the service with the custom-domain callback URL.

## Deploy From Source

This path does not require a local Docker daemon. Cloud Build builds the repo using the checked-in `Dockerfile`, and `gcloud` respects `.gcloudignore`.

Prepare non-secret environment variables:

```bash
cp deploy/cloud-run.env.yaml.example /tmp/coast-cloud-run.env.yaml
```

Edit `/tmp/coast-cloud-run.env.yaml` and replace OAuth client ID, bootstrap email, and allowed origins. Keep secrets out of this file.

If you already know the final URL, such as a custom domain, replace `SERVICE_URL` with that URL before the first deploy. If you are using the generated `run.app` URL, deploy once with temporary `SERVICE_URL` values, read the generated URL, update the env file and Google OAuth redirect URI, then update the Cloud Run service before trying to sign in.

```bash
SERVICE="coast-monitoring"
REGION="asia-east1"
INSTANCE_CONNECTION_NAME="$PROJECT_ID:$REGION:$INSTANCE"

gcloud run deploy "$SERVICE" \
  --source . \
  --region "$REGION" \
  --allow-unauthenticated \
  --service-account "$RUNTIME_SA" \
  --add-cloudsql-instances "$INSTANCE_CONNECTION_NAME" \
  --min-instances 0 \
  --max-instances 2 \
  --concurrency 20 \
  --env-vars-file /tmp/coast-cloud-run.env.yaml \
  --update-secrets "DATABASE_URL=coast-database-url:latest,SESSION_SECRET=coast-session-secret:latest,GOOGLE_CLIENT_SECRET=coast-google-client-secret:latest"
```

Notes:

- Leave `HTTP_ADDR` unset on Cloud Run so the service listens on the injected `PORT`.
- Keep `SECURE_COOKIES=true` in production; it is the default.
- `--allow-unauthenticated` is required because the application implements its own session auth. Admin/app APIs still require application login.
- `ADMIN_ALLOWED_ORIGINS` and `APP_ALLOWED_ORIGINS` are only needed for browser requests from a different origin. Same-origin requests do not need CORS.

After deploy, get the service URL:

```bash
SERVICE_URL="$(gcloud run services describe "$SERVICE" --region "$REGION" --format='value(status.url)')"
echo "$SERVICE_URL"
```

If you used temporary `SERVICE_URL` values, update `/tmp/coast-cloud-run.env.yaml`:

```yaml
GOOGLE_REDIRECT_URL: "https://REAL_SERVICE_URL/api/auth/google/callback"
ADMIN_ALLOWED_ORIGINS: "https://REAL_SERVICE_URL"
```

Then add the same callback URL to the Google OAuth client and update Cloud Run:

```bash
gcloud run services update "$SERVICE" \
  --region "$REGION" \
  --env-vars-file /tmp/coast-cloud-run.env.yaml
```

Do the first admin bootstrap only after this OAuth callback URL is correct.

## Deploy With Cloud Build

`cloudbuild.yaml` runs the same deployment as a repeatable build:

1. Run `go test -count=1 ./...`.
2. Build the Docker image.
3. Push both `SHORT_SHA` and `latest` tags to Artifact Registry.
4. Deploy the `SHORT_SHA` image to Cloud Run.

Manual submit:

```bash
gcloud builds submit \
  --region "$REGION" \
  --config cloudbuild.yaml \
  --substitutions "_REGION=$REGION,_SERVICE_NAME=coast-monitoring,_ARTIFACT_REPOSITORY=$AR_REPOSITORY,_IMAGE_NAME=coast-monitoring,_RUNTIME_SERVICE_ACCOUNT=$RUNTIME_SA,_CLOUDSQL_INSTANCE=$INSTANCE_CONNECTION_NAME,_GOOGLE_CLIENT_ID=replace-with-client-id,_GOOGLE_REDIRECT_URL=https://REAL_SERVICE_URL/api/auth/google/callback,_BOOTSTRAP_ADMIN_EMAIL=admin@example.com,_ADMIN_ALLOWED_ORIGINS=https://REAL_SERVICE_URL,_APP_ALLOWED_ORIGINS=https://app.example.com"
```

Secret substitutions default to:

```text
_DATABASE_URL_SECRET=coast-database-url:latest
_SESSION_SECRET_SECRET=coast-session-secret:latest
_GOOGLE_CLIENT_SECRET_SECRET=coast-google-client-secret:latest
```

Override them in the same `--substitutions` list if you use different Secret Manager names.

For continuous deployment, create a Cloud Build trigger that points at `cloudbuild.yaml` on the `main` branch. Configure trigger substitutions with the same values as the manual submit command. The trigger region should match the Cloud Run deployment region.

Cloud Build's service account needs enough permissions to build, push, and deploy:

- `roles/artifactregistry.writer` for the Artifact Registry repository.
- `roles/run.admin` to deploy Cloud Run.
- `roles/iam.serviceAccountUser` on the Cloud Run runtime service account.
- Permission to read source/log buckets as required by your Cloud Build setup.

The Cloud Run runtime service account still needs `roles/cloudsql.client` and `roles/secretmanager.secretAccessor` as described above.

## First Admin Bootstrap

1. Deploy with `BOOTSTRAP_ADMIN_EMAIL` set to the exact Google account email for the first admin.
2. Open `https://SERVICE_URL/`.
3. Sign in with Google using that email.
4. Confirm the account has role `admin`.
5. Remove `BOOTSTRAP_ADMIN_EMAIL` and redeploy:

```bash
gcloud run services update "$SERVICE" \
  --region "$REGION" \
  --remove-env-vars BOOTSTRAP_ADMIN_EMAIL
```

Do not keep bootstrap enabled long-term.

## App-Facing Frontend Origin

The app-facing frontend can call `/api/app/*` from another origin only when:

- `APP_ALLOWED_ORIGINS` includes the exact browser origin, for example `https://app.example.com`.
- Browser requests use `credentials: "include"`.
- Protected requests send the current CSRF token in the `X-CSRF-Token` header.

Cookies use `SameSite=Lax`. For browser-based cross-origin calls, host the frontend on the same registrable site as the API, for example `https://app.example.com` and `https://api.example.com`, or proxy API calls through the frontend server. A completely different site such as `https://example-app.netlify.app` calling `https://SERVICE.run.app` directly will not reliably carry the session cookie in browser fetch requests.

## Post-Deploy Smoke Test

```bash
curl -i "$SERVICE_URL/healthz"
curl -i "$SERVICE_URL/api/session"
```

Expected:

- `/healthz` returns `200` with `{"status":"ok"}`.
- `/api/session` returns `200` with `{"authenticated":false}` before login.

## Scaling Guidance

Start with:

- min instances: `0`
- max instances: `2`
- concurrency: `20`

Increase these only after reviewing Cloud SQL connection usage. The service opens a pgx pool per Cloud Run instance, so Cloud Run instance count and concurrency should be scaled with database limits in mind.

## Rollback

List revisions:

```bash
gcloud run revisions list --service "$SERVICE" --region "$REGION"
```

Route traffic back to a known-good revision:

```bash
gcloud run services update-traffic "$SERVICE" \
  --region "$REGION" \
  --to-revisions REVISION_NAME=100
```
