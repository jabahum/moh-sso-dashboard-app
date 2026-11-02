# Issue Tracker App

The issue tracker app supports tracking and reviewing data quality issues.

It is a standalone React microfrontend exposed as `@moh-sso/issue-tracker` and mounted by the shell at `/apps/dwh/issue-tracker`.

## Responsibilities

- Display issue tracking workflows for DWH/data quality users.
- Support investigation and resolution of flagged records.
- Integrate with shared API, UI, and type packages.
- Expose single-spa lifecycle functions for shell orchestration.

## Navigation and layout

Issue Tracker is one launcher entry. The workspace side navigation exposes **Issue Summary**
(`/apps/dwh/issue-tracker/dashboard`) and **Registered Issues**
(`/apps/dwh/issue-tracker/issues`). The module root still opens Registered Issues;
existing issue-code query parameters and detail URLs remain supported. Page switching
uses the sidebar without a duplicate tab bar inside the module. Nested sidebar
headings and links align with the other application entries.

Both pages use the shared `PageShell`, consistent action sizes, and responsive
spacing. On narrow screens, the registered-issues table scrolls horizontally in a
keyboard-focusable region instead of compressing its columns.

Registry migration 52 updates existing navigation without changing permissions,
activation, or suspended state. Rollback restores the saved Issue Tracker item
and retains unrelated navigation entries. Seed and Keycloak navigation metadata
use the same single-entry structure for new installations.

## Entry Points

- `src/index.ts`
- `src/routes.tsx`
- `src/single-spa.tsx`

## Development

```bash
npm run dev -w @moh-sso/issue-tracker
npm run build -w @moh-sso/issue-tracker
```
