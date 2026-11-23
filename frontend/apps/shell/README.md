# Shell App

The shell app is the frontend composition root for the MOH Integrated Health Portal.

It is the private workspace package `@moh-sso/shell` and owns global routing, layouts, providers, route guards, runtime configuration, and microfrontend orchestration.

## Responsibilities

- Render public, user, and admin layouts.
- Register and mount standalone microfrontend apps.
- Provide auth bootstrap and route guards.
- Load runtime config and import maps in production.
- Compose shared navigation, headers, panels, modals, and toasts.

## Entry Points

- `src/main.tsx`
- `src/app/App.tsx`
- `src/app/routes`
- `src/app/microfrontends` (route and permission registry in `registry.ts`)

## Development

Run from `frontend/`. The shell uses the root `vite.config.ts` and serves on port 3000 under
the `/portal/` base. `npm run dev` (also `npm run dev:shell`) applies the development runtime
config and then starts the shell; `npm run build` builds it.

```bash
npm run dev
npm run dev -w @moh-sso/shell
npm run build -w @moh-sso/shell
```
