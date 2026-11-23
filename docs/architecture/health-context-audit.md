# Health context audit

This page records the audit of existing location hierarchies, the query surfaces that lacked a contextual-authorization boundary, the integration decision, and the rollout status of health-context enforcement. It is for developers and architects planning feature adoption; the contract itself is in [Health context architecture](./health-context.md).

## Existing hierarchy models

The platform already has several location representations. They serve different
purposes and must not be collapsed without a migration plan.

| Source | Purpose | Current identifiers | Health-context relationship |
| --- | --- | --- | --- |
| `admin_units` | Reads the DWH organisation-unit hierarchy | DHIS2-style UIDs and names | Map with aliases such as `dhis2` or `dwh-org-unit` |
| Surveillance | Operational regions, districts, sub-counties, and facilities | Portal UUIDs and codes | Map each scoped record with `surveillance-*` aliases |
| GeoJSON | Boundary geometry delivery | Static boundary metadata | Reference context aliases; do not store geometry in health context |
| Visualiser | Reads `hiv.organisation_unit` | Organisation-unit UIDs | Constrain requested UIDs using aliases before querying |
| RBAC groups | Identity membership and role/permission inheritance | Portal group UUID, Keycloak ID, path | Explicitly map groups to context nodes |

The canonical authorization hierarchy is `health_context_nodes` and
`health_context_closure`. Domain features retain their own operational fields.
`health_context_aliases` connects an authorization node to a domain or external
identifier without relying on display names.

## Current risk inventory

Before health-context integration, the following query surfaces accept location
filters without a common contextual-authorization boundary:

- Surveillance alerts, weekly status, dashboards, imports, and facility metrics.
- Visualiser organisation-unit selections and report drill-downs.
- Admin-unit hierarchy and GeoJSON boundary endpoints.
- Data-quality runs and issues.
- Issue-tracker records.

This list records the state before adoption; see the rollout status below for
what has since been migrated. Endpoints continue to use their existing
authorization until their owning feature adopts the integration contract. The presence of
`X-Health-Context-ID` or the frontend selector alone does not make an endpoint
context-safe.

## Integration decision

1. Keep clinical, reporting, and geometry attributes in their owning features.
2. Store generic hierarchy, assignments, active selection, and scope mode in
   `health_context`.
3. Map existing records by stable namespace and external ID.
4. Resolve aliases and authorize them server-side before applying query filters.
5. Add resource ownership and query predicates feature by feature.

## Rollout status

| Capability | Status |
| --- | --- |
| Hierarchy, closure table, aliases | Implemented |
| Direct and group assignments | Implemented |
| Active context and `/auth/me` summary | Implemented |
| Permission plus system access plus context policy | Implemented as reusable boundary |
| Shell selector and microfrontend propagation | Implemented |
| Surveillance row and aggregate enforcement | Implemented: routes are in the context-scoped group and queries receive authorized alias IDs (`backend/internal/features/surveillance/health_context.go`, migration `000056`) |
| Visualiser enforcement | Implemented: requested organisation units are constrained by `dwh-org-unit` and `surveillance-*` aliases (`backend/modules/visualiser/health_context.go`) |
| Admin-unit hierarchy | Implemented in the `admin-units` module through the shared context selector and `dwh-org-unit` aliases |
| Report browser enforcement | Pending: depends on the external Health BI service |
| Document ownership, mutations, downloads, previews, and statistics | Implemented; legacy records are initially classified at the Uganda root |
| Issue-tracker (data-quality issue) ownership | Implemented: `data_quality_issue_contexts` (migration `000057`) and scoped `/api/v1/issues` handlers |
| Data-quality runs, rules and schedules | Pending: the routes sit in the context-scoped group, so the header is validated, but results are not filtered by context |
| Context-aware communication audiences | Implemented: announcement `SPECIFIC_HEALTH_CONTEXTS` and `HEALTH_CONTEXT_AND_DESCENDANTS` audiences (migration `000058`), audience count preview, and email `to_health_contexts` recipients |
| Group-mapping drift UI and reconciliation | Implemented: drift report with preview-first apply in RBAC Management (`/api/v1/admin/health-contexts/drift`, `/sync/preview`, `/sync/apply`). Reconciliation applies reviewed mappings; it does not change Keycloak |
