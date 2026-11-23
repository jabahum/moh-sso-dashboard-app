# Utilities App

The utilities app hosts self-service utility workflows for portal users.

It is a standalone React microfrontend exposed as `@moh-sso/utilities` and mounted by the shell at `/apps/utilities/self-service` (`/apps/utilities` redirects there). The
shell route requires the `utilities` system and `systems:launch`
(`apps/shell/src/app/routes/user.routes.tsx`).

## Responsibilities

- Provide self-service routes `timesheet` (default), `elearning`, `leave-plan`,
  `absence-requests`, `absence-dashboard`, `eservice/service-access`, and
  `eservice/equipment-request`. Each currently renders a "Coming soon" page.
- The shell, not this app, mounts the documents app at
  `self-service/eservice/document-upload`.
- Integrate with the shell side navigation and RBAC guards.
- Expose single-spa lifecycle functions for shell orchestration.

## Entry Points

- `src/index.ts`
- `src/routes.tsx`
- `src/single-spa.tsx`

## Development

Run from `frontend/`. `npm run dev:mf:utilities` starts the Vite dev server on port 4112;
`npm run preview:mf:utilities` serves the built bundle on the same port.

```bash
npm run dev:mf:utilities
npm run build -w @moh-sso/utilities
npm run preview:mf:utilities
```
