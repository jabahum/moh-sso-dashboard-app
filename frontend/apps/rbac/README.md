# RBAC App

The RBAC app manages dynamic, system-aware authorization governance.

It is a standalone React microfrontend exposed as `@moh-sso/rbac` and mounted by the shell at `/admin/rbac`.

## Responsibilities

- Manage systems, roles, permissions, and role templates.
- Manage Keycloak group mappings and explain inherited group access.
- Inspect Keycloak-to-portal sync and drift reports.
- Support RBAC policy simulation and governance workflows.
- Provide administrative UI for system-aware access control.

## Workspace layout

The console opens Application settings and keeps the application selector and task
navigation in view. Switch directly between settings, lifecycle, access roles,
system roles, realm permissions, groups, effective access, sync/drift, and
governance. The outer page stays within the viewport; long results scroll inside
the active workspace. On narrow screens, a section dropdown replaces the task
navigation rail.

Application settings are divided into Basics, Ownership & support, Integration &
environment, Launch & visibility, and Navigation. Draft edits are retained across
sections and saved together. Invalid navigation JSON returns the admin to the
Navigation section. Application fields wait for the selected record to load.

Search the application selector by name or client ID, groups by name/path, and
permission matrices by permission or category. Group details have separate
Members, Role mappings, and Direct permissions views. Governance tasks use a
selector so every task remains reachable without horizontal tab scrolling.

Layout verification uses desktop 1440×900, laptop 1280×720, and mobile 390×844:
check section switching, draft retention, search, reachable actions, and absence of
outer-page overflow. Long collections and small screens retain internal scrolling
so no records or controls are hidden.

## Groups

Keycloak remains the source of truth for groups and group membership. The portal syncs groups into the RBAC cache so admins can explain inherited access, assign portal-only group permissions, and audit group changes.

See [RBAC Groups](../../../docs/rbac-groups.md) for the operator workflow.

## Entry Points

- `src/index.ts`
- `src/routes.tsx`
- `src/single-spa.tsx`

## Development

```bash
npm run dev -w @moh-sso/rbac
npm run build -w @moh-sso/rbac
```
