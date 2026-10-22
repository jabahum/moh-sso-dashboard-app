# CI checks and local verification

`PR Quality Checks` runs on pull requests to `main` and is reused by the image
build before publishing. Jobs are independent: a frontend install failure does
not suppress backend, migration, workflow or deployment recovery results.

| Job | Coverage |
| --- | --- |
| Frontend Quality Gate | Lockfile URLs before installation, dependency guard tests, package/version declarations, lint/types, architecture, RBAC, scheduler and release tooling. |
| Backend Quality Gate | Go vet/tests, API contract freshness and backend version consistency. |
| Fast Quality Gate | Combined frontend/backend result; retains the existing required-check name while the jobs run independently. |
| Production Frontend Runtime | Complete production build, CSS/import-map/runtime/bundle checks, then Chromium smoke tests. |
| Production Nginx contracts | Compose/environment/CD unit contracts, real Nginx routing and production runtime fixture tests. |
| Workflow and Shell Lint | Pinned actionlint with ShellCheck integration; deployment `.sh` scripts and the backup-hook example. The commented legacy `ci.yaml` is excluded. |
| Database Migration Paths | Fresh primary/remote schemas, upgrade from each stream's penultimate migration, clean version tracking, repeat-apply no-op and preservation of existing data. |
| Container Deployment Recovery | Actual deployment transaction against local fixture images/containers; rejected version, automatic/manual rollback, image identity, private env restoration and persistent data. |

Existing branch protection requiring **Fast Quality Gate** continues to work: its
combined result fails unless both independent jobs succeed. Add the other job
names as required checks after their first successful run if desired. Repository
rules are configured separately; editing YAML does not change branch protection.

## Fast dependency guards

From `frontend/`:

```bash
node scripts/check-lockfile.mjs       # Node only; run before npm ci
npm ci
npm run test:dependency-guards
npm run audit:packages
```

The lockfile check validates canonical public npm registry paths, package
versions and integrity metadata. It accepts npm aliases and workspace links.
It does not contact the registry or replace `npm ci`'s download/integrity checks.
Other HTTPS registries are allowed; plaintext download URLs and embedded
credentials are rejected.

The import audit uses the TypeScript parser to inspect ordinary imports,
side-effect imports, re-exports and literal dynamic imports. Comments and string
contents are not treated as imports. Package stylesheet subpaths must resolve
after installation; the full build still validates the remaining module paths.

## Workflows and shell scripts

Install ShellCheck using your platform package manager, then install the pinned
workflow checker with Go:

```bash
go install github.com/rhysd/actionlint/cmd/actionlint@v1.7.11
# Ensure the Go bin directory is on PATH.
make workflow-check
```

[actionlint](https://github.com/rhysd/actionlint) checks Actions expressions,
workflow structure and embedded shell commands. ShellCheck also checks the
deployment entrypoints, launchers, bootstrap and backup example directly.

## Browser smoke tests

From `frontend/`, build the same `/portal` deployment layout used by CI:

```bash
npm run config:prod
FRONTEND_ASSET_BASE_URL=/portal FRONTEND_BASE_PATH=/portal npm run build:docker
npx playwright install chromium
npm run test:browser
```

Playwright starts a dedicated server on `127.0.0.1:5417`, serves built assets
with SPA fallback, and overrides only the runtime API URL to `/ssobackend`.
API/session responses are local fixtures; no real credentials or application API
are used. The production import map and shared CDN modules execute unchanged,
so outbound access to the configured CDN is required. This deliberately covers
the deployed browser runtime, rather than replacing its shared libraries.

Tests cover public startup, the users/clients microfrontends, navigation and a
deep-route reload. Uncaught browser exceptions, console errors, failed downloads
and HTTP errors fail the tests. Each test gets a fresh browser context.
Traces/screenshots are retained on failure and uploaded by CI for seven days.
Reports live in ignored `frontend/playwright-report` and `frontend/test-results`.
These tests do not replace a staging Keycloak login/cookie test.

## Database and deployment recovery fixtures

```bash
make migrations-check              # Docker, Go from backend/go.mod
make cd-runtime-check              # Docker and Python 3.9+
```

Migration checks create a uniquely named PostgreSQL 15 container with temporary
storage and an ephemeral localhost port. Tagged Go integration tests create/drop
their own databases for the primary and remote migration streams. They never
read `app.env` or the application's normal database connection settings.
The upgrade baseline is the previous migration version, not an arbitrary
historical production backup; release-specific backup/restore rehearsal remains
part of staging validation. Irreversible down migrations are not exercised.

Recovery checks use the production `Server` transaction, preflight, health/version
and frontend-resource verification, snapshots, locks, state and rollback methods.
Only registry transport maps approved fixture references to exact local image IDs,
and the Compose project name is made unique. Tiny fixture servers replace the
application and database; this checks deployment orchestration, not business logic
or real GHCR/SSH permissions. Public HTTPS routing is covered by the Nginx suite.

Both suites clean up their own containers, networks, volumes and fixture image
tags. They do not stop application stacks or prune shared Docker resources.
CI has a running Docker daemon; start Docker Desktop before running locally.
