# Surveillance App

The surveillance app provides disease surveillance dashboards, imports, and related reporting views.

It is a standalone React microfrontend exposed as `@moh-sso/surveillance` and mounted by the shell at `/apps/dwh/surveillance`. The shell route requires the
`data-statistics` system, `systems:launch`, and `surveillance:read`
(`apps/shell/src/app/microfrontends/registry.ts`).

## Responsibilities

- Display surveillance dashboard views.
- Support disease details and surveillance metrics.
- Provide CSV upload/import workflows where available.
- Integrate with Data & Statistics navigation and RBAC permissions.

## Entry Points

- `src/index.ts`
- `src/routes.tsx`
- `src/single-spa.tsx`

## Development

Run from `frontend/`. `npm run dev:mf:surveillance` starts the Vite dev server on port 4107;
`npm run preview:mf:surveillance` serves the built bundle on the same port.

```bash
npm run dev:mf:surveillance
npm run build -w @moh-sso/surveillance
npm run preview:mf:surveillance
```
