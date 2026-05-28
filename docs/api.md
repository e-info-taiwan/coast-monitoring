# API Usage

Base URL:

```text
https://SERVICE_URL
```

Local default:

```text
http://127.0.0.1:8090
```

All API responses are JSON unless the endpoint redirects or returns `204 No Content`.

Error format:

```json
{"error":"message"}
```

## Browser Session Model

The API uses cookie sessions plus a CSRF token.

Login sets:

- `coast_session`: HTTP-only session token cookie.
- `coast_csrf`: CSRF token cookie readable by the browser.

Login and session responses also return:

```json
{
  "authenticated": true,
  "user": {
    "id": "user-uuid",
    "email": "admin@example.com",
    "name": "Admin",
    "role": "admin"
  },
  "csrfToken": "csrf-token"
}
```

For every protected `/api/admin/*` and `/api/app/*` request, include:

- browser credentials/cookies
- `X-CSRF-Token: <csrfToken>`

Example browser fetch:

```js
const session = await fetch(`${apiBase}/api/session`, {
  credentials: "include",
}).then((res) => res.json())

const locations = await fetch(`${apiBase}/api/app/locations`, {
  credentials: "include",
  headers: {
    "X-CSRF-Token": session.csrfToken,
  },
}).then((res) => res.json())
```

For cross-origin browser calls, the origin must be listed in `ADMIN_ALLOWED_ORIGINS` or `APP_ALLOWED_ORIGINS`, and requests must use `credentials: "include"`. Auth/session routes use the union of both allowed-origin lists because both the admin UI and app-facing frontend need login/session access.

## Public And Auth Endpoints

### Health Check

```http
GET /healthz
```

Response:

```json
{"status":"ok"}
```

### Session Lookup

```http
GET /api/session
```

Returns `authenticated: false` when no valid session is present.

### Email / Password Login

```http
POST /api/auth/password
Content-Type: application/json

{
  "email": "user@example.com",
  "password": "password"
}
```

Success: `200` with `SessionResponse` and session cookies.

Common errors:

- `400`: missing email/password or invalid JSON.
- `401`: invalid credentials or inactive user.
- `413`: request body too large.
- `429`: too many failed attempts.

### Google Login

Start:

```http
GET /api/auth/google/start?redirect=/
```

The server redirects to Google. Google redirects back to:

```http
GET /api/auth/google/callback?state=...&code=...
```

On success, the server creates a session and redirects to the stored safe redirect path.

### Logout

```http
POST /api/auth/logout
```

Response:

```json
{"authenticated":false}
```

## Roles

- `admin`: can use the admin UI and `/api/admin/*`.
- `volunteer`: can use `/api/app/*`.

Disabled users cannot log in.

## Admin UI API

The checked-in admin UI is served by the same Cloud Run service at `/`. It uses `/api/admin/*` and same-origin cookies.

All admin routes require:

- active session
- `admin` role
- `X-CSRF-Token` header

### Admin Users

```http
GET /api/admin/users
```

Response:

```json
[
  {
    "id": "user-uuid",
    "email": "admin@example.com",
    "name": "Admin",
    "role": "admin",
    "status": "active",
    "hasGoogle": true,
    "hasPassword": false,
    "createdAt": "2026-05-28T00:00:00Z",
    "updatedAt": "2026-05-28T00:00:00Z"
  }
]
```

Create:

```http
POST /api/admin/users
Content-Type: application/json

{
  "email": "volunteer@example.com",
  "name": "Volunteer",
  "role": "volunteer",
  "status": "active",
  "password": "initial-password"
}
```

Update:

```http
PATCH /api/admin/users/{id}
Content-Type: application/json

{
  "name": "Updated Name",
  "role": "volunteer",
  "status": "active",
  "password": "new-password"
}
```

Disable:

```http
DELETE /api/admin/users/{id}
```

Returns `204 No Content`.

### Admin Locations

```http
GET /api/admin/locations
POST /api/admin/locations
PATCH /api/admin/locations/{id}
DELETE /api/admin/locations/{id}
```

Create/update body:

```json
{
  "chineseName": "鼻頭角",
  "englishName": "Bitou Cape"
}
```

Response:

```json
{
  "id": "location-uuid",
  "chineseName": "鼻頭角",
  "englishName": "Bitou Cape",
  "createdAt": "2026-05-28T00:00:00Z",
  "updatedAt": "2026-05-28T00:00:00Z"
}
```

### Admin Species

```http
GET /api/admin/species
POST /api/admin/species
PATCH /api/admin/species/{id}
DELETE /api/admin/species/{id}
```

Create/update body:

```json
{
  "chineseName": "石蓴",
  "englishName": "Ulva"
}
```

Response schema is the same as location.

### Admin Observations

```http
GET /api/admin/observations
PATCH /api/admin/observations/{id}
DELETE /api/admin/observations/{id}
```

Update body:

```json
{
  "observedOn": "2026-05-28",
  "locationId": "location-uuid",
  "speciesId": "species-uuid",
  "observerId": "user-uuid",
  "count": 3,
  "notes": "Near tide pool"
}
```

Response:

```json
{
  "id": "observation-uuid",
  "observedOn": "2026-05-28",
  "locationId": "location-uuid",
  "speciesId": "species-uuid",
  "observerId": "user-uuid",
  "count": 3,
  "notes": "Near tide pool",
  "createdAt": "2026-05-28T00:00:00Z",
  "updatedAt": "2026-05-28T00:00:00Z"
}
```

Admin observation creation is intentionally not exposed as an admin route. Use the app observation create endpoint or create records directly through a controlled back-office workflow if needed later.

### Admin Audit Logs

```http
GET /api/admin/audit-logs
```

Response:

```json
[
  {
    "id": "audit-log-uuid",
    "action": "create",
    "targetTable": "observations",
    "targetId": "observation-uuid",
    "actorUserId": "user-uuid",
    "actorEmail": "admin@example.com",
    "beforeData": null,
    "afterData": {},
    "method": "POST",
    "path": "/api/app/observations",
    "ip": "203.0.113.10",
    "userAgent": "Mozilla/5.0",
    "loggedAt": "2026-05-28T00:00:00Z"
  }
]
```

Password hashes and Google subjects are stripped from audit JSON responses.

## App-Facing API

The app-facing API is for the separate frontend / FE server. It does not expose user management data.

All `/api/app/*` routes require:

- active session
- `admin` or `volunteer` role
- `X-CSRF-Token` header

Volunteers only see and mutate their own observations. Admins can see and mutate all observations through the app API.

### App Session

```http
GET /api/app/session
```

Response is the same `SessionResponse` shape.

### App Catalog

```http
GET /api/app/locations
GET /api/app/species
```

Response schema is the same catalog object used by admin locations/species.

### App Observations

List:

```http
GET /api/app/observations
```

Create:

```http
POST /api/app/observations
Content-Type: application/json

{
  "observedOn": "2026-05-28",
  "locationId": "location-uuid",
  "speciesId": "species-uuid",
  "count": 3,
  "notes": "Near tide pool"
}
```

Update:

```http
PATCH /api/app/observations/{id}
Content-Type: application/json

{
  "observedOn": "2026-05-28",
  "locationId": "location-uuid",
  "speciesId": "species-uuid",
  "count": 4,
  "notes": "Updated note"
}
```

Delete:

```http
DELETE /api/app/observations/{id}
```

App observation response intentionally omits `observerId`:

```json
{
  "id": "observation-uuid",
  "observedOn": "2026-05-28",
  "locationId": "location-uuid",
  "speciesId": "species-uuid",
  "count": 3,
  "notes": "Near tide pool",
  "createdAt": "2026-05-28T00:00:00Z",
  "updatedAt": "2026-05-28T00:00:00Z"
}
```

The server always uses the authenticated user as the observer for app create/update requests. Clients must not send `observerId` to `/api/app/*`.

## FE Server Integration Pattern

Recommended same-site browser flow:

1. User logs in through the API service using Google or password login.
2. FE server renders the frontend on the same registrable site as the API.
3. Browser calls `/api/session` with credentials to read `csrfToken`.
4. Browser calls `/api/app/*` with credentials and `X-CSRF-Token`.

Server-side proxy flow:

1. Browser sends requests to the FE server.
2. FE server forwards cookies and `X-CSRF-Token` to the Cloud Run API.
3. FE server returns only app-safe response fields to the browser.

Do not expose `/api/admin/*` through the app-facing FE server.

Example browser helper:

```js
export async function apiFetch(path, options = {}) {
  const session = await fetch(`${API_BASE}/api/session`, {
    credentials: "include",
  }).then((res) => res.json())

  const res = await fetch(`${API_BASE}${path}`, {
    ...options,
    credentials: "include",
    headers: {
      "Content-Type": "application/json",
      "X-CSRF-Token": session.csrfToken,
      ...options.headers,
    },
  })

  if (!res.ok) {
    const error = await res.json().catch(() => ({ error: res.statusText }))
    throw new Error(error.error || res.statusText)
  }
  if (res.status === 204) {
    return null
  }
  return res.json()
}
```

## Status Codes

Common status codes:

- `200`: success.
- `201`: created.
- `204`: delete/logout-style empty success.
- `400`: invalid JSON, invalid UUID/date, or validation error.
- `401`: missing/invalid session or invalid login.
- `403`: role or object scope is not allowed.
- `404`: record not found or unknown `/api/*` route.
- `409`: uniqueness conflict.
- `413`: JSON request body too large.
- `429`: too many password login attempts.
- `500`: unexpected server error.
- `503`: handler dependency is not configured.
