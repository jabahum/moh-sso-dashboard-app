# MOH SSO Dashboard Backend

The backend is a Go/Gin modular monolith for the MOH Integrated Health Portal. It is feature-first and microservice-ready: feature modules own their HTTP handlers, routes, DTOs, mappers, services, and repositories where that ownership is clear, while shared infrastructure remains in cross-cutting packages. Six optional applications (Administrative Units, Announcements, Audit, GeoJSON, Metrics and Report Scheduler) are independently versioned Go modules under `modules/`, compiled into the server through pins in `go.mod`; see [Modular distribution](../docs/modular-distribution.md).

## Runtime Responsibilities

- Keycloak authentication callback, session refresh, logout, and current-user profile.
- Dynamic system-aware RBAC synced from Keycloak clients, realm roles, client roles, users, and portal permission mappings.
- User, client/system, role, permission, and access administration.
- Announcements with persistent attachments, related links, email delivery, and public/news feed visibility (published `modules/announcements` module).
- Email queue, attachment validation, retry, and delivery management.
- Notifications, unread state, delivery history, and delivery retry.
- Audit logging (host) and audit exploration (published `modules/audit` module).
- Documents, templates, data quality, visualiser, surveillance, sessions, and storage location APIs.
- Metrics, GeoJSON map assets, administrative units and report scheduling through the published `modules/metrics`, `modules/geojson`, `modules/admin-units` and `modules/report-scheduler` modules.

## Architecture

```text
backend/
|-- cmd/
|   |-- server/                 # API server entrypoint
|   |-- cli/                    # Operational CLI tools (moh-sso)
|   `-- surveillance-db/        # Standalone Observatory (surveillance) schema initialization
|-- config/                     # RBAC seed and backend configuration assets
|-- core/                       # Standard-library-only platform core (nested Go module)
|-- docs/
|   `-- openapi.yaml            # Generated complete OpenAPI API contract
|-- internal/
|   |-- adapters/modules/       # README only; no published module keeps a host adapter
|   |-- api/                    # Router shell, module registry composition, cross-cutting handlers
|   |-- apidocgen/              # OpenAPI generation and validation
|   |-- architecture/           # Architecture boundary and module catalogue tests
|   |-- authz/                  # Permission constants, policies, and authz helpers
|   |-- bootstrap/              # Composition root
|   |-- cache/                  # Redis cache and publish/subscribe helpers
|   |-- config/                 # Configuration loading, defaults and profiles
|   |-- db/                     # Migrations, sqlc queries, generated sqlc code
|   |-- email/                  # Email templates and attachment normalization
|   |-- events/                 # In-process event contract
|   |-- features/               # Host feature modules
|   |-- geography/              # Shared geographic reference data
|   |-- http/                   # Response envelopes and API errors
|   |-- identityreconcile/      # Reviewed application account links
|   |-- keycloak/               # Keycloak admin/client integration
|   |-- log/                    # Logger
|   |-- middleware/             # Auth, audit, rate-limit, request middleware
|   |-- migrate/                # Migration runner
|   |-- model/                  # Shared models
|   |-- modulehttp/             # Generic HTTP registrar for published modules
|   |-- observability/          # Request and correlation ID context
|   |-- ratelimit/              # Rate-limit policies and middleware
|   |-- repository/             # Shared repository adapters
|   |-- service/                # Shared services
|   |-- storage/                # Local/NFS/S3 storage abstraction
|   |-- utils/                  # Small helpers
|   |-- version/                # Build metadata
|   |-- worker/                 # Background workers
|   `-- ws/                     # Websocket support
|-- module/                     # Gin adapter for the module registry
|-- modules/                    # Published modules: admin-units, announcements, audit,
|                               # geojson, metrics, report-scheduler (nested Go modules)
|-- scripts/                    # Version and release helper scripts
`-- go.mod
```

Host features under `internal/features` are `auth`, `authsession`, `clients`,
`data_quality`, `document_templates`, `documents`, `email`, `health_context`,
`notifications`, `platform`, `rbac`, `sessions`, `storage_locations`,
`surveillance`, `system_rbac`, `users` and `visualiser`.

Feature modules generally use this shape:

```text
internal/features/<feature>/
|-- handler.go
|-- routes.go
|-- dto.go
|-- mapper.go
|-- service.go
|-- repository.go
`-- repository_postgres.go or repository_keycloak.go
```

Not every module needs every file. Thin features such as `auth` or `authsession` may stay smaller when the underlying service is shared. Published modules under `modules/` use their own layout; see [Developer workflows](../docs/module-development.md).

## Architecture Boundary Checks

Backend feature-to-feature imports are guarded by an architecture test:

```bash
go test ./internal/architecture
```

The same check runs inside `go test ./...`. Approved cross-feature imports are documented in [`../docs/backend-microservice-readiness.md`](../docs/backend-microservice-readiness.md). New feature dependencies should normally be expressed as constructor-injected interfaces or shared infrastructure packages, not direct imports.

## Composition Root

`internal/bootstrap` wires the application in one place:

- configuration
- database/sqlc store
- Keycloak clients
- Redis/rate limiter
- repositories
- services
- handlers
- workers
- router
- startup RBAC sync

Keep new construction logic in bootstrap. Feature code should receive dependencies through constructors, not create infrastructure directly.

## Local Development With Auto-Reload

The development backend image includes [Air](https://github.com/air-verse/air), configured by `backend/.air.toml`. Both development Compose variants bind-mount `backend/` into `/app`, so changing a Go file automatically rebuilds and restarts the API process inside the existing container.

Start the full local stack:

```bash
docker compose -f infra/compose/docker-compose.dev.yml up --build
```

Or use the development stack connected to an external Keycloak instance:

```bash
docker compose -f infra/compose/docker-compose-without-keycloak.yml up --build
```

Follow backend rebuild and application logs:

```bash
docker compose -f infra/compose/docker-compose.dev.yml logs -f backend
```

Air watches Go, template, and YAML files. Test files are excluded, while generated sqlc Go files remain watched so regenerated database code reloads the API. A compile error leaves the container running and Air rebuilds again when the source is corrected.

When `Dockerfile.dev`, `.air.toml`, `go.mod`, or `go.sum` changes, rebuild the development image:

```bash
docker compose -f infra/compose/docker-compose.dev.yml up -d --build backend
```

This behavior is development-only. The production `infra/docker/backend/Dockerfile` continues to build and run an immutable compiled binary.

## API Response Rules

Handlers should return feature-owned DTOs, not raw SQL rows or database models.

Success envelope:

```json
{
  "success": true,
  "data": {},
  "meta": {}
}
```

Error envelope:

```json
{
  "success": false,
  "error": {
    "code": "INVALID_INPUT",
    "message": "Safe user-facing message"
  },
  "meta": {}
}
```

Detailed database, Keycloak, or filesystem errors should be logged server-side and converted to safe API errors.

Every request receives a request ID and correlation ID. Clients may send `X-Request-ID` and `X-Correlation-ID`; otherwise the backend generates them. Responses expose both headers, and standard API envelopes include `meta.requestId`.

## RBAC And Keycloak

Keycloak is the identity source:

- users
- credentials
- realm roles
- clients
- client roles
- user role assignments

The portal DB stores authorization metadata:

- system/client display metadata
- launch URLs, icons, category, owner/support metadata
- permission catalog
- realm-role permission mappings
- system-role permission mappings
- access role metadata
- synced user access profiles and drift reports

Startup sync is controlled through `RBAC_STARTUP_*` env vars in `infra/env/dev.env.example`. The common local posture is:

```env
RBAC_STARTUP_SYNC_ENABLED=true
RBAC_STARTUP_SEED_ENABLED=true
RBAC_STARTUP_SYNC_LIVE_KEYCLOAK=true
RBAC_STARTUP_SYNC_USERS=true
RBAC_STARTUP_SYNC_PUSH_TO_KEYCLOAK=false
RBAC_STARTUP_SYNC_FAIL_ON_ERROR=false
```

`RBAC_STARTUP_SYNC_PUSH_TO_KEYCLOAK` defaults to `false` (`internal/config/config.go`); set it to `true` only when you intend startup to create missing Keycloak clients and roles.

See:

- [`../docs/rbac-keycloak-sync.md`](../docs/rbac-keycloak-sync.md)
- [`../docs/rbac-governance.md`](../docs/rbac-governance.md)
- [`../docs/system-onboarding.md`](../docs/system-onboarding.md)

## Announcements, Attachments, And Email

Announcements are implemented by the published `modules/announcements` module. They support:

- persistent attachment storage
- download URLs for news feed and email templates
- `include_in_email` flags per attachment
- optional related links through `link_url` and `link_label`
- publish/schedule flows that queue notification email deliveries

Email attachment payloads must provide exactly one of:

- `path`
- `data_base64`

Persistent announcement attachments are stored through the configured storage provider and can also be sent or linked during announcement email delivery.

## Notifications

Notifications support:

- list, read/unread state, counts, cleanup, and delete
- notification delivery records
- delivery status history
- manual retry for failed/retry/cancelled deliveries
- background email delivery worker
- background SMS delivery worker
- admin delivery dashboard at `/portal/admin/notifications/deliveries`
- delivery metrics grouped by channel/provider
- safe audit logging for retry, cancel, test SMS, and SMS worker outcomes

Delivery records are stored separately from notification records so operations can inspect channel-level failures without changing the notification message.

SMS delivery is controlled by `SMS_*`, `AFRICASTALKING_*`, and `TWILIO_*` environment variables. Local development should use `SMS_PROVIDER=mock`; production Africa's Talking deployments must provide username/API key and should verify delivery through the admin test SMS flow.

See:

- [`../docs/notifications.md`](../docs/notifications.md)
- [`../docs/sms-delivery.md`](../docs/sms-delivery.md)
- [`../docs/operations/notification-delivery-runbook.md`](../docs/operations/notification-delivery-runbook.md)

## Local Development

```bash
cd backend
cp ../infra/env/dev.env.example app.env
make test
go run ./cmd/server
```

## Versioning And Builds

Backend releases use component-scoped semantic tags such as `backend/v1.2.3`. The binary, Docker image labels, health response, `/version` endpoint, and deployment record all use the same linker-injected build metadata.

```bash
make build
./bin/moh-sso-dashboard --version

make build-cli
./bin/moh-sso version --output json

make verify-version
```

Production deployments must set `BACKEND_TAG` to an immutable semantic or SHA image tag. They must not use `latest`.

Release helper commands:

```bash
make next-version BUMP=patch
make release-check VERSION=1.2.3
make release-pr VERSION=1.2.3
make release-dispatch VERSION=1.2.3
make release-status VERSION=1.2.3
make verify-image IMAGE=ghcr.io/mohuganda/moh-sso-dashboard-backend:1.2.3 VERSION=1.2.3
```

See:

- [`../docs/versioning.md`](../docs/versioning.md)
- [`../docs/releasing-backend.md`](../docs/releasing-backend.md)
- [`../docs/deployment.md`](../docs/deployment.md)
- [`../docs/rollback.md`](../docs/rollback.md)

Docker Compose uses service hostnames such as:

- `backend-db`
- `redis`
- `keycloak`

Local direct execution usually needs localhost-facing overrides.

## Migrations

Migrations live under:

```text
internal/db/migrations
```

Do not edit generated sqlc files manually. Update SQL under `internal/db/query` and regenerate sqlc when query shape changes.

## Verification

Run:

```bash
cd backend
make test
```

`make test` runs `make modules-check` (vet and tests for every nested module
under `modules/`) before the host test suite, because `go test ./...` does not
traverse nested Go modules.

Also run frontend verification when backend DTOs or API payloads change:

```bash
cd ../frontend
source ~/.nvm/nvm.sh
nvm use v20.20.1
./node_modules/.bin/tsc --noEmit -p tsconfig.app.json
NODE_OPTIONS=--max-old-space-size=8192 ./node_modules/.bin/eslint . --cache --cache-location .eslintcache --max-warnings=0
./node_modules/.bin/vite build --config vite.config.ts
```

## Related Documentation

Swagger UI is available at `/swagger/` on the running backend. Use
`make openapi-generate` after changing routes/DTOs and `make openapi-validate` to
check the complete contract. See [Swagger setup and maintenance](docs/README.md).

- [`../README.md`](../README.md)
- [`../docs/api-reference.md`](../docs/api-reference.md)
- [`../docs/backend-microservice-readiness.md`](../docs/backend-microservice-readiness.md)
- [`../docs/versioning.md`](../docs/versioning.md)
- [`docs/openapi.yaml`](docs/openapi.yaml)
- [`../docs/tooling.md`](../docs/tooling.md)
