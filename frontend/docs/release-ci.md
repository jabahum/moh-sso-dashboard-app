# Release CI outline

This page outlines the CI jobs that validate, version, publish, deploy and roll back the frontend. It is for release maintainers and anyone changing the release workflows.

The repository's **Build & Push Images** workflow (`.github/workflows/build.yml`) builds the frontend image from `infra/docker/frontend/Dockerfile.npm-modules`, which stages the `@moh-sso/*` releases pinned in `module-sources.json` (`production`) rather than building them from source. It pushes images tagged with the short commit SHA (plus `latest` on the default branch, and the version on `frontend/v<shell-version>` tags) and verifies the runtime version manifest and bundled documentation. Use the root Makefile for verified frontend-only or combined releases; see [Releasing frontend and backend](../../docs/releasing.md). The **Frontend Release** tag workflow verifies source builds and packed packages, publishes all 23 public modules (8 shared packages and 15 apps) to npm, verifies exact versions in the registry, then publishes per-module notes and a combined GitHub Release body. The private shell is excluded. Configure the upstream `NPM_TOKEN` secret or trusted publishers as described in [the release guide](../../docs/releasing.md#publish-frontend-npm-modules). `make frontend-publish` remains available for manual bootstrap.

## Validate job

The **Frontend Release** workflow validates with `npm run release:dry-run` (`scripts/check-frontend-release.mjs`), which runs the versioning and runtime-config tests and then `npm run release:check` while preserving `public/config.js`, `public/import-map.json` and `public/version-manifest.json`. `release:check` runs:

```bash
npm run config:validate
npm run architecture:check
npm run mf:doctor
npm run rbac:doctor
npm run deploy:doctor
npm run audit:packages
npm run version:audit
npm run typecheck
npm run lint
npm run config:prod
FRONTEND_MODULE_SOURCE=workspace FRONTEND_ASSET_BASE_URL=/ FRONTEND_BASE_PATH=/ npm run build:docker
MF_CSS_AUDIT_TARGET=staged npm run audit:mf-css
npm run audit:import-map
npm run runtime:verify
npm run bundle:budget
npm run audit:publishability
npm run audit:packed
```

The local equivalent is:

```bash
npm run release:check
```

`release:check` leaves the production runtime config active; run `npm run config:dev` afterwards (or use `release:dry-run`, which restores the files).

## Version job

```bash
npm run version:packages
```

This applies Changesets version bumps and updates internal dependency ranges.

## Publish job

```bash
npm run release:packages -- --no-git-tag
npm run release:verify-npm
```

The tagged CI workflow runs these commands after release verification to publish all 23 public npm apps/packages and confirm their versions on npm; the private shell is excluded. Rerunning the workflow skips versions already published. Use `npm run release:dry-run` to run release checks without registry writes or tags. Packed-consumer checks extract actual npm tarballs into an isolated temporary project, reuse installed third-party dependencies offline, verify export targets and declaration imports, and compile a consumer including a negative type assertion.

## Source Docker deploy job

Build a production image from source:

```bash
docker build -f infra/docker/frontend/Dockerfile --build-arg FRONTEND_MODULE_SOURCE=workspace .
```

Without the build argument this Dockerfile follows `production.source` in `module-sources.json` (currently `npm`). With `workspace` it builds all workspace packages/apps from source, stages microfrontend artifacts into `dist/mf` and `dist/packages`, inlines the production import map, and serves the shell as a static app.

## Optional npm module Docker job

After packages are published, set exact production pins in `module-sources.json`, refresh `module-locks/production/` with `npm run modules:lock`, and build the npm-module image:

```bash
docker build \
  -f infra/docker/frontend/Dockerfile.npm-modules \
  .
```

`FRONTEND_MODULE_SOURCE` defaults to `npm` in this Dockerfile. This is the image CI publishes.
The npm-module path expects every selected `@moh-sso/*` app/package to have already been published with built `dist` artifacts.

## Deploy job

Generate version metadata:

```bash
npm run generate:versions
```

Generate a versioned import map:

```bash
FRONTEND_ASSET_BASE_URL=https://cdn.example.com FRONTEND_VERSIONED_IMPORTS=true npm run generate:import-map
```

Then upload app and package bundles to versioned paths:

```text
/mf/users/1.5.1/single-spa.js
/mf/documents/1.5.1/single-spa.js
/packages/api/1.6.0/index.js
```

Deploy:

- shell static files
- import map
- `version-manifest.json`
- app/package bundles

Standalone app bundles currently include:

- `@moh-sso/announcements`
- `@moh-sso/audit`
- `@moh-sso/clients`
- `@moh-sso/data-validation`
- `@moh-sso/data-visualizer`
- `@moh-sso/documents`
- `@moh-sso/e-services`
- `@moh-sso/email`
- `@moh-sso/issue-tracker`
- `@moh-sso/rbac`
- `@moh-sso/report-browser`
- `@moh-sso/report-scheduler`
- `@moh-sso/surveillance`
- `@moh-sso/users`
- `@moh-sso/utilities`

## Rollback job

Rollback by updating the import map to previous bundle versions.

Example:

```json
{
  "imports": {
    "@moh-sso/users": "https://cdn.example.com/mf/users/1.5.0/single-spa.js"
  }
}
```

Redeploy the import map or shell config and clear cache if required.
