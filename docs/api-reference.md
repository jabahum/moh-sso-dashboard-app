# API Reference

This document is the human-readable API map for the MOH Integrated Health Portal backend.

Interactive Swagger UI:

- Backend: <http://localhost:9000/swagger/>
- When using Express: <http://localhost:3000/backend/swagger/>
- YAML/JSON: `/swagger/openapi.yaml` and `/swagger/openapi.json`

The complete machine-readable contract is generated from the registered routes and Go
handler types. Regenerate with `cd backend && make openapi-generate`; validate with
`make openapi-validate` from `backend/`. Normal backend tests enforce documentation
freshness. See [Swagger documentation maintenance](../backend/docs/README.md).

Generated OpenAPI specification:

```text
backend/docs/openapi.yaml
```

Base API prefix:

```text
/api/v1
```

Health endpoints are outside `/api/v1`.

## Response Envelope

Successful responses:

```json
{
  "success": true,
  "data": {},
  "meta": {}
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
  "meta": {}
}
```

Handlers should return feature DTOs and safe error messages. They should not expose raw SQL rows, sqlc models, filesystem paths, stack traces, or raw database errors.

## Auth

| Method | Path | Auth | Purpose |
| --- | --- | --- | --- |
| `GET` | `/api/v1/auth/login` | Public, rate limited | Start Keycloak login. |
| `GET` | `/api/v1/auth/launch?returnTo=<portal-path>` | Public, rate limited | Start Keycloak login and return to a validated portal route after authentication. |
| `GET` | `/api/v1/auth/callback` | Public, rate limited | Handle Keycloak authorization callback. |
| `GET` | `/api/v1/auth/me` | Session | Return current user, realm roles, client roles, permissions, and accessible systems. |
| `POST` | `/api/v1/auth/refresh` | Session, rate limited | Refresh portal session. |
| `GET` | `/api/v1/auth/logout` | Session | Log out current browser session. |

`auth/me` is the primary frontend bootstrap endpoint. It is the source for menu visibility, app launcher visibility, permissions, and accessible systems.

`auth/launch` accepts only `/portal` paths or absolute URLs whose origin is listed in
`AUTH_RETURN_URL_ALLOWED_ORIGINS`. The backend binds the target to the OAuth state and
uses the normal fixed Keycloak callback URI; callers must not register arbitrary return
paths as Keycloak redirect URIs.

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

Preferred dynamic access management is through RBAC user-access endpoints, because they validate Keycloak realm roles, client roles, and portal permission mappings together.

## Clients And Systems

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
| `POST` | `/api/v1/admin/clients/:id/roles` | `clients:roles:write` | Create client role. |
| `DELETE` | `/api/v1/admin/clients/:id/roles/:role` | `clients:roles:write` | Delete client role. |

When a system is added to Keycloak, RBAC sync should import it into the portal registry. Users need the relevant client role, such as `<clientId>_access`, before the app appears in menus/launcher.

## RBAC Governance

Mounted under:

```text
/api/v1/admin/rbac
```

| Method | Path | Permission | Purpose |
| --- | --- | --- | --- |
| `GET` | `/systems` | `rbac:read` | List portal systems. |
| `GET` | `/systems/:clientId` | `rbac:read` | Get system metadata. |
| `PUT` | `/systems/:clientId` | `rbac:write` | Update system metadata. |
| `GET` | `/systems/:clientId/roles` | `rbac:read` | List system roles. |
| `POST` | `/systems/:clientId/roles` | `rbac:roles:write` | Create system role. |
| `POST` | `/systems/:clientId/roles/from-template` | `rbac:roles:write` | Create role from template. |
| `POST` | `/systems/:clientId/access-roles` | `rbac:roles:write` | Add access role. |
| `DELETE` | `/systems/:clientId/access-roles/:roleName` | `rbac:roles:write` | Remove access role. |
| `GET` | `/permissions` | `rbac:read` | List permission catalog. |
| `PUT` | `/permissions/:permissionKey` | `rbac:permissions:write` | Update permission metadata. |
| `GET` | `/realm-roles` | `rbac:read` | List realm-role mappings. |
| `GET` | `/realm-roles/:realmRole/usage` | `rbac:read` | Show realm role usage. |
| `POST` | `/realm-roles/:realmRole/permissions` | `rbac:permissions:write` | Assign permission to realm role. |
| `DELETE` | `/realm-roles/:realmRole/permissions/:permissionKey` | `rbac:permissions:write` | Remove permission from realm role. |
| `PUT` | `/roles/:roleId` | `rbac:roles:write` | Update role metadata. |
| `DELETE` | `/roles/:roleId` | `rbac:roles:write` | Delete role. |
| `GET` | `/roles/:roleId/usage` | `rbac:read` | Show role usage. |
| `POST` | `/roles/:roleId/copy-permissions` | `rbac:permissions:write` | Copy permissions. |
| `POST` | `/roles/:roleId/permissions` | `rbac:permissions:write` | Assign permission to role. |
| `DELETE` | `/roles/:roleId/permissions/:permissionKey` | `rbac:permissions:write` | Remove permission from role. |
| `GET` | `/groups` | `rbac:read` | List synced Keycloak/RBAC groups. |
| `POST` | `/groups` | `rbac:roles:write` | Create or update a portal group mapping. |
| `GET` | `/groups/:groupId` | `rbac:read` | Get group roles, permissions, and metadata. |
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
| `GET` | `/drift` | `rbac:read` | Get Keycloak/portal drift report. |
| `POST` | `/drift/realm-export` | `rbac:read` | Generate drift from realm export. |
| `POST` | `/sync/preview` | `rbac:read` | Preview sync changes. |
| `POST` | `/sync/apply` | `rbac:write` | Apply sync changes. |
| `GET` | `/export` | `rbac:read` | Export RBAC seed data. |
| `POST` | `/import/preview` | `rbac:read` | Preview seed/import changes. |
| `POST` | `/import/apply` | `rbac:write` | Apply seed/import changes. |
| `GET` | `/role-templates` | `rbac:read` | List role templates. |
| `POST` | `/bulk/assign-permission` | `rbac:permissions:write` | Bulk assign permission. |
| `POST` | `/bulk/remove-permission` | `rbac:permissions:write` | Bulk remove permission. |
| `GET` | `/access-requests` | `rbac:roles:write` | List access requests. |
| `POST` | `/access-requests/:id/:decision` | `rbac:roles:write` | Approve or reject access request. |
| `GET` | `/change-requests` | `rbac:read` | List change requests. |
| `POST` | `/change-requests` | `rbac:write` | Create change request. |
| `POST` | `/change-requests/:id/:decision` | `rbac:write` | Approve or reject change request. |

Authenticated users can create access requests:

| Method | Path | Permission | Purpose |
| --- | --- | --- | --- |
| `POST` | `/api/v1/access-requests` | `portal:access` | Request access to a system or role. |

## Announcements

Public/user routes:

| Method | Path | Auth | Purpose |
| --- | --- | --- | --- |
| `GET` | `/api/v1/announcements/public` | Public | List public published announcements. |
| `GET` | `/api/v1/announcements/active` | Public | List active published announcements. |
| `GET` | `/api/v1/announcements/me` | `announcements:read` | List announcements targeted to current user. |
| `GET` | `/api/v1/announcements/user` | `announcements:read` | List user-targeted announcements. |
| `GET` | `/api/v1/announcements/role/:role_name` | `announcements:read` | List role-targeted announcements. |
| `GET` | `/api/v1/announcements/client/:client_id` | `announcements:read` | List client-targeted announcements. |
| `GET` | `/api/v1/announcements/:id/attachments/:attachmentId/download` | `announcements:read` | Download attachment. |

Admin routes under `/api/v1/admin/announcements`:

| Method | Path | Permission | Purpose |
| --- | --- | --- | --- |
| `GET` | `/` | `announcements:read` | List announcements. |
| `GET` | `/stats` | `announcements:read` | Announcement metrics. |
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

Announcement links:

- `link_url` can be a `/portal...` path or absolute URL.
- `link_label` controls CTA text in News & Updates and email templates.

Attachment rules:

- Persistent attachments are stored and returned with `download_url`.
- Email transient attachments must provide exactly one of `path` or `data_base64`.
- Persistent attachments marked `include_in_email` can be attached to announcement emails and rendered as download links.

## Audit Logs

Mounted under `/api/v1/admin/audit-logs`. Requires `audit:read`.

| Method | Path | Purpose |
| --- | --- | --- |
| `GET` | `/` | List logs with filters. |
| `GET` | `/actions` | List known audit actions. |
| `GET` | `/metrics/overview` | Audit overview metrics. |
| `GET` | `/metrics/failed-logins-by-day` | Failed login trend. |
| `GET` | `/metrics/top-failure-ips` | Top failed-login IPs. |
| `GET` | `/export` | Export audit logs. |
| `GET` | `/:id` | Get audit log detail. |

Filters include date range, action, user, client/system, IP, outcome, and frontend actor/module refinement.

## Notifications

Mounted under `/api/v1/admin/notifications`.

| Method | Path | Permission | Purpose |
| --- | --- | --- | --- |
| `POST` | `/` | `notifications:write` | Create notification. |
| `GET` | `/` | `notifications:read` | List notifications. Supports `unread`, `limit`, and `offset`. |
| `GET` | `/count` | `notifications:read` | Count notifications. |
| `GET` | `/count/unread` | `notifications:read` | Count unread notifications. |
| `DELETE` | `/cleanup` | `notifications:write` | Delete old notifications. |
| `GET` | `/:id/deliveries` | `notifications:read` | List delivery status/history for one notification. |
| `POST` | `/deliveries/:deliveryID/retry` | `notifications:write` | Requeue a failed/retry/cancelled delivery. |
| `GET` | `/:id` | `notifications:read` | Get notification. |
| `PATCH` | `/:id/read` | `notifications:read` | Mark read. |
| `DELETE` | `/:id` | `notifications:write` | Delete notification. |

Delivery statuses include `PENDING`, `PROCESSING`, `SENT`, `FAILED`, `RETRY`, and `CANCELLED`.

## Email

Mounted under `/api/v1/emails`.

| Method | Path | Permission | Purpose |
| --- | --- | --- | --- |
| `POST` | `/send` | `email:send` | Send immediately. |
| `POST` | `/queue` | `email:send` | Queue for worker delivery. |
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

## Documents

Mounted under `/api/v1/documents`.

| Method | Path | Permission | Purpose |
| --- | --- | --- | --- |
| `GET` | `/` | `documents:read` | List documents. |
| `POST` | `/` | `documents:write` | Upload/create document metadata and file. |
| `GET` | `/:id` | `documents:read` | Get detail. |
| `PUT` | `/:id` | `documents:write` | Edit metadata. |
| `DELETE` | `/:id` | `documents:write` | Delete. |
| `GET` | `/files/:id/view` | `documents:read` | View file. |
| `GET` | `/files/:id/download` | `documents:read` | Download file. |
| `GET` | `/:id/processes` | `documents:read` | Process history. |
| `POST` | `/:id/reprocess` | `documents:process` | Reprocess. |

## Document Templates

Mounted under `/api/v1/document-templates`.

| Method | Path | Permission | Purpose |
| --- | --- | --- | --- |
| `GET` | `/` | `document_templates:read` | List templates. |
| `POST` | `/` | `document_templates:write` | Create template. |
| `POST` | `/structure` | `document_templates:write` | Create template with sheets/columns. |
| `GET` | `/code/:code/structure` | `document_templates:read` | Structure by code. |
| `GET` | `/:id` | `document_templates:read` | Get template. |
| `PUT` | `/:id` | `document_templates:write` | Update template. |
| `DELETE` | `/:id` | `document_templates:write` | Delete template. |
| `POST` | `/:id/publish` | `document_templates:publish` | Publish template. |
| `POST` | `/:id/archive` | `document_templates:publish` | Archive template. |
| `GET` | `/:id/structure` | `document_templates:read` | Full structure. |
| `GET` | `/:id/sheets` | `document_templates:read` | List sheets. |
| `GET` | `/:id/sheets/:sheetId/columns` | `document_templates:read` | List sheet columns. |

## Data Quality

Mounted under `/api/v1/issues`.

| Method | Path | Permission | Purpose |
| --- | --- | --- | --- |
| `POST` | `/` | `data_quality:write` | Create issue. |
| `GET` | `/` | `data_quality:read` | List issues. |
| `PUT` | `/:issueCode` | `data_quality:write` | Update issue. |
| `POST` | `/:issueCode/resolveIssue` | `data_quality:resolve` | Resolve issue. |
| `GET` | `/:issueCode/transactions` | `data_quality:read` | List resolution transactions. |

Data validation UI is a frontend module for validation rules and rule operations. API expansion should stay DTO-first and use the same response envelope.

## Surveillance

Mounted under `/api/v1/surveillance`.

Common permissions:

- `surveillance:read`
- `surveillance:import`
- `surveillance:manage_locations`
- `surveillance:manage_alerts`

Main route groups:

| Method | Path | Purpose |
| --- | --- | --- |
| `GET` | `/weeks` | List epi weeks. |
| `GET` | `/alerts` | List alerts. |
| `GET` | `/diseases` | List diseases. |
| `GET` | `/regions` | List regions. |
| `GET` | `/districts` | List districts. |
| `POST` | `/districts` | Upsert district. |
| `GET` | `/districts/:districtID/subcounties` | List subcounties. |
| `POST` | `/subcounties` | Upsert subcounty. |
| `DELETE` | `/subcounties/:id` | Delete subcounty. |
| `GET` | `/facility-weekly-metrics/...` | Facility, disease, indicator, and trend metrics. |
| `GET` | `/weekly-statuses/...` | Detailed district/region/national weekly status. |
| `GET` | `/imports` | List import batches. |
| `POST` | `/imports` | Create import batch. |
| `GET` | `/imports/:batchID` | Get import batch. |
| `PATCH` | `/imports/:batchID/status` | Update import status. |
| `GET` | `/imports/:batchID/raw-rows` | List raw import rows. |

## Metrics

Mounted under `/api/v1/admin/metrics`. Requires `metrics:read`.

Route groups:

- `/overview`
- `/system/*`
- `/security/*`
- `/clients/*`
- `/users/*`

## Sessions

Mounted under `/api/v1/sessions`. Requires `portal:access`.

| Method | Path | Purpose |
| --- | --- | --- |
| `GET` | `/` | List current user's sessions. |
| `DELETE` | `/:id` | End a session. |

## Storage Locations

Mounted under `/api/v1/storage-locations`.

| Method | Path | Permission | Purpose |
| --- | --- | --- | --- |
| `POST` | `/` | `storage_locations:write` | Create storage location. |
| `GET` | `/` | `storage_locations:read` | List active storage locations. |
| `GET` | `/:id` | `storage_locations:read` | Get storage location. |
| `PUT` | `/:id` | `storage_locations:write` | Update storage location. |
| `DELETE` | `/:id` | `storage_locations:write` | Delete storage location. |

## Visualiser, Admin Units, And GeoJSON

Visualiser is mounted under `/api/v1/visualizer`.

Route groups include:

- `/datasets`
- `/dataelements`
- `/datavalues`
- `/themes`
- `/dataelements/theme`
- `/hiv/*`
- `/adminunits/*`

GeoJSON is mounted under:

```text
/api/v1/geojson/:name
```

Access is granted to users with a relevant reporting, surveillance, or data-quality permission.

## Health

| Method | Path | Auth | Purpose |
| --- | --- | --- | --- |
| `GET` | `/health` | Public | General health check. |
| `GET` | `/health/live` | Public | Liveness. |
| `GET` | `/health/ready` | Public | Readiness. |
| `GET` | `/version` | Public | Dependency-free backend build metadata. |

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
