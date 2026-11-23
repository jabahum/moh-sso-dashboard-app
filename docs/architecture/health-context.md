# Health context architecture

This page describes the health-context model, the request and feature integration contracts, and how health context relates to RBAC and the observatory database. It is for developers adding context-scoped features. Administrators should also read [Health context administration](../admin/health-context-management.md).

Health context describes where, for whom, or over which health data a user may
act. It complements RBAC:

```text
effective access = functional permission + system access + health-context scope
```

RBAC answers what the actor may do. Health context answers where the actor may
do it. Neither replaces the other.

## Model

- `health_context_nodes` stores configurable hierarchy nodes.
- `health_context_closure` supports ancestor and descendant queries.
- `health_context_aliases` maps domain and external identifiers.
- `user_health_context_assignments` stores direct, time-bound assignments.
- `group_health_context_assignments` maps portal/Keycloak groups explicitly.
- `user_active_health_contexts` stores the selected authorized context.

Initial node types include national, regional, district, city, division,
municipality, county, sub-county, parish, facility, program, department, team,
and custom. These types do not impose a hardcoded parent sequence.

Assignments use:

- `NODE_ONLY`: only the assigned node.
- `NODE_AND_DESCENDANTS`: the node and closure-table descendants.

Every authenticated user also receives the enabled Uganda (`UG`) National node
with `NODE_AND_DESCENDANTS` as a built-in `DEFAULT` baseline. Explicit
assignments to that node are folded into the baseline, so their scope mode has
no further effect (`ListEffectiveContexts` in
`backend/internal/features/health_context/repository_postgres.go`). For any
other node, the scope mode applies as written. A facility assignment never
implies district access.

## Request contract

Context-sensitive requests send:

```http
X-Health-Context-ID: <authorized-context-uuid>
```

The backend parses and authorizes the value. The contextual route group
(`ContextScopedGroup` in `backend/internal/api/routes/protected_routes.go`) covers
documents, data quality and issues, visualiser and surveillance routes. It
requires `data-statistics` system access and runs
`RequireAccessibleSelection`. That middleware rejects a missing header with
`400 HEALTH_CONTEXT_REQUIRED` whenever the caller has any effective context.
Because of the National baseline, that means every user while the `UG` node is
enabled. A missing header is accepted only when the user has no effective
contexts at all. A malformed value returns `400`, an unauthorized one `403`,
and a resolution failure `503`. A feature must still load the resource, resolve
its context, and apply ownership checks in its service or repository. The
header is untrusted input.

### Document ownership

Documents created with a selected context store that context as immutable
ownership metadata. Document list, detail, edit, delete, download, preview,
process history, reprocess, and statistics operations apply the selected node
or authorized descendant predicate. Migration `000055` classifies documents
created before contextual authorization at the seeded Uganda national root.
Administrators should reclassify those records to narrower contexts where the
source data supports it.

## Feature integration contract

For each scoped feature:

1. Define the stable alias namespace for existing domain IDs.
2. Add or derive immutable resource-to-context ownership.
3. Require ordinary permission and system access.
4. Authorize the requested resource context with `health_context.Policy`.
5. Apply authorized context IDs in list, aggregate, export, and download SQL.
6. Reject inaccessible filters; never silently widen them.
7. Invalidate frontend queries when active context changes.
8. Add cross-context denial tests before enabling enforcement.

The selector is a working-context convenience, not a security boundary.

## External and incremental modules

Modules that query another backend, such as the report browser (an embedded
Health BI frontend at `HEALTH_BI_BASE_URL`), must propagate `X-Health-Context-ID` and enforce the same context on that
service. Receiving the value is not authorization: the downstream service must
validate the caller and constrain list, detail, drill-down, aggregate, export,
and download operations. Until a downstream service supports this contract,
the portal may limit navigation with RBAC and system access but must not claim
that its data is context-isolated. The issue tracker and visualiser call the
portal's own context-scoped routes unless `VITE_VISUALIZER_API_BASE` points them
at another backend.

The shared `baseApi` adds the active context header for portal RTK Query
requests. Single-spa applications also receive the active and available
contexts through `MicrofrontendRuntimeProps`; independently deployed modules
must consume that public contract rather than importing shell internals.

## Observatory database integration

Health-context ownership, aliases, assignments, and closure remain in the primary
portal database. Migration `000060_primary_geography_registry` also moves the
shared geography reference data (regions, districts, sub-counties, facilities) into
a primary `geography` schema. Its triggers keep matching health-context nodes and
`geography-*` aliases in step, alongside the existing `surveillance-*` aliases.
Surveillance facts and metrics remain in observatory. `backend/internal/geography`
replicates geography one way, from primary to observatory. A full sync runs when
the observatory connection is (re)established, and later refreshes run at most
once a minute. Existing context IDs are retained when names or parents change.
Source/synchronization failures return an error instead of falling back to
unrestricted data access.

Scoped surveillance queries receive authorized UUID lists resolved in the primary
database; they do not join primary tables or call primary functions in observatory.
Hierarchy selectors may include ancestors, while metric and alert queries use only
the selected scope and its permitted descendants. The health-context primary
migrations are versions 54–58 (`000054_add_health_contexts` to
`000058_add_announcement_health_context_audience`), and migration 60 adds the
primary geography registry.

## Observability contract

Context changes and sync operations are audited, and contextual denials are
logged as structured events. When the project observability exporter is wired,
it should expose these low-cardinality instruments:

```text
health_context_assignment_total{source,type}
health_context_authorization_denied_total{feature,context_type}
health_context_switch_total{context_type}
health_context_sync_total{source,result}
health_context_sync_duration_seconds{source}
```

Context IDs, user IDs, facility codes, district names, and group paths must
never be metric labels. These names are the stable instrumentation contract;
the current business metrics feature is not used as a substitute for a
Prometheus/OpenTelemetry exporter.
