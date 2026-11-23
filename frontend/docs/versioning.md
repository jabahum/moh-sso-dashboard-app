# Versioning and releases

The frontend uses a two-lane versioning model so shared packages stay compatible while apps can deploy independently. This page describes the two versioning lanes, SemVer and dependency-range rules, the Changesets release workflow, the version manifest and import-map rollback. It is for frontend developers and release maintainers.

## Lane 1: shared package release train

Shared packages release together with the same version:

- `@moh-sso/api`
- `@moh-sso/auth`
- `@moh-sso/config`
- `@moh-sso/microfrontend`
- `@moh-sso/state`
- `@moh-sso/types`
- `@moh-sso/ui`
- `@moh-sso/utils`

When any shared package has a public change, release all shared packages together.

Example:

```text
@moh-sso/api@1.6.0
@moh-sso/types@1.6.0
@moh-sso/ui@1.6.0
```

This is configured in `.changeset/config.json` through the `fixed` package group.

## Lane 2: independent app versions

Apps version independently because they are deployable units. `@moh-sso/shell` is versioned (its version is the frontend release version) but is `"private": true` and never published to npm:

- `@moh-sso/shell`
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

Examples:

```text
@moh-sso/users@1.5.1
@moh-sso/documents@1.5.1
@moh-sso/report-scheduler@0.6.1
```

## SemVer rules

- `PATCH`: bug fix, internal refactor, visual fix, no public contract change.
- `MINOR`: new feature or backward-compatible route/export/runtime behavior.
- `MAJOR`: breaking change to public exports, route ownership, runtime props, API expectations, app-to-app dependency contract, or shared package contract.

## Internal dependency ranges

Apps should depend on shared packages with compatible ranges:

```json
{
  "dependencies": {
    "@moh-sso/api": "^1.6.0",
    "@moh-sso/auth": "^1.6.0",
    "@moh-sso/ui": "^1.6.0"
  }
}
```

App-to-app dependencies should use stricter ranges:

```json
{
  "dependencies": {
    "@moh-sso/clients": "~1.5.1"
  }
}
```

Shared packages should avoid depending on apps.

## Automatic full release

From the repository root, `make release` generates a Changeset for every frontend app/package, applies all pending bumps, calculates the backend version, commits release metadata, verifies both components, and pushes their tags. PATCH is the default; select `BUMP=minor` or `BUMP=major` when needed. Existing stronger frontend Changesets take precedence. `make release-prepare` prepares and commits versions without publishing. See [releasing frontend and backend](../../docs/releasing.md).

## Changesets workflow

`@changesets/cli` is a dev dependency of `frontend/package.json`.

Create a changeset:

```bash
npm run changeset
```

Run release checks:

```bash
npm run release:check
```

`release:check` validates architecture, package boundaries, versions, TypeScript, lint, source-built Docker artifacts, staged `/portal` import-map coverage, production runtime artifacts, bundle budgets, and npm publishability metadata.

Apply version bumps:

```bash
npm run version:packages
```

Dry-run publish:

```bash
npm run release:dry-run
```

Publish:

```bash
npm run release:packages
```

Do not publish until release checks pass.

## Releasing one app

1. Change the app.
2. Run `npm run changeset`.
3. Select only the affected app and any packages whose public contract changed.
4. Run `npm run release:check`.
5. Run `npm run version:packages`.
6. Build and upload that app bundle to its versioned deployment path.
7. Update the import map to point at the new version.

## Releasing shared packages

1. Change one or more shared packages.
2. Run `npm run changeset`.
3. Select the changed shared package.
4. Changesets will keep the fixed shared package group synchronized.
5. Run `npm run release:check`.
6. Run `npm run version:packages`.
7. Publish/upload all shared package artifacts in the release train.

## Version manifest

Generate:

```bash
npm run generate:versions
```

Output:

```text
frontend/public/version-manifest.json
```

The staging step (`stage:mf` / `stage:mf:npm`) copies it to `frontend/dist/version-manifest.json`. App and package versions come from each `package.json` in workspace mode (the default for `generate:versions`), or from the installed modules matching the `production` npm pins when `FRONTEND_MODULE_SOURCE=npm`.

The manifest records current app/package names and versions.

It also records frontend service version, commit, and build time. The service version defaults to the private shell's package version. CI overrides it through `VITE_APP_VERSION`, `FRONTEND_COMMIT`, and `VITE_BUILD_TIME`; the same version and time are embedded in the shell and shared config bundle.

`npm run version:audit` checks every app including the shell, shared release-group alignment, internal dependency compatibility, and lockfile/manifest consistency. `npm run version:packages` refreshes the lockfile and manifest after Changesets applies bumps.

Frontend Docker releases use `frontend/v<shell-version>` tags. The build rejects tags that disagree with the shell's package version and labels images with version, commit, and build time. Branch builds use the shell's package version as the build version and are tagged with the short commit SHA (plus `latest` on the default branch); only `frontend/v*` tag builds get the version tag (`.github/workflows/build.yml`, `infra/scripts/image_version.py`).

## Versioned import maps

Default import map output uses static local paths:

```text
/mf/users/single-spa.js
/packages/api/index.js
```

Versioned mode uses package versions and a CDN/base URL:

```bash
FRONTEND_ASSET_BASE_URL=https://cdn.example.com FRONTEND_VERSIONED_IMPORTS=true npm run generate:import-map
```

Example output:

```json
{
  "imports": {
    "@moh-sso/users": "https://cdn.example.com/mf/users/1.5.1/single-spa.js",
    "@moh-sso/api": "https://cdn.example.com/packages/api/1.6.0/index.js"
  }
}
```

## Rollback

Rollback by pointing the import map back to previous versioned URLs:

```json
{
  "imports": {
    "@moh-sso/users": "https://cdn.example.com/mf/users/1.5.0/single-spa.js"
  }
}
```

Redeploy the import map or shell config. No app rebuild is required for import-map-only rollback.

## CI/CD outline

1. install dependencies
2. run package dependency audit
3. run typecheck
4. run lint
5. build packages/apps/shell
6. stage Docker/import-map artifacts
7. audit staged import-map coverage
8. run Changesets version/publish
9. upload bundles to versioned CDN paths
10. generate import map
11. deploy import map and shell config
