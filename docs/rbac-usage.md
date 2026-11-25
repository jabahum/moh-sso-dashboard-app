# RBAC usage

The dashboard uses dynamic, system-aware RBAC. This page sets out the rules developers follow when protecting backend routes, frontend routes and launcher entries, and how administrators manage and debug access.

## Source of truth

The frontend reads access from the `user` object returned by `GET /api/v1/auth/me`:

- `permissions`
- `systems`
- `accessibleSystems`
- `realmRoles`
- `clientRoles`

Realm roles and client roles are inputs. Permission checks are the preferred authorization model.

Keycloak clients are the source candidates for portal systems. Use `system-rbac sync-keycloak` against live Keycloak, or use `--realm-export infra/keycloak/realm-export.json`, to draft/apply portal systems from Keycloak clients and client roles. Portal RBAC then maps those discovered roles to application permissions.

For the full operator workflow, including startup sync, new client onboarding, permission mapping, and troubleshooting, see [RBAC And Keycloak Sync](./rbac-keycloak-sync.md). For group-based assignments and inherited access behavior, see [RBAC Groups](./rbac-groups.md).

## Backend rules

Backend routes must enforce permissions with middleware from `backend/internal/middleware`.

Use:

- `RequirePermission(permission)` for one required permission.
- `RequireAnyPermission(...)` when several read permissions can open a shared API.
- `RequireAllPermissions(...)` when all listed permissions are required.
- `RequireSystem(systemClientID)` for system-scoped modules.
- `RequireSystemRole(systemClientID, role)` only when behavior is truly role-specific.

Do not protect new APIs with raw role checks unless the business rule is specifically role-based.

## Frontend rules

Use helpers from `@moh-sso/auth`:

- `hasPermission`
- `hasAnyPermission`
- `hasAllPermissions`
- `hasSystem`
- `hasSystemRole`
- `canLaunchSystem`
- `getAccessibleSystems`

Use `PermissionRoute` for shell route boundaries. Use `PermissionGuard` for page actions and buttons.

## Microfrontend metadata

Every standalone app route should declare access metadata in `frontend/apps/<app>/src/routes.tsx`.

Example:

```ts
export const surveillanceRoute: MicrofrontendRoute = {
  appName: "@moh-sso/surveillance",
  path: "/apps/dwh/surveillance",
  requiredAnyPermissions: ["surveillance:read"],
  requiredSystems: ["data-statistics"],
};
```

The shell uses this metadata to guard routes. `npm run rbac:doctor` checks that route metadata exists.

## System launching

App launcher and side navigation should use:

- `systems:launch`
- `accessibleSystems`

A Keycloak client should not appear only because it exists. It must be mapped through portal RBAC and assigned to the user.

Selected modules within an accessible system may appear as separate launcher cards when their `ui.navigation` item sets `displayInLauncher: true`. They do not become separate systems and do not receive separate Keycloak clients. The shell filters those cards using `permission`, `requiredPermissions`, and `requiredAnyPermissions`, while route guards continue to enforce access when a URL is opened directly. See [System Onboarding](./system-onboarding.md#launch-and-navigation-model) for the complete metadata contract.

## Admin management UI

Portal RBAC can be managed from:

```text
/admin/rbac
```

The route is registered as `/admin/rbac` (`frontend/apps/rbac/src/routes.tsx`). The page is exposed by the `@moh-sso/rbac` frontend app and calls the backend admin API under:

```text
/api/v1/admin/rbac
```

Required permissions (route guards in `backend/internal/features/rbac/routes.go`):

- `rbac:read` opens the page and the read APIs, including drift, sync preview, import preview, effective access, export and policy simulation. `audit:read` also opens the RBAC audit list.
- `rbac:write` registers systems, updates system metadata, applies sync and imports, and creates or decides RBAC change requests.
- `rbac:roles:write` creates, updates and removes system roles and access roles, updates user access, manages groups, group members and group realm/system roles, and lists or decides access requests.
- `rbac:permissions:write` maps permissions to system roles, realm roles and groups, edits permission metadata, and runs bulk permission changes.
- Application manifest validate, preview and import require the admin role plus `rbac:read`, `rbac:write` and `rbac:roles:write`. Lifecycle transitions are checked in the service: configuring needs `rbac:write` and approving needs `rbac:roles:write`.

Use this UI for routine role and permission mapping after a Keycloak client has been onboarded. Use the CLI seed/sync flow for bulk initialization, repeatable environments, and drift checks.

The RBAC page also includes an Effective Access section. Use it to search by user ID, username, or email and explain:

- realm roles from Keycloak
- client roles from Keycloak
- group memberships from Keycloak
- group-derived realm roles, client roles, and portal permissions
- resolved portal permissions
- accessible systems
- the role or system role that granted each permission

Group-derived access is read-only from the user access panel. Change inherited access by editing the Keycloak group, group membership, or portal group permission mapping.

## Debugging

Use the backend CLI to explain expected access. `system-rbac explain` resolves roles against the compiled static mapping in `backend/internal/authz`, not the database, so it does not reflect mappings changed in the admin UI:

```sh
cd backend
GOCACHE=/private/tmp/moh-sso-go-build go run ./cmd/cli system-rbac explain \
  --realm-role user \
  --client-role outbreak-management:viewer
```

Use frontend checks:

```sh
cd frontend
source ~/.nvm/nvm.sh
nvm use v20.20.1
npm run rbac:doctor
```
