# Developing and expanding distribution modules

This page is for backend and frontend developers who move features into compiled
registry modules or independently published packages. Start with [the coverage guide](module-coverage.md). It lists every backend
feature and publishable frontend workspace, its current stage and the next
boundary to resolve. The machine-readable source is `module-catalog.json`.
The catalogue covers code owned by this repository; externally hosted applications
are registered through application manifests and the database application registry.

## Stages and scope

| Stage | Meaning | Version source |
| --- | --- | --- |
| core | Trusted host authentication, authorization or shared platform policy | Backend release |
| planned | Existing feature wiring remains active; extraction work is tracked | Backend release |
| registry-hosted | Compiled registry controls routes and owned workers; implementation still lives in the host | Backend build metadata |
| published | Independent package artifacts are selected through reviewed dependency pins | Selected Go/npm package version |

### Current expansion examples

Every compiled registry module is `published` and `self-contained`: the six
released ones (`metrics`, `audit`, `geojson`, `report-scheduler`,
`announcements`, `admin-units`) and the six extracted onto `core/modulekit`
(`storage-locations`, `visualiser`, `document-templates`, `data-quality`,
`documents`, `surveillance`), which are built from the repository until tagged. each one's handlers, OpenAPI metadata and persistence live in
`backend/modules/<module-id>`, and no host adapter remains. Primary-schema
migrations stay with the host. Audit-event recording is a separate core service
and remains active when the Audit read module is disabled. Metrics remains the
independent Go-package reference in `backend/modules/metrics`.

Choose a bounded migration rather than moving unrelated domains together.
Worker-bearing domains need their processing lifecycle migrated with their
routes. Report Scheduler follows Metrics as an independently versioned Go
module in `backend/modules/report-scheduler`. It owns its public and protected
HTTP routes and OpenAPI contract. The dashboard pins the module in
`backend/go.mod` (currently at a pseudo-version rather than a release tag; see
[Current status](modular-distribution.md#current-status)) and supplies authenticated context, per-IP rate limiting,
platform services, and schema migrations. The registry manages worker
readiness, startup, and shutdown.

Portable implementation tests live in the module. Host integration tests cover
HTTP authorization, queue conversion, and registry integration. Verify changes
with `make -C backend modules-check` and
`python3 infra/scripts/test-report-scheduler-package.py`.

## Add a compiled registry integration

1. Record the feature's dependencies, worker owner, migration owner and next work
   in `docs/module-catalog.json`. Shared core capabilities stay explicit host
   dependencies. Runtime `Requires` refers only to other registered modules.
2. Keep constructor/dependency wiring in `internal/bootstrap` until an independent
   package boundary exists. Add its definition in `internal/api/modules.go`, with
   a stable lowercase module ID and the correct version source.
3. Validate required dependencies during initialization. Disabled definitions
   must skip initialization, routes and owned worker startup. Preserve the
   existing protected/admin group, contextual access checks, permission middleware,
   route order, rate limits, response bodies and audit behavior.
4. Remove the feature's unconditional route registration. Retain the existing
   legacy fallback only where the router is deliberately constructed without a
   registry. Production bootstrap always supplies its initialized registry.
5. Add missing-handler, permission and enabled/disabled route tests. Verify that
   shared capabilities remain available to other consumers. Keep independent
   packaging as a separate status until it has been completed and verified.

The registry validates dependency ordering and enabled dependencies before
startup. The host initializes modules before starting its remaining background
workers. A module acquiring resources must return a cleanup callback, including
after partial initialization failure. Startup failure cancels acquired module
contexts and cleans up in reverse order. Normal shutdown cleans up modules before
closing host database connections.

## Transfer workers and schema ownership

A module initializer must finish required readiness checks before returning.
Workers must observe the module context. Their cleanup must cancel work, join
all goroutines and respect the supplied shutdown deadline. Handle asynchronous
worker failure and readiness recovery explicitly; simply launching a goroutine
does not establish readiness. Remove the corresponding host worker startup only
when the module owns that entire lifecycle, preventing duplicate job claims.

Existing primary migrations and Observatory migrations retain their current
owners and version streams. Route integration alone does not transfer tables.
Before extracting a writing domain, document each table's owner, migration stream,
locking/order, dirty-state handling and supported upgrade/rollback behavior.
Keep core migrations before module initialization. Never replay a copied migration
under a new version stream against tables already owned by the core.

## Extract an independent Go package

Use `backend/modules/metrics` as the constructor, repository-port, route and version
reporting reference. Public signatures cannot import this host's `internal`
packages. Inject repository, cache, authorization, audit, storage and provider
ports as needed. Keep generated SQLC types behind host adapters and preserve public
JSON contracts. The host supplies authentication, contextual policy and rate limits.

Create a module-level `go.mod` and tests that run without the host implementation.
`make -C backend modules-check` discovers direct module directories and runs vet
and tests with `GOWORK=off` and read-only dependency metadata. Add API source
resolution support if an extracted handler is no longer in the host source tree.

For local integration, use an ignored Go workspace from `backend`, for example
`go work init . ./modules/metrics`. Verify production consumption separately with
`GOWORK=off`; production must resolve a released dependency without a filesystem
replacement. Release directory-prefixed tags such as
`backend/modules/<module-id>/v<version>` only after reviewing and committing the
standalone package. Verify a separate consumer can download and use that exact
version, then update host `go.mod`/`go.sum` and mark the catalogue entry published.

## Verify published backend versions

Update the exact module pin in `backend/go.mod` and commit both checksums
recorded in `backend/go.sum`. Run
`python3 infra/scripts/backend_module_pins.py` with the backend Go toolchain
available. It reads every published `backend/modules/*` requirement in
`backend/go.mod` (a release version or a pseudo-version), checks the selected
dependency, rejects replacements, and compares the downloaded module and
manifest checksums with the reviewed values. Distribution metadata, runtime
inventory checks, and standalone release fixtures all read the expected
versions from the backend pins.

A module extracted from the host but not yet tagged is built from the
repository instead: `backend/go.mod` requires it at `v0.0.0` and replaces it
with `./modules/<module-id>` (the directory name must equal the module ID).
`local_modules()` in `infra/scripts/backend_module_pins.py` lists these; they are
excluded from pin verification, report the version `dev` in the module
inventory, and the backend Dockerfiles copy their `go.mod` before downloading
dependencies. To publish one, tag `backend/core` first if the module needs a
newer core, tag the module, then replace the local `replace` line with the
reviewed pin and checksums.

## Published implementation ownership

After publication, a backend module must not retain an implementation directory
under `backend/internal/features`. Its versioned implementation belongs in
`backend/modules/<module-id>`; the dashboard consumes its reviewed version and
checksums without a local replacement.

No published module currently uses `integration: adapter`
(`backend/internal/adapters/modules` contains only a README); the rules below
apply if one is added. For modules using `integration: adapter`, dashboard HTTP authorization/
routes, SQLC mappings and shared provider adapters belong under
`backend/internal/adapters/modules/<feature-id>`. These transitional adapters
delegate domain behavior to the selected package. Core event recording,
primary migrations, shared identity/health-context and provider lifecycle stay
with the host when they are shared capabilities.

The catalogue checks this layout for every published module. Architecture
tests reject retained feature trees/imports, require each adapter to use its
published package, and continue enforcing approved cross-feature dependencies
for adapters. Keep module-owned algorithms and lifecycle policy in the package,
not in a second host feature implementation.

## Self-contained module integration

A `published` module with `integration: self-contained` owns its handlers,
models, persistence implementation, assets and module-specific workers/schema
where applicable. Its host integration is registration/dependency composition,
without an `internal/features` or `internal/adapters/modules` implementation.
Catalogue schema 2 records this distinction explicitly and checks ownership.

Use standard-library HTTP handlers and shared platform callbacks rather than
requiring application-specific wrappers. `internal/modulehttp` translates the
structural HTTP registration contract into Gin once for all modules, preserving
the existing authentication context, request ID, route mount and declared
method surface. A module must require authorization; the shared callback reads
only verified context and fails closed. Headers do not become identity claims.

Modules can provide `OpenAPI() []byte` as a structural contract. API generation
merges their declared operations only for actual registered routes, alongside
existing typed Gin operations. Every module supplies this method.
Keep the published operation metadata and actual permission checks aligned.

Review and commit the package, publish an immutable directory-prefixed tag,
verify the downloaded artifact/checksums from a fresh cache and an independent
consumer, then pin it and remove the host-specific adapter and asset copies.
Shared platform infrastructure and migration history remain host-owned unless
an explicit module migration transition has been reviewed.

## Build a module on core/modulekit

`backend/core/modulekit` is the shared contract for modules extracted from the
host. It is standard-library only, so a module depending on it gains no
framework or host dependency. `backend/modules/storage-locations` is the
reference.

The host authenticates the caller and attaches the verified request state
(`modulekit.WithRequest`: access, request ID and the selected health context)
in `internal/modulehttp`. A module declares its operations and the router
enforces each declaration before the handler runs, with the same status codes
and error bodies as the host middleware:

```go
router, err := modulekit.NewRouter([]modulekit.Endpoint{
	{Method: "GET", Path: "/storage-locations/{id}", Permission: ReadPermission, Handle: h.getByID},
	{Method: "POST", Path: "/uploads", Permission: WritePermission,
		RateLimit: &modulekit.RateLimit{Name: "uploads-write", Limit: 30, Window: time.Minute},
		Handle: h.upload},
}, modulekit.WithLimiter(deps.Limiter))
module, err := modulekit.NewHTTPModule(router, openAPIDocument) // embedded openapi.json
```

| Requirement | Field | Failure |
| --- | --- | --- |
| No authentication | `Public` | none |
| Authenticated caller | any non-public endpoint | 401 `UNAUTHORIZED` |
| Administrator | `Admin` | 403 `admin access required` |
| System access | `System` | 403 `system access required: <system>` |
| Permissions | `Permission`, `AnyPermissions`, `AllPermissions` | 403 with the host middleware's message |
| Rate limit | `RateLimit` (host `modulehttp.PolicyLimiter`) | 429 `RATE_LIMIT_EXCEEDED` |

Handlers receive a `*modulekit.Context` with request helpers (`Param`, `Query`,
`BindJSON`, `FormFile`, ...), the verified caller (`Access`, `Subject`,
`HealthContext`) and the platform response envelope (`OK`, `Fail`). `BindJSON`
applies `binding:"required|omitempty|oneof=..."` tags like the host framework.

Core services reach modules as contracts in `Dependencies`, adapted by the host:

| Contract | Host adapter |
| --- | --- |
| `modulekit.Notifier` | `internal/adapters/modulekit.Notifier` over the notification service |
| `modulekit.Platform` | `internal/adapters/modulekit.Platform(cfg)` |
| `modulekit.HealthContextDirectory` | `health_context.ModuleDirectory` |
| `core.AuditRecorder` | the shared audit service |
| `modulekit.Limiter` | `modulehttp.PolicyLimiter` |

Anything else a module needs from the host (a database handle, a file-storage
provider, a directory lookup) is a narrow interface the module declares in its
own package. Register the module in `internal/api/modules.go`; use
`module.Routes.ContextScoped` for routes that require the data-statistics
system and a health-context selection. Copy the module's existing operations
into `openapi.json` so the generated contract stays unchanged.

## Frontend packages and hosted applications

Existing frontend applications export their single-spa lifecycle through public
npm exports. Shared packages must preserve provider/state identity across the
shell and apps. Build and verify publishable exports before selecting their
released versions. Production uses exact pins in `frontend/module-sources.json`
and a committed npm dependency lock. Source version bumps do not automatically
change deployment pins.

After selecting released versions, run `npm run modules:lock` from `frontend`,
review the lock and commit it with the pins. Add every new publishable workspace
to the catalogue and both environment selections. Keep the shell private and
built from the checkout. For separately hosted applications, follow
`examples/manifests/README.md`; they can be registered without adding a workspace
or making a new backend package.

## Checks and review

```sh
python3 infra/scripts/check-module-catalog.py --write
python3 infra/scripts/check-module-catalog.py
python3 infra/scripts/test-module-catalog.py
go -C backend test ./internal/architecture ./internal/api/...
make -C backend modules-check
go -C backend run ./cmd/cli openapi validate
```

Regenerate OpenAPI through the existing CLI when a deliberate public contract
change requires it. Pure route migration should preserve the existing inventory
and response schemas. Run affected domain tests, registry lifecycle/race tests
and integration checks appropriate to the changed boundary. Worker/schema changes
also require failure, recovery and shutdown coverage.

Commit each reviewed migration independently on a feature branch. Include the
catalogue and regenerated coverage guide with each stage change. CI rejects
untracked backend features, missing frontend packages, invalid relationships,
stale generated coverage and disagreement with the compiled module inventory.
