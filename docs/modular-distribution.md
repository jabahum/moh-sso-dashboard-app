# Modular distribution

## Phase 1 implementation

The backend now has a public module contract in `backend/module`. Composition
lives in `backend/internal/api/modules.go`. Bootstrap constructs and initializes
the registry before starting background workers or serving HTTP traffic.

Metrics is the first feature routed through the registry. Its handlers, SQL,
existing URLs and `metrics:read` permission checks retain their existing behavior.
Its version follows the backend build version until independent packaging in
Phase 2. Other features still use their existing registration and worker wiring.
The inventory lists only features integrated with the module registry.

Set `DISABLED_MODULES=metrics` in backend configuration to omit metrics routes at
startup. The default empty value enables metrics. Unknown IDs fail startup.
This setting takes effect on restart. Frontend navigation is still managed by the
existing application registry and Keycloak metadata. Manifest synchronization is
scheduled for Phase 3.

`GET /api/v1/admin/modules` returns the normal success envelope with an array of
module IDs, versions and enabled states. The endpoint inherits session
authentication, audit and rate limiting middleware and requires administrator
access.

## Module contract

Each definition supplies identity, version, enabled state and optional dependency
IDs, initialization and route registration callbacks. Dependencies are injected
through constructors and closures. Public signatures use standard library and
Gin types so future module consumers can import the contract without importing
the host's internal packages.

Register definitions in explicit dependency order. The registry validates IDs,
versions, duplicate IDs, missing or incorrectly ordered dependencies and disabled
dependencies before initialization. It preserves this order for startup and route
registration. Disabled definitions appear in inventory but their callbacks are
skipped.

Initialization runs once. An initializer must return only after readiness and
return cleanup even when it fails after partially acquiring resources. Workers
must observe the supplied context. Cleanup must wait for them within its supplied
shutdown context. On failure the registry cancels module contexts, then calls
acquired cleanup callbacks in reverse order with a fresh bounded context. Normal
shutdown also cancels module contexts and runs cleanup once in reverse order,
before bootstrap closes database connections. Failed registries are discarded.

Routes receive protected and admin groups that inherit host middleware. The
admin group is a URL namespace. Each module must apply the appropriate resource
permissions or administrator requirement to its endpoints.

## Repository audit and next phases

| Area | Current integration | Next work |
| --- | --- | --- |
| Backend dependencies | One `backend/go.mod`, currently `github.com/moh-sso-dashboard` | Establish publishable repository paths and extract a representative domain |
| Dependency wiring | Bootstrap builds repositories, services and handlers | Move domain construction behind independent module constructors incrementally |
| Routing | Feature registrars under `internal/features`, metrics now uses the registry | Migrate additional registrars with regression checks |
| Workers | Existing bootstrap workers plus a cleanup contract for new modules | Integrate each extracted worker with readiness and shutdown |
| Migrations | Existing golang-migrate runner and deployment migration directories | Preserve migration history and coordinate module ownership in Phase 2 |
| Frontend | npm workspaces, single-spa, package staging, import maps and Changesets | Reuse these mechanisms for pinned distribution assembly |
| Access and navigation | Keycloak, permission middleware and existing application registry | Add validated manifests and explicit synchronization ownership in Phase 3 |
| Deployment | Existing Docker, Compose, Helm and release scripts | Extend existing entry points for distribution releases in Phase 4 |

Phase 1 adds no independently published Go module. The current module path and
internal domain dependencies need resolution during extraction. Existing
database migrations remain owned by their current migration streams until the
module migration design is implemented.

## Verification

Run from `backend`.

```sh
go test ./module ./internal/api/... ./internal/bootstrap ./internal/config ./internal/architecture
go test -race ./module ./internal/api
go run ./cmd/cli openapi generate
go test ./internal/apidocgen
go build ./cmd/server
```

Lifecycle regression tests cover composition validation, partial initialization
rollback, cancellation, disabled callbacks and idempotent cleanup. Router tests
cover inventory authentication and administrator access, disabled metrics routes
and existing metrics permission checks.
