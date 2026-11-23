# Report Browser App

The report browser app embeds the Health BI report browser in the portal.

It is a standalone React microfrontend exposed as `@moh-sso/report-browser` and mounted by the shell at `/apps/dwh/dashboards`. The shell route requires the
`data-statistics` system, `systems:launch`, and `report_browser:read`
(`apps/shell/src/app/microfrontends/registry.ts`).

## Responsibilities

- Render the Health BI report browser in an iframe. The URL comes from
  `HEALTH_BI_BASE_URL` in the runtime config (`window.__APP_CONFIG__`), then
  `VITE_HEALTH_BI_BASE_URL`, then `https://dashboards.health.go.ug/report-browser`
  (`src/pages/report-broswer.tsx`).
- Integrate with the Data & Statistics navigation group.
- Expose single-spa lifecycle functions for shell orchestration.

## Entry Points

- `src/index.ts`
- `src/routes.tsx`
- `src/single-spa.tsx`

## Development

Run from `frontend/`. `npm run dev:mf:report-browser` starts the Vite dev server on port 4110;
`npm run preview:mf:report-browser` serves the built bundle on the same port.

```bash
npm run dev:mf:report-browser
npm run build -w @moh-sso/report-browser
npm run preview:mf:report-browser
```
