# E-Services App

The e-services app is the product module for electronic service workflows.

It is a standalone React microfrontend exposed as `@moh-sso/e-services` and mounted by the shell at `/apps/eservices`. The shell route requires the `eservices` system and
`systems:launch` (`apps/shell/src/app/routes/user.routes.tsx`).

## Responsibilities

- Provide iHRIS service pages under `ihris/`: meeting manager (the default landing page),
  action tracker, clinician outputs, leave and absence management, workplans, budget
  tracker, and activity reporting. These pages currently render empty states only.
- Show "coming soon" pages for `partner-management` and `observatory-uploads`.
- Integrate with shell routing and RBAC guards.
- Expose single-spa lifecycle functions for future independent deployment.

## Entry Points

- `src/index.ts`
- `src/routes.tsx`
- `src/single-spa.tsx`

## Development

Run from `frontend/`. `npm run dev:mf:e-services` starts the Vite dev server on port 4111;
`npm run preview:mf:e-services` serves the built bundle on the same port.

```bash
npm run dev:mf:e-services
npm run build -w @moh-sso/e-services
npm run preview:mf:e-services
```
