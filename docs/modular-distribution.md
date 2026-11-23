# Modular distribution

This page describes how the dashboard backend and frontend are split into
independently versioned modules and composed into one distribution: the module
contract, module stages, packaging and release, installation, disablement and
verification. It is for developers extending or releasing modules and for
operators building and deploying the distribution. Step-by-step developer
procedures are in [Developer workflows](module-development.md), runtime procedures
are in [Operator workflows](module-operations.md), and the mandatory platform
foundation is described in [Platform core boundaries](platform-core.md). The
[Development history](#development-history) at the end records how the current
design was reached, phase by phase.

## Overview

Optional backend applications are compiled modules. Each one is registered with
a module registry at startup, appears in a runtime inventory and can be disabled
by configuration. A module's portable implementation lives in its own nested Go
module under `backend/modules/<module-id>`, is released with an immutable,
directory-prefixed tag and is consumed by the dashboard through a reviewed pin in
`backend/go.mod` (Report Scheduler is currently the exception: its pin is a Go
pseudo-version, described under [Current status](#current-status)). Frontend applications and shared libraries are published npm
packages consumed through reviewed production pins and a committed npm lock.

The registry runs trusted, compiled code. It is not a sandbox and does not
download code at runtime: changing a module version means building and deploying
a new backend image.

Mandatory platform capabilities (identity and sessions, authorization,
application governance, contextual policy, audit recording and operations) are
not optional modules and cannot be disabled. See
[Platform core boundaries](platform-core.md).

## Module contract

The public module contract is in `backend/module`. It is a Gin adapter: it keeps
route registration in the host and delegates inventory, validation and lifecycle
management to the standard-library-only runtime in `backend/core`
(`core.Runtime`). Composition lives in `backend/internal/api/modules.go`.
Bootstrap constructs and initializes the registry before starting background
workers or serving HTTP traffic.

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

Routes receive public, protected and admin groups that inherit host middleware.
The public group has no authentication; its routes must validate their own
access tokens and apply appropriate rate limits. The admin group is a URL
namespace. Each module must apply the appropriate resource permissions or
administrator requirement to its endpoints.

### Composition and registration

`NewModuleRegistry` in `backend/internal/api/modules.go` registers six module
definitions, in this order: `metrics`, `audit`, `geojson`, `report-scheduler`,
`announcements` and `admin-units`. Each reports the version of its selected Go
dependency.

Self-contained modules own their HTTP handlers, request/response DTOs, OpenAPI
metadata and persistence. The host mounts them through the generic
`internal/modulehttp` registrar, which bridges verified identity, request IDs and
shared security capabilities into the module's standard HTTP request context.
The registrar mounts every method/path pattern a module declares. Only operations
whose OpenAPI security declaration is explicitly anonymous are public; missing or
null security stays protected, and missing or invalid declarations reject
registration. API generation merges module-owned operations and component
schemas into the generated contract and rejects schema collisions.

Only enabled modules are constructed. The host supplies caller-owned database
pools and shared platform capabilities (verified administrator and permission
checks, request IDs, per-user rate limits, core audit recording, storage and
notification providers as each module requires). Required capabilities reject
construction when missing and fail closed at runtime.

## Module stages

`docs/module-catalog.json` (catalogue schema 2) tracks every backend feature and
every publishable frontend package. [Module coverage](module-coverage.md) is
generated from it and from the reviewed production npm pins; use it for the
per-module table of stage, integration, dependencies, workers, migration owner
and next work.

| Stage | Meaning |
| --- | --- |
| `core` | Stays in the trusted host as part of the mandatory platform. Cannot be disabled through `DISABLED_MODULES`. |
| `planned` | Still uses legacy host wiring. Its workers and migrations have not been transferred to the registry. |
| `registry-hosted` | Uses the compiled registry (inventory, disablement, lifecycle) but ships inside the host and reports the host build version. |
| `published` | Has an independently versioned package artifact that the host consumes through a reviewed pin. |

The catalogue also records each entry's integration: `host` for core and planned
features, `adapter` when a published package is still wrapped by host adapters
in `backend/internal/adapters/modules/<feature>`, and `self-contained` when the
package owns its HTTP, persistence and API contract and no host feature or
adapter tree is retained. Published modules must not retain directories in
`backend/internal/features`. No module currently uses `adapter`:
`backend/internal/adapters/modules` contains only a README.

### Current status

At the time of writing the catalogue lists 23 backend features and 23 frontend
packages:

- Six backend modules are `published` and `self-contained`: `admin_units`,
  `announcements`, `audit`, `geojson`, `metrics` and `report_scheduler`.
- Seven backend features are `core`: `auth`, `authsession`, `sessions`, `rbac`,
  `system_rbac`, `health_context` and `platform`.
- The remaining ten backend features are `planned`. No feature is currently
  `registry-hosted`.
- All 23 frontend packages are `published`.

Selected backend module versions in `backend/go.mod`:

| Go module | Selected version |
| --- | --- |
| `backend/modules/admin-units` | `v0.2.0` |
| `backend/modules/announcements` | `v0.2.0` |
| `backend/modules/audit` | `v0.2.0` |
| `backend/modules/geojson` | `v0.2.0` |
| `backend/modules/metrics` | `v0.2.0` |
| `backend/modules/report-scheduler` | `v0.1.1-0.20261120164120-d9bb6f8c819f` |

Five modules select their `v0.2.0` release tag. Report Scheduler selects a
pseudo-version of commit `d9bb6f8c` ("Give Report Scheduler a module-owned HTTP
contract"), which is in the main-line history, rather than a release tag; the
repository's `backend/modules/report-scheduler/v0.2.0` tag points at a commit
that is not an ancestor of that commit, and its module source differs from the
source the host builds.

`backend/core` is required as `v0.0.0` and resolved through the local
`replace github.com/mohuganda/moh-sso-dashboard/backend/core => ./core` directive
in `backend/go.mod`; this checkout has no `backend/core/v*` release tag. Check `backend/go.mod` and
[Module coverage](module-coverage.md) for the authoritative current values.

## Packaging, publishing and pinning

### Package layout

Each published module has the module path
`github.com/mohuganda/moh-sso-dashboard/backend/modules/<module-id>` and imports
no dashboard implementation packages. Packages keep small, reviewed dependency
graphs: GeoJSON and Administrative Units use only the standard library, Audit
adds only UUID, Announcements and Report Scheduler use UUID and lib/pq, and
Metrics uses Gin, UUID and lib/pq. Database drivers used only for testing belong to separate consumer
modules. Packages report their dependency version, returning `dev` for local
workspace or replacement builds.

### Schema and migration ownership

Published modules ship no migrations. They read and write tables in the shared
primary schema, which the platform owns. Core migrations complete before module
initialization, using the existing golang-migrate locking and dirty-state
handling. Packages receive caller-owned pools; they neither open nor close
infrastructure nor run migrations. Where a package depends on host-owned schema,
it records that explicitly (for example, Report Scheduler's
`schema-contract.json` records host ownership of scheduler migrations 40 through
47). A future module that owns tables must define a migration stream before it
can be extracted.

### Local development

For local development, create a Go workspace from `backend` with
`go work init . ./modules/metrics` (substituting the module you are changing). If
a workspace already exists, use `go work use ./modules/metrics`. Local workspace
files are ignored by Git. Workspace builds report the module version as `dev`.
Use `GOWORK=off` to verify the tagged dependency. There is no production
filesystem replacement for published modules in the distribution dependency
configuration.

### Release a module version

1. Verify the module's standalone tests and its dashboard integration using a
   local workspace.
2. Run the isolated package verifier (`infra/scripts/test-<module>-package.py`
   and, where present, its HTTP or PostgreSQL consumer variants). These copy the
   module outside the repository with an empty or private module cache,
   workspaces and downloads disabled, check the exact dependency graph, run
   build, vet and uncached race tests and build a separate consumer.
3. Commit the module changes, create the directory-prefixed semantic version tag
   (for example `backend/modules/metrics/v0.2.0`) and push that tag to the
   upstream repository. Tags are immutable.
4. Verify the public download from a separate consumer with `GOWORK=off`. The
   release verifiers (`infra/scripts/test-<module>-release.py`, present for every
   published module except Report Scheduler) download the
   version into a fresh module cache, check reviewed checksums, reject
   replacements, build and run an independent consumer and compare the complete
   published artifact with the local module. They need network access.
5. Update the pinned dependency with `go get`, record checksums in
   `backend/go.sum`, regenerate API contracts and commit the distribution
   integration.

Publishing a tag does not change a running deployment. Build a new backend image
to consume a release. The compiled inventory reports the selected dependency
version, and deployment bundle metadata records the exact pin.

`make -C backend modules-check` discovers every direct `backend/modules/*/go.mod`
package automatically and vets and tests it. The normal backend test command and
backend release/PR checks include this step because `go test ./...` on the host
does not traverse nested Go modules.

### Frontend packages

Production consumes released npm modules using a committed npm lock and
integrity-checked `npm ci` installation. Development uses workspace modules. CI
validates catalogue coverage against source directories, publishable workspaces,
production pins/lock and the generated coverage guide.

## Manifests and hosted applications

The Systems page provides administrator-only manifest upload, validation,
preview and import against the Postgres system registry, navigation model,
access-role tables and lifecycle workflow. External applications use their
deployed URLs. Internal applications reference an enabled compiled module.
Separately hosted single-spa applications use `/apps/extensions/<application-id>`
routes and a browser module entry URL; the runtime endpoint checks application
access and activation before returning that entry point. The schema, examples,
API contracts and configuration ownership rules are documented in
`examples/manifests/README.md`.

## Distribution packaging and installation

- **Images.** The specialized frontend Dockerfile caches toolchain and published
  module installation before copying source; BuildKit keeps the Go and npm
  download caches across builds. Backend builds use the published Go
  dependencies with `GOWORK=off` and checksum verification.
- **Compose distribution.** The distribution Compose entry point reuses the
  production gateway and services, builds versioned application images,
  persists core, Observatory, Redis and upload data, and supports an external
  Keycloak. See `infra/distribution.md`.
- **Release.** The GHCR image workflow builds both images and runs the
  distribution installation gate. Component version/SHA tags and the image-pair
  deployment bundle are the release mechanism. Bundles record the compiled
  module versions, exact frontend versions and hashes of `go.mod`, `go.sum` and
  the production npm lock.
- **Deployment.** Distribution server deployments use digest-pinned images with
  no source-build settings. Runtime verification compares the complete frontend
  module set with the approved pins. Use
  `infra/server/deploy.distribution.json.example` for separate staging and
  production configurations; the trusted-build selector and GitHub environment
  deployment workflow promote the same successful main-build bundle by run ID.

## Disablement and inventory

Set `DISABLED_MODULES` to a comma-separated list of optional module IDs
(`metrics`, `audit`, `geojson`, `report-scheduler`, `announcements`, `admin-units`, `storage-locations`, `visualiser`, `document-templates`, `data-quality`, `documents` and `surveillance`) to omit those modules at startup. The default empty value enables
all of them. The setting takes effect on restart. Unknown IDs and mandatory core
feature IDs fail startup. `PLATFORM_PROFILE=core` disables every optional
module; see [Platform core boundaries](platform-core.md#startup-profiles).

Disabled modules are not constructed, their routes are not registered and their
workers do not run, but data is retained and becomes available again when the
module is re-enabled. Notable effects:

- Disabling `audit` removes the seven read routes; core security/event recording
  continues.
- Disabling `report-scheduler` removes its public delivery-download routes,
  protected routes and worker.
- Disabling `announcements` removes all 26 endpoints, including public listings
  and attachment downloads. Previously queued notifications are not cancelled.
- Disabling `admin-units` removes all nine `/api/v1/visualizer/adminunits/*`
  routes.

`GET /api/v1/admin/modules` returns the normal success envelope with an array of
module IDs, versions and enabled states. The endpoint inherits session
authentication, audit and rate limiting middleware and requires administrator
access. Frontend navigation is managed by the application registry and
Keycloak metadata. See [Operator workflows](module-operations.md) for affected
applications, rollout and recovery.

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

Other checks used for module work:

- `python3 infra/scripts/check-module-catalog.py --write` regenerates
  [Module coverage](module-coverage.md); CI validates the catalogue and a Go
  architecture test checks it against the compiled inventory.
- `infra/scripts/test-<module>-package.py` and `test-<module>-release.py` verify
  standalone packages and published releases; Report Scheduler has a package
  verifier but no release verifier (see
  [Release a module version](#release-a-module-version)).
- `python3 infra/scripts/test-platform-core.py` verifies the portable core in
  isolation.
- The real-Keycloak distribution fixture
  (`infra/scripts/test-distribution-auth.py`, full and core profiles) checks
  module inventory, permissions, disablement and re-enablement, backup/restore,
  restart and full -> core -> full data preservation.

## Development history

The records below are kept in chronological order. They describe the state at
the time of each phase; statements such as "next" or "remains" refer to that
point, not to the current design described above.

### Phase 1 implementation

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

The module contract introduced in this phase (definitions, dependency order,
initialization, cleanup and route groups) is described in
[Module contract](#module-contract); the public route group was added later.

### Repository audit and next phases

| Area | Current integration | Next work |
| --- | --- | --- |
| Backend dependencies | Host module plus independently versioned metrics module | Extract additional domains after their dependency boundaries are reviewed |
| Dependency wiring | Bootstrap builds repositories, services and handlers | Move domain construction behind independent module constructors incrementally |
| Routing | Feature registrars under `internal/features`; metrics, audit reads and GeoJSON use the registry | Migrate additional registrars using the tracked coverage catalogue |
| Workers | Existing bootstrap workers plus a cleanup contract for new modules | Integrate each extracted worker with readiness and shutdown |
| Migrations | Existing golang-migrate runner executes core migrations before metrics initialization | Writing modules will need their own migration ownership and sequencing |
| Frontend | Published npm packages, single-spa, reviewed production pins and dependency lock | Track every new publishable workspace before selecting its release |
| Access and navigation | Keycloak, permission middleware and reviewed manifests in the existing system registry | Preserve authorization and lifecycle governance during each extraction |
| Deployment | Versioned images, immutable deployment bundles and staging promotion | Follow the documented release, compatibility and recovery workflow |

### Phase 2 packaging

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

#### Phase 2 verification

Phase 2 used the commands listed under [Verification](#verification).

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

### Phase 3 manifests

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

### Phase 4 distribution packaging

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

### Phase 5 release and installation verification

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

### Phase 6 expansion and module workflows

The next bounded expansion integrates audit read APIs and GeoJSON serving into
the compiled registry. Their implementations remain hosted by the backend and
report its build version. Metrics remains independently packaged and versioned.
`DISABLED_MODULES` accepts `metrics`, `audit` and `geojson`; existing route paths,
permissions, export rate limits and response contracts are preserved. Disabling
audit read APIs leaves the host's security/event recording service active.

The source catalogue in `docs/module-catalog.json` tracks all 23 backend features
and 23 publishable frontend packages. [Module coverage](module-coverage.md) is
generated from it and the reviewed production npm pins. Entries distinguish core,
planned, registry-hosted and published stages and record dependencies, current
worker/migration owners and next extraction work. Remaining domains are explicit
incremental work, rather than being reported as independently packaged.

[Developer workflows](module-development.md) cover registry integration, public
ports, worker transfer, schema ownership, standalone Go/npm release verification,
dependency pinning and contract checks. [Operator workflows](module-operations.md)
cover runtime inventory, disablement, affected applications and rollout/recovery.
Report Scheduler is tracked as the next worker-bearing candidate after its
provider/delivery boundaries and worker lifecycle are reviewed.

CI validates catalogue coverage against source directories, publishable
workspaces, production pins/lock and the generated guide. A Go architecture test
checks the catalogue against the actual compiled inventory. Standalone module
checks now discover direct `backend/modules/*/go.mod` packages automatically.

Phase 6 verification passed the full backend suite, vet, standalone module checks
and registry/API/architecture race tests. OpenAPI validation remains current at
366 endpoints, with no public contract changes. Seven catalogue regression tests
and 19 frontend module-source tests passed, along with Compose/environment and
workflow checks. Development npm pins now reference available published releases,
and the shell typechecked against that isolated development installation.
The fresh backend image passed real Keycloak, manifest, backup/restore and restart
checks, including enabled/disabled audit and GeoJSON APIs and continued core audit
recording. Temporary fixture resources were removed. No additional independent
Go package release or live deployment was performed in this expansion step.

### Report Scheduler extraction: lifecycle phase

The scheduler now appears as `report-scheduler` in the compiled module inventory.
Its dependency ports and host adapters were isolated in the first extraction
phase. The registry now owns both its public delivery-download routes and
protected scheduler routes, together with worker initialization and cleanup.
`DISABLED_MODULES=report-scheduler` skips all three.

Initialization requires a handler, worker and successful persistence heartbeat
before any job claims. Cleanup cancels the worker and waits for its active cycle,
subject to the host shutdown deadline. The former bootstrap worker start has been
removed. Existing permissions, download-token checks, rate limits, SQL claims,
retries and migration ownership remain in their current domains.

Verification includes readiness failure, canceled initialization, duplicate
starts, in-flight shutdown, shutdown deadlines, rollback cleanup and route
disablement. The authenticated container fixture also checks scheduler role access,
heartbeat preservation while disabled and a fresh worker when re-enabled.
Standalone packaging, publication and a pinned dependency remain the next phases.

### Report Scheduler extraction: standalone package phase

`backend/modules/report-scheduler` now contains a standalone Go module with public
types, service, worker, dependency ports, Health BI client and PostgreSQL adapters.
HTTP authorization and registry integration remain host responsibilities. Its
dependency graph contains UUID and PostgreSQL libraries and no dashboard imports.

`schema-contract.json` explicitly records host ownership of scheduler migrations
40 through 47 and shared directory/hierarchy reads. No migration files or migration
runner are copied into the package. An isolated check copies only the module out
of the repository, disables Go workspaces, builds/vets it, applies those host-owned
migrations to disposable PostgreSQL and runs race tests. External-package tests
exercise concurrent claims, retries, stale claims and idempotent delivery.

The dashboard continues using its phase-2 implementation until a released
dependency can be selected. The catalogue accurately keeps the runtime stage as
registry-hosted. Phase 4 publishes/verifies an immutable version and replaces the
retained host implementation with package adapters and a reviewed dependency pin.

### Report Scheduler extraction: released dependency integration

Report Scheduler `v0.1.0` is published under the immutable nested module tag
`backend/modules/report-scheduler/v0.1.0`. A clean-cache download verified its
checksums and source commit. The host now selects that version in `backend/go.mod`
and records its checksums in `backend/go.sum`, without a local replace directive.

The retained host scheduling/execution/provider/persistence implementation has
been removed. The feature package supplies aliases and thin constructors, HTTP
handlers and permission/rate-limit adapters. Bootstrap keeps the platform
outbox/user/notification translations. Portable implementation tests live in the
standalone module; host tests exercise HTTP and lifecycle integration against the
selected dependency. Public OpenAPI schema names remain stable across extraction.

The compiled `report-scheduler` inventory reports the selected dependency version,
and deployment bundle metadata includes its exact pin. All schema migrations
remain core-owned. Build a new backend image to consume a release; publishing a
tag does not change a running deployment. The runtime fixture verifies the actual
compiled version, real Keycloak permissions, public/protected route disablement,
stopped heartbeat and worker restart/re-enablement.

Later note: Report Scheduler has since become a self-contained module that owns
its HTTP contract, and the host no longer selects `v0.1.0`; see
[Current status](#current-status) for the selected version.

### Audit extraction: released dependency integration

Audit `v0.1.0` is published under `backend/modules/audit/v0.1.0`, pointing to
release commit `bd1bc5d095eaf5db66fccc108b0f1686defb897c`. A clean download in
the verification environment passed the official Go signed checksum log.
The verified archive was transferred to the host using a temporary file proxy;
SHA-256 transfer integrity and full release-source parity were checked before
pinning. The dashboard selects this version and its verified checksums in
`backend/go.mod` and `backend/go.sum`, without local replacements. No persistent
proxy configuration or checksum-verification exception was introduced.

Host public types and constructors now delegate to the package. SQLC adapters,
optional Redis, HTTP validation/permissions, export limits, core event recording
and migrations retain host ownership. The compiled inventory reports the selected
package version; deployment bundle metadata records the pin.

The live export check exposed an existing PostgreSQL error: nullable export
filters lacked explicit parameter types. The host SQL query now casts text and
UUID filters consistently with the list query, and its bindings were regenerated
with sqlc v1.30.0. JSON exports are verified with both absent and populated
action filters, and CSV exports retain their existing eight columns. The
published portable package did not require a change.

Verification passed with `GOWORK=off`: full backend tests, targeted
Audit/API/bootstrap race checks, vet, standalone package/external-consumer
checks, module checks, all 366 OpenAPI routes, catalogue checks and seven tests,
and all 32 distribution bundle tests.

The real Keycloak distribution fixture passed against a freshly compiled Linux
binary packaged on an existing local backend runtime image. It verified the
actual Audit version, administrator/viewer permissions, a seeded typed Redis
cache hit, JSON/CSV exports, module disablement, continued core event recording,
backup/restore and restart recovery. The host recorder deliberately invalidates
Audit caches when recording events, so the fixture seeds a distinguishable cached
response rather than assuming two unrestricted reads return identical records.

The standard production Dockerfile subsequently built successfully after
outbound access recovered. Docker downloaded and verified all Go modules and
completed the full multi-stage build without Dockerfile changes. The local
`moh-sso-audit-phase4:backend` image is Linux ARM64, built from clean integration
commit `477680dd287262ee1727a7b7288a4a342bf98ca9`, with image ID
`sha256:ff6d4a3948c03bf881481c28bf7f52f1dda11e8b8dbfa3decbbd1697d57389c7`.
Its OCI revision label matches that source commit.

The additional authenticated fixture run against this exact production image
was stopped before backend readiness because Docker's local disk filled. Its
disposable containers/volumes were cleaned up. The earlier complete fixture
against the freshly compiled integration binary remains the runtime validation;
the production-image fixture must be repeated after sufficient Docker disk
space is available. Only disposable Audit images and selected build-cache
records were removed; existing application volumes were not changed.

The integration branch has been pushed to origin. Building or pushing source
does not publish this image to a registry or deploy a running installation.

### GeoJSON extraction: asset boundary phase

GeoJSON is the next extraction after Audit. It already participates in registry
enablement/disablement and has no worker or migration lifecycle. Phase 1
introduces standard-library-only `Asset` and `AssetReader` contracts and a
host directory adapter; bootstrap injects the adapter into the HTTP handler.
The existing directory constructor remains compatible.

The three public layer names and their filenames remain fixed. Authentication
and the existing any-permission gate run in host middleware before asset
access. The HTTP adapter preserves raw GeoJSON media type, cache/nosniff
headers, safe error envelopes, conditional responses and byte ranges, and
closes streams on every completion path.

The runtime remains registry-hosted and reports the host version. Assets stay
in the host Docker image. Portable layer resolution, standalone verification,
publication and selection of a released Go dependency are subsequent phases.

Phase 1 verification passed the full backend test suite, targeted
GeoJSON/API/bootstrap race checks, vet, standalone module checks, catalogue
validation and its seven tests. OpenAPI validation reports all 366 endpoints
current. Regression coverage includes all three permission alternatives, denial
before storage access, actual bundled files, conditional/range requests and
stream closure on failure.

### GeoJSON extraction: portable service phase

Phase 2 moves the fixed public layer-to-filename mapping into `ResolveLayer`
and portable read orchestration into `Service.Open`. The service accepts the
existing asset-reader contract, preserves modification times and transfers
successful streams to its caller. It closes streams returned on failed or
canceled reads.

Unknown names are classified as `ErrUnknownLayer` before storage access.
Reader failures, missing readers/content and cancellation match `ErrLoadFailed`
with stable error text; original causes and close failures remain discoverable
through `errors.Is/As`. The HTTP adapter translates these errors into the
existing 404/500 envelopes and preserves streaming headers, range/conditional
handling and host authorization. A storage error cannot turn a resolved layer
into an HTTP 404.

The service and its reader contract contain only standard-library types. The
host retains directory access, bundled assets, Docker packaging and permission
middleware. The module remains registry-hosted at the host version; standalone
packaging and release selection are subsequent phases.

Phase 2 verification passed an isolated temporary module containing only the
reader contract, service and portable tests: build, vet and uncached race tests
completed with external downloads disabled. The full backend suite, targeted
GeoJSON/API/bootstrap race tests, vet, existing standalone module checks,
catalogue validation and its seven tests also passed. All 366 OpenAPI routes
remain current.

### GeoJSON extraction: standalone package phase

The standalone release candidate at `backend/modules/geojson` declares
`github.com/mohuganda/moh-sso-dashboard/backend/modules/geojson` and Go 1.25.4.
It copies the portable layer/asset contracts and service without changing their
behavior, and adds dependency-version reporting and external-package tests.
There are no third-party dependencies, map assets, storage/HTTP adapters,
migrations or workers in the module. A checksum file is unnecessary until an
external dependency is introduced.

`infra/scripts/test-geojson-package.py` copies only this module outside the
dashboard with an empty module cache, disables workspaces and dependency
downloads, verifies its dependency graph and runs build/vet/uncached race tests.
It also compiles and runs a separate consumer module, checking that public APIs
and local-replacement version reporting work without host files. All dependency
metadata remains read-only. The PR workflow runs the verifier automatically.

The dashboard still uses its retained phase-2 implementation, remains
registry-hosted and reports the host version. It has no GeoJSON dependency pin
or production replacement. Publishing and verifying an immutable release,
selecting its checksums and replacing retained code with adapters are the next
phase. Host assets, directory access, permissions, HTTP serving and Docker
packaging keep their existing ownership.

Phase 3 verification passed the isolated empty-cache/no-download verifier,
uncached race tests, external-package tests and a separately built consumer.
The consumer verified local replacement versions report `dev`, including Go
build metadata containing `(devel)`. Portable source/tests match the retained
host implementation byte for byte. Existing module checks, the full backend
suite, vet, all 366 OpenAPI routes, catalogue validation and its seven tests,
and workflow actionlint checks also passed.

### GeoJSON extraction: released dependency integration

GeoJSON `v0.1.0` is published under the immutable nested module tag
`backend/modules/geojson/v0.1.0`, pointing to release commit
`f6d681c5db790653f91658be3e44772075e6db1f`. A clean-cache official Go proxy
download passed signed checksum verification, release-commit validation and
complete source parity before selecting the host dependency.

The dashboard selects the released dependency and checksums in `backend/go.mod`
and `backend/go.sum`, without a local replacement. The retained host asset
contracts and portable service implementation have been removed. The feature's
`package.go` provides aliases, error sentinels and thin constructors/resolution
functions. Existing host tests now exercise the selected dependency.

Storage, map files, Docker packaging, HTTP streaming/headers and authorization
remain host adapters. All three existing layer names, safe 404/500 envelopes,
range/conditional responses and permission alternatives retain their contracts.
The compiled inventory reports the selected library version; distribution bundle
metadata records the exact pin. Disabling the module omits its routes without
affecting core event recording or unrelated modules.

The full backend suite, targeted GeoJSON/API/bootstrap race checks, vet, module
checks, all 366 OpenAPI contracts, standalone external-consumer verification,
catalogue validation and its seven tests, and all 32 bundle tests passed with
the released dependency selected.

The standard production Dockerfile built the integration candidate successfully
as `moh-sso-geojson-phase4:backend` (Linux ARM64), image ID
`sha256:f93fdc92e0b8328e508862912e6fe7b04a16c79f885ea634b417c509d0c65f4d`.
This review image was built from the working integration tree with `DIRTY=true`.

The full disposable real-Keycloak fixture passed against that image. It checked
the actual selected GeoJSON version, all three packaged nonempty map layers,
GeoJSON/cache/nosniff headers, byte ranges and conditional 304 responses,
authentication, roleless-user denial and existing realm-user map access.
Default realm-user grants intentionally permit map reads; the fixture uses a
separate roleless principal to test denial without changing host policy.

The same fixture passed Audit cache/export checks, application lifecycle review,
backup/restore, clean shutdown/restart recovery, module disablement, continued
core audit recording and scheduler stop/resume. Disposable fixture containers
and volumes were removed on completion. The library release and source push
do not publish this image to a registry or deploy a running installation.

### Announcements extraction: dependency boundary phase

Announcements is the next selected extraction. Storage Locations, Document
Templates and Documents are deferred and unchanged. Phase 1 introduces
feature-owned user, group-member and client-access models, narrow audience and
attachment-storage interfaces, and a single-operation notification publisher.
The service no longer imports users, RBAC, health-context, Keycloak, shared
notification-service or storage implementation packages directly; compatibility
constructors and identity translations live in host adapters.

Bootstrap composes these ports explicitly. Actor-authorized health-context
resolution and stored-audience expansion remain distinct host operations.
Existing enabled-user filtering, fallback group recipients, email deduplication,
client matching, attachment checksums/orphan cleanup and best-effort delivery
are preserved. Context-aware host calls receive request contexts; legacy user
list/detail calls only reject cancellation before starting because their
underlying signatures lack context support.

The feature remains planned/host-wired in the catalogue. SQLC persistence,
configuration, notification payload types, email normalization, HTTP policy and
host delivery workers are not portable or transferred yet. Subsequent phases
must isolate those models/values, register all public/protected/admin routes and
verify disablement together, then build an independent module before release
publication and host pinning.

Phase 1 verification passed targeted Announcements/bootstrap/API race tests,
the full backend suite, vet, existing independent-module checks, catalogue
validation and its seven tests, and all 366 current OpenAPI endpoints.
Audience/storage contracts also built and passed vet outside the host tree with
only the existing UUID dependency and module downloads disabled. Regression
tests exercised actor scope and contextual errors, user/group translation and
filtering, both client identifiers, cancellation before legacy calls, attachment
checksums and cleanup, notification payload forwarding, and permission denial
before dependency access.

### Announcements extraction: portable models and delivery phase

Phase 2 introduces 20 feature-owned persistence model/input types, local enum
values, service inputs and a 53-method repository contract without SQLC or host
model imports. Standard-library nullable values and UUIDs retain existing
null-versus-zero semantics. Explicit host converters preserve every field,
including unknown enum values and nullable values with nonzero inactive data.

A translating repository adapter wraps the existing SQLC implementation.
Underlying SQL queries, transaction handling and repository method bodies
remain unchanged. The constructor now returns the adapter; model translation
is contained in host-only files.

Notifications, deliveries and email attachments now use feature-owned payloads.
The host publisher converts envelopes, channels, recipient/template/payload
maps, schedules, retries and raw/base64/path attachment representations. The
host email normalizer retains existing validation/limits. Publication still
uses best-effort notification forwarding and existing event labels.

The service consumes a non-secret `Settings` snapshot instead of host Config.
Host constructors and configuration-based construction remain available through
adapters; a portable constructor accepts only repository, dependency ports and
settings. The service, attachment orchestration, inputs and contracts now build
without host files. HTTP request/response DTOs and mapping stay in the host.

`infra/scripts/test-announcements-core.py` checks a temporary isolated module's
dependency graph and runs read-only build/vet/uncached race checks with module
downloads disabled. Its tests exercise create/publish, delivery metadata,
repository errors and normalization delegation. The PR workflow runs this check.

Announcements remains planned/host-wired. Registry integration for all route
groups, module-disablement checks, independent module packaging and published
version selection remain subsequent phases. No migration or delivery-worker
ownership changed, and the deferred document/storage modules were not modified.

Phase 2 verification passed the isolated portable-core dependency/build/vet/race
checks, targeted Announcements/bootstrap/API race tests, the full backend suite,
vet, existing module checks, catalogue validation and its seven tests, workflow
actionlint, and all 366 current API contracts. Host regression tests exercised
all 20 model round trips with both nullable validity states, unknown enum
strings, repository context/error propagation, nil/empty collection shape,
notification wire/byte payload parity, normalization limits and non-secret
settings conversion. SQL definitions, migrations and generated SQLC files are
unchanged.

### Announcements phase 3 — registry integration and disablement

Announcements is registry-hosted as `announcements`, reporting the host build
version pending independent packaging. All 26 public, protected and admin
routes are composed through the registry. Legacy registrars run only without
a registry, so disabled modules cannot leak routes through the compatibility
path. Existing permissions and write/publication rate limits stay attached to
the same handlers. Enabled initialization requires a handler; disabled
initialization requires no Announcements dependencies.

`DISABLED_MODULES=announcements` removes all announcement endpoints, including
public listings and both attachment download paths, after restart. Records,
audience mappings and stored attachments remain available when re-enabled.
Announcements owns no worker. Shared notification workers and queued deliveries,
core audit recording and migrations retain their host lifecycle. Disablement
does not cancel previously queued notifications.

The runtime catalogue now marks Announcements registry-hosted. Independent
packaging, external-consumer verification, publishing and host pin selection
remain subsequent phases. No SQL definitions, migrations or generated SQLC
files changed, and deferred document/storage features remain outside this phase.

Phase 3 verification passed the portable-core build/vet/race check, targeted
Announcements/bootstrap/API race checks, the full backend suite, vet, all
packaged-module checks, catalogue validation and its seven tests, and all 366
API contracts. The standard production Dockerfile built successfully. Its
disposable real-Keycloak/PostgreSQL fixture passed publication and permission
checks, CSV attachment uploads and both download paths, disablement of all
three route groups, and re-enablement with records and attachment bytes intact.
Existing manifest lifecycle, database/upload backup restore, core audit
recording and scheduler stop/resume checks also passed.

### Announcements phase 4 — standalone package and external consumer

`backend/modules/announcements` contains the unchanged portable lifecycle,
audience, persistence, attachment and notification contracts/core. It depends
only on the standard library and UUID v1.6.0; its module path is
`github.com/mohuganda/moh-sso-dashboard/backend/modules/announcements`.
It exposes dependency version reporting with `dev` for local replacements.

The package has no HTTP/SQLC/Keycloak/cache adapter, schema migration or worker.
Existing host adapters retain authentication/authorization, storage, SQL
transactions, email normalization, notification execution and core audit.
The dashboard retains its current core until a published version is selected;
the standalone verifier rejects drift between the two portable copies.

The package verifier copies the module outside the host tree with workspaces
and network downloads disabled. A private cache contains only the pinned UUID
artifacts checked against reviewed checksums. It checks dependency graphs,
build/vet/uncached race tests and unchanged metadata. An independent consumer
builds and runs the public constructor, models, lifecycle, best-effort
notifications, attachment streams/checksums, context and error propagation.
CI runs the verifier; `modules-check` automatically discovers the new module.

Announcements remains registry-hosted in the runtime catalogue. Publication,
immutable tag verification, host Go pin selection and replacement of retained
host implementation with adapters are the next phase. No host Go pin, runtime
version, route, migration or worker ownership changed.

Phase 4 verification passed isolated build/vet/uncached race checks with the
exact UUID-only dependency graph, checksum verification, immutable dependency
metadata and the independent public-API consumer. Portable tests cover audience
filtering/deduplication, group fallbacks, health-context scope, lifecycle
metadata, notification best-effort behavior, attachment checksums/orphan cleanup,
configured upload limits and missing email-normalizer behavior. The retained
host-core verifier, full backend suite, backend vet, all packaged-module checks,
all 366 API contracts, catalogue validation/seven tests and workflow actionlint
also passed. Publication and dashboard dependency integration were not performed.

### Announcements phase 5 — publication and pinned host integration

Published `backend/modules/announcements/v0.1.0` upstream from the verified
phase-4 commit `e384437e`. The host selects
`github.com/mohuganda/moh-sso-dashboard/backend/modules/announcements v0.1.0`
with Go checksum verification and no local replacement. Runtime inventory
reports the selected dependency version rather than the host build version.

The retained models, contracts, lifecycle, audience and attachment core have
been removed from the host. Compatibility aliases point to the selected
package; a thin composition wrapper embeds its service. Existing startup-only
setters recompose ports before serving requests. SQLC/postgres operations,
mapping, identity/storage adapters, HTTP permissions/rate limits, normalization,
shared delivery workers, core audit recording and migrations remain host-owned.

Portable business regression tests remain in the independent module. Host
tests retain SQLC/model and wire-payload regression coverage and add public
delegation/compatibility composition checks. CI runs the standalone package
verifier and packaged-module checks. The obsolete retained-core verifier was
removed.

`infra/scripts/test-announcements-release.py` downloads v0.1.0 into a fresh
module cache, verifies reviewed host checksums and absence of replacements,
builds/vets/runs a separate consumer with race instrumentation and checks its
reported dependency version. It compares all shipped Go sources to the release.
This explicit publication check requires network access; the package verifier
continues to run with downloads disabled.

Phase 5 verification passed the fresh-cache published consumer and checksum/
source-parity check; the isolated package build/vet/uncached race check; targeted
Announcements/bootstrap/API race tests; the full backend suite and vet; all
packaged-module checks and 366 API contracts; catalogue validation/seven tests;
32 CD tests and workflow actionlint. The standard production Dockerfile built
successfully using the published dependency. Its disposable real-Keycloak/
PostgreSQL fixture verified inventory v0.1.0, permissions, publication, both
stored attachment download paths, all-group disablement and re-enablement with
records/bytes preserved, backup/restore, core audit recording and scheduler
stop/resume. SQL definitions, migrations and generated SQLC files are unchanged.
The release tag is upstream; publication does not deploy the backend.

### Administrative Units phase 1 — portable contracts and visibility ports

Created `feat/extract-admin-units` from the completed Announcements integration.
Administrative Units remains planned/host-wired. Its five models and nine
repository contracts now live in portable files. Existing PostgreSQL queries
remain in a host adapter; the hierarchy builder moved unchanged from the HTTP
handler to the portable core.

A feature-owned visibility port returns only node code and alias namespace/
external ID. Portable resolution retains DWH alias requirements, namespace/
whitespace/deduplication behavior and the exact UG-descendants exception.
Filtering retains ancestor matching for full/facility rows and direct matching
for simple rows. The host adapts validated selected UUID/scope context and shared
health-context calls, classifies forbidden errors and preserves provider causes.
Existing unselected behavior, all nine routes, error envelopes and hierarchy
semantics remain unchanged. Bootstrap composes the ports explicitly and the
previous constructor remains available for compatibility.

The catalogue now identifies the actual warehouse dependency (the existing DWH
connection), alongside health_context. Database/migration ownership and query
contents are unchanged. No registry module or new worker is introduced.

`infra/scripts/test-admin-units-core.py` builds/vets/race-tests the portable
core in a temporary standard-library-only module with workspaces/downloads
disabled and an empty module cache. CI runs the check. Portable tests cover
hierarchy and visibility behavior; host regressions exercise all nine route
families against allowed/denied/missing mapping and context cases.
Registry integration and disablement checks remain the next phase.

Phase 1 verification passed the isolated standard-library-only build/vet/
uncached race check, targeted Administrative Units/bootstrap/API race tests,
the full backend suite, backend vet, existing packaged-module checks, all 366
API contracts, catalogue validation/seven tests and workflow actionlint.
Model fields/JSON tags and the hierarchy algorithm were compared with the
preceding commit and are unchanged. SQL implementation bodies match after
trailing whitespace cleanup in three moved SELECT lines. Registry
integration, standalone publication and deployment were not performed.

### Administrative Units phase 2 — registry integration and disablement

Administrative Units is registry-hosted as `admin-units`, reporting the host
build version until independently packaged/published. Enabled initialization
requires both its handler and the shared health-context handler. Disabled
initialization and registration are skipped.

All nine `/api/v1/visualizer/adminunits/*` routes now enter through the
registry. Legacy registration is gated on the absence of a registry. Shared
host route-group helpers retain the same Data Statistics system requirement,
health-context selection validation and OR read permissions (report browser,
documents, data quality or surveillance). Unrelated Visualizer routes retain
their registration and guards.

`DISABLED_MODULES=admin-units` removes all nine routes after a backend restart.
The module owns no workers. Shared health-context services, warehouse rows and
schema ownership remain unchanged; re-enablement restores existing lookups.

Unit regressions exercise production-router route parity and disabled 404s,
unrelated Visualizer preservation, required enabled dependencies, host version
inventory, anonymous/system/permission denials, all read alternatives, selected
context requirements/errors and scoped rows. The real authenticated fixture
creates a small warehouse dimension only in its disposable database and checks
all nine reads, context selection, disablement and data parity after restart.
The catalogue now marks the feature registry-hosted. A catalogue negative test
now explicitly marks its malformed entry planned instead of depending on the
first feature's current stage. Standalone packaging remains the next phase.

Phase 2 verification passed the portable-core dependency/build/vet/uncached
race check, targeted Administrative Units/health-context/bootstrap/API race
tests, the full backend suite and vet, all existing packaged-module checks and
366 API contracts, catalogue validation/seven tests and workflow actionlint.
The standard production Dockerfile built successfully. Its disposable real
Keycloak/PostgreSQL fixture passed all nine warehouse reads, system and
authentication denial, required/selected national context, inventory,
disablement and re-enablement with identical data. Backup/restore, core audit
recording and scheduler stop/resume checks also passed. No production SQL,
migrations, dependency pins or worker ownership changed.

### Administrative Units phase 3 — standalone package and external consumer

`backend/modules/admin-units` contains the unchanged portable models,
repository/service contracts, hierarchy and visibility core with their
regression tests. The module path is
`github.com/mohuganda/moh-sso-dashboard/backend/modules/admin-units`; Go 1.25.4
and the standard library are sufficient. A public `BuildHierarchyTree`
wrapper lets host adapters delegate construction after scoping caller-supplied
rows. Dependency version reporting returns dev for local source replacements.

The package owns no HTTP, database/identity adapters, migrations or workers.
The host retains its core until a verified release is selected. The standalone
verifier rejects drift between matching portable source/test files, copies the
package outside the host tree and checks exact module graphs, build/vet/
uncached race tests and unchanged metadata with an empty cache and workspaces/
downloads disabled. A separate consumer module builds/vets/runs public APIs
with race instrumentation, covering all nine lookups, optional filter values,
hierarchy construction/deduplication/nullability, visibility and error causes.
CI runs this check; modules-check discovers the new nested Go module.

Administrative Units remains registry-hosted with the host build version.
Publication, download/checksum verification, host Go pin selection and replacing
retained implementation with adapters remain the next phase. No host Go pin,
runtime routes, SQL, migration or worker ownership changes in this phase.

Phase 3 verification passed the isolated standard-library-only dependency graph,
build/vet/uncached race tests and separate public-API consumer with race
instrumentation, unchanged dependency metadata and retained-core parity checks.
The full backend suite, backend vet, all packaged-module checks, all 366 API
contracts, catalogue validation/seven tests and workflow actionlint also passed.
No version was published or pinned, and no runtime deployment was performed.

### Administrative Units phase 4 — publication and pinned host integration

Published `backend/modules/admin-units/v0.1.0` upstream from the verified
phase-3 commit `d299ea76`. The host selects
`github.com/mohuganda/moh-sso-dashboard/backend/modules/admin-units v0.1.0`
with reviewed Go checksums and no local replacement. Runtime inventory now
reports the selected dependency version instead of the host build version.

Removed the retained host models, repository/service contracts and
implementations, hierarchy algorithm and visibility policy. Compatibility names
refer to published types/functions and the same forbidden sentinel. The HTTP
adapter delegates hierarchy construction after existing row scoping.
Warehouse queries, health-context authorization/selection, HTTP response
mapping, system/read permissions, registration, migrations and lifecycle remain
host-owned and unchanged.

Portable regressions remain in the independent package; host tests retain the
nine-route/adapter coverage and verify type/sentinel identity, public delegation,
request contexts and error causes. The obsolete retained-core verifier and CI
step are removed. The standalone package verifier and modules-check remain in
CI, while CD and real-runtime checks require the v0.1.0 dependency/inventory.

`infra/scripts/test-admin-units-release.py` downloads the published version
into a fresh module cache, validates exact reviewed checksums and the selected
dependency graph, rejects replacements and builds/vets/runs an independent
consumer with race instrumentation and v0.1.0 version reporting. All shipped Go
sources are compared to the release. This explicit release check requires
network access; the isolated package verifier keeps downloads disabled.

Phase 4 verification passed the fresh-cache published consumer and exact
checksum/dependency/source-parity check, isolated package build/vet/uncached
race tests, targeted Administrative Units/health-context/bootstrap/API race
checks, the full backend suite and vet, all packaged-module checks and 366 API
contracts, catalogue validation/seven tests, 32 CD tests and workflow actionlint.
The standard production Dockerfile built successfully with the published pin.
Its disposable real-Keycloak/PostgreSQL fixture verified inventory v0.1.0, all
nine warehouse reads, authentication/system/context guards, disablement and
re-enablement with data parity, backup/restore, core audit recording and
scheduler stop/resume. Warehouse SQL, migrations and worker ownership are
unchanged. The release tag is upstream; publication does not deploy the backend.

### Platform core phase 1 — contracts and dependency boundaries

Created `feat/platform-core` from the completed Administrative Units integration.
`backend/core` now owns the framework-independent compiled-module inventory,
validation and lifecycle contracts/runtime using only the standard library.
The existing `backend/module` package adapts Gin routing and delegates lifecycle
management to core, preserving current constructors, inventory JSON, callbacks,
disablement, dependency order, rollback and shutdown behavior.

`docs/platform-core.md` defines mandatory identity/session, authorization,
application-governance, contextual-policy, audit-recording, operations and
frontend-shell responsibilities, separating shared contracts from optional apps
and provider adapters. This is an incremental host-integrated foundation, not
a newly published package or completed minimal-boot distribution.

Core/adapter architecture checks prevent framework/provider/feature dependencies
from entering the portable core and prevent feature imports into the module
HTTP adapter. A new CI verifier copies the complete core tree outside the host
and builds/vets/race-tests it with an empty cache and downloads/workspaces
disabled. Regressions cover dependency validation, disabled modules, failed/
partial initialization, reverse rollback, cancellation, concurrent idempotent
cleanup, joined causes and detached inventory. Router checks ensure mandatory
core feature IDs cannot enter DISABLED_MODULES.

Identity and access contracts/adapters are next. Bootstrap remains coupled to
its existing databases/providers and legacy workers until an explicit later
core-only profile is implemented and verified. No authentication policy,
provider lifecycle, route, schema, dependency pin or release changed.

Phase 1 verification passed complete-core isolated dependency/build/vet/
uncached race checks; core/module/API/bootstrap race tests; architecture checks;
the full backend suite and vet; all packaged-module checks and 366 unchanged API
contracts; catalogue validation/seven tests and workflow actionlint. The host
inventory view preserves its module.Info schema name and JSON shape. The
standard production image built successfully, and its real Keycloak/PostgreSQL
fixture passed inventory/permissions, application governance, warehouse reads,
attachments, backup/restore, clean shutdown/restart, module disablement/
re-enablement, continued audit recording and scheduler worker stop/resume.
No release was published, deployment performed or minimal-core boot claimed.

### Platform core phase 2 — identity/session and authorization contracts

Added standard-library-only verified-claim identity, generic session-store,
authorization-resolver/access and system metadata contracts to core. No token
payload or storage provider was introduced there. The existing Redis store
satisfies the generic contract while its credential bundle stays host-owned;
the auth handler now uses that contract and shares the same missing-session
sentinel. Cookie names, Redis keys, ID generation, TTL/rotation, fallback
precedence and persistence methods remain unchanged.

The host authz adapter preserves named public response types and converts
resolver decisions without losing metadata or nil/empty collections.
Authentication middleware constructs core identity only after existing
credential verification. Host constructors normalize claims and retain the
existing DB/static selection, role defaults and admin/user flags. Core resolver
errors discard partial grants without inventing fallback access. Existing
authoritative-resolver and disabled-application safeguards remain in place.
Shared predicates preserve exact permission, normalized role and system
matching behavior.

Portable tests exercise verified claims/context forwarding, explicit decisions,
partial-grant rejection and predicate semantics. Adapter tests cover complete
system metadata, nil/empty shapes, host normalization and preserved causes.
Existing session-store and refresh regressions run through the generic
contract. The real Keycloak/Redis fixture additionally checks opaque-session
authentication, bearer precedence, legacy cookies, persisted refresh, logout
deletion/cookie clearing and expiry before the existing module lifecycle checks.

Core-only boot is still a later phase. Application-governance and contextual
policy contracts/adapters are next; no current provider is made optional here.

Phase 2 verification passed isolated whole-core build/vet/uncached race checks
with standard-library-only dependencies, targeted identity/session/authz/
middleware race tests, the full backend suite and vet, all packaged-module
checks, all 366 unchanged API contracts, catalogue validation/seven tests and
workflow actionlint. The standard production image built successfully. Its
real Keycloak/PostgreSQL/Redis fixture passed the new session/credential checks
and the existing permissions, governance, warehouse, attachment, backup/restore,
shutdown/restart, disablement, audit and scheduler recovery checks. No token/
cookie format, schema, published dependency pin, release or deployment changed.

### Platform core phase 3 — application governance and contextual policy

Added portable application registry/metadata, lifecycle and review-authority
contracts. RBAC adapts its authoritative registry to the core descriptor while
retaining its named API DTOs. Existing metadata checks and sequential/terminal
lifecycle rules now live in core. Request normalization, permission catalogue
mapping, reviewed manifests, provisioning, launch/navigation behavior,
enabled-role checks and transactional audit/state persistence stay host-owned.

Added generic contextual-authorization, effective-assignment and hierarchy
contracts. The health-context adapters preserve exact permission/system checks,
their evaluation order, direct/descendant scope selection, original assignment
order, default-National explanations and complete group/node provenance.
Provider errors discard partial grants. Existing alias resolution and selection
middleware retain their HTTP behavior. No warehouse/Observatory integration or
bootstrap/provider/worker wiring changes in this phase.

Core-only startup remains a later phase. No release, dependency pin or deployment
is introduced here.

Phase 3 verification passed isolated whole-core build/vet/uncached race checks
with the standard library only, targeted core/architecture/authz/RBAC/context/
API/bootstrap race tests, the full backend suite and vet, packaged-module
checks, all 366 unchanged API contracts and catalogue validation/seven tests.
Regressions cover the full lifecycle matrix, required review capabilities,
metadata error order, inactive registry metadata, detached role collections,
exact contextual matching, direct/descendant boundaries, assignment ordering,
provider-error propagation and complete host group/node explanations.

The production Docker image built successfully. Its real Keycloak/PostgreSQL/
Redis fixture passed authentication/session checks, reviewed manifest import,
lifecycle activation and idempotence, warehouse/announcement/GeoJSON access,
backup/restore, clean shutdown/restart, module disablement/re-enablement,
continued core audit recording and scheduler worker stop/resume.

### Platform core phase 4 — core-only startup and optional-provider wiring

Added explicit `PLATFORM_PROFILE=full|core` selection with the full distribution
as default. Core composes mandatory identity/session, authorization/governance,
health-context and audit capabilities against primary Postgres, Redis and the
configured Keycloak. It omits optional database pools/replication, file storage,
SMTP/SMS/template providers, channel queues, legacy workers, packaged workers
and optional public/protected/admin/machine-delivery routes. Internal
auth/audit/startup notification records persist without channel delivery.
Unknown profiles and module IDs fail before provider startup.

The existing primary migrations and data are shared across profiles.
All optional modules remain visible but disabled in core. In the full profile
each module, including Documents, Data Quality, Visualiser and Surveillance,
can be disabled on its own; their database and storage providers are still
constructed.
A dedicated backend-only Compose entry point and core environment example
require no optional services, storage mounts or provider secrets.

Core readiness now excludes optional-provider degradation, which stays visible
in aggregate health. Platform readiness checks mandatory dependencies and only
enabled delivery workers, avoiding false failures for disabled workers.
Legacy workers are prepared atomically, cancelled and joined before normal
resource cleanup. Shutdown tests cover completion, timeout/retry, idempotence
and preserved failures. Pools are not explicitly closed underneath workers
that fail to join before their shutdown deadline.

The real-Keycloak fixture gains a core profile and CI coverage for absent/
unconfigured optional providers, opaque sessions, governance/context access,
optional route removal, persistent core audit, absent worker/channel queues
and clean restart. No release, dependency pin, push or deployment is included.

Phase 4 verification passed the full backend suite and vet, profile/bootstrap/
API/health/platform race regressions, the isolated portable-core build/vet/race
check, all packaged-module checks, all 366 unchanged API contracts, catalogue
validation/seven tests, workflow actionlint and core Compose model resolution.
The production Docker build succeeded after a registry TLS retry.

The dedicated core Compose fixture passed real Keycloak sessions, governance/
health-context access, absent optional providers/routes/workers/queues,
continued audit recording and clean shutdown/restart. The full production
fixture passed the existing permissions, reviewed import/lifecycle, warehouse,
attachment, backup/restore, disablement and worker-recovery checks. It also
passed a full -> core -> full switch on the same primary persistence, retaining
reviewed manifests, audit, published announcements and attachment contents and
resuming the scheduler with a fresh worker. The fixture refreshes Docker's
ephemeral host port after restart.

### Published module ownership cleanup

Moved the six published modules' host adapters from `internal/features` to
`internal/adapters/modules`. Portable implementations remain owned by their
versioned `backend/modules` packages, and the host continues consuming the same
published pins and checksums. No module release is required for the host-only
relocation. Shared Metrics service/repository compatibility shims still delegate
to the published package; shared audit event recording stays mandatory.

Updated composition, HTTP imports, development commands and source links.
The catalogue now distinguishes unpublished/core feature trees from published
adapter trees. Tests reject duplicate published feature directories, missing
adapters and moving unpublished features into the published adapter namespace.
Architecture checks require delegation to the published package, reject old
feature imports and retain approved host-feature dependency boundaries for the
moved adapters. Runtime APIs, migrations and module enablement remain unchanged.

Verification passed adapter/API/bootstrap/architecture race tests, the full
backend suite and vet, packaged-module checks, all 366 API contracts and ten
catalogue regressions. OpenAPI changes are limited to internal handler source
locations; public paths, schemas and permission metadata remain unchanged.
The production image built successfully, and the real Keycloak fixture passed
module permissions, reviewed lifecycle, warehouse/attachment access, backup/
restore, disablement, audit/worker recovery and full -> core -> full data
preservation. Published module Go sources, versions and checksums are unchanged.

### Self-contained modules phase 1 — GeoJSON candidate

Started the move from package-backed host adapters to complete module-owned
capabilities. GeoJSON is the first candidate because it owns no database schema
or workers. Its unreleased API now owns HTTP registration, required permission
metadata, authorization/error handling, streaming/ranges/conditional reads,
filesystem overrides and three embedded map snapshots with provenance/hashes.
The existing low-level public service API remains available.

Registration uses a standard-library structural HTTP interface and generic
platform authorization/trace callbacks. Missing authorization is rejected;
unknown provider failures fail closed. Callers cannot bypass access checks with
conditional/range requests. Embedded assets require no dashboard directory.

This is the package-build phase. The dashboard remains pinned to v0.1.0 and
continues using its current adapter/assets. Next: publish the candidate as a new
version, verify its downloaded artifact/checksums, pin it, integrate through
shared platform HTTP/security registration and remove the GeoJSON-specific host
adapter and duplicate assets. No immutable tag, host pin or deployment changes
in this phase. Subsequent modules will take ownership of their HTTP and
persistence implementation while consuming mandatory shared platform contracts;
audit recording and shared migration history remain protected.

Phase 1 verification passed the isolated whole-package build/vet/uncached race
suite with an empty dependency cache and downloads/workspaces disabled. A
separate compiled consumer ran from an empty directory and served embedded
maps through the registration API, checked access denial before conditional
reads, byte ranges and ETag 304s. Checksums match the current host snapshots.
Host backend tests/vet, packaged-module checks, all 366 existing API contracts
and ten catalogue regressions passed. The existing production pin and adapter
remain unchanged pending publication/integration.

### Self-contained modules phase 2 — GeoJSON v0.2.0 release and integration

Published `backend/modules/geojson/v0.2.0` from release commit `2c78cc74`,
verified its public download/checksums in a fresh cache and a separately compiled
HTTP consumer, and selected v0.2.0 in the dashboard without a replacement.
The module includes its HTTP/API contract and all three versioned map snapshots.

Removed the GeoJSON-specific host adapter and duplicate host maps/Docker COPY.
The host now composes the published module with a generic platform HTTP/security/
trace registrar. Verified identity is bridged from existing authentication,
permission checks remain exact and fail closed, mount paths/request IDs are
preserved, and the Gin route surface remains GET-only. No new credential parser
or fallback is introduced. The module owns safe errors, ranges and ETag 304s.

API generation consumes module-owned OpenAPI metadata for the actual registered
route; its GeoJSON operation now documents raw 200/206 content, 304, range and
authorization-service failures. Other endpoint contracts remain unchanged.
Catalogue schema 2 records adapter versus self-contained ownership, rejecting
retained feature/adapter trees for self-contained modules. Boundary refreshes
now target module-owned maps and regenerate their hashes/ETags; selecting new
map bytes requires a new module release. Browser fixtures use those same maps.

The other five published modules retain their current adapters until each
self-contained release is implemented and integrated. No running deployment
is changed by this release or local integration commit.

Phase 2 verification passed the fresh-cache v0.2.0 download and both host
checksums, exact replacement-free dependency graph, complete release artifact
parity and independently compiled embedded-map HTTP consumer. The updated
isolated module suite still uses the standard library only with downloads and
workspaces disabled. Shared registrar/API/bootstrap/architecture race tests,
the full backend suite and vet, all packaged-module checks, 366 API endpoints,
12 catalogue regressions and workflow actionlint passed.

The production image built without loose map files. Real Keycloak/PostgreSQL/
Redis fixtures passed full and core profiles, embedded layers/permissions/ranges/
ETag 304s, reviewed lifecycle, backup/restore, shutdown/restart, disablement,
audit/worker recovery and full -> core -> full data preservation. The Docker
runtime-stage fixture passed, and all 15 surveillance-map browser tests passed
against the module-owned snapshots. The release tag is pushed; the dashboard
integration remains a local feature-branch commit until explicitly pushed.

### Self-contained Administrative Units — package candidate

On `feat/self-contained-admin-units`, the standalone package now owns the nine
existing HTTP endpoints, safe JSON responses and OpenAPI model schemas,
PostgreSQL warehouse queries, visibility mapping and hierarchy construction.
Platform integration uses primitive security/context capabilities, leaving
identity and contextual-policy decisions in the shared core. Caller-owned pools
and externally managed DWH schema remain infrastructure responsibilities.
Malformed POST bodies now fail before querying; bodies are bounded to 1 MiB
by default.

This is a package-build phase. The dashboard remains pinned to v0.1.0 with its
existing adapter. No tag, pin, release-checker version, catalogue ownership or
production deployment changes in this phase. Next: publish v0.2.0, verify
checksums, pin it, extend generic HTTP registration to multiple patterns and
merge OpenAPI component schemas, then delete the Admin Units host adapter and
verify disablement plus real Keycloak/full/core behavior.

Verification includes an empty-cache, downloads-disabled package build/vet/race
suite and two external consumers, plus a disposable read-only PostgreSQL
warehouse for all SQL queries and nine HTTP routes. The PostgreSQL driver is
owned by that separate consumer, preserving the module's standard-library-only
dependency graph.

Phase verification passed: standalone build/vet/race and both external
consumers with downloads disabled; real PostgreSQL read-only queries and all
nine HTTP routes; all independent Go modules; the full host backend test suite
and vet; the unchanged 366-endpoint API contract; 12 catalogue regressions;
workflow actionlint and whitespace checks. The candidate remains local and
unpublished; the production pin and module-disablement behavior are unchanged.

### Self-contained Administrative Units — v0.2.0 release and host integration

Published `backend/modules/admin-units/v0.2.0` upstream at release commit
`ddbc5a31` and selected v0.2.0 in the dashboard with both reviewed Go checksums
and no replacement. A fresh public download passed complete artifact parity,
exact dependency graph and separate portable/HTTP consumers; the HTTP binary
runs outside the source tree from an empty directory.

Removed `internal/adapters/modules/admin_units`, including warehouse SQL,
Gin handlers, DTO aliases and context conversions. The host now composes the
published module directly with its DWH pool and generic platform capabilities.
The shared HTTP registrar mounts all declared method/path patterns while
preserving verified identity, request IDs and the existing nine-route surface.
Administrative Units disablement omits construction and all its routes.

Shared health-context capabilities return primitive node codes and alias lists.
Gin middleware and published-module requests use one selection policy for
assigned users, legacy unassigned requests, active nodes and contextual scope.
Identity comes exclusively from the verified registrar context. No module
models enter the shared capability implementation. System and functional
permissions remain enforced before warehouse access.

API generation now merges module-owned component schemas and rejects schema
collisions. The generated 366-endpoint API includes Administrative Units
nullable DTOs, recursive hierarchy models, request filters and safe error
responses directly from the published package. Catalogue ownership is now
self-contained; architecture checks forbid retaining a host adapter.

The standalone package remains standard-library-only. Its PostgreSQL driver,
warehouse schema, connection pools and shared identity/context services remain
platform infrastructure. No running deployment is changed. Only the release
tag is pushed; dashboard integration remains on
`feat/admin-units-v020-integration` until explicitly pushed.

Integration verification passed: fresh-cache public release/checksums and
complete artifact parity; isolated standalone build/vet/race and external
consumers; read-only PostgreSQL warehouse checks; all independent Go modules;
full backend tests and vet; shared registrar/API/bootstrap/context/architecture
race checks; 366 current API endpoints; 12 catalogue regressions; workflow
actionlint; production Docker build; real Keycloak full and core profiles.
The full fixture verified all nine Admin Units routes, authorization/context
checks, disablement/re-enablement, clean restart, backups, core audit/worker
preservation and full -> core -> full data preservation.

### Self-contained Metrics — package candidate

On `feat/self-contained-metrics`, Metrics now supplies a complete standard HTTP
entry point for all 19 existing endpoints, its OpenAPI operation/model schemas,
and all 32 PostgreSQL repository reads. SQL was extracted from the existing
host query store into package-owned `internal/postgres` reads, retaining bound
parameters and public model/null/array semantics. Query changes are now
maintained with the standalone package. Shared primary schema/migrations and
pool lifetime remain platform infrastructure.

Both verified administrator access and `metrics:read` are mandatory capabilities
before validation or SQL. The module exposes detached route patterns to the
generic registrar and receives shared request IDs through a primitive callback.
The existing Gin handler API remains compatible. Gin/UUID dependencies remain;
PostgreSQL array scanning adds the reviewed lib/pq v1.10.9 dependency. No host
or sibling module imports are required.

The independent verifier builds/vets/race-tests outside the host with workspaces
and downloads disabled using installed checksum-verified third-party modules.
A separately compiled consumer runs from an empty directory, then checks all
32 reads and 19 routes against an isolated, read-only PostgreSQL fixture.
The overview's existing best-effort/partial-result behavior is preserved.

This phase does not publish or push a release or change the dashboard's
v0.1.0 pin, host adapters or module availability. Next: publish v0.2.0, verify
the artifact and checksums, pin it and remove the Metrics-specific
HTTP/repository/service wrappers using shared admin/permission capabilities;
then verify disabled, full and core startup behavior.

Phase verification passed: independent Metrics build/vet/uncached race tests
and HTTP consumer with downloads disabled; PostgreSQL SELECT-only fixture for
all 32 repository methods and 19 HTTP routes; bound filters, UUID/NULL/roles
arrays, cancellation and pool lifetime; all packaged Go modules; the full
backend suite and vet; unchanged 366-endpoint API contract; 12 catalogue
regressions; workflow actionlint and whitespace checks. The candidate remains
local and unpublished.

### Self-contained Metrics — v0.2.0 release and host integration

Published `backend/modules/metrics/v0.2.0` upstream from release commit
`ce93be80` and selected v0.2.0 in the dashboard with both reviewed checksums
and no replacement. The fresh-cache release verifier checks the public module
and dependency versions, verifies sums, builds an independent HTTP consumer
and compares the complete published artifact with the local module.

Removed the Metrics-specific host HTTP/DTO adapters, repository implementation,
service alias and their bootstrap wiring. All 32 Metrics-only SQL definitions
were removed from the host query sources and SQLC output was regenerated.
The published package is now the sole owner of those reads and all 19
endpoints. Platform primary schema/migrations and audit recording are retained.

The host constructs Metrics only when enabled, using the caller-owned primary
pool and shared verified admin/permission/request-ID capabilities. Both
administrator identity and `metrics:read` are required before SQL; raw request
headers cannot create an administrator. The generic registrar preserves the
19 GET routes and path parameters. Module OpenAPI operations and schemas
feed the unchanged 366-endpoint API inventory. Catalogue ownership and
architecture checks now reject retained repository/service/adapter trees.

The existing overview partial-result behavior remains part of the Metrics
contract. No running deployment is changed. Only the release tag is pushed;
the host integration stays on `feat/metrics-v020-integration` until explicitly
pushed.

The isolated fixture verifier now waits for final PostgreSQL TCP readiness
rather than the temporary initialization server. Real-authentication fixtures
cover every Metrics route, a seeded audit subject, complete disablement and
re-enablement. Scheduler lifecycle comparisons use the current worker after
the additional intentional Metrics-disablement restart.

Integration verification passed: fresh public v0.2.0 artifact/checksums,
replacement-free dependency graph and external HTTP consumer; independent
build/vet/uncached race and PostgreSQL checks for all 32 reads/19 routes;
SQLC compile and regeneration freshness; all packaged Go modules; full host
backend tests and vet; API/bootstrap/registrar/architecture race checks;
366 current API operations; 12 catalogue regressions; workflow actionlint;
production Docker build; real Keycloak full and core profiles. The full
fixture verified all Metrics routes, unauthenticated/non-admin denials,
disablement/re-enablement, audit/worker preservation, backups, restarts and
full -> core -> full data preservation. The working integration is committed
locally; no running deployment is changed.

### Self-contained Audit — package candidate

Created `feat/self-contained-audit` from the completed Metrics integration.
Audit now owns its seven HTTP endpoints and PostgreSQL read queries,
pagination/mapping, query validation, cached responses and CSV/JSON exports.
Shared primitive capabilities provide verified admin/permission checks,
request IDs, per-user export rate enforcement and optional stream-error
reporting. The export policy remains 10 requests per minute; the package
requires enforcement rather than silently omitting throttling.

No Gin, SQLC, Redis or database-driver dependency is added: the module retains
only UUID outside the standard library. A private nullable-JSON scanner handles
PostgreSQL metadata without importing the host SQL generator. Core event
recording, primary schema migrations and pool lifetime remain platform-owned.
The host is still pinned to v0.1.0; its current adapter remains for the release
integration phase.

A real PostgreSQL fixture exposed an existing failed-login-series scan mismatch
(timestamp query result versus integer row field). The candidate fixes the scan
and returns public numeric day buckets as Unix milliseconds. Tests cover tied
timestamp pagination, SQL NULL metadata/UUIDs, filters, canceled contexts,
read-only permissions, all seven routes and CSV/JSON manifest integrity.
Next: publish v0.2.0, verify/download/pin it, remove the host read implementation
and verify shared authorization/rate limiting plus full/core disablement without
stopping core audit recording.

Phase verification passed: portable Audit dependency checks; independent HTTP
build/vet/uncached race tests and external consumer with downloads disabled;
SELECT-only PostgreSQL checks for all seven reads/routes, timestamp pagination,
day-bucket conversion, NULL metadata/UUIDs, bound filters and cancellation;
CSV/JSON manifest integrity, large-stream headers, export-rate header precedence
and write-error reporting; all packaged Go modules; full host backend tests and
vet; unchanged 366-operation API contract; 12 catalogue regressions; workflow
actionlint and whitespace checks. No version tag or branch has been pushed.

### Self-contained Audit — v0.2.0 release and host integration

Published `backend/modules/audit/v0.2.0` upstream at `fe1e6f6e` and pinned the
dashboard to v0.2.0 with reviewed checksums and no replacement. Fresh-cache
verification checks the public module, exact dependency graph and artifact
inventory, then builds/runs its separate HTTP consumer. PostgreSQL driver
ownership stays with that consumer/platform, not the Audit read module.

Removed the host Audit read adapter and seven unused read-query definitions;
regenerated SQLC retains core event-writing queries. The existing writer's
implementation is unchanged and its file is now named
`internal/service/audit_recorder.go` to distinguish its required core role.
Shared primary migrations, notifications and audit cache invalidation remain
platform responsibilities.

Only enabled Audit readers are constructed. The published module receives the
primary pool, optional typed Redis cache, verified administrator identity,
`audit:read` checks, request IDs, stream-error reporting and a generic per-user
rate capability. Redis keys keep `rl:audit-export:user:<verified-subject>`.
The export policy remains 10 requests per minute; rate headers override outer
general-policy headers. Unlike the older fail-open Gin rate middleware, this
required capability returns 503 when enforcement is missing/unavailable.
Other existing rate policies retain their behavior.

The seven module-owned OpenAPI operations and nullable/export schemas feed the
366-endpoint API inventory. Catalogue ownership is self-contained and its
adapter-regression test now selects a remaining adapter entry dynamically.
The selected package also fixes failed-login day-bucket scanning and returns
Unix milliseconds for the public numeric field.

Only the release tag is pushed. Dashboard integration remains local on
`feat/audit-v020-integration` until explicitly pushed; no running deployment
is changed by this work.

Integration verification passed: fresh public v0.2.0 artifact/checksums,
replacement-free graph and independent HTTP consumer; standalone PostgreSQL,
pagination/NULL/export integrity and race suites; SQLC compile/regeneration
freshness; all packaged Go modules; full host tests/vet; API/bootstrap/shared
rate-limit/registrar/architecture race checks; 366 current API operations;
12 catalogue regressions; workflow actionlint; production Docker build; real
Keycloak full and core fixtures. Real Redis verified typed cache behavior and
the ten-per-user export limit; all seven read routes disappear when disabled
while core event recording continues. Restart, backup/restore, worker
stop/resume and full -> core -> full data preservation checks passed.

### Self-contained Announcements — phase 1: PostgreSQL persistence

Created `feat/self-contained-announcements` from the completed Audit integration.
The Announcements candidate now owns all 53 persistence operations via
`NewRepository(*sql.DB)`, private SQL records/queries, public-model mapping and
audience replacement transactions. The caller supplies the primary pool and
existing schema; the package neither opens/closes infrastructure nor runs
migrations. Shared group/context rows and SMS notification preferences remain
explicit platform schema prerequisites.

The extracted persistence preserves lifecycle transitions, notification markers,
append-only audience snapshots, attachment visibility/scoping, targeted feeds,
nullable values and SQL error identity. All five replacement transactions
preserve the original audience if a later insert fails. Transaction cleanup
also defers rollback for an interrupted callback.

Added an independent PostgreSQL consumer exercising all 53 operations, with
deduplication, clearing, rollback after partial writes, enum/NULL handling,
attachment deletion/email filters, active/public/targeted feed isolation,
pagination, statistics, cancellation and caller-owned pool checks. Its disposable
database uses a test-only snapshot of the existing schema. Client/user rollback
checks inject failures through test-only triggers because those mappings have
no production identity foreign key. Group/context rollback uses existing foreign
keys; role rollback uses the existing length constraint.

The package and PostgreSQL consumers run outside the source tree with workspaces
and module downloads disabled. Their private caches contain only reviewed UUID
and lib/pq artifacts. Both consumers preserve dependency metadata; the package
verifier also checks the exact dependency graph, build, vet and uncached race
tests. CI now runs the PostgreSQL consumer.

Phase verification passed: independent package and race-enabled PostgreSQL
consumers; all packaged Go modules; full host backend tests and vet; unchanged
366-operation API contract; 12 catalogue regressions; workflow actionlint and
whitespace checks.

The host still selects Announcements v0.1.0; catalogue ownership remains
`adapter`. No release tag, dependency pin or running deployment changes in this
phase. Next: package-owned HTTP routes/DTOs and provider-capability wiring, with
authorization, attachment and delivery behavior checked independently. Publish,
pin and host-adapter removal follow that verification.

### Self-contained Announcements — phase 2: HTTP and provider composition

Created `feat/announcements-http-providers` from persistence commit `cc46893c`.
The candidate now owns all 26 HTTP operations, request/response DTOs, mapping,
attachment streaming and OpenAPI metadata. `NewModule`/`Register` compose the
package repository and service with caller-owned `ServicePorts`, verified actor,
administrator/permission checks, per-user rate enforcement and core audit
recording. HTTP uses the standard library; dependencies remain UUID and lib/pq,
with no Gin or dashboard imports.

Required capabilities reject construction when missing and fail closed on
runtime failure. Write and publish policies retain their names and 60/minute
and 30/minute limits. Access checks precede body parsing/dependency use; policy
headers override outer rate headers. Raw headers cannot supply actor identity.
Identity/groups, context expansion, blob storage, notification delivery and
attachment normalization remain shared supplied capabilities. Core audit and
notification forwarding retain best-effort semantics; no delivery worker or
atomic outbox is introduced.

The extraction exposed existing audience-isolation gaps. Both candidate public
feeds now select only live ALL_USERS announcements. Signed-in feeds filter
verified user/role/client/group/context audiences before pagination, and explicit
role/client selectors cannot grant access beyond verified claims. Protected
downloads check live announcement visibility before opening a blob; admin
downloads retain administrator/permission checks. Missing contextual/group
resolution fails closed. These fixes apply to the candidate; the running host
still uses its reviewed v0.1.0 pin until release integration.

HTTP bodies have a configurable 16 MiB default bound, multipart files are
cleaned up, MIME-encoded download filenames reject header injection and stream
failures are reported without appending JSON. Publish requests accept optional
bodies for both fixed-length and chunked transports. Offset-bearing lifecycle
timestamps are normalized to UTC before PostgreSQL binding, fixing offset loss
in the existing timestamp-without-time-zone columns; existing rows are not
rewritten.

The isolated race-enabled PostgreSQL HTTP consumer verifies all 26 operations,
JSON/multipart uploads, stream download/deletion, scoped preview, audit events,
email/SMS forwarding and eligible/ineligible audience visibility. Unit tests
cover all protected-route denials before dependencies, exact rate policies and
headers, required fields/body bounds, stream failures, metadata inventory and
pagination across private pages. The 53-operation persistence consumer also
checks timestamp-offset preservation. CI runs both PostgreSQL consumers.

Verification passed: isolated package build/vet/uncached race and dependency
graph checks with downloads disabled; both independent PostgreSQL consumers;
all packaged Go modules; full host tests/vet; unchanged 366-operation API
contract; 12 catalogue regressions; workflow actionlint and whitespace checks.
No version is published or pushed in this phase. The host remains pinned to
v0.1.0 with its adapters and catalogue ownership unchanged. Next: publish
v0.2.0, verify its download/checksums and consumer, pin the dashboard, remove host
feature implementations/adapters and verify authorization, optional-provider
wiring and full/core disablement against real Keycloak.

### Self-contained Announcements — v0.2.0 release and host integration

Published `backend/modules/announcements/v0.2.0` upstream at `e79ca827`.
The dashboard selects v0.2.0 with reviewed module/archive checksums and no local
replacement. A fresh-cache verifier checks the checksum service, exact graph,
selected version, complete file inventory/parity, portable consumer execution
and independent HTTP consumer build. Dependencies remain UUID and lib/pq.

Removed the entire host Announcements adapter tree, repository/service
composition objects, SQL query source and generated SQL query implementation.
Regenerated SQLC no longer exposes those feature operations. Primary migrations
and shared identity/context/notification tables remain platform-owned. The
published package owns all 53 persistence operations, lifecycle/audience
orchestration, attachments, 26 HTTP operations, DTOs and OpenAPI metadata.

Only enabled Announcements modules are constructed. The composition root maps
shared provider values through the package's functional capabilities; it retains
no feature handler, persistence adapter type, audience policy or delivery worker.
Storage, contextual expansion and core audit satisfy their interfaces directly.
User/group lookup, notification envelopes and attachment normalization use
composition callbacks; settings contain only non-secret decision values.
Verified actor data comes from the shared registrar/context and identity
directory, never request headers. Core or disabled initialization needs none of
the optional module's providers.

The generic HTTP registrar now mounts mixed public/protected modules from their
own OpenAPI security declarations. Only explicitly anonymous operations are
public; missing/null security stays protected and missing/invalid declarations
reject registration. The host keeps its authentication/audit middleware and the
module enforces administrator/permission and per-user write/publish policies.
These required rate capabilities fail closed; policy headers override general
outer rate headers. The host duplicates no Announcements route list.

The imported module closes the earlier public-active/me-feed audience leak and
checks live announcement visibility before protected attachment downloads.
Administrator download routes still allow management of draft attachments.
The real fixture explicitly checks both behaviors and private-user audience
isolation. Lifecycle timestamp instants are normalized to UTC by package
persistence; existing schema/data are preserved.

Catalogue ownership is now `self-contained`. The 26 package-owned operations
bring modular contract ownership to 62 operations while the complete API remains
366 endpoints. CI verifies the published Announcements artifact in addition to
the isolated package and PostgreSQL/HTTP consumers.

Only the immutable release tag is pushed. The dashboard integration branch is
`feat/announcements-v020-integration`; pushing that branch or deploying it is
separate from publishing the Go module.

Integration verification passed: fresh public v0.2.0 download/checksums, exact
replacement-free graph and complete artifact parity; isolated package and
PostgreSQL/HTTP consumers; all packaged Go modules; full backend tests/vet;
API/bootstrap/shared registrar/architecture race checks; 366 current API
operations; 12 catalogue regressions; workflow actionlint; SQLC regeneration
and whitespace checks; production Docker build; real Keycloak full and
core-only fixtures. Publication, private-audience isolation and attachment
streams passed. Disable/re-enable, backup/restore, restart recovery, worker
stop/resume and full -> core -> full preserve existing data; core-only startup
continues audit recording without optional providers or delivery workers.
