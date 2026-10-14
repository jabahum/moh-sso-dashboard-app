# Release CI Outline

The repository's **Build & Push Images** workflow publishes source-built frontend images on `frontend/v<shell-version>` tags and verifies their runtime version manifest. Use the root Makefile for verified frontend-only or combined releases; see [Releasing frontend and backend](../../docs/releasing.md). The **Frontend Release Notes** tag workflow publishes per-app/package notes and a combined GitHub Release body. Npm publication remains a separate operation through `make frontend-publish`.

## Validate Job

```bash
npm ci
npm run audit:packages
npm run typecheck
npm run lint
npm run config:prod
FRONTEND_ASSET_BASE_URL=/portal FRONTEND_BASE_PATH=/portal npm run build:docker
npm run audit:import-map
npm run runtime:verify
npm run bundle:budget
npm run audit:publishability
npm run config:dev
```

The local equivalent is:

```bash
npm run release:check
```

## Version Job

```bash
npm run version:packages
```

This applies Changesets version bumps and updates internal dependency ranges.

## Publish Job

```bash
npm run release:packages
```

This publishes npm packages according to Changesets.

## Source Docker Deploy Job

Build the default production image from source:

```bash
docker build -f frontend/Dockerfile frontend
```

This path remains the default production deployment.
It builds all workspace packages/apps from source, stages microfrontend artifacts into `dist/mf` and `dist/packages`, inlines the production import map, and serves the shell as a static app.

## Optional NPM Module Docker Job

After packages are published, build the optional npm-module image with exact package versions:

```bash
docker build \
  -f frontend/Dockerfile.npm-modules \
  --build-arg MOH_SSO_NPM_MODULES="@moh-sso/users@0.4.1 @moh-sso/api@0.2.0" \
  frontend
```

Use this only when package publishing and version pinning are stable.
The npm-module path expects every selected `@moh-sso/*` app/package to have already been published with built `dist` artifacts.

## Deploy Job

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
/mf/users/0.4.1/single-spa.js
/mf/documents/0.8.0/single-spa.js
/packages/api/0.2.0/index.js
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
- `@moh-sso/surveillance`
- `@moh-sso/users`
- `@moh-sso/utilities`

## Rollback Job

Rollback by updating the import map to previous bundle versions.

Example:

```json
{
  "imports": {
    "@moh-sso/users": "https://cdn.example.com/mf/users/0.4.0/single-spa.js"
  }
}
```

Redeploy the import map or shell config and clear cache if required.
