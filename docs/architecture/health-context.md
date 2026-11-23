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

A national assignment does not include descendants unless its scope mode says
so. A facility assignment never implies district access.

## Request contract

Context-sensitive requests send:

```http
X-Health-Context-ID: <authorized-context-uuid>
```

The backend parses and authorizes the value. Contextual route groups require
the header when the caller has one or more health-context assignments. Users
without assignments retain legacy behavior while rollout is in progress. A
feature must still load the resource, resolve its context, and apply ownership
checks in its service or repository. The header is untrusted input.

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

Modules that query another backend, including report browser and issue tracker,
must propagate `X-Health-Context-ID` and enforce the same context on that
service. Receiving the value is not authorization: the downstream service must
validate the caller and constrain list, detail, drill-down, aggregate, export,
and download operations. Until a downstream service supports this contract,
the portal may limit navigation with RBAC and system access but must not claim
that its data is context-isolated.

The shared `baseApi` adds the active context header for portal RTK Query
requests. Single-spa applications also receive the active and available
contexts through `MicrofrontendRuntimeProps`; independently deployed modules
must consume that public contract rather than importing shell internals.

## Observatory database integration

Health-context ownership, aliases, assignments, and closure remain in the primary
portal database. Surveillance geography and metrics remain in observatory.
Geography aliases are synchronized through the two application database connections
when context geography is requested, with a one-minute cache. Existing context IDs
are retained when names or parents change. Source/synchronization failures return
an error instead of falling back to unrestricted data access.

Scoped surveillance queries receive authorized UUID lists resolved in the primary
database; they do not join primary tables or call primary functions in observatory.
Hierarchy selectors may include ancestors, while metric and alert queries use only
the selected scope and its permitted descendants. The new primary migrations use
versions 54–58, after the current upstream history, to avoid version collisions.

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
