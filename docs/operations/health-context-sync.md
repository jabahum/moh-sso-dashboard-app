# Health context sync

This page covers how health contexts and group mappings are seeded at startup and reconciled with Keycloak groups, including the operating procedure, rollback and troubleshooting. It is for operators and administrators.

## Startup seed

`backend/config/system-rbac.seed.yaml` may define:

```yaml
healthContexts:
  - code: UG
    name: Uganda
    type: NATIONAL
    scopeMode: NODE_AND_DESCENDANTS

groupContextMappings:
  - groupPath: /MOH/Platform Administrators
    contextCode: UG
    scopeMode: NODE_AND_DESCENDANTS
```

Mappings are explicit. Group path parsing is not used to infer facilities or
districts. Generic seed files must not contain production user assignments.

Startup applies hierarchy parents before children and maps only known portal
RBAC groups. Keycloak membership synchronization remains responsible for the
members of those groups; health context does not become an identity provider.

## Operational procedure

1. Synchronize Keycloak groups into the portal RBAC registry.
2. Create or import health-context nodes.
3. Add stable aliases for surveillance, DHIS2, DWH, or other identifiers.
4. Map Keycloak-backed portal groups to contexts and choose a scope mode.
5. Open RBAC Management and review the Health Context drift report.
6. Preview the proposed group mappings. A preview never mutates assignments.
7. Apply only reviewed mappings with an actor that has
   `health_contexts:sync`.
8. Review effective access for representative users.
9. Test a denied sibling context before enabling feature enforcement.
10. Audit the mapping and assignment changes.

### Reconciliation behaviour

Missing, renamed, deleted, and unmapped Keycloak groups should be reviewed
before destructive reconciliation. Applying mappings is non-destructive by
default. `replaceExisting` replaces mappings only for groups explicitly present
in that request; it does not globally delete mappings for omitted groups.

### Drift report

The drift endpoint reports missing Keycloak links, disabled groups, unmapped
groups, inactive contexts, and mappings that are in sync. Apply operations are
audited with the actor, requested mapping count, replacement mode, and result.
Recipient or group-member identities are not written to sync audit metadata.

## Failure and rollback

- Do not apply while Keycloak group synchronization is unhealthy.
- Export or record the current mappings before a large reconciliation.
- Correct invalid or disabled context nodes before retrying.
- Restore a mapping by applying its previous group/context/scope tuple.
- Never enable automated destructive cleanup merely to clear a drift warning.
- A failed transaction leaves the previous mappings unchanged.

## Troubleshooting

- No contexts in `/auth/me`: verify direct/group assignments and group member
  synchronization.
- Selection denied: verify node is enabled and assignment covers the selected
  node.
- Domain record cannot resolve: add the expected namespace/external-ID alias.
- Stale browser selection: the provider falls back to an authorized default;
  clear `moh.activeHealthContextId` only for troubleshooting.
