# RBAC Governance

The MOH SSO Dashboard uses Keycloak as the identity manager and keeps portal RBAC as the application authorization layer.

For the practical Keycloak sync and system onboarding runbook, see [RBAC And Keycloak Sync](./rbac-keycloak-sync.md).

Keycloak owns:
- users
- realm roles
- client roles
- clients/systems
- authentication and tokens

Portal RBAC owns:
- portal permission keys
- system display metadata
- system ownership metadata
- mapping Keycloak realm roles to portal permissions
- mapping Keycloak client roles to portal permissions
- access request and RBAC change request governance
- drift detection, seed import/export, and environment promotion

## Onboarding A System

1. Create or confirm the Keycloak client.
2. Create client roles in Keycloak.
3. Run drift detection from `/admin/rbac` or the CLI.
4. Preview sync before applying it.
5. Apply sync to create inert system and role records in portal RBAC.
6. Fill system ownership metadata:
   - owner team
   - owner name
   - owner email
   - support URL
   - documentation URL
   - environment
   - criticality
7. Map portal permissions to system roles.
8. Use effective access preview to confirm the access model.

Newly discovered roles do not grant portal permissions until an admin maps them.

## Drift Detection

Use the admin UI or CLI to compare Keycloak with portal RBAC.

```bash
system-rbac sync-keycloak --realm-export ../infra/keycloak/realm-export.json --draft-file /tmp/rbac-draft.yaml
system-rbac unmapped --realm-export ../infra/keycloak/realm-export.json
```

The UI supports:
- live/current drift check
- pasted realm export drift check
- sync preview
- sync apply

## Import And Export

Export the current database state:

```bash
system-rbac export-db --file config/system-rbac.seed.yaml
```

Preview or apply imports from `/admin/rbac`.

Imports support YAML or JSON. Destructive pruning is intentionally disabled unless explicitly implemented and enabled later.

The RBAC console can also:
- download the exported seed as `system-rbac.seed.json`
- upload a JSON/YAML seed for preview
- apply a previewed import after review

Use the CLI for source-controlled promotion and the UI for operational review or emergency inspection.

## Permission Catalog

The permission catalog in `/admin/rbac` is searchable by key, label, description, category, and status.

Each permission shows usage counts for:
- mapped system roles
- mapped realm roles

Use these counts before renaming, disabling, or removing a permission. A permission with active role mappings can affect existing users even when the permission itself is not directly visible in Keycloak.

## Promotion Flow

Recommended flow:

1. Export from development:
   ```bash
   system-rbac export-db --file rbac-dev.yaml
   ```
2. Review changes in version control.
3. Compare the approved file with another environment:
   ```bash
   system-rbac diff --left rbac-staging.yaml --right rbac-dev.yaml
   ```
4. Dry-run promotion:
   ```bash
   system-rbac promote --file approved-rbac.yaml
   ```
5. Apply promotion:
   ```bash
   system-rbac promote --file approved-rbac.yaml --apply
   ```

## Approvals And Requests

Access requests let users request system roles. Admins can approve, reject, or cancel requests in the RBAC console.

RBAC change requests let admins stage sensitive changes for review. High-risk examples include:
- deleting a role
- disabling a system
- removing access roles
- bulk removing permissions
- importing destructive changes

Automatic Keycloak role assignment on access approval is a future integration point. Until configured, approved requests represent a governed decision and can be completed manually in Keycloak.

## Auditing

RBAC write operations emit audit events for:
- system metadata updates
- system roles
- access roles
- permission assignments
- realm role permission mappings
- sync apply
- seed import apply
- access request decisions
- RBAC change request decisions

The audit trail is available from `/admin/rbac`.

The audit view supports filtering by:
- actor user ID
- system/client ID
- role name
- permission key
- action

Use these filters when reviewing sensitive changes, access request decisions, and environment sync/import operations.

## Troubleshooting Access

Use Effective Access in `/admin/rbac` to explain access for a user.

Check:
- Keycloak realm roles
- Keycloak client roles
- portal permission mappings
- accessible systems
- grant sources for each permission

If a user has a Keycloak role but no portal access, confirm that the role is mapped to portal permissions and that the system is enabled.

## Policy Simulation

The policy simulator can evaluate both a raw role set and a real user's current access.

It supports hypothetical changes for:
- adding/removing realm roles
- adding/removing client roles
- adding/removing direct permission overrides

The result includes:
- baseline permissions when a user is supplied
- simulated permissions
- added permissions
- removed permissions
- accessible systems
- grant sources
- warnings for incomplete inputs or unknown mappings

Use this before approving broad role changes or permission remaps.
