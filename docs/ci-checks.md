# CI checks and local verification

This page lists the pull request checks that CI runs and how to run the same checks
locally before you push. It is for developers and reviewers.

## Workflow jobs

`PR Quality Checks` (`.github/workflows/pr-check.yml`) runs on pull requests to
`main` and on manual dispatch, and is reused by the image build (`build.yml`)
before publishing. Jobs are independent: a frontend install failure does not
suppress backend, migration, workflow or deployment recovery results.

On pull requests, **Detect changed areas** compares the test merge commit with
its base and skips jobs the change cannot affect. Pushes, manual dispatches and
the image build always run every job.

| Change touches only | Skipped |
| --- | --- |
| Prose: `docs/**/*.md` (except the generated `docs/module-coverage.md`), `documentation/`, `frontend/docs/`, root `*.md` | Every job except Workflow and Shell Lint. The Documentation workflow builds the site. |
| `backend/` | Production Frontend Runtime, Frontend Packed Modules, Frontend Browser Smoke Tests |
| `frontend/` | Backend Module Packages, Database Migration Paths |

Any other path, including `infra/`, `scripts/` and `.github/`, runs everything.

| Job | Coverage |
| --- | --- |
| Distribution installation gate | Reuses `distribution-check.yml` as two parallel jobs. **Build pinned images and verify installation** builds the backend image and a frontend image from published npm pins (`Dockerfile.npm-modules`), verifies image metadata and runs `test-distribution-runtime.py`. **Keycloak authentication against the backend image** runs `test-distribution-auth.py` (full and `--profile core`). |
| Production Nginx contracts | `make compose-config COMPOSE_ENV=gateway` against `infra/env/prod.env.example`, `make infra-check` (including `test-cd.py`), `make infra-nginx-check` and `make infra-runtime-check`. |
| Frontend Quality Gate | Lockfile check before `npm ci`; dependency-guard, npm-release, module-source, runtime-config-path, API-base-URL and client-onboarding tests; OpenAPI bot update tests; version/package audits; local SSO startup contracts; release tooling tests; lint and typecheck; report scheduler parameter/permission tests; `config:validate`, `architecture:check`, `mf:doctor`, `rbac:doctor`; deployment verifier and deployment-selection tests. |
| Backend Quality Gate | With a PostgreSQL 16 service: sqlc compile/generate freshness (pinned, checksum-verified sqlc release binary), Go module downloads, module catalogue checks, `go vet`, platform-core boundary test, `make contracts-check`, `go test ./...`, race-enabled application manifest/registry tests against PostgreSQL, surveillance import tests, platform onboarding examples, Keycloak user-default, platform-scope and realm/client provisioning tests, and backend version consistency. |
| Backend Module Packages | With a PostgreSQL 16 service: independent module downloads, published backend module pins, module release declarations (`module_release.py` tests and `check`), `make modules-check`, and standalone/published package tests for Metrics, Audit, GeoJSON, Administrative Units, Announcements and the Report Scheduler schema. |
| Production Frontend Runtime | `config:prod` plus `build:docker`, then `audit:import-map`, `runtime:verify` and `bundle:budget`. |
| Frontend Packed Modules | Builds workspace packages, then apps three at a time, for `audit:publishability`, `audit:packed` and `test:module-consumption`. |
| Frontend Browser Smoke Tests | Builds and stages a workspace frontend and runs the Chromium browser tests; Playwright browsers are cached by version. |
| Workflow and Shell Lint | Pinned actionlint with ShellCheck integration (`make workflow-check`); ShellCheck also checks every `infra/**/*.sh` script and `infra/server/hooks/*.sh.example`. The retired Kubernetes workflow is archived at `docs/archive/kubernetes-ci.yaml.example`, outside the active workflow directory. Always runs. |
| Database Migration Paths | `make migrations-check`: fresh primary/remote schemas, upgrade from each stream's penultimate migration, clean version tracking, repeat-apply no-op and preservation of existing data. |
| Container Deployment Recovery | `make cd-runtime-check` and `test-cd-runtime.py --mode distribution`: actual deployment transaction against local fixture images/containers; rejected version, automatic/manual rollback, image identity, private env restoration and persistent data. |
| Fast Quality Gate | Passes only when every job above succeeded or was skipped by change detection. This is the one check to require on `main`. |

Other workflows that run on pull requests:

| Workflow | Trigger | Runs |
| --- | --- | --- |
| Documentation (`documentation.yml`) | Changes under `docs/`, `frontend/docs/`, `documentation/` or the workflow itself; also pushes to `main` and manual dispatch | `npm ci` and `npm run build` in `documentation/` on Node 22, then uploads the `documentation-site` artifact. |
| Update OpenAPI contract (`update-openapi.yml`) | Every pull request to `main` | `make contracts-generate`, then `make contracts-check` and the generator tests. Fails if the platform API contract (`backend/core/platform/contract/platform.openapi.yaml`) breaks clients of the base branch's copy (`openapi platform-compat`; see the [contract README](../backend/core/platform/contract/README.md)). For same-repository pull requests it commits a regenerated `backend/docs/openapi.yaml` and platform contract and re-runs the quality checks. |
| Platform API conformance (`platform-conformance.yml`) | Pull requests and pushes to `main` touching the platform API, `backend/core/platform`, backend bootstrap or migrations, the backend image, or the stack | `infra/scripts/test-platform-conformance.sh`: starts Keycloak, Postgres, Redis and the portal backend from `infra/compose/platform-conformance`, runs the platform conformance suite with the Go client (including revocation checks) and removes the stack. See [Platform API, section 11](integrations/platform-api.md#11-checking-your-integration-the-conformance-suite). |
| Helm Lint (`helm-lint.yml`) | Changes under `infra/helm/`; also pushes to `main` | Helm v3.15.0 `helm lint infra/helm/moh-sso` and `helm template` with `values-dev.yaml` and `values-prod.yaml`. |

Require **Fast Quality Gate** on `main` (repository ruleset or branch protection).
It covers every job, so jobs can be split or renamed without editing the
ruleset.
Repository rules are configured separately; editing YAML does not change them.

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

From `frontend/`, stage the same workspace frontend that CI tests:

```bash
FRONTEND_MODULE_SOURCE=workspace npm run build:packages
FRONTEND_MODULE_SOURCE=workspace npm run build:apps
FRONTEND_MODULE_SOURCE=workspace npm run generate:import-map
FRONTEND_MODULE_SOURCE=workspace npm run build:shell
FRONTEND_MODULE_SOURCE=workspace npm run stage:mf
npx playwright install chromium
npm run test:browser
```

CI runs these with `FRONTEND_ASSET_BASE_URL=/` and `FRONTEND_BASE_PATH=/`
after the production build. Playwright (`frontend/playwright.config.mjs`) starts
`infra/scripts/serve-browser-fixture.py` on `127.0.0.1:5417`, which serves
`frontend/dist` with SPA fallback and replaces `/config.js` with a fixture
that sets the API URL to `/ssobackend` (a prefixed service base, which the frontend still supports) and selects remote, orchestrated
microfrontend mode. API/session responses are local fixtures; no real credentials
or application API are used. `scripts/prepare-browser-dependencies.mjs` mirrors the
import map's `esm.sh` shared modules from `node_modules` for the test server, so no
CDN access is needed; production artifacts and import maps are unchanged.

Specs live in `frontend/tests/browser/` and cover shell and portal startup,
navigation and deep links, microfrontend layout and lifecycle, application
manifests, client onboarding, notifications, health context, surveillance pages
and user self-disable. Uncaught browser exceptions, console errors, failed downloads
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
