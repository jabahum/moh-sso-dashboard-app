# Standard human-user role defaults

This page describes the default roles every human user receives, how the application, database and Keycloak apply them, and how to reconcile existing Keycloak accounts. It is for administrators and operators who manage Keycloak and portal releases.

The standard baseline is the realm role `user`, with these client-role composites:

| Client | Default roles |
| --- | --- |
| `dashboard-web` | `dashboard-web_access`, `portal_user` |
| `data-statistics` | `report-scheduler_access`, `report_viewer`, `data-statistics_access`, `issue_tracker_viewer`, `document_viewer` |
| `settings` | `settings_access` |

These are defaults, not a restriction on explicit grants. Individual assignments,
group assignments, administrator and manager roles retain their existing behavior.
The existing `report-scheduler_access` definition permits creating, updating,
deleting, executing and viewing schedules. This change does not redefine that role.

## Application and database

`authz.DefaultUserClientRoles()` defines the compiled baseline. The YAML seed has
the same mappings, checked by backend tests. The default Portal Users group also
lists the baseline roles. Human accounts created through the portal/management CLI
are assigned the `user` realm role after creation. Failure to resolve that role
prevents creation; failure to assign it reports the created account ID for recovery.

Migration 53 (`000053_standard_user_client_role_defaults`) replaces only `ihp_realm_role_system_roles` rows for realm role `user`
and registers `portal_user` as a portal access role. It preserves individual/group
grants, role permission definitions, and other realm-role mappings. Its private
database snapshot allows the down migration to restore the original mappings.
Startup discovery still uses the existing lifecycle/metadata behavior.

Deploy a release that includes migration 53 through the normal release process
to apply the migration and provisioning change. Updating Keycloak alone does not remove the old application's
database-derived Utilities default. Avoid reseeding with an older application
image after applying the new database baseline.

## Keycloak reconciliation

The `user` role in `infra/keycloak/realm-export.json` and
`infra/keycloak/realm-export-prod.json` carries the full baseline as composites
(checked against migration 53 by a backend test), so a freshly imported realm starts
with it. An existing realm keeps whatever composites it already has until it is
reconciled. When
`RBAC_STARTUP_SYNC_PUSH_TO_KEYCLOAK` is enabled, backend startup adds the mapped
client roles as composites of `user`, but it does not assign `user` to existing
accounts. It is off by default (the compiled default and every shipped
`infra/env/*.env.example` are `false`). The reconciliation tool below handles both steps.

Do not add `user` to the realm-wide default role or a realm-wide default group:
service accounts may inherit those defaults. Provisioning outside the portal
(for example, manual Keycloak or external imports) must assign `user` only to
human accounts. The reconciliation tool backfills existing human accounts and
can be rerun after such imports.

### Generate the policy and dry run

Generate the nonsecret policy from the same compiled baseline:

```sh
cd backend
go run ./cmd/cli system-rbac user-defaults > /tmp/standard-user-defaults.json
cd ..
python3 infra/scripts/reconcile-keycloak-user-defaults.py \
  --policy /tmp/standard-user-defaults.json
```

Use the configured `KEYCLOAK_INTERNAL_URL` (or `KEYCLOAK_BASE_URL`),
`KEYCLOAK_REALM`, `KEYCLOAK_ADMIN_CLIENT_ID`, and `KEYCLOAK_ADMIN_CLIENT_SECRET`
environment without printing their values. The service account needs permission
to manage realm-role composites and human-user realm-role mappings. A dry run
reports aggregate counts only and does not change Keycloak.

### Apply the plan

To apply the reviewed plan:

```sh
python3 infra/scripts/reconcile-keycloak-user-defaults.py \
  --policy /tmp/standard-user-defaults.json \
  --apply --backup /secure/new-user-defaults-before.json
```

### Backup and rollback

The backup must be a new file in an operator-owned directory. It is created with
mode `0600` before the first write and contains previous composites and the human
account IDs to backfill. Keep it outside the repository. A partial failure can be
retried with a fresh backup: role additions are idempotent. For rollback, restore
the saved `beforeComposites` on realm role `user`; review the snapshot before
removing any backfilled `user` assignments, since later grants may be intentional.

### Safeguards

The tool validates all baseline roles before writes, paginates accounts and role
members, explicitly excludes service accounts, and refuses to proceed if a service
account already inherits `user`. It updates only `user` composites and adds `user`
to human accounts missing that role. It never rewrites client-role definitions,
individual client-role assignments, group mappings or realm-wide default roles.
Existing tokens/sessions need refresh or a new sign-in to receive the new claims.

## Verification

Backend tests cover compiled/YAML consistency, explicit grant preservation,
administrator permissions, and human creation failure handling. The disposable
PostgreSQL suite checks migration upgrade/rollback and preservation of elevated
and group grants. `python3 infra/scripts/test-reconcile-user-defaults.py` covers
dry-run behavior, pagination, service-account exclusion and backup permissions.
