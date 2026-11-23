# Announcements App

The announcements app manages platform notices, scheduled announcements, and announcement attachments.

It is a standalone React microfrontend exposed as `@moh-sso/announcements` and mounted by the shell at `/admin/announcements`. The shell route requires `announcements:read`
(`apps/shell/src/app/microfrontends/registry.ts`).

## Responsibilities

- List and manage announcements.
- Create, update, delete, publish, return to draft, schedule, pin, prioritise, and archive
  announcements, with an audience preview.
- Manage announcement attachments used in news feed and email delivery.
- Expose single-spa lifecycle functions for shell orchestration.

## Entry Points

- `src/index.ts`
- `src/routes.tsx`
- `src/single-spa.tsx`

## Development

Run from `frontend/`. `npm run dev:mf:announcements` starts the Vite dev server on port 4103;
`npm run preview:mf:announcements` serves the built bundle on the same port.

```bash
npm run dev:mf:announcements
npm run build -w @moh-sso/announcements
npm run preview:mf:announcements
```
