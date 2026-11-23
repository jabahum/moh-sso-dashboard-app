# Users App

The users app manages platform user accounts and access assignments.

It is a standalone React microfrontend exposed as `@moh-sso/users` and mounted by the shell at `/admin/users`. The shell route requires `users:read`
(`apps/shell/src/app/microfrontends/registry.ts`).

## Responsibilities

- List and filter users.
- Create and update user accounts.
- Enable, disable, and reset passwords for users.
- Manage dynamic RBAC-based user access through the user access panel.
- Use shared table, modal, panel, and toast UI patterns.

## Entry Points

- `src/index.ts`
- `src/routes.tsx`
- `src/single-spa.tsx`

## Development

Run from `frontend/`. `npm run dev:mf:users` starts the Vite dev server on port 4101;
`npm run preview:mf:users` serves the built bundle on the same port.

```bash
npm run dev:mf:users
npm run build -w @moh-sso/users
npm run preview:mf:users
```
