# Health context mapping

This page explains how to map health-context nodes to identifiers in other features and systems, how to onboard a district or facility, and how modules adopt the mapping. It is for administrators and integrators.

## Alias namespaces

Aliases link a health-context node to stable identifiers owned by other
features or systems.

Recommended namespace examples:

| Namespace | External ID |
| --- | --- |
| `dhis2` | DHIS2 organisation-unit UID |
| `dwh-org-unit` | DWH organisation-unit UID |
| `surveillance-region` | Surveillance region UUID |
| `surveillance-district` | Surveillance district UUID |
| `surveillance-sub-county` | Surveillance sub-county UUID |
| `surveillance-facility` | Surveillance facility UUID |
| `geography-region`, `geography-district`, `geography-sub-county`, `geography-facility` | Primary geography registry UUID |

In the current code, the visualiser (`backend/modules/visualiser/health_context.go`)
reads `dwh-org-unit` and `surveillance-*` aliases. The `admin-units` module reads
`dwh-org-unit`, and surveillance reads `surveillance-*`. Migration
`000060_primary_geography_registry` creates and maintains `geography-*` aliases
automatically for the primary geography registry. No code currently reads
`dhis2`; it is a recommended namespace only.

Never map by a display name alone. Names can be duplicated or renamed.

## Onboarding a district or facility

1. Identify or create its parent context.
2. Create the node with a stable code and appropriate type.
3. Add aliases for each integrated source.
4. Map the responsible Keycloak group explicitly.
5. Assign `NODE_ONLY` or `NODE_AND_DESCENDANTS`.
6. Confirm `/api/v1/me/health-contexts/:id/explanation` for a member.
7. Test access to an authorized node and denial for a sibling.

## Module adoption

A module resolves its domain identifier through the alias namespace, then uses
the contextual policy before querying or mutating data. Modules must fail closed
when a scoped record has no resolvable context after their compatibility period.
The frontend may send the active context, but the backend owns alias resolution
and authorization.
