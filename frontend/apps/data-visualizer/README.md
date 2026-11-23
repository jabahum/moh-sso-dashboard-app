# Data Visualizer App

The data visualizer app supports exploratory data viewing and charting for platform datasets.

It is a standalone React microfrontend exposed as `@moh-sso/data-visualizer` and mounted by the shell at `/apps/dwh/data-visualizer`. The shell route requires the
`data-statistics` system, `systems:launch`, and either `documents:read` or `data_quality:read`
(`apps/shell/src/app/microfrontends/registry.ts`).

## Responsibilities

- Display data tables and visualizations.
- Provide data model, period, organisation unit, and visualization controls.
- Support DWH-style analysis workflows.
- Expose single-spa lifecycle functions for shell orchestration.

## Entry Points

- `src/index.ts`
- `src/routes.tsx`
- `src/single-spa.tsx`

## Development

Run from `frontend/`. `npm run dev:mf:data-visualizer` starts the Vite dev server on port 4108;
`npm run preview:mf:data-visualizer` serves the built bundle on the same port.

```bash
npm run dev:mf:data-visualizer
npm run build -w @moh-sso/data-visualizer
npm run preview:mf:data-visualizer
```
