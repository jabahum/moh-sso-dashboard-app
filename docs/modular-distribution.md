# Modular distribution

## Phase 1 implementation

The backend now has a public module contract in `backend/module`. Composition
lives in `backend/internal/api/modules.go`. Bootstrap constructs and initializes
the registry before starting background workers or serving HTTP traffic.

Metrics is the first feature routed through the registry. Its handlers, SQL,
existing URLs and `metrics:read` permission checks retain their existing behavior.
Its implementation is independently packaged under `backend/modules/metrics`.
Other features still use their existing registration and worker wiring.
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
| Backend dependencies | Host module plus independently versioned metrics module | Extract additional domains after their dependency boundaries are reviewed |
| Dependency wiring | Bootstrap builds repositories, services and handlers | Move domain construction behind independent module constructors incrementally |
| Routing | Feature registrars under `internal/features`, metrics now uses the registry | Migrate additional registrars with regression checks |
| Workers | Existing bootstrap workers plus a cleanup contract for new modules | Integrate each extracted worker with readiness and shutdown |
| Migrations | Existing golang-migrate runner executes core migrations before metrics initialization | Writing modules will need their own migration ownership and sequencing |
| Frontend | npm workspaces, single-spa, package staging, import maps and Changesets | Reuse these mechanisms for pinned distribution assembly |
| Access and navigation | Keycloak, permission middleware and application manifests in the existing system registry | Package deployment configuration in Phase 4 |
| Deployment | Existing Docker, Compose, Helm and release scripts | Extend existing entry points for distribution releases in Phase 4 |

## Phase 2 packaging

Metrics has the public module path
`github.com/mohuganda/moh-sso-dashboard/backend/modules/metrics`. It contains
the service, handlers, route definitions, response DTOs and repository port.
Public repository records live in its `model` package. The module imports no
dashboard implementation packages. Internal host constructors remain available
as compatibility adapters. The host SQL adapter maps its generated records into
the module contract, and the host supplies the existing metrics permission
middleware to every module route.

`backend/go.mod` pins the distribution's module version. The module directory is
separately versioned using `backend/modules/metrics/v0.1.0` style tags in the
existing upstream repository. Go downloads the tagged module during a normal
build, including the dependency-download stage of the existing Dockerfiles.
Metrics inventory reports the selected Go dependency version, independently of
the backend executable version.

For local development, create a Go workspace from `backend` with
`go work init . ./modules/metrics`. If a workspace already exists, use
`go work use ./modules/metrics`. Local workspace files are ignored by Git.
Workspace builds report the metrics version as `dev`. Use `GOWORK=off` to verify
the tagged dependency. There is no production filesystem replacement in the
distribution dependency configuration.

Metrics owns no tables and ships no migrations. Its SQL adapter reads tables
owned by the core. Existing migration files, versions and migration tracking
tables retain their ownership. Core migrations continue to complete before
module initialization, using the existing golang-migrate locking and dirty-state
handling. Module packaging adds no schema upgrade or data rewrite for either a
fresh installation or an existing database. Future modules that own tables must
define a migration stream before they can be extracted.

`make -C backend modules-check` vets and tests the independently packaged module.
The normal backend test command and backend release/PR checks include this step
because `go test ./...` on the host does not traverse nested Go modules.
The API contract generator loads the module's handler source so published
handlers retain their typed response schemas in the generated contract.

To release an updated module, verify its standalone tests and dashboard
integration using a local workspace. Commit the module changes, create the
directory-prefixed semantic version tag and push that tag to the upstream
repository. Verify download from a separate consumer with `GOWORK=off`, then
update the pinned dependency with `go get`, regenerate API contracts and commit
the distribution integration. See `backend/modules/metrics/README.md` for the
public constructor and route contracts.

## Verification

Run from `backend`.

```sh
make modules-check
go test ./module ./internal/api/... ./internal/bootstrap ./internal/config ./internal/architecture
go test -race ./module ./internal/api
(cd modules/metrics && GOWORK=off go test -race ./...)
go run ./cmd/cli openapi generate
go test ./internal/apidocgen
go build ./cmd/server
```

Lifecycle regression tests cover composition validation, partial initialization
rollback, cancellation, disabled callbacks and idempotent cleanup. Router tests
cover inventory authentication and administrator access, disabled metrics routes
and existing metrics permission checks.

Phase 2 verification passed for the full backend suite, standalone module tests,
race checks, vet, Linux amd64 compilation, temporary-workspace integration and
dependency downloads using only `go.mod` and `go.sum`. A separate temporary Go
project downloaded `v0.1.0`, verified its reported version and registered all 19
guarded routes. The published dependency is recorded in the server binary's Go
build metadata. Generated API contract changes are limited to the 19 handler
source paths.

Full Docker image and database-backed runtime checks remain pending because the
local Docker daemon was unavailable during Phase 2 verification.

## Phase 3 manifests

The Systems page now provides administrator-only manifest upload, validation,
preview and import. The implementation uses the current Postgres system registry,
navigation model, access-role tables and lifecycle workflow. A serializable
transaction applies registry metadata, access-role changes and audit records.
Stale previews require review again. Repeated imports are idempotent.

External applications use their deployed URLs. Internal applications reference
an enabled compiled module. Separately hosted single-spa applications use
`/apps/extensions/<application-id>` routes and a browser module entry URL. The
runtime endpoint checks application access and activation before returning that
entry point. The existing shell host supplies the normal application props and
handles mounting and unmounting.

The implemented manifest schema uses the registry's own-client access-role model.
Keycloak client provisioning and user role assignment use the existing explicit
administration workflows. Startup discovery preserves manifest-managed registry
configuration. No additional database migration is required because documents
are stored in existing system metadata. The schema, examples, API contracts and
configuration ownership rules are documented in `examples/manifests/README.md`.

Phase 3 verification passed the full backend suite with isolated Postgres schemas,
transaction and lifecycle race tests, vet, standalone module checks and generated
OpenAPI validation. The backend Docker image built successfully and its version
command ran successfully. Frontend workspace production builds, type checking,
lint and architecture checks passed. Browser coverage includes the manifest
review/import flow on desktop and mobile, dynamic application mount, refresh,
unmount and denied access, plus 50 existing portal page regression checks.

The published-npm production build could not resolve the existing pinned
`@moh-sso/announcements@1.3.1` package. Workspace builds were used for runtime
verification; published-npm deployment verification requires that release to be
available. Live Keycloak provisioning and production deployment were not run.

## Phase 4 distribution packaging

Production now consumes released npm modules (`1.3.2`, with report-scheduler
`0.4.2`) using a committed npm lock and integrity-checked `npm ci` installation.
This resolves the unavailable `1.3.1` pins found in Phase 3. Development continues
to use workspace modules. Backend builds use the published metrics dependency
with `GOWORK=off`, checksum verification and Go build/download caches.

The specialized frontend Dockerfile caches toolchain and published module
installation before copying source. BuildKit keeps the Go and npm download
caches across builds. The distribution Compose entry point reuses the production
gateway and services, builds versioned application images, persists core,
Observatory, Redis and upload data, and supports the existing external Keycloak.
Startup migration behavior remains owned by the existing backend lifecycle.
See `infra/distribution.md` for prerequisites, commands and upgrade guidance.

Phase 4 verification passed registry-backed frontend builds, the production npm
shell type check, lint, architecture checks, 19 module-source tests and 54 browser
tests. Both Docker images built successfully; the server version command and
the frontend production runtime checks passed. A repeated dependency-stage build
reused the toolchain and published-module layers. All 14 Compose/environment
contracts and the existing disposable Postgres migration/registry checks passed.
The new isolated Compose smoke test verified the real frontend image, secret
mounts, database/Redis readiness, module versions and core database persistence
across container recreation. It cleaned up its own containers and volumes.
Authenticated external Keycloak and production deployment were not exercised.

## Phase 5 release and installation verification

The existing GHCR image workflow now builds the frontend with the pinned npm
Dockerfile. Its validation dependency includes the new distribution installation
gate, which builds both images before exercising runtime, authentication and
deployment recovery. Component version/SHA tags and the existing image-pair
deployment bundle remain the release mechanism.

Bundles record the compiled module versions, exact frontend versions and hashes
of `go.mod`, `go.sum` and the production npm lock. Distribution server deployments
use digest-pinned images with no source-build settings. Runtime verification
compares the complete frontend module set with the approved pins. Deployment
snapshots retain their Compose project name, and an existing installation cannot
silently change topology/project. Legacy deployment records remain compatible.

Use `infra/server/deploy.distribution.json.example` for separate staging and
production configurations. The existing trusted-build selector and GitHub
environment deployment workflow promote the same successful main-build bundle
by run ID. The operator guide describes staging-first settings, secret files,
backup hooks, schema compatibility and rollback.

Verification passed 32 deployment/archive contracts, 14 Compose/environment
contracts and 8 trusted-build/promotion tests. Workflow lint and the new backup
hook's shell checks passed. Isolated tests using the real backend and Keycloak
26.0.5 verified fresh migrations, real token/role authorization, module inventory,
manifest import/approval, audit idempotence, startup preservation, graceful
shutdown, restart, and restoration of both databases and uploads. Container
transaction tests passed for legacy and distribution deployments, including
failed-version detection, automatic/operator rollback and data preservation.
Temporary test resources were removed. These are local verification results;
publishing images and deploying to real staging/production require the merged
workflows and configured operator environments and were not triggered here.
