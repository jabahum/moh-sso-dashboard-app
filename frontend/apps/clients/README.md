# Clients App

The clients app manages Keycloak clients, which represent integrated systems on the platform.

It is a standalone React microfrontend exposed as `@moh-sso/clients` and mounted by the shell at `/admin/clients`. The shell route requires `clients:read`
(`apps/shell/src/app/microfrontends/registry.ts`).

## Responsibilities

- List registered clients/systems.
- Onboard new applications through the guided onboarding panel (requires `clients:write`)
  and update client metadata.
- Enable or disable clients.
- Manage client roles where supported.
- Use shared table, modal, toast, and header panel UI patterns.

## Entry Points

- `src/index.ts`
- `src/routes.tsx`
- `src/single-spa.tsx`

## Development

Run from `frontend/`. `npm run dev:mf:clients` starts the Vite dev server on port 4102;
`npm run preview:mf:clients` serves the built bundle on the same port.

```bash
npm run dev:mf:clients
npm run build -w @moh-sso/clients
npm run preview:mf:clients
```
