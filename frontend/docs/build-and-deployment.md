# Build And Deployment

## Build Commands

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

Build everything:

```bash
npm run build:all
```

Build the static Docker-ready artifact:

```bash
npm run build:docker
```

Build the same `/portal`-based artifact that production Docker uses:

```bash
npm run config:prod
FRONTEND_ASSET_BASE_URL=/portal FRONTEND_BASE_PATH=/portal npm run build:docker
npm run runtime:verify
npm run config:dev
```

Generate version metadata:

```bash
npm run generate:versions
```

## What `build:docker` Does

`build:docker` runs:

1. build packages
2. generate import map
3. build apps
4. build shell
5. generate version manifest
6. stage microfrontends into `dist`
7. inline the generated import map into `dist/index.html`

## Final `dist` Layout

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

This layout supports single static deployment and prepares remote import-map deployment.

`stage:mf` copies complete app/package build folders, not only `single-spa.js`, so relative chunks and CSS are available to remote imports.

## Docker Production

From the repository root:

```bash
docker build -f infra/docker/frontend/Dockerfile frontend
```

The default production image builds the frontend from monorepo source and serves the final static files on port `3000`.

During Docker builds, `node scripts/use-runtime-config.mjs production` activates `public/config.production.js` before `build:docker` runs. That production config enables:

- `microfrontendMode: "remote"`
- `singleSpaOrchestration: true`
- `microfrontendMountMode: "orchestrated"`

The shell build keeps `import-map.json` as a static artifact for inspection and tooling. During staging, the generated map is also inlined into `dist/index.html` so browsers can use it without an import-map shim.

The nginx image is still available:

```bash
docker build -f infra/docker/frontend/Dockerfile.nginx frontend
```

It serves on port `80`, so compose or Kubernetes service ports must match that image.

## Optional NPM Module Docker Path

The default Docker build remains source-based. For a future package-versioned deployment, use:

```bash
docker build \
  -f infra/docker/frontend/Dockerfile.npm-modules \
  --build-arg MOH_SSO_NPM_MODULES="@moh-sso/users@0.1.0 @moh-sso/api@0.1.0" \
  frontend
```

This optional path builds the shell from source and stages installed `@moh-sso/*` package `dist/` folders from `node_modules`.

## Docker Dev

From the repository root:

```bash
docker build -f infra/docker/frontend/Dockerfile.dev frontend
```

Run with compose:

```bash
docker compose -f infra/compose/docker-compose.dev.yml up frontend
```

Run all remote dev servers in Docker:

```bash
FRONTEND_DEV_COMMAND="npm run dev:remote:all" docker compose -f infra/compose/docker-compose.dev.yml up frontend
```

## Deployment Modes

### Single Static Deployment

Deploy one artifact containing:

- shell
- staged app bundles
- staged package bundles
- import map

This is the simplest deployment mode.

This is also the default source-mode Docker deployment. More detail: [Module Source Strategy](module-source-strategy.md).

### NPM Package Module Deployment

Publish apps/packages to npm first, then build a Docker image that stages installed package artifacts:

```bash
npm run build:shell
npm run generate:versions
npm run stage:mf:npm
npm run audit:import-map
```

Use this mode only after published package versions are available in the configured npm registry.

### Remote Microfrontend Deployment

Deploy shell and apps independently. The import map controls app and package versions.

In this mode:

- shell reads the import map
- apps are hosted separately
- changing an app version can be done by updating the import map

Versioned deployment paths should look like:

```text
/mf/users/0.4.1/single-spa.js
/packages/api/0.2.0/index.js
```

## Static Hosting Notes

The static server must:

- serve `dist`
- serve `/*` for the production shell base path
- provide SPA fallback to `index.html`
- serve `/mf/*`
- serve `/packages/*`
- serve `/config.js`
- serve `/import-map.json`

Before deploying remote/import-map mode, run:

```bash
npm run audit:import-map
npm run runtime:verify
npm run bundle:budget
```

`runtime:verify` confirms that the staged production artifact has:

- `/portal/config.js`
- an inlined import map
- `/portal/mf/*` app bundles
- `/portal/packages/*` shared package bundles
- `version-manifest.json`

`bundle:budget` keeps the current shell and microfrontend bundle sizes from growing silently. Override thresholds with:

```bash
BUNDLE_BUDGET_SHELL_JS_KB=9000 BUNDLE_BUDGET_APP_CHUNK_JS_KB=11000 npm run bundle:budget
```
