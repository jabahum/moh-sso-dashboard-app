# Email App

The email app manages email outbox and delivery operations for platform messaging.

It is a standalone React microfrontend exposed as `@moh-sso/email` and mounted by the shell at `/admin/emails`. The shell route requires `email:read`
(`apps/shell/src/app/microfrontends/registry.ts`).

## Responsibilities

- Display email outbox messages.
- Send email immediately or queue it with an optional scheduled time; recipients can be
  direct addresses or RBAC groups, with CC, BCC, a default template or HTML body, and
  file attachments (up to 10 MB each).
- Inspect, retry, and delete email delivery records, individually or in bulk.
- Use shared panel, toast, and table UI patterns.

## Entry Points

- `src/index.ts`
- `src/routes.tsx`
- `src/single-spa.tsx`

## Development

Run from `frontend/`. `npm run dev:mf:email` starts the Vite dev server on port 4105;
`npm run preview:mf:email` serves the built bundle on the same port.

```bash
npm run dev:mf:email
npm run build -w @moh-sso/email
npm run preview:mf:email
```
