# Versioning And Releases

The frontend uses a two-lane versioning model so shared packages stay compatible while apps can deploy independently.

## Lane 1: Shared Package Release Train

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
@moh-sso/api@0.2.0
@moh-sso/types@0.2.0
@moh-sso/ui@0.2.0
```

This is configured in `.changeset/config.json` through the `fixed` package group.

## Lane 2: Independent App Versions

Apps version independently because they are future deployable units:

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
- `@moh-sso/surveillance`
- `@moh-sso/users`
- `@moh-sso/utilities`

Examples:

```text
@moh-sso/users@0.4.1
@moh-sso/documents@0.8.0
@moh-sso/surveillance@1.2.0
```

## SemVer Rules

- `PATCH`: bug fix, internal refactor, visual fix, no public contract change.
- `MINOR`: new feature or backward-compatible route/export/runtime behavior.
- `MAJOR`: breaking change to public exports, route ownership, runtime props, API expectations, app-to-app dependency contract, or shared package contract.

## Internal Dependency Ranges

Apps should depend on shared packages with compatible ranges:

```json
{
  "dependencies": {
    "@moh-sso/api": "^0.2.0",
    "@moh-sso/types": "^0.2.0",
    "@moh-sso/ui": "^0.2.0"
  }
}
```

App-to-app dependencies should use stricter ranges:

```json
{
  "dependencies": {
    "@moh-sso/clients": "~0.4.0"
  }
}
```

Shared packages should avoid depending on apps.

## Automatic full release

From the repository root, `make release` generates a Changeset for every frontend app/package, applies all pending bumps, calculates the backend version, commits release metadata, verifies both components, and pushes their tags. PATCH is the default; select `BUMP=minor` or `BUMP=major` when needed. Existing stronger frontend Changesets take precedence. `make release-prepare` prepares and commits versions without publishing. See [releasing frontend and backend](../../docs/releasing.md).

## Changesets Workflow

Install Changesets if it is not available:

```bash
npm install -D @changesets/cli
```

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

## Releasing One App

1. Change the app.
2. Run `npm run changeset`.
3. Select only the affected app and any packages whose public contract changed.
4. Run `npm run release:check`.
5. Run `npm run version:packages`.
6. Build and upload that app bundle to its versioned deployment path.
7. Update the import map to point at the new version.

## Releasing Shared Packages

1. Change one or more shared packages.
2. Run `npm run changeset`.
3. Select the changed shared package.
4. Changesets will keep the fixed shared package group synchronized.
5. Run `npm run release:check`.
6. Run `npm run version:packages`.
7. Publish/upload all shared package artifacts in the release train.

## Version Manifest

Generate:

```bash
npm run generate:versions
```

Output:

```text
frontend/public/version-manifest.json
frontend/dist/version-manifest.json
```

The manifest records current app/package names and versions.

It also records frontend service version, commit, and build time. The service version defaults to the private shell's package version. CI overrides it through `VITE_APP_VERSION`, `FRONTEND_COMMIT`, and `VITE_BUILD_TIME`; the same version and time are embedded in the shell and shared config bundle.

`npm run version:audit` checks every app including the shell, shared release-group alignment, internal dependency compatibility, and lockfile/manifest consistency. `npm run version:packages` refreshes the lockfile and manifest after Changesets applies bumps.

Frontend Docker releases use `frontend/v<shell-version>` tags. The build rejects tags that disagree with the shell's package version and labels images with version, commit, and build time. Branch builds use `<shell-base-version>-dev.<commit>`.

## Versioned Import Maps

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
    "@moh-sso/users": "https://cdn.example.com/mf/users/0.4.1/single-spa.js",
    "@moh-sso/api": "https://cdn.example.com/packages/api/0.2.0/index.js"
  }
}
```

## Rollback

Rollback by pointing the import map back to previous versioned URLs:

```json
{
  "imports": {
    "@moh-sso/users": "https://cdn.example.com/mf/users/0.4.0/single-spa.js"
  }
}
```

Redeploy the import map or shell config. No app rebuild is required for import-map-only rollback.

## CI/CD Outline

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
