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
| `surveillance-region` | Surveillance region UUID or stable code |
| `surveillance-district` | Surveillance district UUID or stable code |
| `surveillance-sub-county` | Surveillance sub-county UUID or stable code |
| `surveillance-facility` | Surveillance facility UUID or stable code |

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
