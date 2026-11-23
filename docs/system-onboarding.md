# System onboarding

This page is the CLI/seed and navigation reference for administrators and developers
adding a new Keycloak client/system to the Integrated Health Portal. For the complete
UI walkthrough with screenshots, field values, Keycloak setup, role mapping,
activation, and acceptance tests, start with the
[Application and system onboarding handbook](platform-onboarding.md).

Registering a system does not create its OAuth client, deploy an external
application, or install a hosted frontend module. For the full RBAC and Keycloak
sync operating model, see [RBAC And Keycloak Sync](./rbac-keycloak-sync.md).

## Application lifecycle

The starting lifecycle state of a new registry entry depends on how it is created (`upsertSystemTx` in `backend/internal/features/rbac/repository_postgres.go`, and `backend/internal/features/system_rbac/seed_apply.go`):

- Systems registered by an administrator (**Register application**, `POST /admin/rbac/systems`, or a new client ID saved through `PUT /admin/rbac/systems/:clientId`), application-manifest imports, and Keycloak clients that carry a `portal.onboarding.profile` attribute start `registered` and disabled. Configure metadata and access roles, then validate, approve, and activate them in `/admin/rbac`.
- Clients discovered by Keycloak sync (admin UI or startup discovery) without that attribute start `active`, with `enabled` taken from the Keycloak client.
- Systems inserted by `system-rbac seed` start `active` when the seed entry is enabled (the default) and `registered` when it sets `enabled: false`.

Migration 48 (`000048_application_registry_lifecycle`) added the lifecycle and left existing records `active`. Migration 59 (`000059_grandfather_existing_client_lifecycle`) moved every record still in `registered`, `configured`, `validated` or `approved` to `active` and enabled, and added the `lifecycle_managed` flag so that only onboarding-managed records drop back to `configured` when their configuration changes. See [Platform development](platform-development.md) for transitions, permissions, API examples, rollout, and rollback.

## Launch and navigation model

### System types and launch modes

Portal systems explicitly declare `systemType` (`platform` or `external`), `displayInLauncher`, `displayInSideNav`, and `launchMode` (`internal`, `new_tab`, or `same_tab`). Platform systems use an internal `/portal` or `/apps` path and may provide navigation. External systems require an absolute HTTP or HTTPS URL and never provide portal side navigation.

### Module launcher cards

A single platform system may also expose selected navigation modules as individual application-launcher cards. These entries remain modules of the parent Keycloak client: do not create a Keycloak client or duplicate roles for each module. The user's access to the parent system still comes from `accessibleSystems`; module visibility is then filtered by the module's permission metadata.

### Navigation item fields

Navigation items accept these optional fields:

```json
{
  "id": "documents",
  "label": "Document Management",
  "path": "/apps/dwh/documents",
  "description": "Upload and process health data files.",
  "icon": "document",
  "order": 30,
  "requiredAnyPermissions": ["documents:read", "documents:write"],
  "displayInLauncher": true,
  "displayInSideNav": true,
  "launchMode": "internal"
}
```

- `permission` requires one permission.
- `requiredPermissions` requires every listed permission.
- `requiredAnyPermissions` requires at least one listed permission.
- `displayInLauncher` defaults to `false` for navigation items, preserving existing behavior.
- `displayInSideNav` defaults to `true` for navigation items.
- A launcher item must have a valid `path`.
- `launchMode` accepts `internal`, `new_tab`, or `same_tab`.
- Nested `children` use the same contract and are evaluated recursively.

### Display combinations

Useful display combinations are:

- System card only: system `displayInLauncher: true`; no module has `displayInLauncher: true`.
- Module cards only: system `displayInLauncher: false`; selected modules set `displayInLauncher: true`.
- Both: enable the system card and selected module cards.
- Side navigation only: leave module `displayInLauncher` false and `displayInSideNav` true.

The shell assigns module cards stable synthetic IDs in the form `<clientId>:<moduleId>`, but activates the parent client when a module is launched. Direct routes remain protected by the route permission guards; launcher visibility is not an authorization boundary.

### Keycloak client attributes

Keycloak clients must set `portal.system=true` for automatic enrollment. Configure `portal.accessRoles` as a comma-separated list of roles that expose the system, plus `ui.systemType`, `ui.displayInLauncher`, `ui.displayInSideNav`, `ui.launchMode`, `ui.launchUrl`, `ui.icon`, `ui.category`, and optional `ui.navigation`.

For the Integrated Health Portal itself, keep the ordinary public URL in `ui.launchUrl` and configure `ui.authenticatedLaunchUrl` separately. External applications should use the authenticated launch URL when they do not already hold a portal backend session. See [Cross-Application SSO](./cross-application-sso.md).

Newly discovered roles remain inert until they are explicitly configured as access roles. Development fixtures demonstrate all modes through `demo-platform-system`, `demo-external-new-tab`, and `demo-external-same-tab`; they are absent from the production realm export.

## 1. Create or confirm the Keycloak client

Create the client in Keycloak using the admin UI or CLI. Add the client roles that represent the system's access model.

Examples:

- `<system>_access`
- `viewer`
- `data_entry`
- `manager`
- `admin`

## 2. Draft the portal RBAC mapping

From `backend/`, draft from a checked-in or exported Keycloak realm file:

```sh
go run ./cmd/cli system-rbac sync-keycloak \
  --realm-export ../infra/keycloak/realm-export.json \
  --draft-file /tmp/system-rbac.seed.yaml
```

Or draft from a running Keycloak instance:

```sh
go run ./cmd/cli system-rbac sync-keycloak \
  --base-url "$KEYCLOAK_BASE_URL" \
  --realm "$KEYCLOAK_REALM" \
  --admin-client-id "$KEYCLOAK_ADMIN_CLIENT_ID" \
  --admin-client-secret "$KEYCLOAK_ADMIN_CLIENT_SECRET" \
  --draft-file /tmp/system-rbac.seed.yaml
```

The sync treats non-Keycloak-internal clients as candidate systems and imports their client roles as portal system roles/access roles. Service-account-only admin clients with no client roles are ignored.

Review the draft and assign permissions to each system role. A generated role without permissions is intentionally inert until mapped. This is deliberate: Keycloak says which role a user has, while portal RBAC says what that role can do inside the Integrated Health Portal.

## 3. Validate the seed

```sh
go run ./cmd/cli system-rbac validate --file /tmp/system-rbac.seed.yaml
```

## 4. Apply the seed to the portal database

```sh
go run ./cmd/cli system-rbac seed --file /tmp/system-rbac.seed.yaml
```

## 5. Manage or adjust in the admin UI

After the first seed, portal administrators can manage system metadata, access roles, system roles, and permission mappings at:

```text
/admin/rbac
```

The signed-in administrator needs `rbac:read` to open the page. Write actions require `rbac:write`, `rbac:roles:write`, or `rbac:permissions:write` depending on the operation.

## 6. Explain a user's expected access

```sh
go run ./cmd/cli system-rbac explain \
  --realm-role user \
  --client-role outbreak-management:viewer
```

## 7. Check for drift

Against a realm export:

```sh
go run ./cmd/cli system-rbac unmapped \
  --realm-export ../infra/keycloak/realm-export.json \
  --file config/system-rbac.seed.yaml
```

Against live Keycloak:

```sh
go run ./cmd/cli system-rbac unmapped \
  --base-url "$KEYCLOAK_BASE_URL" \
  --realm "$KEYCLOAK_REALM" \
  --admin-client-id "$KEYCLOAK_ADMIN_CLIENT_ID" \
  --admin-client-secret "$KEYCLOAK_ADMIN_CLIENT_SECRET" \
  --file config/system-rbac.seed.yaml
```

Any unmapped client or role should either be added to the seed or intentionally ignored in an onboarding note.
