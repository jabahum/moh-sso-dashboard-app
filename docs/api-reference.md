# API reference

This document is the human-readable API map for the MOH Integrated Health Portal backend.

Interactive Swagger UI:

- Backend: <http://localhost:9000/swagger/>
- When using Express: <http://localhost:3000/backend/swagger/>
- YAML/JSON: `/swagger/openapi.yaml` and `/swagger/openapi.json`

The complete machine-readable contract is generated from the registered routes and Go
handler types and is stored at `backend/docs/openapi.yaml`. Regenerate it with
`cd backend && make openapi-generate`; validate it with `make openapi-validate` from
`backend/`. Normal backend tests enforce documentation freshness. See
[Swagger documentation maintenance](../backend/docs/README.md). When this page and the
OpenAPI file disagree, the OpenAPI file is correct.

Base API prefix:

```text
/api/v1
```

Health and version endpoints are outside `/api/v1`.

Conventions used in the tables:

- Where a section says "Mounted under `/api/v1/...`", paths in its tables are relative to that prefix, and `/` means the prefix itself. Other tables use full paths.
- Path parameters are written `:name`. The OpenAPI file writes the same parameter as `{name}`.
- The Permission column lists the portal permission checked by the route. "A or B" means either is enough. "—" means no route-level permission beyond an authenticated session.

## Contents

- [Response envelope](#response-envelope)
- [Authentication and authorization](#authentication-and-authorization)
- [Health and version](#health-and-version)
- [Auth](#auth)
- [Platform integration API](#platform-integration-api)
- [Announcements](#announcements)
- [Users](#users)
- [Clients and systems](#clients-and-systems)
- [RBAC governance](#rbac-governance)
- [Health contexts](#health-contexts)
- [Audit logs](#audit-logs)
- [Metrics](#metrics)
- [Notifications](#notifications)
- [Email](#email)
- [Documents](#documents)
- [Document templates](#document-templates)
- [Storage locations](#storage-locations)
- [Sessions](#sessions)
- [Data quality](#data-quality)
- [Report Scheduler](#report-scheduler)
- [Surveillance](#surveillance)
- [Visualiser, admin units, and GeoJSON](#visualiser-admin-units-and-geojson)
- [Verification](#verification)

## Response envelope

Successful responses:

```json
{
  "success": true,
  "data": {},
  "meta": { "requestId": "..." }
}
```

Error responses:

```json
{
  "success": false,
  "error": {
    "code": "INVALID_INPUT",
    "message": "Safe user-facing message"
  },
  "meta": { "requestId": "..." }
}
```

`meta` carries only an optional `requestId` (`backend/internal/http/response/response.go`).
File endpoints (downloads, views, exports) stream binary content instead of the envelope.

Handlers should return feature DTOs and safe error messages. They should not expose raw SQL rows, sqlc models, filesystem paths, stack traces, or raw database errors.

General payload conventions:

- Send date/time values as ISO-8601/RFC3339 strings unless a feature documents another format.
- IDs are UUID strings unless the route name says otherwise (for example `:clientId`, `:issueCode`, `:permissionKey`).

## Authentication and authorization

Protected routes accept any of:

- An existing `sso_session` HttpOnly cookie, set by the Keycloak login flow below. Swagger UI cannot set this cookie manually.
- A bearer JWT in the `Authorization` header.
- The legacy `access_token` HttpOnly cookie, kept for backward compatibility.

When more than one is present, the bearer header wins, then the `sso_session` cookie, then the
legacy cookie (`extractAccessToken` in `backend/internal/middleware/auth.go`).

Admin routes are mounted under:

```text
/api/v1/admin
```

Feature routes use permission middleware from `backend/internal/authz`. Common permissions:

| Permission | Typical use |
| --- | --- |
| `portal:access` | Basic authenticated portal/session access. |
| `users:read`, `users:write`, `users:roles:write` | User management. |
| `clients:read`, `clients:write`, `clients:roles:write`, `systems:read` | Client/system management. |
| `rbac:read`, `rbac:write`, `rbac:roles:write`, `rbac:permissions:write` | RBAC governance. |
| `health_contexts:read`, `health_contexts:write`, `health_contexts:assign`, `health_contexts:sync` | Health context hierarchy and assignments. |
| `announcements:read`, `announcements:write`, `announcements:publish` | Announcement lifecycle. |
| `audit:read`, `metrics:read` | Audit logs and admin metrics. |
| `notifications:read`, `notifications:write` | Notification administration. |
| `email:read`, `email:send`, `email:manage` | Email sending and history. |
| `documents:read`, `documents:write`, `documents:process` | Document flows. |
| `document_templates:read`, `document_templates:write`, `document_templates:publish` | Document templates. |
| `storage_locations:read`, `storage_locations:write` | Storage locations. |
| `issue_tracker:read`, `issue_tracker:write`, `issue_tracker:assign`, `issue_tracker:comment`, `issue_tracker:manage`, `issue_tracker:close` | Data quality issue tracker. |
| `data_quality:read`, `data_quality:write` | Data quality rule engine (DQA). |
| `report_scheduler:read`, `report_scheduler:create`, `report_scheduler:update`, `report_scheduler:delete`, `report_scheduler:execute`, `report_scheduler:history`, `report_scheduler:manage` | Report scheduler. |
| `surveillance:read`, `surveillance:import`, `surveillance:manage_locations`, `surveillance:manage_alerts` | Surveillance module. |
| `report_browser:read` | Report browser and visualiser reads. |

Context-scoped routes: documents, document templates, data quality, surveillance, the
visualiser, and admin units also require access to the `data-statistics` system and an
accessible health context selection. Clients may send the selected context in the
`X-Health-Context-ID` header. The report scheduler also requires `data-statistics`
system access.

## Health and version

| Method | Path | Auth | Purpose |
| --- | --- | --- | --- |
| `GET` | `/health` | Public | General health check. |
| `GET` | `/health/live` | Public | Liveness. |
| `GET` | `/health/ready` | Public | Readiness. |
| `GET` | `/version` | Public | Dependency-free backend build metadata. |
| `GET` | `/api/v1/admin/modules` | Admin | List installed backend modules. Registered only when the backend runs with the module registry. |

## Auth

| Method | Path | Auth | Purpose |
| --- | --- | --- | --- |
| `GET` | `/api/v1/auth/login` | Public, rate limited | Start Keycloak login. |
| `GET` | `/api/v1/auth/launch?returnTo=<portal-path>` | Public, rate limited | Start Keycloak login and return to a validated portal route after authentication. |
| `GET` | `/api/v1/auth/callback` | Public, rate limited | Handle Keycloak authorization callback. |
| `GET` | `/api/v1/auth/me` | Session | Return current user, realm roles, client roles, permissions, and accessible systems. |
| `POST` | `/api/v1/auth/refresh` | Session, rate limited | Refresh portal session. |
| `GET` | `/api/v1/auth/logout` | Public | Log out the current browser session. |

`auth/me` is the primary frontend bootstrap endpoint. It is the source for menu visibility, app launcher visibility, permissions, and accessible systems.

`auth/launch` accepts only `/portal` paths or absolute URLs whose origin is listed in
`AUTH_RETURN_URL_ALLOWED_ORIGINS`. The backend binds the target to the OAuth state and
uses the normal fixed Keycloak callback URI; callers must not register arbitrary return
paths as Keycloak redirect URIs.

## Platform integration API

Used by integrated applications rather than the portal UI. See
[Platform identity](platform-identity.md) and
[Platform notifications](platform-notifications.md).

Mounted under `/api/v1/platform`. These routes authenticate a Keycloak access token
(bearer), not the portal session, and are rate limited per IP (120 requests per minute,
policy `platform-auth`).

| Method | Path | Auth | Purpose |
| --- | --- | --- | --- |
| `GET` | `/applications/:clientId/access` | User token with `identity:read` scope | Return the calling user's access to an application. Service-account tokens are rejected. |
| `GET` | `/identity` | Service identity with `identity:read` scope | Return the calling application's registered service identity. |
| `GET` | `/readiness` | Service identity with `identity:read` scope | Report integration readiness for the calling application. |
| `POST` | `/notifications` | Service identity with `notifications:write` scope | Submit a notification for delivery. Registered only when platform delivery is enabled. |
| `GET` | `/notifications/:id` | Service identity with `notifications:read` scope | Get a submitted notification. Registered only when platform delivery is enabled. |

Portal-session routes related to integrated applications:

| Method | Path | Permission | Purpose |
| --- | --- | --- | --- |
| `GET` | `/api/v1/applications/:clientId/manifest` | `systems:launch` plus access to the application (or admin), checked in the handler | Get the runtime application manifest. |
| `GET` | `/api/v1/admin/platform/applications/:clientId/operations` | `rbac:read` | Get platform operations status for an application. |
| `PUT` | `/api/v1/admin/platform/applications/:clientId/identities/:oauthClientId` | `rbac:write` and `rbac:roles:write` | Register or update an application's service identity. Rate limited. |

## Announcements

Public and user routes:

| Method | Path | Auth | Purpose |
| --- | --- | --- | --- |
| `GET` | `/api/v1/announcements/public` | Public | List public published announcements. |
| `GET` | `/api/v1/announcements/active` | Public | List active published announcements. |
| `GET` | `/api/v1/announcements/me` | `announcements:read` | List announcements targeted to current user. |
| `GET` | `/api/v1/announcements/user` | `announcements:read` | List user-targeted announcements. |
| `GET` | `/api/v1/announcements/role/:role_name` | `announcements:read` | List role-targeted announcements. |
| `GET` | `/api/v1/announcements/client/:client_id` | `announcements:read` | List client-targeted announcements. |
| `GET` | `/api/v1/announcements/:id/attachments/:attachmentId/download` | `announcements:read` | Download attachment. |

Admin routes, mounted under `/api/v1/admin/announcements`. Write and publish routes are rate limited.

| Method | Path | Permission | Purpose |
| --- | --- | --- | --- |
| `GET` | `/` | `announcements:read` | List announcements. |
| `GET` | `/stats` | `announcements:read` | Announcement metrics. |
| `POST` | `/audience-preview` | `announcements:write` | Preview the recipients an announcement audience resolves to. |
| `GET` | `/:id` | `announcements:read` | Get announcement detail. |
| `POST` | `/` | `announcements:write` | Create announcement. |
| `PUT` | `/:id` | `announcements:write` | Update announcement. |
| `DELETE` | `/:id` | `announcements:write` | Soft delete announcement. |
| `POST` | `/:id/restore` | `announcements:write` | Restore announcement. |
| `POST` | `/:id/publish` | `announcements:publish` | Publish immediately and trigger delivery. |
| `POST` | `/:id/draft` | `announcements:publish` | Move to draft. |
| `POST` | `/:id/schedule` | `announcements:publish` | Schedule announcement. |
| `POST` | `/:id/archive` | `announcements:publish` | Archive announcement. |
| `PATCH` | `/:id/pin` | `announcements:write` | Pin/unpin. |
| `PATCH` | `/:id/priority` | `announcements:write` | Update priority. |
| `GET` | `/:id/attachments` | `announcements:read` | List attachments. |
| `POST` | `/:id/attachments` | `announcements:write` | Upload persistent attachment. |
| `PATCH` | `/:id/attachments/:attachmentId` | `announcements:write` | Update attachment metadata. |
| `DELETE` | `/:id/attachments/:attachmentId` | `announcements:write` | Delete attachment. |
| `GET` | `/:id/attachments/:attachmentId/download` | `announcements:read` | Download attachment. |

### Announcement payloads

Create announcement (`POST /api/v1/admin/announcements`). `title`, `message`, and `level` are required.

```json
{
  "title": "Surveillance update",
  "message": "Weekly outbreak bulletin is available.",
  "summary": "Weekly bulletin",
  "level": "information",
  "tag": "surveillance",
  "link_url": "/portal/apps/dwh/surveillance",
  "link_label": "Open surveillance dashboard",
  "priority": 5,
  "is_pinned": true,
  "status": "draft",
  "publish_at": "2026-06-16T09:00:00Z",
  "expires_at": "2026-07-16T09:00:00Z",
  "audience_type": "client",
  "client_ids": ["outbreak-management"],
  "role_names": ["outbreak-management_access"],
  "group_ids": [],
  "health_context_ids": [],
  "user_ids": [],
  "notify_by_email": true,
  "notify_by_sms": false,
  "sms_message": ""
}
```

Persistent attachment upload (`POST /:id/attachments`). Send JSON as below, or
`multipart/form-data` with a `file` part plus the same optional fields.
`file_name` and `data_base64` are required for JSON.

```json
{
  "file_name": "weekly-bulletin.pdf",
  "content_type": "application/pdf",
  "data_base64": "base64-encoded-content",
  "include_in_email": true,
  "inline": false,
  "content_id": "",
  "sort_order": 1
}
```

Publish (`POST /:id/publish`) or schedule (`POST /:id/schedule`) with transient email
attachments. `publish_at` is required for schedule and is not part of the publish body.

```json
{
  "publish_at": "2026-06-16T09:00:00Z",
  "include_attachments_in_email": true,
  "attachments": [
    {
      "file_name": "bulletin.pdf",
      "content_type": "application/pdf",
      "data_base64": "base64-encoded-content",
      "inline": false
    }
  ]
}
```

Links and attachments:

- `link_url` can be a `/portal...` path or absolute URL. When it is set, the email template and News & Updates UI use `link_label` as the call-to-action text, falling back to a generic label.
- Persistent attachments are stored as announcement assets and returned with `download_url`.
- Transient email attachments must provide exactly one of `path` or `data_base64`.
- When an announcement is published with email delivery, persistent attachments marked `include_in_email` are attached to the email and also rendered as secure download links in the template.

## Users

Protected user routes:

| Method | Path | Permission | Purpose |
| --- | --- | --- | --- |
| `GET` | `/api/v1/users` | `users:read` | List users. |
| `GET` | `/api/v1/users/:id` | `users:read` | Get user. |

Admin user routes:

| Method | Path | Permission | Purpose |
| --- | --- | --- | --- |
| `GET` | `/api/v1/admin/users` | `users:read` | List users. |
| `GET` | `/api/v1/admin/users/:id` | `users:read` | Get user. |
| `POST` | `/api/v1/admin/users` | `users:write` | Create Keycloak user. |
| `PUT` | `/api/v1/admin/users/:id` | `users:write` | Update user profile. |
| `DELETE` | `/api/v1/admin/users/:id` | `users:write` | Delete user. |
| `PATCH` | `/api/v1/admin/users/:id/enabled` | `users:write` | Enable or disable user. |
| `PATCH` | `/api/v1/admin/users/:id/toggle` | `users:write` | Toggle enabled state. |
| `POST` | `/api/v1/admin/users/:id/onboarding-email` | `users:write` | Send onboarding email. |
| `POST` | `/api/v1/admin/users/:id/verification-email` | `users:write` | Send verification email. |
| `POST` | `/api/v1/admin/users/:id/password-reset` | `users:write` | Send password reset email. |
| `POST` | `/api/v1/admin/users/:id/reset-password` | `users:write` | Reset password. |
| `GET` | `/api/v1/admin/users/:id/client-roles` | `users:read` | List user roles grouped by client. |
| `GET` | `/api/v1/admin/users/:id/clients/:clientID/roles` | `users:read` | List one user's roles for one client. |
| `POST` | `/api/v1/admin/users/:id/clients/:clientID/roles` | `users:roles:write` | Add client roles to user. |
| `DELETE` | `/api/v1/admin/users/:id/clients/:clientID/roles` | `users:roles:write` | Remove client roles from user. |
| `PUT` | `/api/v1/admin/users/:id/client-roles` | `users:roles:write` | Replace/update user client role assignments. |

User health context assignments are listed under [Health contexts](#health-contexts).

Preferred dynamic access management is through the RBAC user-access endpoints, because they validate Keycloak realm roles, client roles, and portal permission mappings together.

### User payloads

Add or remove client roles (`POST`/`DELETE /api/v1/admin/users/:id/clients/:clientID/roles`):

```json
{
  "clientId": "data-statistics",
  "clientUuid": "optional-keycloak-client-uuid",
  "roles": ["data-statistics_access", "report_admin"]
}
```

Set enabled state (`PATCH /api/v1/admin/users/:id/enabled`). `enabled` is required.

```json
{
  "enabled": true
}
```

## Clients and systems

Clients are Keycloak clients represented as portal systems.

| Method | Path | Permission | Purpose |
| --- | --- | --- | --- |
| `GET` | `/api/v1/clients` | `systems:read` or `clients:read` | List clients/systems. |
| `GET` | `/api/v1/clients/:id` | `clients:read` | Get client/system. |
| `POST` | `/api/v1/clients` | `clients:write` | Create client/system. |
| `PUT` | `/api/v1/clients/:id` | `clients:write` | Update client/system. |
| `PATCH` | `/api/v1/clients/:id/toggle` | `clients:write` | Enable or disable client. |
| `DELETE` | `/api/v1/clients/:id` | `clients:write` | Delete client. |
| `GET` | `/api/v1/clients/:id/roles` | `clients:read` | List client roles. |
| `GET` | `/api/v1/clients/onboarding/context` | `clients:write` | Get the context needed to onboard a new system. |
| `POST` | `/api/v1/clients/onboarding/preview` | `clients:write` | Preview system onboarding changes. |
| `POST` | `/api/v1/clients/onboarding/apply` | `clients:write` | Apply system onboarding. |
| `POST` | `/api/v1/admin/clients/:id/roles` | `clients:roles:write` | Create client role. |
| `DELETE` | `/api/v1/admin/clients/:id/roles/:role` | `clients:roles:write` | Delete client role. |

When a system is added to Keycloak, RBAC sync should import it into the portal registry. Users need the relevant client role, such as `<clientId>_access`, before the app appears in menus/launcher. See [System onboarding](system-onboarding.md).

### Client payload

Create (`POST /api/v1/clients`) and update (`PUT /api/v1/clients/:id`). `clientId` is
accepted on create only. Create also accepts Keycloak flow settings (for example
`standardFlowEnabled`, `serviceAccountsEnabled`, `defaultClientScopes`, `protocolMappers`);
see the OpenAPI schema `clients.CreateClientRequest` for the full list.

```json
{
  "clientId": "data-statistics",
  "name": "Data & Statistics",
  "description": "Data quality, reports, surveillance, and dashboards",
  "icon": "home",
  "publicClient": true,
  "enabled": true,
  "rootUrl": "https://portal.example.org/portal/apps/dwh",
  "baseUrl": "/portal/apps/dwh",
  "adminUrl": "",
  "redirectUris": ["https://portal.example.org/portal/*"],
  "webOrigins": ["https://portal.example.org"],
  "attributes": {
    "category": "platform"
  }
}
```

## RBAC governance

See [RBAC governance](rbac-governance.md) and [RBAC groups](rbac-groups.md) for concepts.

Mounted under:

```text
/api/v1/admin/rbac
```

Sensitive and bulk writes are rate limited.

| Method | Path | Permission | Purpose |
| --- | --- | --- | --- |
| `GET` | `/systems` | `rbac:read` | List portal systems. |
| `POST` | `/systems` | `rbac:write` | Register a system. |
| `GET` | `/systems/:clientId` | `rbac:read` | Get system metadata. |
| `PUT` | `/systems/:clientId` | `rbac:write` | Update system metadata. |
| `POST` | `/systems/:clientId/lifecycle` | Checked in the service: `rbac:write` to configure, `rbac:roles:write` to approve | Transition a system's lifecycle state. |
| `GET` | `/systems/:clientId/roles` | `rbac:read` | List system roles. |
| `POST` | `/systems/:clientId/roles` | `rbac:roles:write` | Create system role. |
| `POST` | `/systems/:clientId/roles/from-template` | `rbac:roles:write` | Create role from template. |
| `POST` | `/systems/:clientId/access-roles` | `rbac:roles:write` | Add access role. |
| `DELETE` | `/systems/:clientId/access-roles/:roleName` | `rbac:roles:write` | Remove access role. |
| `POST` | `/application-manifests/validate` | Admin, plus `rbac:read`, `rbac:write`, and `rbac:roles:write` | Validate an application manifest. |
| `POST` | `/application-manifests/preview` | Admin, plus `rbac:read`, `rbac:write`, and `rbac:roles:write` | Preview the changes an application manifest would make. |
| `POST` | `/application-manifests/import` | Admin, plus `rbac:read`, `rbac:write`, and `rbac:roles:write` | Import an application manifest. |
| `GET` | `/permissions` | `rbac:read` | List permission catalog. |
| `PUT` | `/permissions/:permissionKey` | `rbac:permissions:write` | Update permission metadata. |
| `GET` | `/realm-roles` | `rbac:read` | List realm-role mappings. |
| `GET` | `/realm-roles/:realmRole/usage` | `rbac:read` | Show realm role usage. |
| `POST` | `/realm-roles/:realmRole/permissions` | `rbac:permissions:write` | Assign permission to realm role. |
| `DELETE` | `/realm-roles/:realmRole/permissions/:permissionKey` | `rbac:permissions:write` | Remove permission from realm role. |
| `PUT` | `/roles/:roleId` | `rbac:roles:write` | Update role metadata. |
| `DELETE` | `/roles/:roleId` | `rbac:roles:write` | Delete role. |
| `GET` | `/roles/:roleId/usage` | `rbac:read` | Show role usage. |
| `POST` | `/roles/:roleId/copy-permissions` | `rbac:permissions:write` | Copy permissions from another role. |
| `POST` | `/roles/:roleId/permissions` | `rbac:permissions:write` | Assign permission to role. |
| `DELETE` | `/roles/:roleId/permissions/:permissionKey` | `rbac:permissions:write` | Remove permission from role. |
| `GET` | `/groups` | `rbac:read` | List synced Keycloak/RBAC groups. |
| `POST` | `/groups` | `rbac:roles:write` | Create or update a portal group mapping. |
| `GET` | `/groups/:groupId` | `rbac:read` | Get group roles, permissions, and metadata. |
| `PUT` | `/groups/:groupId` | `rbac:roles:write` | Create or update one group. |
| `GET` | `/groups/:groupId/members` | `rbac:read` | List synced group members. |
| `POST` | `/groups/:groupId/members` | `rbac:roles:write` | Add a user to the Keycloak group, then refresh the portal cache. |
| `DELETE` | `/groups/:groupId/members/:userId` | `rbac:roles:write` | Remove a user from the Keycloak group, then refresh the portal cache. |
| `PUT` | `/groups/:groupId/members` | `rbac:roles:write` | Replace cached members for sync/import use. |
| `POST` | `/groups/:groupId/sync-members` | `rbac:roles:write` | Refresh one group's members from Keycloak. |
| `POST` | `/groups/:groupId/permissions` | `rbac:permissions:write` | Assign a direct portal permission to a group. |
| `DELETE` | `/groups/:groupId/permissions/:permissionKey` | `rbac:permissions:write` | Remove a direct portal permission from a group. |
| `POST` | `/groups/:groupId/realm-roles` | `rbac:roles:write` | Map a realm role to a group. |
| `DELETE` | `/groups/:groupId/realm-roles/:realmRole` | `rbac:roles:write` | Remove a group realm-role mapping. |
| `POST` | `/groups/:groupId/system-roles` | `rbac:roles:write` | Map a client/system role to a group. |
| `DELETE` | `/groups/:groupId/system-roles/:clientId/:roleName` | `rbac:roles:write` | Remove a group client/system-role mapping. |
| `GET` | `/user-access/assignable` | `rbac:read` | List assignable realm roles, systems, roles, and permissions. |
| `GET` | `/user-access/users/:userId` | `rbac:read` | Get user access profile. |
| `PUT` | `/user-access/users/:userId` | `rbac:roles:write` | Update user realm roles, system roles, and permissions. |
| `GET` | `/effective-access/users/:userId` | `rbac:read` | Explain effective user access. |
| `POST` | `/simulate` | `rbac:read` | Simulate access changes. |
| `POST` | `/changes/preview` | `rbac:read` | Preview policy/role changes. |
| `GET` | `/drift` | `rbac:read` | Get Keycloak/portal drift report. |
| `POST` | `/drift/realm-export` | `rbac:read` | Generate drift from an uploaded realm export. |
| `POST` | `/sync/preview` | `rbac:read` | Preview sync changes. |
| `POST` | `/sync/apply` | `rbac:write` | Apply sync changes. |
| `GET` | `/export` | `rbac:read` | Export RBAC seed data. |
| `POST` | `/import/preview` | `rbac:read` | Preview seed/import changes. |
| `POST` | `/import/apply` | `rbac:write` | Apply seed/import changes. |
| `GET` | `/audit` | `rbac:read` or `audit:read` | List RBAC audit events. |
| `GET` | `/role-templates` | `rbac:read` | List role templates. |
| `POST` | `/bulk/assign-permission` | `rbac:permissions:write` | Bulk assign permission. |
| `POST` | `/bulk/remove-permission` | `rbac:permissions:write` | Bulk remove permission. |
| `GET` | `/access-requests` | `rbac:roles:write` | List access requests. |
| `POST` | `/access-requests/:id/:decision` | `rbac:roles:write` | Approve or reject access request. |
| `GET` | `/change-requests` | `rbac:read` | List change requests. |
| `POST` | `/change-requests` | `rbac:write` | Create change request. |
| `POST` | `/change-requests/:id/:decision` | `rbac:write` | Approve or reject change request. |

Group health context assignments are listed under [Health contexts](#health-contexts).

Authenticated users can create access requests:

| Method | Path | Permission | Purpose |
| --- | --- | --- | --- |
| `POST` | `/api/v1/access-requests` | `portal:access` | Request access to a system or role. |

### RBAC payloads

Update user access (`PUT /user-access/users/:userId`):

```json
{
  "realmRoles": ["admin"],
  "clientRoles": {
    "dashboard-web": ["dashboard-web_access"],
    "data-statistics": ["data-statistics_access", "report_admin"]
  },
  "permissions": ["portal:access", "report_browser:read"]
}
```

Update system metadata (`PUT /systems/:clientId`). The schema `rbac.UpsertSystemInput`
also accepts launcher and integration fields such as `displayInLauncher`,
`displayInSideNav`, `launchMode`, `authenticatedLaunchUrl`, `integrationType`, and
`systemType`.

```json
{
  "clientId": "data-statistics",
  "displayName": "Data & Statistics",
  "description": "Data quality, reports, surveillance, and dashboards",
  "icon": "home",
  "launchUrl": "/portal/apps/dwh",
  "category": "platform",
  "ownerTeam": "Analytics",
  "ownerName": "System Owner",
  "ownerEmail": "owner@example.org",
  "supportUrl": "https://support.example.org",
  "documentationUrl": "https://docs.example.org",
  "environment": "production",
  "criticality": "medium",
  "enabled": true,
  "sortOrder": 20
}
```

Create a system role (`POST /systems/:clientId/roles`) or update role metadata (`PUT /roles/:roleId`):

```json
{
  "name": "report_admin",
  "displayName": "Report Administrator",
  "description": "Can administer report browser content",
  "enabled": true
}
```

Assign a permission to a role or realm role (`POST /roles/:roleId/permissions`,
`POST /realm-roles/:realmRole/permissions`). `permissionKey` is required.

```json
{
  "permissionKey": "report_browser:read"
}
```

Preview or apply RBAC sync from a realm export (`POST /sync/preview`, `POST /sync/apply`):

```json
{
  "source": "realm-export",
  "realmExport": {}
}
```

Simulate access (`POST /simulate`):

```json
{
  "userId": "keycloak-user-id",
  "username": "admin",
  "email": "admin@example.org",
  "realmRoles": ["admin"],
  "clientRoles": {
    "dashboard-web": ["dashboard-web_access"]
  },
  "addRealmRoles": [],
  "removeRealmRoles": [],
  "addClientRoles": {},
  "removeClientRoles": {},
  "addPermissions": ["systems:launch"],
  "removePermissions": []
}
```

## Health contexts

Health contexts are the administrative hierarchy (for example national, region,
district, facility) that scopes data access for context-scoped routes.

Admin routes, mounted under `/api/v1/admin/health-contexts`:

| Method | Path | Permission | Purpose |
| --- | --- | --- | --- |
| `GET` | `/` | `health_contexts:read` | List context nodes. |
| `POST` | `/` | `health_contexts:write` | Create a context node. |
| `GET` | `/drift` | `health_contexts:sync` | Get drift between the hierarchy and its source. |
| `POST` | `/sync/preview` | `health_contexts:sync` | Preview hierarchy sync. |
| `POST` | `/sync/apply` | `health_contexts:sync` | Apply hierarchy sync. |
| `GET` | `/:contextId` | `health_contexts:read` | Get a context node. |
| `PUT` | `/:contextId` | `health_contexts:write` | Update a context node. |
| `DELETE` | `/:contextId` | `health_contexts:write` | Delete a context node. |
| `GET` | `/:contextId/children` | `health_contexts:read` | List child nodes. |
| `POST` | `/:contextId/children` | `health_contexts:write` | Create a child node. |
| `GET` | `/:contextId/descendants` | `health_contexts:read` | List descendant nodes. |
| `GET` | `/:contextId/aliases` | `health_contexts:read` | List node aliases. |
| `PUT` | `/:contextId/aliases` | `health_contexts:sync` | Create or update a node alias. |
| `DELETE` | `/:contextId/aliases/:aliasId` | `health_contexts:sync` | Delete a node alias. |

Assignments:

| Method | Path | Permission | Purpose |
| --- | --- | --- | --- |
| `GET` | `/api/v1/admin/users/:id/health-contexts` | `health_contexts:assign` | List a user's context assignments. |
| `PUT` | `/api/v1/admin/users/:id/health-contexts` | `health_contexts:assign` | Replace a user's context assignments. |
| `GET` | `/api/v1/admin/rbac/groups/:groupId/health-contexts` | `health_contexts:assign` | List a group's context assignments. |
| `PUT` | `/api/v1/admin/rbac/groups/:groupId/health-contexts` | `health_contexts:assign` | Replace a group's context assignments. |

Current user, mounted under `/api/v1/me/health-contexts`:

| Method | Path | Permission | Purpose |
| --- | --- | --- | --- |
| `GET` | `/` | `health_contexts:read` | List contexts available to the current user. |
| `GET` | `/:contextId` | `health_contexts:read` | Get one of the current user's contexts. |
| `GET` | `/:contextId/explanation` | `health_contexts:read` | Explain why the current user has access to a context. |
| `POST` | `/select` | `health_contexts:read` | Select the current user's active context. |

## Audit logs

Mounted under `/api/v1/admin/audit-logs`. Requires `audit:read`.

| Method | Path | Purpose |
| --- | --- | --- |
| `GET` | `/` | List logs with filters. |
| `GET` | `/actions` | List known audit actions. |
| `GET` | `/metrics/overview` | Audit overview metrics. |
| `GET` | `/metrics/failed-logins-by-day` | Failed login trend. |
| `GET` | `/metrics/top-failure-ips` | Top failed-login IPs. |
| `GET` | `/export` | Export audit logs. Rate limited to 10 requests per minute per user. |
| `GET` | `/:id` | Get audit log detail. |

Filters include date range, action, user, client/system, IP, outcome, and frontend actor/module refinement.

RBAC-specific audit events are at `GET /api/v1/admin/rbac/audit` (see [RBAC governance](#rbac-governance)).

## Metrics

Mounted under `/api/v1/admin/metrics`. Requires `metrics:read`.

| Method | Path | Purpose |
| --- | --- | --- |
| `GET` | `/overview` | Dashboard overview metrics. |
| `GET` | `/system/count-users` | Count users. |
| `GET` | `/system/count-disabled-users` | Count disabled users. |
| `GET` | `/system/active-today` | Active users today. |
| `GET` | `/system/active-this-week` | Active users this week. |
| `GET` | `/system/login-trend` | Login trend. |
| `GET` | `/system/login-trend-range` | Login trend by date range. |
| `GET` | `/security/failed-logins` | Failed login count. |
| `GET` | `/security/failed-logins-range` | Failed login count by range. |
| `GET` | `/security/suspicious-logins` | Suspicious login records. |
| `GET` | `/clients/count` | Count clients. |
| `GET` | `/clients/most-accessed` | Most accessed clients. |
| `GET` | `/clients/login-count` | Login count for a client. |
| `GET` | `/clients/active-today` | Active users per client today. |
| `GET` | `/users/new-range` | New users in range. |
| `GET` | `/users/new-trend` | New users trend. |
| `GET` | `/users/never-logged-in` | Users who never logged in. |
| `GET` | `/users/last-login/:userID` | Last login for user. |
| `GET` | `/users/client-usage/:userID` | Client usage for user. |

## Notifications

See [Notifications](notifications.md) and [SMS delivery](sms-delivery.md).

Admin routes, mounted under `/api/v1/admin/notifications`:

| Method | Path | Permission | Purpose |
| --- | --- | --- | --- |
| `POST` | `/` | `notifications:write` | Create notification. |
| `GET` | `/` | `notifications:read` | List notifications. Supports `unread`, `limit`, and `offset`. |
| `GET` | `/count` | `notifications:read` | Count notifications. |
| `GET` | `/count/unread` | `notifications:read` | Count unread notifications. |
| `DELETE` | `/cleanup` | `notifications:write` | Delete old notifications. |
| `POST` | `/test-sms` | `notifications:write` | Send a test SMS. |
| `GET` | `/deliveries` | `notifications:read` | List all notification deliveries. |
| `GET` | `/deliveries/metrics` | `notifications:read` | Notification delivery metrics. |
| `GET` | `/deliveries/:deliveryID` | `notifications:read` | Get one delivery. |
| `POST` | `/deliveries/:deliveryID/retry` | `notifications:write` | Requeue a failed/retry/cancelled delivery. |
| `POST` | `/deliveries/:deliveryID/cancel` | `notifications:write` | Cancel a delivery. |
| `GET` | `/:id/deliveries` | `notifications:read` | List delivery status/history for one notification. |
| `GET` | `/:id` | `notifications:read` | Get notification. |
| `PATCH` | `/:id/read` | `notifications:read` | Mark read. |
| `DELETE` | `/:id` | `notifications:write` | Delete notification. |

Delivery statuses include `PENDING`, `PROCESSING`, `SENT`, `FAILED`, `RETRY`, and `CANCELLED`.

Current-user preferences:

| Method | Path | Permission | Purpose |
| --- | --- | --- | --- |
| `GET` | `/api/v1/notifications/preferences` | — | Get the current user's notification preferences. |
| `PUT` | `/api/v1/notifications/preferences` | — | Update the current user's notification preferences. |

Notifications submitted by integrated applications use the [Platform integration API](#platform-integration-api).

## Email

Mounted under `/api/v1/emails`.

| Method | Path | Permission | Purpose |
| --- | --- | --- | --- |
| `POST` | `/send` | `email:send` | Send immediately. |
| `POST` | `/queue` | `email:send` | Queue for worker delivery. |
| `POST` | `/recipient-preview` | `email:send` | Preview the resolved recipient list. |
| `GET` | `/` | `email:read` | List emails. |
| `GET` | `/status/:status` | `email:read` | List emails by status. |
| `GET` | `/:id` | `email:read` | Get email detail. |
| `POST` | `/:id/retry` | `email:manage` | Retry failed email. |
| `DELETE` | `/:id` | `email:manage` | Delete email record. |

Attachment config:

```env
EMAIL_MAX_ATTACHMENTS=5
EMAIL_MAX_ATTACHMENT_BYTES=10485760
EMAIL_ALLOWED_ATTACHMENT_TYPES=pdf,doc,docx,xls,xlsx,csv,png,jpg,jpeg
```

### Email payload

Used by `POST /api/v1/emails/send` and `POST /api/v1/emails/queue`. `subject` is
required; each address needs `email`. Besides `to`, recipients can be resolved from
Keycloak groups (`to_groups`, `to_group_paths`) or health contexts
(`to_health_contexts`, with `include_health_context_descendants`).

```json
{
  "to": [
    {
      "name": "Health Worker",
      "email": "worker@example.org"
    }
  ],
  "cc": [],
  "bcc": [],
  "reply_to": [],
  "to_groups": [],
  "to_group_paths": [],
  "to_health_contexts": [],
  "include_health_context_descendants": false,
  "subject": "MOH bulletin",
  "text_body": "Plain text body",
  "html_body": "<p>HTML body</p>",
  "template_name": "",
  "template_data": {},
  "attachments": [
    {
      "file_name": "bulletin.pdf",
      "content_type": "application/pdf",
      "data_base64": "base64-encoded-content",
      "inline": false
    }
  ],
  "headers": {},
  "metadata": {},
  "scheduled_at": "2026-06-16T09:00:00Z"
}
```

Attachment payloads must provide exactly one of `path` or `data_base64`.

## Documents

Mounted under `/api/v1/documents`. Context-scoped (see [Authentication and authorization](#authentication-and-authorization)).

| Method | Path | Permission | Purpose |
| --- | --- | --- | --- |
| `GET` | `/` | `documents:read` | List documents. |
| `POST` | `/` | `documents:write` | Upload/create document metadata and file. |
| `GET` | `/stats` | `documents:read` | Document statistics. |
| `POST` | `/scan-structure` | `documents:write` | Scan an uploaded file's sheet/column structure. |
| `GET` | `/:id` | `documents:read` | Get detail. |
| `PUT` | `/:id` | `documents:write` | Edit metadata. |
| `DELETE` | `/:id` | `documents:write` | Delete. |
| `GET` | `/:id/parse-structure` | `documents:read` | Parse a stored document's structure. |
| `GET` | `/:id/data-preview` | `documents:read` | Preview parsed document data. |
| `POST` | `/:id/data-preview/export` | `documents:read` | Export the data preview. |
| `GET` | `/files/:id/view` | `documents:read` | View file. |
| `GET` | `/files/:id/download` | `documents:read` | Download file. |
| `GET` | `/:id/processes` | `documents:read` | Process history. |
| `POST` | `/:id/reprocess` | `documents:process` | Reprocess. |

### Document payloads

`POST /api/v1/documents` is `multipart/form-data` with fields `file` (required),
`storage_location` (required), and optional `process_type`, `is_template`, and
`metadata`.

`PUT /api/v1/documents/:id` updates metadata:

```json
{
  "original_filename": "report.xlsx",
  "content_type": "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet"
}
```

## Document templates

Mounted under `/api/v1/document-templates`. Context-scoped.

| Method | Path | Permission | Purpose |
| --- | --- | --- | --- |
| `GET` | `/` | `document_templates:read` | List templates. |
| `POST` | `/` | `document_templates:write` | Create template. |
| `POST` | `/structure` | `document_templates:write` | Create template with sheets/columns. |
| `GET` | `/code/:code/structure` | `document_templates:read` | Structure by code. |
| `GET` | `/code/:code/has-data` | `document_templates:read` | Check whether data exists for a template code. |
| `GET` | `/:id` | `document_templates:read` | Get template. |
| `PUT` | `/:id` | `document_templates:write` | Update template. |
| `DELETE` | `/:id` | `document_templates:write` | Delete template. |
| `POST` | `/:id/publish` | `document_templates:publish` | Publish template. |
| `POST` | `/:id/archive` | `document_templates:publish` | Archive template. |
| `GET` | `/:id/structure` | `document_templates:read` | Full structure. |
| `PUT` | `/:id/structure` | `document_templates:write` | Replace the template's sheets/columns. |
| `GET` | `/:id/sheets` | `document_templates:read` | List sheets. |
| `GET` | `/:id/sheets/:sheetId/columns` | `document_templates:read` | List sheet columns. |

## Storage locations

Mounted under `/api/v1/storage-locations`.

| Method | Path | Permission | Purpose |
| --- | --- | --- | --- |
| `POST` | `/` | `storage_locations:write` | Create storage location. |
| `GET` | `/` | `storage_locations:read` | List active storage locations. |
| `GET` | `/:id` | `storage_locations:read` | Get storage location. |
| `PUT` | `/:id` | `storage_locations:write` | Update storage location. |
| `DELETE` | `/:id` | `storage_locations:write` | Delete storage location. |

## Sessions

Mounted under `/api/v1/sessions`. Requires `portal:access`.

| Method | Path | Purpose |
| --- | --- | --- |
| `GET` | `/` | List current user's sessions. |
| `DELETE` | `/:id` | End a session. |

## Data quality

Context-scoped (see [Authentication and authorization](#authentication-and-authorization)).

Issue tracker, mounted under `/api/v1/issues`:

| Method | Path | Permission | Purpose |
| --- | --- | --- | --- |
| `POST` | `/` | `issue_tracker:write` | Create issue. |
| `GET` | `/` | `issue_tracker:read` | List issues. |
| `GET` | `/summary-by-program` | `issue_tracker:read` | Issue summary by program. |
| `GET` | `/keycloak-groups` | `issue_tracker:read` | List Keycloak groups available for assignment. |
| `GET` | `/keycloak-groups/:groupId/members` | `issue_tracker:read` | List members of a Keycloak group. |
| `POST` | `/assign` | `issue_tracker:assign`, `issue_tracker:manage`, or `issue_tracker:write` | Assign issues. |
| `GET` | `/:issueCode` | `issue_tracker:read` | Get issue. |
| `PUT` | `/:issueCode` | `issue_tracker:write` | Update issue. |
| `POST` | `/:issueCode/assign` | `issue_tracker:assign`, `issue_tracker:manage`, or `issue_tracker:write` | Assign one issue. |
| `POST` | `/:issueCode/resolveIssue` | `issue_tracker:comment`, `issue_tracker:manage`, or `issue_tracker:close` | Add a resolution stage to an issue. |
| `GET` | `/:issueCode/transactions` | `issue_tracker:read` | List resolution transactions. |

Rule engine (DQA). The same routes are mounted under both `/api/v1/data-quality/dqa`
and `/api/v1/data-validation/dqa`:

| Method | Path | Permission | Purpose |
| --- | --- | --- | --- |
| `GET` | `/tables` | `data_quality:read` | List registered DQA tables. |
| `POST` | `/tables` | `data_quality:write` | Register or update a DQA table. |
| `DELETE` | `/tables/:tableId` | `data_quality:write` | Delete a DQA table. |
| `GET` | `/columns` | `data_quality:read` | List physical columns. |
| `GET` | `/rules` | `data_quality:read` | List rules. |
| `POST` | `/rules` | `data_quality:write` | Create or update a rule. |
| `POST` | `/rules/compile` | `data_quality:write` | Compile a rule (preview). |
| `POST` | `/rules/mapping/preview` | `data_quality:read` | Preview a rule mapping. |
| `POST` | `/rules/mapping/apply` | `data_quality:write` | Apply a rule mapping. |
| `DELETE` | `/rules/:tableId/:code` | `data_quality:write` | Delete a rule. |
| `POST` | `/run` | `data_quality:write` | Run checks for a table. |
| `GET` | `/schedules` | `data_quality:read` | List scheduled runs. |
| `POST` | `/schedules` | `data_quality:write` | Schedule a run. |
| `DELETE` | `/schedules/:scheduleId` | `data_quality:write` | Cancel a scheduled run. |
| `GET` | `/runs` | `data_quality:read` | List runs. |
| `GET` | `/runs/:runId/flags` | `data_quality:read` | List flags raised by a run. |
| `GET` | `/runs/:runId/flag-filters` | `data_quality:read` | List filter values for a run's flags. |
| `POST` | `/completeness/run` | `data_quality:write` | Run a completeness check. |
| `GET` | `/completeness/runs` | `data_quality:read` | List completeness runs. |
| `GET` | `/completeness/runs/:runId` | `data_quality:read` | Get a completeness run. |
| `GET` | `/completeness/runs/:runId/districts` | `data_quality:read` | List district results for a completeness run. |
| `GET` | `/completeness/runs/:runId/vhts` | `data_quality:read` | List VHT results for a completeness run. |

### Data quality payloads

Create issue (`POST /api/v1/issues`). `issue` is required. `time_period` is also
accepted as `time_Period` for older clients.

```json
{
  "program": "HMIS",
  "dataset": "HMIS",
  "data_element": "Malaria cases",
  "org_unit": "Kampala",
  "issue": "Outlier detected",
  "issue_type": "outlier",
  "priority": "high",
  "severity": "medium",
  "reported_by": "analyst@example.org",
  "time_period": "2026W24"
}
```

Resolve issue (`POST /api/v1/issues/:issueCode/resolveIssue`):

```json
{
  "stage": "resolution",
  "status": "resolved",
  "assigned_to": "analyst@example.org",
  "resolution_action": "Corrected source data",
  "resolved_by": "analyst@example.org",
  "resolution_date": "2026-06-16",
  "verification_status": "verified",
  "verified_by": "supervisor@example.org",
  "verification_date": "2026-06-16",
  "preventive_action": "Added validation rule",
  "process_change": "Review before submission",
  "preventive_owner": "DQA team",
  "due_date": "2026-06-30"
}
```

## Report Scheduler

See [Scheduler backend](scheduler-backend.md) and [Scheduler configuration](scheduler-configuration.md).

Mounted under `/api/v1/report-scheduler`. All routes except the public delivery
download require `data-statistics` system access.

| Method | Path | Permission | Purpose |
| --- | --- | --- | --- |
| `GET` | `/` | `report_scheduler:read` | Get module information. |
| `GET` | `/overview` | `report_scheduler:history` | Scheduler overview. |
| `GET` | `/metrics` | `metrics:read` and `report_scheduler:manage` | Scheduler metrics. |
| `GET` | `/reports` | `report_scheduler:read` | List reports. |
| `GET` | `/reports/:reportId` | `report_scheduler:read` | Get report. |
| `GET` | `/reports/:reportId/parameters` | `report_scheduler:read` | Get report parameters. |
| `GET` | `/report` | `report_scheduler:read` | Get report (query-parameter form). |
| `GET` | `/report-parameters` | `report_scheduler:read` | Get report parameters (query-parameter form). |
| `GET` | `/portal-reports` | `report_scheduler:read` | List portal reports. |
| `GET` | `/schedules` | `report_scheduler:read` | List schedules. |
| `POST` | `/schedules` | `report_scheduler:create` | Create schedule. |
| `GET` | `/schedules/:scheduleId` | `report_scheduler:read` | Get schedule. |
| `PUT` | `/schedules/:scheduleId` | `report_scheduler:update` | Update schedule. |
| `DELETE` | `/schedules/:scheduleId` | `report_scheduler:delete` | Delete schedule. |
| `POST` | `/schedules/:scheduleId/pause` | `report_scheduler:update` | Pause schedule. |
| `POST` | `/schedules/:scheduleId/resume` | `report_scheduler:update` | Resume schedule. |
| `POST` | `/schedules/:scheduleId/duplicate` | `report_scheduler:create` | Duplicate schedule. |
| `POST` | `/schedules/:scheduleId/run` | `report_scheduler:execute` | Run schedule now. |
| `GET` | `/executions` | `report_scheduler:history` | List executions. |
| `GET` | `/executions/:executionId` | `report_scheduler:history` | Get execution. |
| `POST` | `/executions/:executionId/retry` | `report_scheduler:execute` | Retry execution. |
| `GET` | `/recipients/options` | `report_scheduler:create` or `report_scheduler:update` | List recipient options. |
| `POST` | `/recipients/preview` | `report_scheduler:create` or `report_scheduler:update` | Preview resolved recipients. |
| `GET` | `/artifacts/:artifactId/download` | `report_scheduler:read` | Download a generated artifact. |
| `GET` | `/public/deliveries/:token/download` | Public, rate limited per IP | Download a delivered report through an emailed capability link. |

## Surveillance

Mounted under `/api/v1/surveillance`. Context-scoped (see
[Authentication and authorization](#authentication-and-authorization)). See also the
[surveillance database guide](surveillance-database.md).

| Method | Path | Permission | Purpose |
| --- | --- | --- | --- |
| `GET` | `/weeks` | `surveillance:read` | List epi weeks by year. |
| `GET` | `/alerts` | `surveillance:read` | List alerts. |
| `GET` | `/diseases` | `surveillance:read` | List diseases. |
| `GET` | `/regions` | `surveillance:read` | List regions. |
| `GET` | `/regions/:regionID/districts` | `surveillance:read` | List districts by region. |
| `GET` | `/districts` | `surveillance:read` | List districts. |
| `POST` | `/districts` | `surveillance:manage_locations` | Upsert district. |
| `GET` | `/districts/:districtID/subcounties` | `surveillance:read` | List subcounties by district. |
| `GET` | `/subcounties/:id` | `surveillance:read` | Get subcounty. |
| `POST` | `/subcounties` | `surveillance:manage_locations` | Upsert subcounty. |
| `DELETE` | `/subcounties/:id` | `surveillance:manage_locations` | Delete subcounty. |
| `GET` | `/facility-weekly-metrics/week/:epiWeekID` | `surveillance:read` | Facility weekly metrics by week. |
| `GET` | `/facility-weekly-metrics/week/:epiWeekID/disease/:diseaseID` | `surveillance:read` | Disease metrics by week. |
| `GET` | `/facility-weekly-metrics/facility/:facilityID` | `surveillance:read` | Facility metrics by facility. |
| `GET` | `/facility-weekly-metrics/facility/:facilityID/disease/:diseaseID/trend` | `surveillance:read` | Disease trend for facility. |
| `GET` | `/facility-weekly-metrics/facility/:facilityID/indicator/:indicatorID/trend` | `surveillance:read` | Indicator trend for facility. |
| `GET` | `/facility-weekly-metrics/disease-trend` | `surveillance:read` | Aggregated disease weekly trend. |
| `GET` | `/weekly-statuses/list` | `surveillance:read` | List weekly statuses. |
| `GET` | `/weekly-statuses/detailed` | `surveillance:read` | Detailed weekly statuses. |
| `GET` | `/weekly-statuses/district/week/:epiWeekID` | `surveillance:read` | District weekly statuses. |
| `GET` | `/weekly-statuses/region/week/:epiWeekID` | `surveillance:read` | Region weekly statuses. |
| `GET` | `/weekly-statuses/national/week/:epiWeekID` | `surveillance:read` | National weekly status. |
| `GET` | `/imports` | `surveillance:read` | List import batches. |
| `POST` | `/imports` | `surveillance:import` | Create import batch. |
| `GET` | `/imports/:batchID` | `surveillance:read` | Get import batch. |
| `PATCH` | `/imports/:batchID/status` | `surveillance:import` | Update import batch status. |
| `GET` | `/imports/:batchID/raw-rows` | `surveillance:read` | List raw import rows. |

## Visualiser, admin units, and GeoJSON

Visualiser and admin-unit routes are mounted under `/api/v1/visualizer`. They are
context-scoped and require one of `report_browser:read`, `documents:read`,
`data_quality:read`, or `surveillance:read`.

| Method | Path | Purpose |
| --- | --- | --- |
| `GET` | `/datasets` | List datasets. |
| `GET` | `/dataelements` | List data elements. |
| `POST` | `/datavalues` | Query data values. |
| `GET` | `/adminunits/orgunits` | Org unit lookup. |
| `GET` | `/adminunits/facilities` | Facility lookup. |
| `GET` | `/adminunits/district` | District lookup. |
| `POST` | `/adminunits/subcounties` | Subcounty lookup. |
| `POST` | `/adminunits/localgovt` | Local government lookup. |
| `POST` | `/adminunits/districts` | Districts by region. |
| `GET` | `/adminunits/region` | Region lookup. |
| `GET` | `/adminunits/national` | National hierarchy root. |
| `GET` | `/adminunits/hierarchy` | Full administrative hierarchy. |

GeoJSON, mounted under `/api/v1/geojson`:

| Method | Path | Permission | Purpose |
| --- | --- | --- | --- |
| `GET` | `/:name` | `surveillance:read`, `data_quality:read`, or `report_browser:read` | Load a named GeoJSON asset. |

## Verification

Backend:

```bash
cd backend
GOCACHE=/private/tmp/moh-sso-go-build go test ./...
```

Frontend after API type changes:

```bash
cd frontend
source ~/.nvm/nvm.sh
nvm use v20.20.1
./node_modules/.bin/tsc --noEmit -p tsconfig.app.json
NODE_OPTIONS=--max-old-space-size=8192 ./node_modules/.bin/eslint . --cache --cache-location .eslintcache --max-warnings=0
./node_modules/.bin/vite build --config vite.config.ts
```
