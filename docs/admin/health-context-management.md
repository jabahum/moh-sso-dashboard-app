# Health context administration

This page describes how administrators manage the health-context hierarchy, assignments and group mappings, and lists the related admin and user API endpoints. For the underlying model, see [Health context architecture](../architecture/health-context.md).

## Required permissions

Administrators need the corresponding permissions:

- `health_contexts:read`
- `health_contexts:write`
- `health_contexts:assign`
- `health_contexts:sync`
- `health_contexts:audit`

## Administration screen

RBAC Management includes a Health Context workspace for:

- browsing and searching hierarchy nodes;
- creating, editing, enabling, and disabling nodes;
- selecting parents without creating hierarchy cycles;
- assigning direct user scope;
- mapping Keycloak-backed groups to context scope;
- distinguishing direct and group-derived access;
- reviewing effective-access explanations; and
- previewing and applying group mapping drift reconciliation.

Destructive actions use shared confirmation modals. Mutations report through
the shared toast system. Drift application is preview-first and requires
`health_contexts:sync`.

Announcement and email audience editors can select one or more authorized
health contexts. Announcement authors can preview the recipient count before
publishing. The preview returns a count only and does not expose recipient
personal data.

## Default national context and assignments

Every authenticated portal user receives the enabled Uganda (`UG`) National
context automatically, with `NODE_AND_DESCENDANTS` scope. This is a built-in
`DEFAULT` entitlement; it needs no Keycloak role or per-user database assignment
and applies to existing users and first-time logins. It appears once even when
National is also assigned directly or through groups. Editing assignments cannot
remove this baseline. National becomes the default when no other enabled, valid
direct default exists; a saved active selection is preserved when still available.
Application roles and permissions continue to govern access to each feature.
National access also includes users in geographic notification audiences beneath
Uganda; disabled cached users are not added through this baseline.

Use direct assignments for exceptional individual scope. Prefer explicit
Keycloak group mappings for managed teams. Select one direct assignment as the
default where appropriate.

## Admin API

Current hierarchy endpoints:

```text
GET    /api/v1/admin/health-contexts
POST   /api/v1/admin/health-contexts
GET    /api/v1/admin/health-contexts/:contextId
PUT    /api/v1/admin/health-contexts/:contextId
DELETE /api/v1/admin/health-contexts/:contextId
GET    /api/v1/admin/health-contexts/:contextId/children
POST   /api/v1/admin/health-contexts/:contextId/children
GET    /api/v1/admin/health-contexts/:contextId/descendants
GET    /api/v1/admin/health-contexts/:contextId/aliases
PUT    /api/v1/admin/health-contexts/:contextId/aliases
DELETE /api/v1/admin/health-contexts/:contextId/aliases/:aliasId
```

Assignment endpoints:

```text
GET /api/v1/admin/users/:userId/health-contexts
PUT /api/v1/admin/users/:userId/health-contexts
GET /api/v1/admin/rbac/groups/:groupId/health-contexts
PUT /api/v1/admin/rbac/groups/:groupId/health-contexts
```

Updates require the current `version`; stale updates return `409`. Reparenting
maintains the closure table and rejects cycles. Deletion returns `409` while a
node has children or direct/group assignments.

## User endpoints

```text
GET  /api/v1/me/health-contexts
GET  /api/v1/me/health-contexts/:contextId
GET  /api/v1/me/health-contexts/:contextId/explanation
POST /api/v1/me/health-contexts/select
```
