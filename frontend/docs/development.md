# Frontend development

This page explains how to set up and run the frontend locally, switch runtime modes and run the checks expected before a pull request. It is for frontend developers.

## Prerequisites

- Node 22 (CI workflows use `node-version: 22`; the frontend Docker images use `node:22-alpine`)
- npm
- Docker, optional

Vite 7 needs Node `20.19+` or `22.12+`. `frontend/.nvmrc` still pins `20.20.1`, which satisfies Vite but is not the version CI builds with; `frontend/package.json` declares no `engines` field.

With `nvm`:

```bash
nvm install 22
nvm use 22
node --version
```

## Install

```bash
cd frontend
npm install
```

## Shell-only development

```bash
npm run dev:shell
```

This runs `scripts/frontend-modules.mjs dev`: it selects the `development` entry of `module-sources.json` (source `workspace`, so apps and packages resolve from this checkout), copies `public/config.development.js` to `public/config.js` (skipped when `FRONTEND_RUNTIME_CONFIG_PREPARED=true`, as in the Docker dev image), and starts the shell on port `3000` at `http://localhost:3000/`. `npm run dev` is the same command.

## Remote microfrontend development

Run shell plus selected app dev servers:

```bash
npm run dev:remote:users
npm run dev:remote:clients
npm run dev:remote:documents
```

Run shell plus all app dev servers:

```bash
npm run dev:remote:all
```

These commands run `config:local-remote` and then `scripts/dev-microfrontends.mjs`, which starts `dev:shell` plus the selected `dev:mf:<app>` servers with `VITE_SINGLE_SPA_ORCHESTRATION=true` and `VITE_MICROFRONTEND_MODE=remote` (unless already set). Because `dev:shell` copies `config.development.js` over `public/config.js` when `FRONTEND_RUNTIME_CONFIG_PREPARED` is not `true`, and values in `config.js` take precedence over `VITE_*` variables, set `FRONTEND_RUNTIME_CONFIG_PREPARED=true` when running these outside Docker if you need the `local-remote` runtime config to stay active.

For true import-map testing of `/single-spa.js`, use build + preview instead of relying on Vite dev to serve library output:

```bash
npm run remote:preview:users
npm run remote:preview:clients
npm run remote:preview:documents
npm run remote:preview:surveillance
npm run remote:preview:all
```

## Docker development

Shell-only (the default `FRONTEND_DEV_COMMAND` is `node scripts/frontend-modules.mjs dev`):

```bash
docker compose -f infra/compose/docker-compose.dev.yml up frontend
```

The container writes `public/config.js` from `FRONTEND_MICROFRONTEND_MODE`, `FRONTEND_SINGLE_SPA_ORCHESTRATION`, `FRONTEND_MICROFRONTEND_MOUNT_MODE`, `FRONTEND_SIDE_PANEL_DOCKING` and `API_BASE_URL` (`infra/docker/frontend/40-runtime-config.sh`) and publishes ports `3000` and `4101-4115` on `127.0.0.1`.

All remote app dev servers:

```bash
FRONTEND_DEV_COMMAND="npm run dev:remote:all" docker compose -f infra/compose/docker-compose.dev.yml up frontend
```

## Runtime config

`public/config.js` is the active runtime config file.

Use these scripts to switch modes:

```bash
npm run config:dev
npm run config:local-remote
npm run config:prod
```

Each script copies `public/config.<mode>.js` to `public/config.js` (`scripts/use-runtime-config.mjs`):

| Script | Source file | `API_BASE_URL` | `microfrontendMode` | `singleSpaOrchestration` | `microfrontendMountMode` |
| --- | --- | --- | --- | --- | --- |
| `config:dev` | `config.development.js` | `http://localhost:9000` | `local` | `false` | `hybrid` |
| `config:local-remote` | `config.local-remote.js` | `http://localhost:9000` | `remote` | `false` | `hybrid` |
| `config:prod` | `config.production.js` | `https://portals.health.go.ug/ssobackend` | `remote` | `true` | `orchestrated` |

Normal development uses local/hybrid mode, which `npm run dev:shell` activates automatically.

If the frontend behaves like production locally, reset it:

```bash
npm run config:dev
```

## Desktop side panels

Side panels dock alongside the page at desktop widths by default. To use overlays for every panel, set
`FRONTEND_SIDE_PANEL_DOCKING=false` in the Docker Compose environment,
`frontend.sidePanelDocking: false` in Helm values, or `sidePanelDocking: false` in the served
`config.js` for static hosting. A panel caller can also use `openPanel({ ..., dockOnDesktop: false })`
for an individual overlay. Small screens and maximized panels use overlays.

Build from the workspace with `FRONTEND_MODULE_SOURCE=workspace` to test the current checkout.
Builds that use npm module pins need a published `@moh-sso/ui` version that includes docking.

## Dependency audit

```bash
npm run audit:packages
```

Run this after:

- adding imports
- moving code between apps/packages
- editing any app/package `package.json`

## Daily checks

Use these before opening a PR or handing off work:

```bash
npm run audit:packages
npm run typecheck
npm run lint
npm run build:shell
```

Run app/package builds when changing shared package exports, app entrypoints, route ownership, or package metadata.

## Ports

- shell: `3000`
- users: `4101`
- clients: `4102`
- announcements: `4103`
- audit: `4104`
- email: `4105`
- documents: `4106`
- surveillance: `4107`
- data-visualizer: `4108`
- issue-tracker: `4109`
- report-browser: `4110`
- e-services: `4111`
- utilities: `4112`
- rbac: `4113`
- data-validation: `4114`
- report-scheduler: `4115`

## Environment variables

The first three are fallbacks: the shell reads `singleSpaOrchestration`, `microfrontendMode` and `microfrontendMountMode` from `window.__APP_CONFIG__` (`public/config.js`) first and uses these variables only when a key is missing (`apps/shell/src/app/microfrontends/orchestrator.ts`).

- `VITE_SINGLE_SPA_ORCHESTRATION`: set to `true` to enable single-spa registration.
- `VITE_MICROFRONTEND_MODE`: `local` uses workspace imports; `remote` uses import maps/runtime imports.
- `VITE_MICROFRONTEND_MOUNT_MODE`: `hybrid` by default; `orchestrated` enables true `registerApplication` mode.
- `VITE_VISUALIZER_API_BASE`: optional override for visualizer and issue-tracker API calls. Use `/visualizer-proxy` (or `/visualizer_proxy`) for local proxying.
- `VITE_VISUALIZER_PROXY_TARGET`: optional Vite dev proxy target. When unset, `/visualizer-proxy` is not created unless `VITE_VISUALIZER_API_BASE` is set.
- `VITE_VISUALIZER_PROXY_COOKIE`: optional cookie header forwarded by the local Vite proxy. Keep this in `.env.local`, not source control.
- `FRONTEND_DEV_COMMAND`: Docker dev command override.
- `FRONTEND_RUNTIME_CONFIG_PREPARED`: when `true`, `npm run dev` / `dev:shell` leaves `public/config.js` untouched.
- `FRONTEND_MODULE_SOURCE`: `workspace` or `npm`; overrides the `source` in `module-sources.json`.

For local-only visualizer proxying, copy `infra/env/frontend.env.example` to `frontend/.env.local` and set the cookie there if needed.
