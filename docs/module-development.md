# Developing and expanding distribution modules

Start with [the coverage guide](module-coverage.md). It lists every backend
feature and publishable frontend workspace, its current stage and the next
boundary to resolve. The machine-readable source is `module-catalog.json`.
The catalogue covers code owned by this repository; externally hosted applications
are registered through application manifests and the database application registry.

## Stages and scope

| Stage | Meaning | Version source |
| --- | --- | --- |
| core | Trusted host authentication, authorization or shared platform policy | Backend release |
| planned | Existing feature wiring remains active; extraction work is tracked | Backend release |
| registry-hosted | Compiled registry controls its HTTP availability; implementation still lives in the host | Backend build metadata |
| published | Independent package artifacts are selected through reviewed dependency pins | Selected Go/npm package version |

Audit read APIs and GeoJSON are the first expansion after Metrics. Their
constructors, cache/storage adapters and migration ownership remain in the host.
They require no module workers or cleanup resources. Audit-event recording is a
separate core service and remains active when the audit read module is disabled.
Metrics remains the independent Go-package reference in `backend/modules/metrics`.

Choose a bounded migration rather than moving unrelated domains together.
Worker-bearing domains need their processing lifecycle migrated with their
routes. Report Scheduler is the next worker-bearing candidate in the catalogue;
its provider/delivery ports, job claims and shutdown contract need review first.

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
