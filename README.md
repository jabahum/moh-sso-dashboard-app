# MOH SSO Dashboard

MOH SSO Dashboard is the Integrated Health Portal for Ministry of Health Uganda. It provides a central authenticated portal for system launch, user and client administration, dynamic RBAC governance, announcements, email delivery, audit logs, document uploads, data quality workflows, surveillance dashboards, reports, and shared operational tooling.

The project is now organized as a feature-first backend and a monorepo-style microfrontend-ready frontend.

The system registry explicitly supports portal-owned modules and external launcher-only applications. Keycloak clients opt in with `portal.system=true`; registry metadata controls launcher visibility, side-navigation visibility, and internal, new-tab, or same-tab launch behavior. See [System onboarding](docs/system-onboarding.md).

## Contents

- [Platform Overview](#platform-overview)
- [Repository Layout](#repository-layout)
- [Frontend Documentation](#frontend-documentation)
- [Backend Documentation](#backend-documentation)
- [RBAC And Keycloak](#rbac-and-keycloak)
- [Configuration](#configuration)
- [Local Development](#local-development)
- [Docker And Production Deployment](#docker-and-production-deployment)
- [API Reference](#api-reference)
- [Documentation Map](#documentation-map)
- [Verification](#verification)
- [Troubleshooting](#troubleshooting)

## Platform Overview

The portal has four major responsibilities:

1. Central sign-on through Keycloak.
2. System-aware authorization where Keycloak clients are represented as portal systems.
3. Modular product experiences delivered as frontend apps.
4. Backend APIs grouped by feature modules with clear ownership of handlers, routes, services, and repositories.

Core runtime components:

| Component | Responsibility |
| --- | --- |
| Frontend shell | Hosts layouts, routing, auth bootstrap, app launcher, import maps, and single-spa orchestration. |
| Frontend apps | Product modules such as users, clients, RBAC, surveillance, documents, audit, email, and data validation. |
| Frontend packages | Shared UI, API hooks, auth helpers, runtime config, state, types, utilities, and microfrontend contracts. |
| Go backend | Gin API server, feature modules, Keycloak integration, RBAC sync, notifications, email, audit, documents, and surveillance APIs. |
| Keycloak | Identity source of truth for users, credentials, realm roles, clients, and client roles. |
| PostgreSQL | Portal data, RBAC metadata, documents metadata, announcements, audit logs, notifications, email queue, and feature data. |
| Redis | Runtime cache/session/rate-limit support where configured. |

## Repository Layout

```text
.
|-- backend/
|   |-- cmd/server/                 # Go server entrypoint
|   |-- internal/
|   |   |-- api/                    # Global router shell and shared handlers
|   |   |-- bootstrap/              # Composition root
|   |   |-- authz/                  # Permission constants and authorization model
|   |   |-- features/               # Feature-first backend modules
|   |   |-- middleware/             # Auth, audit, request, and RBAC middleware
|   |   |-- repository/             # Shared/cross-cutting repository adapters
|   |   |-- service/                # Shared/cross-cutting services
|   |   `-- db/migrations/          # SQL migrations
|   |-- app.env.example
|   `-- Dockerfile
|-- frontend/
|   |-- apps/                       # Shell and standalone frontend apps
|   |-- packages/                   # Shared frontend packages
|   |-- public/                     # Runtime config and import map
|   |-- scripts/                    # Build, import-map, doctor, and release scripts
|   |-- Dockerfile
|   `-- package.json
|-- keycloak/
|   `-- realm-export.json           # Local/default realm export
|-- docker-compose.yml
|-- docker-compose.dev.yml
`-- README.md
```

## Frontend Documentation

### Frontend Architecture

The frontend is a workspace under `frontend/`. It supports local monorepo development, npm-published app/package artifacts, and production import-map deployment.

```text
frontend/
|-- apps/
|   |-- shell/
|   |-- announcements/
|   |-- audit/
|   |-- clients/
|   |-- data-validation/
|   |-- data-visualizer/
|   |-- documents/
|   |-- e-services/
|   |-- email/
|   |-- issue-tracker/
|   |-- rbac/
|   |-- report-browser/
|   |-- surveillance/
|   |-- users/
|   `-- utilities/
`-- packages/
    |-- api/
    |-- auth/
    |-- config/
    |-- microfrontend/
    |-- state/
    |-- types/
    |-- ui/
    `-- utils/
```

### Frontend Apps

| App | Package | Purpose |
| --- | --- | --- |
| Shell | `@moh-sso/shell` | Portal host, global routes, layouts, providers, auth bootstrap, and microfrontend orchestration. |
| Announcements | `@moh-sso/announcements` | Admin announcement management and public/news feed announcement UI integration. |
| Audit | `@moh-sso/audit` | Audit log search, filtering, metrics, export, and detail views. |
| Clients | `@moh-sso/clients` | Keycloak client/system management and client role administration. |
| Data Validation | `@moh-sso/data-validation` | Data quality validation rules and rule management UI. |
| Data Visualizer | `@moh-sso/data-visualizer` | DWH datasets, data elements, visualizer flows, and report-adjacent tooling. |
| Documents | `@moh-sso/documents` | Document upload, metadata, process history, download, and reprocess flows. |
| E-services | `@moh-sso/e-services` | E-service entry surfaces. |
| Email | `@moh-sso/email` | Email queue, send, retry, status, and delivery management. |
| Issue Tracker | `@moh-sso/issue-tracker` | Data quality issue tracking flows. |
| RBAC | `@moh-sso/rbac` | System-aware RBAC governance, sync, drift, role, permission, and access management. |
| Report Browser | `@moh-sso/report-browser` | Report browsing module. |
| Surveillance | `@moh-sso/surveillance` | Surveillance dashboard, outbreak metrics, imports, locations, and status views. |
| Users | `@moh-sso/users` | User administration, profile editing, enable/disable, and access assignment. |
| Utilities | `@moh-sso/utilities` | Utility/self-service surfaces. |

Each app owns a local README, a public package entrypoint, and single-spa lifecycle exports.

Expected app entrypoints:

```text
frontend/apps/<app>/src/index.ts
frontend/apps/<app>/src/root.component.tsx
frontend/apps/<app>/src/routes.tsx
frontend/apps/<app>/src/single-spa.tsx
frontend/apps/<app>/vite.config.ts
```

### Frontend Packages

| Package | Purpose |
| --- | --- |
| `@moh-sso/api` | RTK Query base API and feature API hooks. |
| `@moh-sso/auth` | Auth bootstrap, session helpers, route guards, and auth types. |
| `@moh-sso/config` | Runtime config helpers and frontend constants. |
| `@moh-sso/microfrontend` | Runtime props, lifecycle types, React lifecycle helper, and route contracts. |
| `@moh-sso/state` | Shared Redux store and global state slices. |
| `@moh-sso/types` | Shared TypeScript domain types. |
| `@moh-sso/ui` | MOH theme, shared layout primitives, panels, tables, actions, forms, and reusable UI. |
| `@moh-sso/utils` | Shared utilities that are not feature-specific. |

### Microfrontend Model

The frontend supports two runtime modes:

| Mode | Config | Purpose |
| --- | --- | --- |
| Local/hybrid | `microfrontendMode: "local"`, `microfrontendMountMode: "hybrid"` | Fast local development using workspace imports and local lifecycle wrappers. |
| Remote/orchestrated | `microfrontendMode: "remote"`, `microfrontendMountMode: "orchestrated"` | Production-style single-spa orchestration with import maps and independently staged bundles. |

Production config lives in `frontend/public/config.production.js`:

```js
window.__APP_CONFIG__ = {
  API_BASE_URL: "/api",
  microfrontendMode: "remote",
  singleSpaOrchestration: true,
  microfrontendMountMode: "orchestrated",
};
```

Development config lives in `frontend/public/config.development.js`:

```js
window.__APP_CONFIG__ = {
  API_BASE_URL: "http://localhost:9000",
  microfrontendMode: "local",
  singleSpaOrchestration: false,
  microfrontendMountMode: "hybrid",
};
```

Switch runtime config with:

```bash
cd frontend
npm run config:dev
npm run config:prod
npm run config:local-remote
```

### Frontend Build Scripts

Run these from `frontend/`.

| Script | Purpose |
| --- | --- |
| `npm run dev` | Development shell with local config. |
| `npm run dev:shell` | Shell-only development. |
| `npm run dev:remote:all` | Local remote microfrontend development. |
| `npm run build:shell` | Build the shell app. |
| `npm run build:packages` | Build shared packages. |
| `npm run build:apps` | Build all standalone app bundles. |
| `npm run generate:import-map` | Generate import map entries for apps/packages. |
| `npm run build:docker` | Build packages, app bundles, shell, versions, and stage local app bundles. |
| `npm run build:docker:npm-modules` | Build shell and stage apps/packages from npm package artifacts. |
| `npm run audit:import-map` | Validate import map consistency. |
| `npm run audit:publishability` | Check npm publish readiness for apps/packages. |
| `npm run release:check` | Full release readiness check. |

### Helm Chart Checks

Run these from the repository root to validate chart changes:

```bash
make helm-lint
make helm-check
make helm-check-prod BACKEND_TAG=1.2.3 FRONTEND_TAG=1.2.3
make helm-template-prod BACKEND_TAG=1.2.3 FRONTEND_TAG=1.2.3
```

These commands keep Helm changes easy to track locally and mirror the checks used in CI.

### Local M1 Helm Smoke Test

On Apple Silicon, you can validate the chart and local stack in the same order you would during a real deployment:

```bash
make helm-lint
make helm-check
make helm-check-prod BACKEND_TAG=1.2.3 FRONTEND_TAG=1.2.3
make local-build
make local-up
make local-forward
curl -fsS http://localhost:9000/version
curl -fsS http://localhost:9000/health/live
```

If Docker Buildx needs a refresh on your machine, bootstrap it first:
****
```bash
docker buildx inspect --bootstrap
```

This path is useful for confirming that:

- Helm templates render correctly
- immutable prod tags are required
- arm64 local images build on M1
- the full local stack starts cleanly
- backend runtime metadata is reachable after deployment

## Backend Documentation

Backend-specific development, architecture, and operations notes live in:

- [`backend/README.md`](backend/README.md)
- [`docs/api-reference.md`](docs/api-reference.md)
- [`backend/docs/openapi.yaml`](backend/docs/openapi.yaml)

### Backend Architecture

The backend follows a feature-first modular monolith structure. It is designed to stay deployable as one service now, while keeping boundaries clear enough for future service extraction.

```text
backend/internal/
|-- bootstrap/
|   |-- app.go
|   |-- infrastructure.go
|   |-- repositories.go
|   |-- services.go
|   |-- handlers.go
|   |-- workers.go
|   `-- server.go
|-- api/
|   |-- router.go
|   |-- handler/health_handler.go
|   `-- routes/
|-- authz/
|-- features/
|   |-- announcements/
|   |-- audit/
|   |-- clients/
|   |-- data_quality/
|   |-- document_templates/
|   |-- documents/
|   |-- email/
|   |-- rbac/
|   |-- sessions/
|   |-- storage_locations/
|   |-- surveillance/
|   |-- system_rbac/
|   `-- users/
|-- middleware/
|-- repository/
`-- service/
```

Feature modules generally use this shape:

```text
backend/internal/features/<feature>/
|-- handler.go
|-- routes.go
|-- dto.go
|-- mapper.go
|-- service.go
|-- repository.go
`-- repository_postgres.go or repository_keycloak.go
```

Not every feature has every file. Smaller HTTP adapters may only have handlers and routes. Cross-cutting services remain in `backend/internal/service`.

### Backend Composition

`backend/internal/bootstrap` is the composition root. It constructs:

- Config
- Database connections
- Keycloak client
- Redis/rate limiter
- Repositories
- Services
- Handlers
- Workers
- HTTP router
- Startup RBAC sync

`backend/internal/api/router.go` is the global HTTP shell. It mounts:

- Unauthenticated auth routes under `/api/v1/auth`
- Public announcement routes under `/api/v1/announcements`
- Protected routes under `/api/v1`
- Admin routes under `/api/v1/admin`
- Health routes outside the API prefix under `/health`, `/health/live`, and `/health/ready`

### Backend Feature Modules

| Feature | Backend folder | Notes |
| --- | --- | --- |
| Admin units | `features/admin_units` | Visualizer administrative unit lookup endpoints. |
| Announcements | `features/announcements` | Announcement lifecycle, targeting, email triggers, and persistent attachments. |
| Audit | `features/audit` | Audit log search, export, and audit metrics. |
| Auth | `features/auth` | Keycloak login, callback, session refresh, logout, and current user profile. |
| Clients | `features/clients` | Keycloak clients as portal systems and client roles. |
| Data quality | `features/data_quality` | Data quality issue and transaction APIs. |
| Document templates | `features/document_templates` | Template, sheet, column, publish, archive, and structure APIs. |
| Documents | `features/documents` | Document metadata, upload, view, download, process history, and reprocess. |
| Email | `features/email` | Email send, queue, retry, listing, and delete. |
| GeoJSON | `features/geojson` | GeoJSON assets for maps. |
| Metrics | `features/metrics` | Admin metrics for users, clients, logins, and security. |
| Notifications | `features/notifications` | Notifications, unread counts, read state, delivery history, retry, cleanup, and deletion. |
| RBAC | `features/rbac` | System-aware RBAC, Keycloak sync, drift, permissions, roles, and user access. |
| Sessions | `features/sessions` | Current user sessions and logout. |
| Storage locations | `features/storage_locations` | Storage configuration records. |
| Surveillance | `features/surveillance` | Epi weeks, alerts, diseases, locations, metrics, weekly statuses, and import batches. |
| Users | `features/users` | Keycloak user administration and user client role management. |
| Visualiser | `features/visualiser` | Dataset, data element, data value, theme, and HIV summary endpoints. |

## RBAC And Keycloak

Keycloak remains the identity source of truth:

- Users and credentials live in Keycloak.
- Realm roles live in Keycloak.
- Clients in Keycloak are systems in the portal RBAC model.
- Client roles in Keycloak are system roles in the portal RBAC model.
- Portal permissions live in the portal DB and are mapped to realm roles and system roles.

The portal DB stores RBAC metadata that Keycloak does not model directly:

- System display names, categories, icons, launch URLs, and visibility metadata.
- Permission catalog entries.
- Role-to-permission mappings.
- Realm-role-to-permission mappings.
- Access role metadata.
- Sync drift reports and audit events.
- User access profiles derived from Keycloak role assignments plus portal permission mappings.

### Startup RBAC Sync

Startup sync is configured through backend env vars:

| Variable | Purpose |
| --- | --- |
| `RBAC_STARTUP_SYNC_ENABLED` | Enables or disables startup sync. |
| `RBAC_STARTUP_SEED_ENABLED` | Applies the portal RBAC seed file. |
| `RBAC_STARTUP_SEED_PATH` | Path to seed file, usually `config/system-rbac.seed.yaml`. |
| `RBAC_STARTUP_SYNC_REALM_EXPORT` | Imports local realm-export metadata if available. |
| `RBAC_STARTUP_SYNC_REALM_EXPORT_PATH` | Optional explicit realm export path. |
| `RBAC_STARTUP_SYNC_LIVE_KEYCLOAK` | Pulls live Keycloak clients, roles, and users. |
| `RBAC_STARTUP_SYNC_USERS` | Syncs user access profiles from Keycloak. |
| `RBAC_STARTUP_SYNC_PUSH_TO_KEYCLOAK` | Creates missing portal-defined Keycloak clients and realm/client roles from the RBAC registry. |
| `RBAC_STARTUP_SYNC_FAIL_ON_ERROR` | Fails backend startup if RBAC sync fails. Useful in stricter environments. |

Recommended development settings:

```env
RBAC_STARTUP_SYNC_ENABLED=true
RBAC_STARTUP_SEED_ENABLED=true
RBAC_STARTUP_SYNC_LIVE_KEYCLOAK=true
RBAC_STARTUP_SYNC_USERS=true
RBAC_STARTUP_SYNC_PUSH_TO_KEYCLOAK=true
RBAC_STARTUP_SYNC_FAIL_ON_ERROR=false
```

Recommended production posture:

- Keep Keycloak as the source for user role assignments.
- Keep portal DB as the source for permission metadata and launch metadata.
- Use RBAC drift tools before applying changes.
- Enable `RBAC_STARTUP_SYNC_FAIL_ON_ERROR=true` only when operational readiness requires sync failures to block startup.
- Keep `RBAC_STARTUP_SYNC_PUSH_TO_KEYCLOAK=true` when the portal should create missing registry clients and roles in Keycloak during startup. User access is still granted only after assigning the relevant client roles, such as `data-statistics_access`.

## Configuration

### Backend Environment

Start from `backend/app.env.example`.

Required groups:

| Group | Key variables |
| --- | --- |
| Global | `ENVIRONMENT`, `GIN_MODE` |
| Frontend | `FRONTEND_BASE_URL`, `FRONTEND_REDIRECT_URI`, `API_BASE_URL`, `COOKIE_DOMAIN` |
| Keycloak | `KEYCLOAK_EXTERNAL_URL`, `KEYCLOAK_BASE_URL`, `KEYCLOAK_REALM`, `KEYCLOAK_ADMIN_CLIENT_ID`, `KEYCLOAK_ADMIN_CLIENT_SECRET`, `KEYCLOAK_WEB_CLIENT_ID`, `KEYCLOAK_WEB_CLIENT_SECRET`, `KEYCLOAK_REDIRECT_URI` |
| Backend | `SERVER_PORT`, `APP_BASE_URL` |
| RBAC sync | `RBAC_STARTUP_*` |
| Database | `DB_DRIVER`, `DB_HOST`, `DB_PORT`, `DB_NAME`, `DB_USER`, `DB_PASSWORD`, `DB_ENABLE_SSL` |
| Redis | `REDIS_HOST`, `REDIS_PORT`, `REDIS_PASSWORD` |
| JWT/session | `TOKEN_SYMMETRIC_KEY`, `ACCESS_TOKEN_DURATION`, `REFRESH_TOKEN_DURATION` |
| Storage | `STORAGE_PROVIDER`, `LOCAL_BASE_PATH`, `NFS_BASE_PATH`, `S3_*`, `MINIO_*` |
| Visualiser/DWH | `DWH_HOST`, `DWH_PORT`, `DWH_USERNAME`, `DWH_PASSWORD`, `DWH_DB` |
| Email | `SMTP_*`, `EMAIL_MAX_ATTACHMENTS`, `EMAIL_ALLOWED_ATTACHMENT_TYPES` |
| Announcements | `ANNOUNCEMENT_MAX_ATTACHMENTS`, `ANNOUNCEMENT_ALLOWED_ATTACHMENT_TYPES` |
| Notifications | `PLATFORM_NAME`, `SYSTEM_ADMIN_EMAIL`, `SYSTEM_ADMIN_NAME`, `ADMIN_DASHBOARD_URL` |

Important local cookie note:

```env
COOKIE_DOMAIN=
```

Do not use `COOKIE_DOMAIN=localhost` for browser development. Browsers can reject or ignore cookies with `Domain=localhost`.

### Frontend Runtime Config

Runtime config is copied into `frontend/public/config.js`:

```bash
cd frontend
npm run config:dev
npm run config:prod
npm run config:local-remote
```

The frontend reads `window.__APP_CONFIG__` at runtime. This allows the same source to support local hybrid development and production import-map orchestration.

## Local Development

### Prerequisites

- Go matching `backend/go.mod`
- Docker and Docker Compose
- Node managed with nvm
- Node `v20.20.1` recommended for the frontend
- npm
- PostgreSQL/Redis/Keycloak through Docker, unless using external services

### Frontend

```bash
cd frontend
source ~/.nvm/nvm.sh
nvm use v20.20.1
npm install
npm run config:dev
npm run dev
```

The local shell is served under `/portal`.

### Backend

```bash
cd backend
cp app.env.example app.env
GOCACHE=/private/tmp/moh-sso-go-build go test ./...
go run ./cmd/server
```

When running with Docker Compose, backend uses service hostnames such as `backend-db`, `redis`, and `keycloak`.

### Docker Development

```bash
docker compose -f docker-compose.dev.yml up --build
```

Use the development compose file when you want local containers for backend dependencies and development-friendly ports.

## Docker And Production Deployment

### Backend Image

`backend/Dockerfile` builds a static Go binary and runs it in Alpine:

```bash
docker build -t moh-sso-dashboard-backend ./backend
```

Production backend expects `app.env` or equivalent environment variables.

### Frontend Image

`frontend/Dockerfile` builds the production shell, packages, microfrontend bundles, version manifest, and staged assets.

```bash
docker build -t moh-sso-dashboard-frontend ./frontend
```

The production Docker build uses:

```text
FRONTEND_ASSET_BASE_URL=/portal
FRONTEND_BASE_PATH=/portal
```

and runs:

```bash
node scripts/use-runtime-config.mjs production
npm run build:docker
```

### Source-Based Frontend Deployment

Use this when building all apps and packages from local source:

```bash
cd frontend
source ~/.nvm/nvm.sh
nvm use v20.20.1
npm ci
npm run config:prod
npm run build:docker
```

### NPM-Package-Based Frontend Deployment

Use this when apps/packages have been published and deployment should stage app bundles from npm package artifacts:

```bash
cd frontend
source ~/.nvm/nvm.sh
nvm use v20.20.1
npm ci
npm run config:prod
npm run build:docker:npm-modules
```

### Production Compose

`docker-compose.yml` defines the direct-host production stack:

- `redis`
- `backend-db`
- `backend`
- `frontend`

`docker-compose-nginx.yml` adds an Nginx reverse proxy in front of the frontend and backend. Use this file when the server should expose the portal on port `80` and proxy `/api/*` to the backend.

Production compose expects:

- immutable image tags in `BACKEND_TAG` and `FRONTEND_TAG`
- runtime environment in `./app.env`
- secret files in `./secrets/*`
- persistent named volumes for Postgres, Redis, and local uploads

Prepare the server once:

```bash
cp app.env.example app.env
mkdir -p secrets
printf '%s' 'replace-db-password' > secrets/db_password.txt
printf '%s' 'replace-admin-client-secret' > secrets/keycloak_admin_client_secret.txt
printf '%s' 'replace-web-client-secret' > secrets/keycloak_web_client_secret.txt
chmod 600 app.env secrets/*.txt
```

Validate the compose file without real production values:

```bash
APP_ENV_FILE=./app.env.example BACKEND_TAG=1.2.3 FRONTEND_TAG=1.2.3 \
  docker compose -f docker-compose-nginx.yml config --quiet
```

Start production with the reverse proxy:

```bash
BACKEND_TAG=1.2.3 FRONTEND_TAG=1.2.3 \
  docker compose -f docker-compose-nginx.yml up -d
```

Or start the direct-host compose:

```bash
BACKEND_TAG=1.2.3 FRONTEND_TAG=1.2.3 \
  docker compose up -d
```

## API Reference

Base API prefix:

```text
/api/v1
```

Health routes are not under `/api/v1`.

### Response Envelope

Successful responses generally use:

```json
{
  "success": true,
  "data": {},
  "meta": {}
}
```

Error responses generally use:

```json
{
  "success": false,
  "error": {
    "code": "INVALID_INPUT",
    "message": "human readable message"
  },
  "meta": {}
}
```

### Common API Payload Notes

Date/time strings should be sent as ISO-8601/RFC3339 strings unless a feature explicitly documents another format. IDs are UUID strings unless the route name says otherwise.

#### Client Create/Update Payload

Used by `POST /api/v1/clients` and `PUT /api/v1/clients/:id`.

```json
{
  "name": "Data & Statistics",
  "description": "Data quality, reports, surveillance, and dashboards",
  "icon": "home",
  "publicClient": true,
  "enabled": true,
  "rootUrl": "https://portal.example.org/portal/apps/dwh",
  "baseUrl": "/portal/apps/dwh",
  "adminUrl": "",
  "redirectUris": ["https://portal.example.org/portal/*"],
  "webOrigins": ["https://portal.example.org"],
  "attributes": {
    "category": "platform"
  }
}
```

#### User Role Payloads

Add/remove roles for a user and client:

```json
{
  "clientId": "data-statistics",
  "clientUuid": "optional-keycloak-client-uuid",
  "roles": ["data-statistics_access", "report_admin"]
}
```

Set enabled state:

```json
{
  "enabled": true
}
```

Dynamic RBAC user access is managed through `PUT /api/v1/admin/rbac/user-access/users/:userId`:

```json
{
  "realmRoles": ["admin"],
  "clientRoles": {
    "dashboard-web": ["dashboard-web_access"],
    "data-statistics": ["data-statistics_access", "report_admin"]
  },
  "permissions": ["portal:access", "report_browser:read"]
}
```

#### Announcement Payloads

Create announcement:

```json
{
  "title": "Surveillance update",
  "message": "Weekly outbreak bulletin is available.",
  "summary": "Weekly bulletin",
  "level": "information",
  "tag": "surveillance",
  "link_url": "/portal/apps/dwh/surveillance",
  "link_label": "Open surveillance dashboard",
  "priority": 5,
  "is_pinned": true,
  "status": "draft",
  "publish_at": "2026-06-16T09:00:00Z",
  "expires_at": "2026-07-16T09:00:00Z",
  "audience_type": "client",
  "client_ids": ["outbreak-management"],
  "role_names": ["outbreak-management_access"],
  "user_ids": [],
  "notify_by_email": true
}
```

Persistent announcement attachment upload:

```json
{
  "file_name": "weekly-bulletin.pdf",
  "content_type": "application/pdf",
  "data_base64": "base64-encoded-content",
  "include_in_email": true,
  "inline": false,
  "content_id": "",
  "sort_order": 1
}
```

Publish or schedule with transient email attachments:

```json
{
  "publish_at": "2026-06-16T09:00:00Z",
  "include_attachments_in_email": true,
  "attachments": [
    {
      "file_name": "bulletin.pdf",
      "content_type": "application/pdf",
      "data_base64": "base64-encoded-content",
      "inline": false
    }
  ]
}
```

For transient email attachments, provide exactly one of `path` or `data_base64`.
Persistent announcement attachments are stored as announcement assets and exposed
through `download_url`. When an announcement is published with email delivery,
attachments marked `include_in_email` are sent with the email and also rendered
as secure download links in the announcement email template. If `link_url` is set,
the template and News & Updates UI use `link_label` as the call-to-action text,
falling back to a generic label when it is omitted.

#### Email Payload

Used by `POST /api/v1/emails/send` and `POST /api/v1/emails/queue`.

```json
{
  "to": [
    {
      "name": "Health Worker",
      "email": "worker@example.org"
    }
  ],
  "cc": [],
  "bcc": [],
  "reply_to": [],
  "subject": "MOH bulletin",
  "text_body": "Plain text body",
  "html_body": "<p>HTML body</p>",
  "template_name": "",
  "template_data": {},
  "attachments": [
    {
      "file_name": "bulletin.pdf",
      "content_type": "application/pdf",
      "data_base64": "base64-encoded-content",
      "inline": false
    }
  ],
  "headers": {},
  "metadata": {},
  "scheduled_at": "2026-06-16T09:00:00Z"
}
```

Attachment payloads must provide exactly one of `path` or `data_base64`.

#### Document Payloads

`POST /api/v1/documents` accepts document upload data as implemented by the documents handler. `PUT /api/v1/documents/:id` updates metadata:

```json
{
  "original_filename": "report.xlsx",
  "content_type": "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet"
}
```

#### Data Quality Payloads

Create issue:

```json
{
  "dataset": "HMIS",
  "data_element": "Malaria cases",
  "org_unit": "Kampala",
  "issue": "Outlier detected",
  "issue_type": "outlier",
  "reported_by": "analyst@example.org",
  "time_Period": "2026W24"
}
```

Resolve issue:

```json
{
  "stage": "resolution",
  "status": "resolved",
  "resolution_action": "Corrected source data",
  "resolved_by": "analyst@example.org",
  "resolution_date": "2026-06-16",
  "verification_status": "verified",
  "verified_by": "supervisor@example.org",
  "verification_date": "2026-06-16",
  "preventive_action": "Added validation rule",
  "process_change": "Review before submission",
  "preventive_owner": "DQA team",
  "due_date": "2026-06-30"
}
```

#### RBAC Governance Payloads

Update system metadata:

```json
{
  "clientId": "data-statistics",
  "displayName": "Data & Statistics",
  "description": "Data quality, reports, surveillance, and dashboards",
  "icon": "home",
  "launchUrl": "/portal/apps/dwh",
  "category": "platform",
  "ownerTeam": "Analytics",
  "ownerName": "System Owner",
  "ownerEmail": "owner@example.org",
  "supportUrl": "https://support.example.org",
  "documentationUrl": "https://docs.example.org",
  "environment": "production",
  "criticality": "medium",
  "enabled": true,
  "sortOrder": 20
}
```

Create or update role metadata:

```json
{
  "name": "report_admin",
  "displayName": "Report Administrator",
  "description": "Can administer report browser content",
  "enabled": true
}
```

Assign permission to a role or realm role:

```json
{
  "permissionKey": "report_browser:read"
}
```

Preview or apply RBAC sync from a realm export:

```json
{
  "source": "realm-export",
  "realmExport": {}
}
```

Simulate access:

```json
{
  "userId": "keycloak-user-id",
  "username": "admin",
  "email": "admin@example.org",
  "realmRoles": ["admin"],
  "clientRoles": {
    "dashboard-web": ["dashboard-web_access"]
  },
  "addRealmRoles": [],
  "removeRealmRoles": [],
  "addClientRoles": {},
  "removeClientRoles": {},
  "addPermissions": ["systems:launch"],
  "removePermissions": []
}
```

### Authentication And Authorization

Protected routes require a valid portal session established through Keycloak login.

Admin routes are mounted under:

```text
/api/v1/admin
```

Feature routes use permission middleware from `backend/internal/authz`. Examples:

| Permission | Typical use |
| --- | --- |
| `portal:access` | Basic authenticated portal/session access. |
| `users:read`, `users:write`, `users:roles:write` | User management. |
| `clients:read`, `clients:write`, `clients:roles:write` | Client/system management. |
| `rbac:read`, `rbac:write`, `rbac:roles:write`, `rbac:permissions:write` | RBAC governance. |
| `announcements:read`, `announcements:write`, `announcements:publish` | Announcement lifecycle. |
| `documents:read`, `documents:write`, `documents:process` | Document flows. |
| `surveillance:read`, `surveillance:import`, `surveillance:manage_locations`, `surveillance:manage_alerts` | Surveillance module. |

### Health

| Method | Path | Auth | Purpose |
| --- | --- | --- | --- |
| `GET` | `/health` | Public | General health check. |
| `GET` | `/health/live` | Public | Liveness check. |
| `GET` | `/health/ready` | Public | Readiness check. |

### Auth

| Method | Path | Auth | Purpose |
| --- | --- | --- | --- |
| `GET` | `/api/v1/auth/login` | Public, rate limited | Starts Keycloak login. |
| `GET` | `/api/v1/auth/callback` | Public, rate limited | Handles Keycloak authorization callback. |
| `GET` | `/api/v1/auth/me` | Session | Returns current user profile, roles, permissions, and accessible systems. |
| `POST` | `/api/v1/auth/refresh` | Session, rate limited | Refreshes the portal session/token. |
| `GET` | `/api/v1/auth/logout` | Session | Logs the current browser session out. |

### Public Announcements

| Method | Path | Auth | Purpose |
| --- | --- | --- | --- |
| `GET` | `/api/v1/announcements/public` | Public | Lists public announcements. |
| `GET` | `/api/v1/announcements/active` | Public | Lists active published announcements. |

### User Announcements

Requires `announcements:read`.

| Method | Path | Purpose |
| --- | --- | --- |
| `GET` | `/api/v1/announcements/me` | Lists announcements targeted to current user. |
| `GET` | `/api/v1/announcements/user` | Lists user-targeted announcements. |
| `GET` | `/api/v1/announcements/role/:role_name` | Lists announcements for a role. |
| `GET` | `/api/v1/announcements/client/:client_id` | Lists announcements for a client/system. |
| `GET` | `/api/v1/announcements/:id/attachments/:attachmentId/download` | Downloads an announcement attachment. |

### Admin Announcements

Mounted under `/api/v1/admin/announcements`.

| Method | Path | Permission | Purpose |
| --- | --- | --- | --- |
| `GET` | `/` | `announcements:read` | List announcements for administrators. |
| `GET` | `/stats` | `announcements:read` | Announcement metrics. |
| `GET` | `/:id` | `announcements:read` | Get announcement detail. |
| `POST` | `/` | `announcements:write` | Create announcement. |
| `PUT` | `/:id` | `announcements:write` | Update announcement. |
| `DELETE` | `/:id` | `announcements:write` | Soft delete announcement. |
| `POST` | `/:id/restore` | `announcements:write` | Restore deleted announcement. |
| `POST` | `/:id/publish` | `announcements:publish` | Publish immediately and trigger delivery. |
| `POST` | `/:id/draft` | `announcements:publish` | Move announcement back to draft. |
| `POST` | `/:id/schedule` | `announcements:publish` | Schedule announcement. |
| `POST` | `/:id/archive` | `announcements:publish` | Archive announcement. |
| `PATCH` | `/:id/pin` | `announcements:write` | Pin or unpin announcement. |
| `PATCH` | `/:id/priority` | `announcements:write` | Update announcement priority. |
| `GET` | `/:id/attachments` | `announcements:read` | List attachments. |
| `POST` | `/:id/attachments` | `announcements:write` | Upload attachment. |
| `PATCH` | `/:id/attachments/:attachmentId` | `announcements:write` | Update attachment metadata. |
| `DELETE` | `/:id/attachments/:attachmentId` | `announcements:write` | Delete attachment. |
| `GET` | `/:id/attachments/:attachmentId/download` | `announcements:read` | Download attachment. |

### Users

Protected user read routes:

| Method | Path | Permission | Purpose |
| --- | --- | --- | --- |
| `GET` | `/api/v1/users` | `users:read` | List users. |
| `GET` | `/api/v1/users/:id` | `users:read` | Get user. |

Admin user routes:

| Method | Path | Permission | Purpose |
| --- | --- | --- | --- |
| `GET` | `/api/v1/admin/users` | `users:read` | List users. |
| `GET` | `/api/v1/admin/users/:id` | `users:read` | Get user. |
| `POST` | `/api/v1/admin/users` | `users:write` | Create Keycloak user. |
| `PUT` | `/api/v1/admin/users/:id` | `users:write` | Update user profile. |
| `DELETE` | `/api/v1/admin/users/:id` | `users:write` | Delete user. |
| `PATCH` | `/api/v1/admin/users/:id/enabled` | `users:write` | Enable or disable user. |
| `PATCH` | `/api/v1/admin/users/:id/toggle` | `users:write` | Toggle enabled state. |
| `POST` | `/api/v1/admin/users/:id/onboarding-email` | `users:write` | Send onboarding email. |
| `POST` | `/api/v1/admin/users/:id/verification-email` | `users:write` | Send verification email. |
| `POST` | `/api/v1/admin/users/:id/password-reset` | `users:write` | Send password reset email. |
| `POST` | `/api/v1/admin/users/:id/reset-password` | `users:write` | Reset user password. |
| `GET` | `/api/v1/admin/users/:id/client-roles` | `users:read` | List user roles by client. |
| `GET` | `/api/v1/admin/users/:id/clients/:clientID/roles` | `users:read` | List roles for one client. |
| `POST` | `/api/v1/admin/users/:id/clients/:clientID/roles` | `users:roles:write` | Add client roles to user. |
| `DELETE` | `/api/v1/admin/users/:id/clients/:clientID/roles` | `users:roles:write` | Remove client roles from user. |
| `PUT` | `/api/v1/admin/users/:id/client-roles` | `users:roles:write` | Replace/update user client role assignments. |

### Clients And Systems

Clients are Keycloak clients represented as systems in portal RBAC.

| Method | Path | Permission | Purpose |
| --- | --- | --- | --- |
| `GET` | `/api/v1/clients` | `systems:read` or `clients:read` | List clients/systems. |
| `GET` | `/api/v1/clients/:id` | `clients:read` | Get client/system. |
| `POST` | `/api/v1/clients` | `clients:write` | Create client/system. |
| `PUT` | `/api/v1/clients/:id` | `clients:write` | Update client/system. |
| `PATCH` | `/api/v1/clients/:id/toggle` | `clients:write` | Enable or disable client. |
| `DELETE` | `/api/v1/clients/:id` | `clients:write` | Delete client. |
| `GET` | `/api/v1/clients/:id/roles` | `clients:read` | List client roles. |
| `POST` | `/api/v1/admin/clients/:id/roles` | `clients:roles:write` | Create client role. |
| `DELETE` | `/api/v1/admin/clients/:id/roles/:role` | `clients:roles:write` | Delete client role. |

### RBAC

Mounted under `/api/v1/admin/rbac`.

| Method | Path | Permission | Purpose |
| --- | --- | --- | --- |
| `GET` | `/systems` | `rbac:read` | List portal systems from Keycloak/DB sync. |
| `GET` | `/systems/:clientId` | `rbac:read` | Get system metadata. |
| `PUT` | `/systems/:clientId` | `rbac:write` | Update system metadata such as display name, launch URL, icon, category. |
| `GET` | `/permissions` | `rbac:read` | List permission catalog. |
| `PUT` | `/permissions/:permissionKey` | `rbac:permissions:write` | Update permission metadata. |
| `GET` | `/drift` | `rbac:read` | Get Keycloak/portal drift report. |
| `POST` | `/drift/realm-export` | `rbac:read` | Generate drift from uploaded realm export. |
| `POST` | `/sync/preview` | `rbac:read` | Preview RBAC sync changes. |
| `POST` | `/sync/apply` | `rbac:write` | Apply RBAC sync changes. |
| `GET` | `/effective-access/users/:userId` | `rbac:read` | Get effective user access. |
| `GET` | `/user-access/assignable` | `rbac:read` | List assignable realm roles, systems, roles, and permissions. |
| `GET` | `/user-access/users/:userId` | `rbac:read` | Get user access profile. |
| `PUT` | `/user-access/users/:userId` | `rbac:roles:write` | Update user realm roles, system roles, and mapped permissions. |
| `POST` | `/changes/preview` | `rbac:read` | Preview policy/role changes. |
| `GET` | `/export` | `rbac:read` | Export RBAC seed data. |
| `POST` | `/import/preview` | `rbac:read` | Preview seed/import changes. |
| `POST` | `/import/apply` | `rbac:write` | Apply seed/import changes. |
| `GET` | `/audit` | `rbac:read` or `audit:read` | List RBAC audit events. |
| `GET` | `/role-templates` | `rbac:read` | List role templates. |
| `POST` | `/bulk/assign-permission` | `rbac:permissions:write` | Bulk assign permission. |
| `POST` | `/bulk/remove-permission` | `rbac:permissions:write` | Bulk remove permission. |
| `GET` | `/access-requests` | `rbac:roles:write` | List access requests. |
| `POST` | `/access-requests/:id/:decision` | `rbac:roles:write` | Approve or reject access request. |
| `GET` | `/change-requests` | `rbac:read` | List RBAC change requests. |
| `POST` | `/change-requests` | `rbac:write` | Create RBAC change request. |
| `POST` | `/change-requests/:id/:decision` | `rbac:write` | Approve or reject change request. |
| `POST` | `/simulate` | `rbac:read` | Simulate user/system/permission access. |
| `GET` | `/systems/:clientId/roles` | `rbac:read` | List system roles. |
| `POST` | `/systems/:clientId/roles` | `rbac:roles:write` | Create system role. |
| `POST` | `/systems/:clientId/roles/from-template` | `rbac:roles:write` | Create role from template. |
| `POST` | `/systems/:clientId/access-roles` | `rbac:roles:write` | Add access role for system visibility/launch. |
| `DELETE` | `/systems/:clientId/access-roles/:roleName` | `rbac:roles:write` | Remove access role. |
| `PUT` | `/roles/:roleId` | `rbac:roles:write` | Update role. |
| `DELETE` | `/roles/:roleId` | `rbac:roles:write` | Delete role. |
| `GET` | `/roles/:roleId/usage` | `rbac:read` | Show role usage. |
| `POST` | `/roles/:roleId/copy-permissions` | `rbac:permissions:write` | Copy permissions from another role. |
| `POST` | `/roles/:roleId/permissions` | `rbac:permissions:write` | Assign permission to role. |
| `DELETE` | `/roles/:roleId/permissions/:permissionKey` | `rbac:permissions:write` | Remove permission from role. |
| `GET` | `/realm-roles` | `rbac:read` | List realm-role permission mappings. |
| `GET` | `/realm-roles/:realmRole/usage` | `rbac:read` | Show realm role usage. |
| `POST` | `/realm-roles/:realmRole/permissions` | `rbac:permissions:write` | Assign permission to realm role. |
| `DELETE` | `/realm-roles/:realmRole/permissions/:permissionKey` | `rbac:permissions:write` | Remove permission from realm role. |

Authenticated users can also create access requests:

| Method | Path | Permission | Purpose |
| --- | --- | --- | --- |
| `POST` | `/api/v1/access-requests` | `portal:access` | Request access to a system or role. |

### Audit Logs

Mounted under `/api/v1/admin/audit-logs`. Requires `audit:read`.

| Method | Path | Purpose |
| --- | --- | --- |
| `GET` | `/` | List audit logs with filters. |
| `GET` | `/actions` | List known audit action names. |
| `GET` | `/metrics/overview` | Audit overview metrics. |
| `GET` | `/metrics/failed-logins-by-day` | Failed login trend. |
| `GET` | `/metrics/top-failure-ips` | Top failed-login IPs. |
| `GET` | `/export` | Export audit logs. Rate limited. |
| `GET` | `/:id` | Get audit log detail. |

### Metrics

Mounted under `/api/v1/admin/metrics`. Requires `metrics:read`.

| Method | Path | Purpose |
| --- | --- | --- |
| `GET` | `/overview` | Dashboard overview metrics. |
| `GET` | `/system/count-users` | Count users. |
| `GET` | `/system/count-disabled-users` | Count disabled users. |
| `GET` | `/system/active-today` | Active users today. |
| `GET` | `/system/active-this-week` | Active users this week. |
| `GET` | `/system/login-trend` | Login trend. |
| `GET` | `/system/login-trend-range` | Login trend by date range. |
| `GET` | `/security/failed-logins` | Failed login count. |
| `GET` | `/security/failed-logins-range` | Failed login count by range. |
| `GET` | `/security/suspicious-logins` | Suspicious login records. |
| `GET` | `/clients/count` | Count clients. |
| `GET` | `/clients/most-accessed` | Most accessed clients. |
| `GET` | `/clients/login-count` | Login count for client. |
| `GET` | `/clients/active-today` | Active users per client today. |
| `GET` | `/users/new-range` | New users in range. |
| `GET` | `/users/new-trend` | New users trend. |
| `GET` | `/users/never-logged-in` | Users who never logged in. |
| `GET` | `/users/last-login/:userID` | Last login for user. |
| `GET` | `/users/client-usage/:userID` | Client usage for user. |

### Notifications

Mounted under `/api/v1/admin/notifications`.

| Method | Path | Permission | Purpose |
| --- | --- | --- | --- |
| `POST` | `/` | `notifications:write` | Create notification. |
| `GET` | `/` | `notifications:read` | List notifications. |
| `GET` | `/count` | `notifications:read` | Count notifications. |
| `GET` | `/count/unread` | `notifications:read` | Count unread notifications. |
| `DELETE` | `/cleanup` | `notifications:write` | Delete old notifications. |
| `GET` | `/:id/deliveries` | `notifications:read` | List delivery history and status records for a notification. |
| `POST` | `/deliveries/:deliveryID/retry` | `notifications:write` | Requeue a failed, retry, or cancelled notification delivery. |
| `GET` | `/:id` | `notifications:read` | Get notification. |
| `PATCH` | `/:id/read` | `notifications:read` | Mark notification as read. |
| `DELETE` | `/:id` | `notifications:write` | Delete notification. |

Notification delivery statuses include `PENDING`, `PROCESSING`, `SENT`, `FAILED`, `RETRY`, and `CANCELLED`.

### Email

Mounted under `/api/v1/emails`.

| Method | Path | Permission | Purpose |
| --- | --- | --- | --- |
| `POST` | `/send` | `email:send` | Send email immediately. Supports validated attachments. |
| `POST` | `/queue` | `email:send` | Queue email for worker delivery. Supports validated attachments. |
| `GET` | `/` | `email:read` | List emails. |
| `GET` | `/status/:status` | `email:read` | List emails by status. |
| `GET` | `/:id` | `email:read` | Get email detail. |
| `POST` | `/:id/retry` | `email:manage` | Retry failed email. |
| `DELETE` | `/:id` | `email:manage` | Delete email record. |

Attachment rules are controlled by:

```env
EMAIL_MAX_ATTACHMENTS=5
EMAIL_MAX_ATTACHMENT_BYTES=10485760
EMAIL_ALLOWED_ATTACHMENT_TYPES=pdf,doc,docx,xls,xlsx,csv,png,jpg,jpeg
```

### Documents

Mounted under `/api/v1/documents`.

| Method | Path | Permission | Purpose |
| --- | --- | --- | --- |
| `GET` | `/` | `documents:read` | List documents. |
| `POST` | `/` | `documents:write` | Upload/create document metadata and file. |
| `GET` | `/:id` | `documents:read` | Get document detail. |
| `PUT` | `/:id` | `documents:write` | Edit document metadata. |
| `DELETE` | `/:id` | `documents:write` | Delete document. |
| `GET` | `/files/:id/view` | `documents:read` | View document file. |
| `GET` | `/files/:id/download` | `documents:read` | Download document file. |
| `GET` | `/:id/processes` | `documents:read` | List document process history. |
| `POST` | `/:id/reprocess` | `documents:process` | Reprocess document. |

### Document Templates

Mounted under `/api/v1/document-templates`.

| Method | Path | Permission | Purpose |
| --- | --- | --- | --- |
| `GET` | `/` | `document_templates:read` | List templates. |
| `POST` | `/` | `document_templates:write` | Create template. |
| `POST` | `/structure` | `document_templates:write` | Create template with sheets/columns. |
| `GET` | `/code/:code/structure` | `document_templates:read` | Get structure by code. |
| `GET` | `/:id` | `document_templates:read` | Get template. |
| `PUT` | `/:id` | `document_templates:write` | Update template. |
| `DELETE` | `/:id` | `document_templates:write` | Delete template. |
| `POST` | `/:id/publish` | `document_templates:publish` | Publish template. |
| `POST` | `/:id/archive` | `document_templates:publish` | Archive template. |
| `GET` | `/:id/structure` | `document_templates:read` | Get full template structure. |
| `GET` | `/:id/sheets` | `document_templates:read` | List template sheets. |
| `GET` | `/:id/sheets/:sheetId/columns` | `document_templates:read` | List sheet columns. |

### Storage Locations

Mounted under `/api/v1/storage-locations`.

| Method | Path | Permission | Purpose |
| --- | --- | --- | --- |
| `POST` | `/` | `storage_locations:write` | Create storage location. |
| `GET` | `/` | `storage_locations:read` | List active storage locations. |
| `GET` | `/:id` | `storage_locations:read` | Get storage location. |
| `PUT` | `/:id` | `storage_locations:write` | Update storage location. |
| `DELETE` | `/:id` | `storage_locations:write` | Delete storage location. |

### Sessions

Mounted under `/api/v1/sessions`. Requires `portal:access`.

| Method | Path | Purpose |
| --- | --- | --- |
| `GET` | `/` | List current user's sessions. |
| `DELETE` | `/:id` | Logout/remove a session. |

### Data Quality

Mounted under `/api/v1/issues`.

| Method | Path | Permission | Purpose |
| --- | --- | --- | --- |
| `POST` | `/` | `data_quality:write` | Create issue. |
| `GET` | `/` | `data_quality:read` | List issues. |
| `PUT` | `/:issueCode` | `data_quality:write` | Update issue. |
| `POST` | `/:issueCode/resolveIssue` | `data_quality:resolve` | Resolve issue. |
| `GET` | `/:issueCode/transactions` | `data_quality:read` | List issue resolution transactions. |

### Surveillance

Mounted under `/api/v1/surveillance`. Requires access to the outbreak management plus route-specific surveillance permissions.

| Method | Path | Permission | Purpose |
| --- | --- | --- | --- |
| `GET` | `/weeks` | `surveillance:read` | List epi weeks by year. |
| `GET` | `/alerts` | `surveillance:read` | List alerts. |
| `GET` | `/diseases` | `surveillance:read` | List diseases. |
| `GET` | `/regions` | `surveillance:read` | List regions. |
| `GET` | `/districts` | `surveillance:read` | List districts. |
| `GET` | `/regions/:regionID/districts` | `surveillance:read` | List districts by region. |
| `POST` | `/districts` | `surveillance:manage_locations` | Upsert district. |
| `GET` | `/districts/:districtID/subcounties` | `surveillance:read` | List subcounties by district. |
| `GET` | `/subcounties/:id` | `surveillance:read` | Get subcounty. |
| `POST` | `/subcounties` | `surveillance:manage_locations` | Upsert subcounty. |
| `DELETE` | `/subcounties/:id` | `surveillance:manage_locations` | Delete subcounty. |
| `GET` | `/facility-weekly-metrics/week/:epiWeekID` | `surveillance:read` | Facility weekly metrics by week. |
| `GET` | `/facility-weekly-metrics/facility/:facilityID` | `surveillance:read` | Facility metrics by facility. |
| `GET` | `/facility-weekly-metrics/facility/:facilityID/disease/:diseaseID/trend` | `surveillance:read` | Disease trend for facility. |
| `GET` | `/facility-weekly-metrics/facility/:facilityID/indicator/:indicatorID/trend` | `surveillance:read` | Indicator trend for facility. |
| `GET` | `/facility-weekly-metrics/week/:epiWeekID/disease/:diseaseID` | `surveillance:read` | Disease metrics by week. |
| `GET` | `/facility-weekly-metrics/disease-trend` | `surveillance:read` | Aggregated disease weekly trend. |
| `GET` | `/weekly-statuses/list` | `surveillance:read` | List weekly statuses. |
| `GET` | `/weekly-statuses/detailed` | `surveillance:read` | Detailed weekly statuses. |
| `GET` | `/weekly-statuses/district/week/:epiWeekID` | `surveillance:read` | District weekly statuses. |
| `GET` | `/weekly-statuses/region/week/:epiWeekID` | `surveillance:read` | Region weekly statuses. |
| `GET` | `/weekly-statuses/national/week/:epiWeekID` | `surveillance:read` | National weekly status. |
| `GET` | `/imports` | `surveillance:read` | List import batches. |
| `POST` | `/imports` | `surveillance:import` | Create import batch. |
| `GET` | `/imports/:batchID` | `surveillance:read` | Get import batch. |
| `PATCH` | `/imports/:batchID/status` | `surveillance:import` | Update import batch status. |
| `GET` | `/imports/:batchID/raw-rows` | `surveillance:read` | List raw import rows. |

### Visualiser And Admin Units

Mounted under `/api/v1/visualizer`. Requires one of `report_browser:read`, `documents:read`, `data_quality:read`, or `surveillance:read`.

| Method | Path | Purpose |
| --- | --- | --- |
| `GET` | `/datasets` | List datasets. |
| `POST` | `/dataelements` | List data elements for criteria. |
| `POST` | `/datavalues` | Query data values. |
| `GET` | `/themes` | List themes. |
| `POST` | `/dataelements/theme` | List data elements by theme. |
| `GET` | `/hiv/summary` | HIV summary. |
| `GET` | `/hiv/tested` | HIV tested metrics. |
| `GET` | `/hiv/regimen` | HIV regimen metrics. |
| `GET` | `/adminunits/orgunits` | Org unit lookup. |
| `GET` | `/adminunits/facilities` | Facility lookup. |
| `GET` | `/adminunits/district` | District lookup. |
| `POST` | `/adminunits/subcounties` | Subcounty lookup. |
| `POST` | `/adminunits/localgovt` | Local government lookup. |
| `POST` | `/adminunits/districts` | Districts by region. |
| `GET` | `/adminunits/region` | Region lookup. |
| `GET` | `/adminunits/national` | National hierarchy root. |
| `GET` | `/adminunits/hierarchy` | Full administrative hierarchy. |

### GeoJSON

Mounted under `/api/v1/geojson`.

| Method | Path | Permission | Purpose |
| --- | --- | --- | --- |
| `GET` | `/:name` | `surveillance:read` or `data_quality:read` or `report_browser:read` | Load named GeoJSON asset. |

## Verification

### Backend

```bash
cd backend
GOCACHE=/private/tmp/moh-sso-go-build go test ./...
```

### Frontend

```bash
cd frontend
source ~/.nvm/nvm.sh
nvm use v20.20.1
npm run typecheck
npm run lint
npm run build:shell
```

Full frontend release check:

```bash
cd frontend
source ~/.nvm/nvm.sh
nvm use v20.20.1
npm run release:check
```

## Troubleshooting

### Frontend blank page under `/portal`

Check runtime config:

```bash
cd frontend
npm run config:dev
```

For production Docker, make sure `config.js` contains:

```js
microfrontendMode: "remote";
singleSpaOrchestration: true;
microfrontendMountMode: "orchestrated";
```

Also make sure import map paths are rooted under `/portal`.

### Vite cannot resolve a microfrontend package

Check that the app package exists, has package exports, and is included in workspace aliases/import map generation.

Useful checks:

```bash
cd frontend
npm run mf:doctor
npm run audit:import-map
```

### RBAC sync starts but finds no realm export

Set `RBAC_STARTUP_SYNC_REALM_EXPORT_PATH` or place the realm export where the backend expects it. Live Keycloak sync can still run if `RBAC_STARTUP_SYNC_LIVE_KEYCLOAK=true`.

### Keycloak users have roles but cannot see apps

Confirm all of the following:

1. The Keycloak client exists.
2. The user has the correct client role.
3. The client role is registered as an access role in RBAC.
4. The system metadata has a valid `/portal/...` launch URL or absolute URL.
5. Startup sync or manual RBAC sync has run.
6. The frontend menu/app launcher gates the app using the expected permission or accessible system.

### Browser cookies not returned in local development

Use:

```env
COOKIE_DOMAIN=
```

Do not use `COOKIE_DOMAIN=localhost`.

### Announcement update fails on enum values

Announcement enum values must match database enum values. Normalize UI values before sending update payloads.

### Email or announcement attachments are rejected

Validate:

- `EMAIL_ALLOWED_ATTACHMENT_TYPES`
- `EMAIL_MAX_ATTACHMENTS`
- `EMAIL_MAX_ATTACHMENT_BYTES`
- `ANNOUNCEMENT_ALLOWED_ATTACHMENT_TYPES`
- `ANNOUNCEMENT_MAX_ATTACHMENTS`
- `ANNOUNCEMENT_MAX_ATTACHMENT_BYTES`

Email attachment payloads should provide exactly one of `path` or `data_base64`.

## Documentation Map

Each frontend app and package has its own README:

```text
frontend/apps/*/README.md
frontend/packages/*/README.md
```

Backend docs:

| Document | Purpose |
| --- | --- |
| [`backend/README.md`](backend/README.md) | Backend architecture, module boundaries, operations, and verification. |
| [`docs/api-reference.md`](docs/api-reference.md) | Human-readable API endpoint map and payload notes. |
| [`backend/docs/openapi.yaml`](backend/docs/openapi.yaml) | Machine-readable OpenAPI starter contract. |
| [`docs/versioning.md`](docs/versioning.md) | Backend SemVer, build metadata, and image tag policy. |
| [`docs/releasing.md`](docs/releasing.md) | Automatic version preparation and verified frontend/backend releases. |
| [`docs/releasing-backend.md`](docs/releasing-backend.md) | Verified backend release workflow and artifacts. |
| [`docs/deployment.md`](docs/deployment.md) | Immutable backend deployment and runtime verification. |
| [`docs/rollback.md`](docs/rollback.md) | Automated and manual backend rollback procedure. |

RBAC and onboarding docs:

| Document | Purpose |
| --- | --- |
| [`docs/rbac-keycloak-sync.md`](docs/rbac-keycloak-sync.md) | Keycloak/client/role sync operating model. |
| [`docs/rbac-governance.md`](docs/rbac-governance.md) | RBAC governance center behavior and workflows. |
| [`docs/rbac-usage.md`](docs/rbac-usage.md) | How RBAC permissions are used by backend and frontend. |
| [`docs/system-onboarding.md`](docs/system-onboarding.md) | How to onboard a new Keycloak client/system. |
| [`docs/tooling.md`](docs/tooling.md) | Project-wide doctor/audit/tooling commands. |

Frontend docs:

| Document | Purpose |
| --- | --- |
| [`frontend/README.md`](frontend/README.md) | Frontend workspace overview. |
| [`frontend/docs/architecture.md`](frontend/docs/architecture.md) | Frontend app/package architecture. |
| [`frontend/docs/development.md`](frontend/docs/development.md) | Local development modes. |
| [`frontend/docs/build-and-deployment.md`](frontend/docs/build-and-deployment.md) | Build, Docker, import-map, and deployment modes. |
| [`frontend/docs/import-maps.md`](frontend/docs/import-maps.md) | Import map behavior. |
| [`frontend/docs/package-publishing.md`](frontend/docs/package-publishing.md) | npm package publish readiness. |
| [`frontend/docs/versioning.md`](frontend/docs/versioning.md) | App/package versioning strategy. |
| [`frontend/docs/verification.md`](frontend/docs/verification.md) | Frontend verification commands. |
| [`frontend/docs/troubleshooting.md`](frontend/docs/troubleshooting.md) | Common frontend issues. |

Use this root README for system-level orientation. Use module READMEs for module-specific UI behavior and ownership, `backend/README.md` for backend implementation rules, and `docs/api-reference.md` for endpoint lookup.
