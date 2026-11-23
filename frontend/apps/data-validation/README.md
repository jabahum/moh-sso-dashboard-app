# Data Validation App

The data validation app runs the data quality assessment (DQA) engine: it manages validation rules, runs data quality checks, and lists flagged records.

It is a standalone React microfrontend exposed as `@moh-sso/data-validation` and mounted by the shell at `/apps/dwh/data-validation`. The shell route requires the
`data-statistics` system, `systems:launch`, and `data_quality:read`
(`apps/shell/src/app/microfrontends/registry.ts`).

## Responsibilities

- Register the tables to validate and load their columns (**Tables** view).
- Create, compile-preview, and delete validation rules (**Rules** view).
- Preview and apply existing rules onto another table (**Rule mapping** view).
- Trigger or schedule runs, cancel scheduled runs, and review flags by severity
  (**Runs & flags** view).
- Use the endpoints under `API.dataValidation.dqa` in `@moh-sso/config`
  (`src/api/dqa.api.ts`).

## Entry Points

- `src/index.ts`
- `src/routes.tsx`
- `src/single-spa.tsx`

## Development

Run from `frontend/`. `npm run dev:mf:data-validation` starts the Vite dev server on port 4114;
`npm run preview:mf:data-validation` serves the built bundle on the same port.

```bash
npm run dev:mf:data-validation
npm run build -w @moh-sso/data-validation
npm run preview:mf:data-validation
```
