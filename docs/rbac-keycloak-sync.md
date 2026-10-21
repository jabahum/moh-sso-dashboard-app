# RBAC And Keycloak Sync

The MOH Integrated Health Portal uses Keycloak for identity and role assignment, and portal RBAC for application authorization.

This means:

- Keycloak owns users, passwords, sessions, realm roles, client roles, and client membership.
- Keycloak owns groups, group membership, and group role mappings.
- Keycloak clients are treated as portal systems.
- Keycloak client roles are treated as system roles.
- Portal RBAC owns permission keys and maps Keycloak roles to portal permissions.
- The portal decides app/menu/API access from the effective permission set.

Only clients marked `portal.system=true`, or clients already registered for backward compatibility, participate in system discovery. `portal.accessRoles` identifies the roles that expose a system. Launch behavior is synchronized through `ui.systemType`, `ui.displayInLauncher`, `ui.displayInSideNav`, `ui.launchMode`, `ui.launchUrl`, and optional `ui.navigation`.

## Why Sync Is Needed

Keycloak can tell us that a user has:

```text
realm role: admin
client role: data-statistics:report_admin
client role: outbreak-management:super_admin
```

But Keycloak does not know that `report_admin` should mean:

```text
report_browser:read
metrics:read
audit:read
```

Those mappings live in the portal RBAC database and are seeded from:

```text
backend/config/system-rbac.seed.yaml
```

If roles sync from Keycloak but permissions are empty, it usually means the Keycloak catalog was synced but the portal RBAC seed/mapping was not applied.

## Startup Sync

Backend startup sync is controlled by `backend/app.env`.

Recommended local/default settings:

```env
RBAC_STARTUP_SYNC_ENABLED=true
RBAC_STARTUP_SEED_ENABLED=true
RBAC_STARTUP_SEED_PATH=config/system-rbac.seed.yaml
RBAC_STARTUP_SYNC_REALM_EXPORT=true
RBAC_STARTUP_SYNC_REALM_EXPORT_PATH=
RBAC_STARTUP_SYNC_LIVE_KEYCLOAK=true
RBAC_STARTUP_SYNC_USERS=true
RBAC_STARTUP_SYNC_PUSH_TO_KEYCLOAK=true
RBAC_STARTUP_SYNC_FAIL_ON_ERROR=false
```

Startup order:

1. Sync systems and roles from the local Keycloak realm export, if enabled.
2. Sync systems and roles from live Keycloak, if enabled.
3. Apply the curated portal RBAC seed.
4. Sync Keycloak groups, group memberships, and group role mappings into the local portal RBAC cache.
5. Sync Keycloak users into the local portal DB user cache.
6. Optionally create missing portal-defined roles back into Keycloak, if enabled.

The curated seed is applied after discovery so known portal metadata and permission mappings win over generic Keycloak metadata.

Group membership can grant inherited realm roles, client roles, and portal group permissions. For the effective-access model, admin workflow, and QA checklist, see [RBAC Groups](./rbac-groups.md).

### Default Access For The `user` Realm Role

The curated seed maps the `user` realm role to these default system roles:

| System | Client role |
| --- | --- |
| Data & Statistics (`data-statistics`) | `data-statistics_access` |
| Utilities (`utilities`) | `utilities_access` |
| Settings (`settings`) | `settings_access` |

Migration `000030_add_realm_role_system_roles` stores these defaults in the portal DB. The authorization resolver therefore exposes the three systems to every user carrying the `user` realm role, even before a refreshed Keycloak token contains the composite client roles.

When `RBAC_STARTUP_SYNC_PUSH_TO_KEYCLOAK=true`, startup reconciliation also adds those client roles as composites of the Keycloak `user` realm role. Existing users receive the composite roles on their next login or token refresh.

The `accessible_systems` attribute in the local realm export is descriptive seed metadata only. Runtime authorization is derived from realm roles, client-role assignments/composites, and the portal RBAC mappings; changing that attribute alone never grants access.

## Safe Source Of Truth Rules

Use these rules to avoid accidental overwrites:

- Assign users to roles in Keycloak or through portal APIs that write to Keycloak.
- Define what roles can do in portal RBAC.
- Keep `RBAC_STARTUP_SYNC_PUSH_TO_KEYCLOAK=true` when startup should create missing portal-defined Keycloak clients, realm/client roles, and configured realm-role composites from the RBAC registry. Apart from explicitly configured realm-role defaults such as `user`, other systems still require explicit client-role assignments.
- Do not use startup sync to delete Keycloak roles or users.
- Do not store passwords or credentials in portal RBAC.

## Retiring The Integrated Outbreak System Client

`outbreak-management` is the canonical replacement for the retired
`integrated-outbreak-system` client. Migration
`000031_consolidate_outbreak_management_system` merges the legacy portal RBAC
roles, permissions, access-role markers, realm-role mappings, access requests,
and audit references into the replacement system.

Startup synchronization is intentionally non-destructive and will not delete a
live Keycloak client. For an existing realm:

1. Confirm `outbreak-management` exists with the required roles.
2. Move or recreate user, group, service-account, and realm-role composite
   assignments on `outbreak-management`.
3. Refresh a representative user's token and verify portal access.
4. Delete `integrated-outbreak-system` and its legacy admin client in Keycloak.
5. Run the RBAC drift report and confirm the retired client is absent.

Do not delete the legacy Keycloak client before its assignments have been
verified on the replacement. Realm exports in this repository now contain only
`outbreak-management`.

### Retiring The Report Browser Client

The Report Browser remains a frontend microfrontend under Data & Statistics;
it is no longer a standalone Keycloak/system client. Migration
`000032_consolidate_report_browser_system` moves its specialized report roles,
permissions, realm-role mappings, access requests, and audit references to
`data-statistics`. Existing Keycloak installations should move assignments to
the matching roles on `data-statistics`, verify report access, and then delete
the retired `report-browser` client. Startup synchronization will not perform
that destructive deletion automatically.

## Adding A New System / Client

Use this flow when a new application wants to connect to the portal.

### 1. Create The Keycloak Client

Create a Keycloak client with a stable `clientId`.

Example:

```text
laboratory-information-system
```

Add client attributes for portal display where possible:

```text
ui.icon=lab
ui.category=laboratory
ui.launchUrl=/portal/apps/lab
```

Add client roles:

```text
laboratory-information-system_access
viewer
data_entry
manager
admin
```

At least one role should be an access role. A good convention is:

```text
<clientId>_access
```

### 2. Sync Keycloak Into Portal RBAC

Restart the backend with startup sync enabled, or use the RBAC admin UI drift/sync flow.

You can also draft from live Keycloak:

```sh
cd backend
go run ./cmd/cli system-rbac sync-keycloak \
  --base-url "$KEYCLOAK_BASE_URL" \
  --realm "$KEYCLOAK_REALM" \
  --admin-client-id "$KEYCLOAK_ADMIN_CLIENT_ID" \
  --admin-client-secret "$KEYCLOAK_ADMIN_CLIENT_SECRET" \
  --draft-file /tmp/system-rbac.seed.yaml
```

Or draft from a realm export:

```sh
cd backend
go run ./cmd/cli system-rbac sync-keycloak \
  --realm-export ../infra/keycloak/realm-export.json \
  --draft-file /tmp/system-rbac.seed.yaml
```

### 3. Map Roles To Permissions

Newly discovered roles are intentionally inert until permissions are mapped.

Map permissions in one of two ways:

1. Admin UI:

```text
/portal/admin/rbac
```

2. Seed file:

```yaml
systems:
  - clientId: laboratory-information-system
    displayName: Laboratory Information System
    icon: lab
    launchUrl: /portal/apps/lab
    category: laboratory
    enabled: true
    accessRoles:
      - laboratory-information-system_access
      - viewer
      - data_entry
      - manager
      - admin
    roles:
      - name: laboratory-information-system_access
        permissions:
          - portal:access
          - systems:read
          - systems:launch
      - name: viewer
        permissions:
          - laboratory:read
      - name: data_entry
        permissions:
          - laboratory:read
          - laboratory:write
      - name: admin
        permissions:
          - laboratory:read
          - laboratory:write
          - laboratory:manage
```

Only use permission keys that exist in `backend/internal/authz/permissions.go`. If a new system needs new permission keys, add them there first, then add route guards and frontend guards.

### 4. Assign Users In Keycloak

Assign the user the relevant client roles in Keycloak, assign the user to a Keycloak group that carries those roles, or use the portal user access panel if it writes through to Keycloak.

Example:

```text
user: jane.doe
client: laboratory-information-system
roles:
  - laboratory-information-system_access
  - viewer
```

### 5. Verify Effective Access

Use the admin UI:

```text
/portal/admin/rbac
```

Open Effective Access and search by user ID, username, or email.

Verify:

- realm roles
- client roles
- permissions
- accessible systems
- grant sources

Or use the API:

```text
GET /api/v1/auth/me
```

Expected response fields:

```text
permissions
systems
accessibleSystems
realmRoles
clientRoles
```

## Managing Client RBAC After Onboarding

Use `/portal/admin/rbac` for routine updates.

Common changes:

- Rename display name, icon, launch URL, category, owner, support URL, or documentation URL.
- Enable or disable a system.
- Add or remove access roles.
- Add a system role.
- Map or unmap permissions from a system role.
- Map or unmap permissions from a realm role.

Recommended workflow:

1. Make the change in a lower environment first.
2. Use Effective Access or Policy Simulation to confirm impact.
3. Export the seed.
4. Review the diff.
5. Promote the seed to staging/production.

Export current DB mapping:

```sh
cd backend
go run ./cmd/cli system-rbac export-db --file config/system-rbac.seed.yaml
```

Validate seed:

```sh
cd backend
go run ./cmd/cli system-rbac validate --file config/system-rbac.seed.yaml
```

## Editing Keycloak Roles

If a client role is added in Keycloak:

1. Run drift/sync or restart backend with live sync enabled.
2. Map the new role to portal permissions.
3. Verify with Effective Access.

If a client role is renamed in Keycloak:

1. Treat it as a new role.
2. Sync it into portal RBAC.
3. Map permissions to the new role.
4. Move users from the old role to the new role in Keycloak.
5. Remove or disable the old role mapping after verification.

If a client is removed from Keycloak:

1. Run drift detection.
2. Confirm no users still rely on the system.
3. Disable the system in portal RBAC before deleting any mapping.

## Troubleshooting

### Roles Exist But Permissions Are Empty

Check that the seed was applied:

```env
RBAC_STARTUP_SEED_ENABLED=true
RBAC_STARTUP_SEED_PATH=config/system-rbac.seed.yaml
```

Then restart backend and check logs for:

```text
RBAC startup seed applied
```

Also confirm that the role names in Keycloak exactly match the role names in the seed.

In production Docker images, `backend/config/` is copied into `/app/config`, so the default
seed path resolves to `/app/config/system-rbac.seed.yaml`. The Keycloak realm export is not
bundled into the backend image by default. If `RBAC_STARTUP_SYNC_REALM_EXPORT=true`, mount a
realm export file into the container and set `RBAC_STARTUP_SYNC_REALM_EXPORT_PATH`; otherwise
set `RBAC_STARTUP_SYNC_REALM_EXPORT=false` and rely on live Keycloak sync plus the seed.

### System Appears But Cannot Launch

Check:

- the system is enabled in portal RBAC
- the user has one of the system access roles
- the access role is listed under `accessRoles`
- the role maps to `systems:launch` or the UI uses `accessibleSystems`
- `launchUrl` starts with `/portal` or is a valid absolute URL

### User Has Role In Keycloak But Portal Does Not Show App

Check Effective Access:

- Is the client role visible under `clientRoles`?
- Is the system visible under `accessibleSystems`?
- Does the role have mapped permissions?
- Is the system enabled?
- Is the frontend route guarded by matching permissions/system metadata?

### Startup Sync Fails

For local development, keep:

```env
RBAC_STARTUP_SYNC_FAIL_ON_ERROR=false
```

For production, consider:

```env
RBAC_STARTUP_SYNC_FAIL_ON_ERROR=true
```

only when Keycloak availability is mandatory before the backend starts.

## Production Notes

In production:

- Keep seed files source-controlled and reviewed.
- Keep Keycloak admin credentials restricted to the backend.
- Prefer drift preview before applying major changes.
- Keep `RBAC_STARTUP_SYNC_PUSH_TO_KEYCLOAK=true` only when the approved operating procedure allows the portal to create missing registry clients and roles in Keycloak.
- Use audit logs when changing high-risk role mappings.
