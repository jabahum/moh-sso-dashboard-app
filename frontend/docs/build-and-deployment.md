# Build and deployment

This page lists the frontend build commands, the staged `dist` layout, the Docker images and the supported deployment modes. It is for frontend developers and operators building or hosting the frontend.

## Build commands

### Build steps

Build shared packages:

```bash
npm run build:packages
```

Build standalone apps:

```bash
npm run build:apps
```

Generate import map:

```bash
npm run generate:import-map
```

Build shell:

```bash
npm run build:shell
```

Build everything from the workspace (packages, import map, apps, then the shell with `FRONTEND_MODULE_SOURCE=workspace`):

```bash
npm run build:all
```

### Docker-ready artifact and version metadata

Build the static Docker-ready artifact (uses the `production` entry of `module-sources.json`, whose source is `npm`):

```bash
npm run build:docker
```

Build the same artifact from this checkout instead of the npm pins:

```bash
FRONTEND_MODULE_SOURCE=workspace npm run build:docker
```

Build the same root-based artifact that production Docker uses:

```bash
npm run config:prod
FRONTEND_MODULE_SOURCE=workspace FRONTEND_ASSET_BASE_URL=/ FRONTEND_BASE_PATH=/ npm run build:docker
npm run runtime:verify
npm run config:dev
```

Generate version metadata:

```bash
npm run generate:versions
```

## What `build:docker` does

`build:docker` runs `node scripts/frontend-modules.mjs build`, which selects the `production` entry of `module-sources.json` (overridable with `FRONTEND_MODULE_SOURCE` and `FRONTEND_MODULE_CONFIG`) and sets `NODE_ENV=production`.

With source `workspace` it runs `build:docker:source`:

1. build packages, generate the import map, build apps and build the shell (`build:all`)
2. generate the version manifest
3. stage microfrontends into `dist` (`stage:mf`)
4. inline the generated import map into `dist/index.html`
5. audit staged microfrontend CSS (`audit:mf-css`)

With source `npm` (the default) it first installs the pinned modules into `.npm-modules/production` from `module-locks/production/package-lock.json`, checks the shell against them (`scripts/check-npm-shell.mjs`), and then runs `build:docker:npm-prepared`:

1. generate the import map and the version manifest
2. build the shell
3. stage the installed app and package bundles into `dist` (`stage:mf:npm`) and inline the import map
4. audit staged microfrontend CSS and the import map

## Final `dist` layout

```text
dist/index.html
dist/assets/*
dist/import-map.json
dist/version-manifest.json
dist/mf/<app>/single-spa.js
dist/mf/<app>/root.component-*.js
dist/mf/<app>/*.css
dist/packages/<package>/index.js
```

The shell is served from the root, so the Docker images build with `FRONTEND_BASE_PATH=/` and no copy is made. If `FRONTEND_BASE_PATH` is set to a sub-path (for example `/sub`), staging also copies the artifact into `dist/<base path>/`, for example `dist/sub/mf/<app>/single-spa.js`.

This layout supports single static deployment and prepares remote import-map deployment.

`stage:mf` copies complete app/package build folders, not only `single-spa.js`, so relative chunks and CSS are available to remote imports.

## Docker production

From the repository root:

```bash
docker build -f infra/docker/frontend/Dockerfile .
```

This image is the one `infra/Makefile` builds locally. It runs `npm ci`, `npm run version:audit`, activates the production runtime config and runs `npm run build:docker` with `FRONTEND_ASSET_BASE_URL=/` and `FRONTEND_BASE_PATH=/`. Its `FRONTEND_MODULE_SOURCE` build argument defaults to empty, so the `production` entry of `module-sources.json` applies (source `npm`); pass `--build-arg FRONTEND_MODULE_SOURCE=workspace` to build the apps and packages from this checkout. It serves the final static files on port `3000` and also contains the documentation site under `/docs/`.

During Docker builds, `node scripts/use-runtime-config.mjs production` activates `public/config.production.js` before `build:docker` runs. That production config enables:

- `microfrontendMode: "remote"`
- `singleSpaOrchestration: true`
- `microfrontendMountMode: "orchestrated"`

`config.production.js` sets `API_BASE_URL` to `https://dashboards.health.go.ug/ssobackend`. At container start, `infra/docker/frontend/40-runtime-config.sh` overwrites `config.js` (and `portal/config.js`) from `API_BASE_URL` (default `/api`), `FRONTEND_MICROFRONTEND_MODE` (default `remote`), `FRONTEND_SINGLE_SPA_ORCHESTRATION` (default `true`), `FRONTEND_MICROFRONTEND_MOUNT_MODE` (default `orchestrated`) and `FRONTEND_SIDE_PANEL_DOCKING` (default `true`), unless `config.js` is a read-only mount (as with the Helm ConfigMap).

The shell build keeps `import-map.json` as a static artifact for inspection and tooling. During staging, the generated map is also inlined into `dist/index.html` so browsers can use it without an import-map shim.

`infra/docker/frontend/Dockerfile.nginx` builds the same way:

```bash
docker build -f infra/docker/frontend/Dockerfile.nginx .
```

It serves on port `80`, so compose or Kubernetes service ports must match that image.

## Optional npm module Docker path

Select workspace or npm modules independently for development and production in [module-sources.json](../module-sources.json). See [module selection](module-source-strategy.md) for exact pins and overrides. For a package-versioned deployment, use:

```bash
docker build \
  -f infra/docker/frontend/Dockerfile.npm-modules \
  .
```

`FRONTEND_MODULE_SOURCE` defaults to `npm` in this Dockerfile, and it installs the modules from `module-locks/production/` in a separate cached layer. The CI image build (`.github/workflows/build.yml`) and the distribution check use this Dockerfile. This path builds the shell against pinned public npm exports and stages installed module bundles from an isolated installation. See [module selection](module-source-strategy.md) for development and production configuration.

## Docker development

From the repository root:

```bash
docker build -f infra/docker/frontend/Dockerfile.dev .
```

Run with compose:

```bash
docker compose -f infra/compose/docker-compose.dev.yml up frontend
```

Run all remote dev servers in Docker:

```bash
FRONTEND_DEV_COMMAND="npm run dev:remote:all" docker compose -f infra/compose/docker-compose.dev.yml up frontend
```

## Deployment modes

The frontend can be deployed in three ways.

### Single static deployment

Deploy one artifact containing:

- shell
- staged app bundles
- staged package bundles
- import map

This is the simplest deployment mode.

Build it from the workspace with `FRONTEND_MODULE_SOURCE=workspace`. More detail: [Module Source Strategy](module-source-strategy.md).

### npm package module deployment

Publish apps/packages to npm first, pin them in the `production` entry of `module-sources.json` (and refresh the lock with `npm run modules:lock`), then build an artifact that stages the installed package artifacts:

```bash
npm run build:docker
```

This is the default for `build:docker`, `Dockerfile` and `Dockerfile.npm-modules`.

Use this mode only after published package versions are available in the configured npm registry.

### Remote microfrontend deployment

Deploy shell and apps independently. The import map controls app and package versions.

In this mode:

- shell reads the import map
- apps are hosted separately
- changing an app version can be done by updating the import map

Versioned deployment paths should look like:

```text
/mf/users/1.5.1/single-spa.js
/packages/api/1.6.0/index.js
```

## Static hosting requirements

The static server must:

- serve `dist`
- serve the shell from the root (`/`), with SPA fallback to `/index.html`
- serve `/mf/*` and `/packages/*`
- serve `/config.js`
- serve `/import-map.json`
- redirect legacy `/portal` and `/portal/*` URLs to the matching root path, except `/portal/sw.js`, which must be served as a file (it retires service workers installed under the old `/portal/` scope)

`infra/docker/frontend/nginx.conf` implements these rules.

Before deploying remote/import-map mode, run:

```bash
npm run audit:import-map
npm run runtime:verify
npm run bundle:budget
```

`runtime:verify` confirms that the staged production artifact has:

- `/config.js`
- an inlined import map
- `/mf/*` app bundles
- `/packages/*` shared package bundles
- the `/portal/sw.js` service worker kill switch, and no shell copy under `dist/portal/`
- `version-manifest.json`

`bundle:budget` keeps the current shell and microfrontend bundle sizes from growing silently. The defaults are 8500 KB for the shell entry JS, 1200 KB for shell CSS and 10000 KB for an app chunk. Override thresholds with `BUNDLE_BUDGET_SHELL_JS_KB`, `BUNDLE_BUDGET_SHELL_CSS_KB` and `BUNDLE_BUDGET_APP_CHUNK_JS_KB`, for example:

```bash
BUNDLE_BUDGET_SHELL_JS_KB=9000 BUNDLE_BUDGET_APP_CHUNK_JS_KB=11000 npm run bundle:budget
```
