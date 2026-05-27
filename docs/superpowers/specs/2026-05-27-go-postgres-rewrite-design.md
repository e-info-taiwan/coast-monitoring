# Go/PostgreSQL Rewrite Design

## Goal

Replace the PocketBase/SQLite backend with a small Go service backed by PostgreSQL, while preserving the existing admin UI design and user workflows already built in this repository.

## Deployment Shape

The service will be a single Cloud Run service. It will serve static frontend assets and REST API routes from the same container to minimize cost and operational overhead.

Production data will live in Cloud SQL for PostgreSQL. The Go service will connect through the Cloud SQL connector or Unix socket in production and through a normal PostgreSQL URL in local development.

## Architecture

The backend will use route-level separation and shared business logic:

```text
/api/admin/*  -> admin HTTP handlers
/api/app/*    -> app-facing HTTP handlers
              -> shared services
              -> shared repositories
              -> PostgreSQL
```

The separation is part of the security model. The admin frontend can call admin routes, but the app-facing frontend cannot reach user-management data through app routes. Shared services keep validation and business rules from being duplicated.

## Frontend Strategy

The current `pb_public` UI work is preserved. During the rewrite it will move into a frontend asset directory, then its API client will be changed from PocketBase collection URLs to first-party REST endpoints.

The rewrite will not keep PocketBase API compatibility. Carrying PocketBase-shaped URLs forward would preserve the wrong abstraction and make authorization harder to reason about.

## Auth And Roles

The system has exactly two roles:

- `admin`: manage users, locations, species, observations, and audit logs.
- `volunteer`: read locations/species, create observations, and view/update/delete their own observations.

There is no `superuser` role and no separate system user table. PostgreSQL `users` is the single source of truth for identity and authorization:

```text
users
- id
- email
- name
- role: admin | volunteer
- status: active | disabled
- google_sub
- password_hash
- created_at
- updated_at
```

The first admin is created through `BOOTSTRAP_ADMIN_EMAIL` when no admin exists. After bootstrap, normal user creation is done by an admin.

The service supports both Google Login and email/password login. Public self-registration is not enabled. Google Login signs in an existing active user by `google_sub` or email. The only unknown Google account allowed to create a user is the bootstrap admin email when the system has no admin.

Sessions use secure cookies. The browser does not store bearer tokens in `localStorage`.

## API Boundaries

Admin API:

```text
GET    /api/admin/users
POST   /api/admin/users
PATCH  /api/admin/users/{id}
DELETE /api/admin/users/{id}
GET    /api/admin/locations
POST   /api/admin/locations
PATCH  /api/admin/locations/{id}
DELETE /api/admin/locations/{id}
GET    /api/admin/species
POST   /api/admin/species
PATCH  /api/admin/species/{id}
DELETE /api/admin/species/{id}
GET    /api/admin/observations
PATCH  /api/admin/observations/{id}
DELETE /api/admin/observations/{id}
GET    /api/admin/audit-logs
```

App API:

```text
GET    /api/app/session
GET    /api/app/locations
GET    /api/app/species
GET    /api/app/observations
POST   /api/app/observations
PATCH  /api/app/observations/{id}
DELETE /api/app/observations/{id}
```

App API responses do not expose user-management records. If the app needs current-user information, it receives a narrow shape: `id`, `email`, `name`, and `role`.

## Security Model

Authorization is enforced server-side by middleware and policy functions. CORS and `Origin` checks are used only as browser-facing defense layers, not as the source of truth.

The service will use:

- HttpOnly secure session cookies.
- Server-side session storage with hashed session tokens.
- CSRF protection for cookie-authenticated state-changing requests.
- CORS allowlists for admin and app origins.
- Password hashing with bcrypt.
- Login throttling by email and IP.
- Field-specific response DTOs so app routes cannot accidentally serialize full user records.
- Audit logging for create/update/delete operations on managed data.

## Data Model

Core tables:

```text
users
sessions
oauth_states
locations
species
observations
audit_logs
login_attempts
```

`observations` reference `locations`, `species`, and the observing `users` record.

`audit_logs` store actor id, action, target table, target id, before/after JSON, request metadata, and timestamp.

## Repository Layout

Target layout:

```text
cmd/server/
internal/config/
internal/db/
internal/auth/
internal/policy/
internal/repository/
internal/service/
internal/http/
internal/audit/
migrations/
web/
```

PocketBase-specific folders are removed only after replacement routes and frontend behavior are working:

```text
pb_hooks/
pb_migrations/
```

The existing `pb_public` assets are moved into `web/admin/` during the rewrite.

## Testing

The implementation will use Go unit tests for auth, policy, handler boundaries, and service behavior. Repository tests use a local PostgreSQL database from Docker Compose. Route-boundary tests must verify that `/api/app/*` cannot list or expose admin user records.

## Not In Scope

This rewrite does not implement a new visual design. It preserves the existing admin UI direction and replaces the backend, auth, API client, and deployment path.

This rewrite does not include an external app frontend implementation. It provides `/api/app/*` for that frontend and can serve a future in-repo app bundle under the same Cloud Run service.
