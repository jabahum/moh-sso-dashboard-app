# Package publishing

Every frontend app and shared package has npm-style metadata so it can build and be published independently. This page lists the package requirements, dependency rules and publishing commands; it is for frontend developers and release maintainers.

Changesets controls normal version bumps. Do not manually edit app/package versions unless doing an emergency/manual release.

## Package requirements

Each app/package should have:

- `package.json`
- `version`
- `license`
- `exports`
- `publishConfig`
- `dependencies`
- `peerDependencies`
- `files` containing `dist`

Published packages should ship built `dist` artifacts, not source files.

Current versions (each `package.json`):

```text
@moh-sso/<package> (8 shared packages, fixed release train) -> 1.6.0
@moh-sso/<app> (14 apps)                                    -> 1.5.1
@moh-sso/report-scheduler                                   -> 0.6.1
@moh-sso/shell                                              -> private, not published
```

The shared packages are a `fixed` group in `.changeset/config.json`, so they always share one version; apps version independently.

## Dependency rules

- Shared platform dependencies are peer dependencies (each package declares only the ones it uses):
  - `react`
  - `react-dom`
  - `react-redux`
  - `react-router-dom`
  - `single-spa`
  - `single-spa-react`
  - `@carbon/react`
  - `@reduxjs/toolkit`
- Direct `@moh-sso/*` imports are dependencies.
- Type-only packages may be dev dependencies.

Run the audit after import or manifest changes:

```bash
npm run audit:packages
npm run audit:publishability
```

## Lockfile rule

After changing any app/package `package.json`, update the lockfile:

```bash
npm install
```

Commit both `package.json` and `package-lock.json`. Docker uses `npm ci`, so stale lockfiles will fail builds.

## Build a package

```bash
npm run build -w @moh-sso/api
npm run build -w @moh-sso/ui
```

## Build an app

```bash
npm run build -w @moh-sso/users
npm run build -w @moh-sso/documents
```

## Registry and authentication

Configure npm before publishing:

```bash
npm login
npm config set registry <registry-url>
```

Every published app and package sets `"publishConfig": { "access": "public" }`, and `.changeset/config.json` sets `"access": "public"`. The modules are published publicly to the npm registry; only `@moh-sso/shell` is `"private": true`.

## Publish dry run

Run release checks before any publish attempt:

```bash
npm run release:check
npm run audit:publishability
```

```bash
npm publish --dry-run -w @moh-sso/users
```

## Publish

```bash
npm publish -w @moh-sso/users
```

The preferred release command is:

```bash
npm run release:packages
```

## Changesets commands

```bash
npm run changeset
npm run version:packages
npm run release:dry-run
npm run release:packages
```

`@changesets/cli` is already a dev dependency in `frontend/package.json`.

More detail: [Versioning and releases](versioning.md).
