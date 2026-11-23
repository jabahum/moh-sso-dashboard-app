# Audit App

The audit app provides administrative visibility into platform activity and security events.

It is a standalone React microfrontend exposed as `@moh-sso/audit` and mounted by the shell at `/admin/audit-logs`. The shell route requires `audit:read`
(`apps/shell/src/app/microfrontends/registry.ts`).

## Responsibilities

- Display audit logs and audit metrics.
- Filter logs by date range, action, module, actor, system (client), user ID, IP address, and
  result.
- Export loaded, selected, or server-filtered audit logs as CSV or JSON.
- Open audit log details in the shared header panel.

## Entry Points

- `src/index.ts`
- `src/routes.tsx`
- `src/single-spa.tsx`

## Development

Run from `frontend/`. `npm run dev:mf:audit` starts the Vite dev server on port 4104;
`npm run preview:mf:audit` serves the built bundle on the same port.

```bash
npm run dev:mf:audit
npm run build -w @moh-sso/audit
npm run preview:mf:audit
```
