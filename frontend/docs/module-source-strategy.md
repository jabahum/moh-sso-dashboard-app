# Selecting frontend module versions

Development and production independently choose workspace modules or published npm modules in [`module-sources.json`](../module-sources.json). The shell is always built from this checkout. Development defaults to workspace modules. Production defaults to published npm modules with exact pins and a committed dependency lock.

To use npm in either environment, change that environment's `source` to `"npm"` and set its `versions` map to the exact published version of every app and shared package. The file includes all 23 modules, so development can test new package versions while production keeps older pins. Ranges, tags such as `latest`, missing modules and incompatible internal dependencies are rejected.

```json
{
  "development": {
    "source": "workspace",
    "versions": { "@moh-sso/api": "1.3.2", "...": "..." }
  },
  "production": {
    "source": "npm",
    "lockfile": "module-locks/production/package-lock.json",
    "versions": { "@moh-sso/api": "1.3.2", "...": "..." }
  }
}
```

The example abbreviates the maps; retain every entry in the real configuration. Versions must exist in npm before switching to npm mode. Publishing modules and selecting deployed versions remain separate operations. Source version bumps do not change deployment pins; update pins intentionally after publishing.

Both selections currently reference the verified published `1.3.2` packages (`0.4.2` for report-scheduler), so the development npm override is usable as well as workspace development. They can be changed independently after verifying the selected releases.

## Development

```bash
cd frontend
npm ci
npm run dev
```

The command reads `development.source`, installs missing npm pins when needed, typechecks the shell against the selected package declarations, then starts Vite. To override the configured source for one run:

```bash
FRONTEND_MODULE_SOURCE=npm npm run dev
FRONTEND_MODULE_SOURCE=workspace npm run dev
```

Development Compose uses the same selection. For example, `FRONTEND_MODULE_SOURCE=npm make dev` uses the pinned development modules when the normal development stack starts. A custom `FRONTEND_DEV_COMMAND` should use `node scripts/frontend-modules.mjs dev` to retain automatic preparation.

## Production

```bash
cd frontend
npm run config:prod
FRONTEND_ASSET_BASE_URL=/portal FRONTEND_BASE_PATH=/portal npm run build:docker
```

This reads `production.source`. Workspace mode builds all modules from source. Npm mode installs the production pins, typechecks and builds the shell against their public exports, and stages the installed app and package bundles into the static deployment. The import map and version manifest use the selected versions.

The standard Dockerfile supports both modes:

```bash
docker build -f infra/docker/frontend/Dockerfile .
docker build -f infra/docker/frontend/Dockerfile --build-arg FRONTEND_MODULE_SOURCE=npm .
```

`Dockerfile.npm-modules` defaults to npm mode and uses the same pipeline. Changing pins requires rebuilding and deploying the frontend image. Server runtime environment changes do not change modules inside an existing image.

## Installation and overrides

Npm modules live in independent projects under `.npm-modules/development` and `.npm-modules/production`. They never replace the monorepo's workspace links. Production uses the reviewed `module-locks/production/package-lock.json`, including registry URLs and integrity hashes, with `npm ci`. A missing or stale configured lock fails the build. Existing installations are reused only when the manifest, lock and installed module contracts match. Configurations without a `lockfile` retain the local development installation behavior. Standard npm registry/authentication settings apply, and install lifecycle scripts are disabled.

After selecting newly published production versions, deliberately regenerate and review the lock:

```bash
npm run modules:lock
npm run modules:prepare
```

Commit `module-sources.json` and both files in `module-locks/production/` together. Normal builds never update this committed lock. `Dockerfile.npm-modules` caches the shell toolchain and published module installation in separate layers before copying source. Its preinstalled layer uses the default production lock location; custom selections are still validated by the normal preparation step.

```bash
npm run modules:prepare -- development
npm run modules:prepare -- production
```

`FRONTEND_MODULE_SOURCE=workspace|npm` overrides the selected source. `FRONTEND_MODULE_CONFIG` selects another configuration file relative to `frontend/` (or an absolute path); Docker accepts it as a build argument and development Compose passes it through. `FRONTEND_MODULE_ENV=development|production` overrides which configuration entry is selected. `FRONTEND_MODULE_DIRECTORY` optionally selects a separate installation directory, for example for an isolated test fixture.

`build:all` remains the source-build command used when authoring and publishing packages. Release checks explicitly use workspace modules regardless of deployment configuration. `build:docker` is the environment-configured deployment command. In npm mode, shell imports must use public package exports; private source paths fail instead of falling back to workspace files. Published apps retain dependencies bundled when those apps were released, so select compatible app and shared-package versions together.

## Verification and rollback

```bash
npm run test:module-sources
npm run test:module-consumption
```

Selection tests cover environment independence, exact pins, incompatible dependencies, missing artifacts and workspace-link rejection. The consumption test packs current built modules into a temporary npm fixture, checks Vite development resolution, builds/stages production against those artifacts, and verifies version metadata. It requires existing module builds and does not publish or contact the registry. CI runs it after the source build.

To roll back npm versions, restore the previous pins, rebuild and redeploy. To roll back a complete deployment, redeploy its previous verified image. Keep the selected configuration alongside the release for review.
