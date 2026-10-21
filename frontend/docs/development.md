# Frontend Development

## Prerequisites

- Node `20.19+` or `22.12+`
- npm
- Docker, optional

Vite 7 warns on Node 18. Use Node 20.19+ or 22.12+ for the clean path.

With `nvm`:

```bash
nvm install 20
nvm use 20
node --version
```

If your local `nvm` does not resolve the `20` alias, use an exact installed version that satisfies Vite, for example:

```bash
nvm use v20.20.1
```

## Install

```bash
cd frontend
npm install
```

## Shell-Only Development

```bash
npm run dev:shell
```

This starts the shell on port `3000` and resolves apps through the local workspace.

## Remote Microfrontend Development

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

These commands set remote microfrontend mode for local testing.

For true import-map testing of `/single-spa.js`, use build + preview instead of relying on Vite dev to serve library output:

```bash
npm run remote:preview:users
npm run remote:preview:clients
npm run remote:preview:documents
npm run remote:preview:surveillance
npm run remote:preview:all
```

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

## Environment Variables

- `VITE_SINGLE_SPA_ORCHESTRATION`: set to `true` to enable single-spa registration.
- `VITE_MICROFRONTEND_MODE`: `local` uses workspace imports; `remote` uses import maps/runtime imports.
- `VITE_MICROFRONTEND_MOUNT_MODE`: `hybrid` by default; `orchestrated` enables true `registerApplication` mode.
- `VITE_VISUALIZER_API_BASE`: optional override for visualizer and issue-tracker API calls. Use `/visualizer-proxy` for local proxying.
- `VITE_VISUALIZER_PROXY_TARGET`: optional Vite dev proxy target. When unset, `/visualizer-proxy` is not created.
- `VITE_VISUALIZER_PROXY_COOKIE`: optional cookie header forwarded by the local Vite proxy. Keep this in `.env.local`, not source control.
- `FRONTEND_DEV_COMMAND`: Docker dev command override.

For local-only visualizer proxying, copy `infra/env/frontend.env.example` to `frontend/.env.local` and set the cookie there if needed.

## Runtime Config

`public/config.js` is the active runtime config file.

Use these scripts to switch modes:

```bash
npm run config:dev
npm run config:local-remote
npm run config:prod
```

Normal development uses local/hybrid mode:

```bash
npm run config:dev
npm run dev:shell
```

If the frontend behaves like production locally, reset it:

```bash
npm run config:dev
```

## Docker Development

Shell-only:

```bash
docker compose -f infra/compose/docker-compose.dev.yml up frontend
```

All remote app dev servers:

```bash
FRONTEND_DEV_COMMAND="npm run dev:remote:all" docker compose -f infra/compose/docker-compose.dev.yml up frontend
```

## Dependency Audit

```bash
npm run audit:packages
```

Run this after:

- adding imports
- moving code between apps/packages
- editing any app/package `package.json`

## Daily Checks

Use these before opening a PR or handing off work:

```bash
npm run audit:packages
npm run typecheck
npm run lint
npm run build:shell
```

Run app/package builds when changing shared package exports, app entrypoints, route ownership, or package metadata.
