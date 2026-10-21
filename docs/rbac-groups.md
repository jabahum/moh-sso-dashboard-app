# RBAC Groups

Keycloak groups are access containers for many users. The portal treats groups as a first-class RBAC source while keeping Keycloak as the identity source for group identity, group membership, realm roles, and client roles.

Effective access is resolved from:

- direct user realm roles
- direct user system/client roles
- direct user platform permissions, where supported
- group direct permissions
- group realm roles mapped to portal permissions
- group system/client roles mapped to portal permissions and app access

## Data Ownership

Keycloak owns:

- group IDs, names, paths, and nested group structure
- group membership
- group realm-role mappings
- group client-role mappings

The portal database owns:

- group metadata cache for RBAC explanations
- group direct permission mappings
- synced group membership cache
- audit history
- UI-friendly effective access explanations

Do not manually edit Keycloak-owned group membership directly in the portal database. If membership is changed through the portal UI, the portal must write the change to Keycloak first and then refresh the local RBAC membership cache.

## Admin Workflow

1. Create or update groups in Keycloak.
2. Assign realm roles and client roles to the group in Keycloak.
3. Run RBAC sync from the portal or allow startup sync to pull groups.
4. Open `Admin > RBAC > Groups`.
5. Review the synced groups.
6. Use the group members panel to add or remove users when portal-managed membership changes are needed. These changes call Keycloak first, then sync the portal cache.
7. Add direct portal permissions to the group only when permissions are not already represented by realm/client roles.
8. Inspect a user from `Admin > Users > Manage roles` to confirm inherited access.

The users access panel separates:

- direct editable roles
- inherited read-only group access
- final effective permissions

Inherited access should not be removed from the direct user-role controls. Change the Keycloak group, group membership, or group RBAC mappings instead.

## App Visibility

The app launcher, side navigation, and microfrontend route guards use the current user's final authorization payload:

- `permissions`
- `systems`
- `accessibleSystems`

Keycloak group realm/client roles are normally included in the token/userinfo role mappings and are therefore available to guards after login/refresh. Portal-only group permission mappings are visible in RBAC effective-access endpoints and should be reflected in auth/session payloads only when the auth layer is wired to the RBAC resolver.

Manual QA:

- A user with group-derived `data_quality:read` can see and open Data Validation.
- A user with group-derived `documents:read` can see and open Document Management.
- A user with group-derived `data-statistics` system access can see Data & Statistics navigation.
- Direct navigation without direct or group-derived access shows Access Denied, not a blank page.
- After Keycloak group changes, refresh login/session or run sync before testing visibility.

## Audit Events

Group RBAC mutations write audit events with safe metadata:

- `group.upserted`
- `group.member_added`
- `group.member_removed`
- `group.members_synced`
- `group.members_replaced`
- `group.permission_assigned`
- `group.permission_removed`
- `group.realm_role_assigned`
- `group.realm_role_removed`
- `group.system_role_assigned`
- `group.system_role_removed`
- `rbac.groups_synced`

Audit details include group ID/path/name and counts where useful. Full Keycloak payloads, tokens, and secrets are not logged.

## Startup Sync Notes

Group sync can come from two sources:

- realm export sync reads `groups`, nested `subGroups`, group `realmRoles`, group `clientRoles`, and user `groups` memberships from `infra/keycloak/realm-export.json`
- live Keycloak sync reads groups, members, realm-role mappings, and client-role mappings through the Keycloak Admin API

Realm export sync is useful for local bootstrap and repeatable seed environments. Live Keycloak sync is the production source for an already deployed realm.

Required realm export shape:

```json
{
  "groups": [
    {
      "name": "MOH",
      "subGroups": [
        {
          "name": "Document Viewers",
          "realmRoles": ["user"],
          "clientRoles": {
            "data-statistics": ["document_viewer"]
          }
        }
      ]
    }
  ],
  "users": [
    {
      "username": "document.viewer",
      "email": "document.viewer@example.org",
      "groups": ["/MOH/Document Viewers"]
    }
  ]
}
```

Startup sync applies group records after system/client roles have been discovered so group role references can resolve against the system registry.

Recommended production posture:

- keep Keycloak as source of truth for membership
- use the portal group members panel only for Keycloak-backed membership changes, not local DB overrides
- keep push-to-Keycloak disabled unless intentionally provisioning systems/roles
- do not delete Keycloak groups automatically
- treat sync warnings as operational signals unless fail-on-error is explicitly enabled

## Troubleshooting

If a user does not see an app expected from group access:

1. Confirm the user is a member of the Keycloak group.
2. Confirm the group has the expected realm/client role in Keycloak.
3. Confirm the client/system exists in the portal system registry.
4. Run RBAC sync and check warnings.
5. Inspect `Admin > Users > Manage roles` and review the Groups section.
6. Confirm the permission appears in effective permissions.
7. Refresh the user's session or log out/in.

If inherited access appears as removable direct access, verify the users access panel is using `directAccess` for editable controls and `effectiveAccess` only for read-only explanation.

## Announcement And Email Targeting

Groups can also be used as message audiences after they have been synced into portal RBAC.

Announcements support a `SPECIFIC_GROUPS` audience type. Admins select one or more synced RBAC groups, and the backend resolves the current group members when the announcement is published or scheduled. The resolver:

- reads members from the synced RBAC group membership cache
- looks up users through the users repository where possible
- skips disabled users and users without email addresses
- deduplicates recipients by email address
- sends or queues email only when the announcement email notification flag is enabled

Direct admin email also supports group recipients. Admins can select groups instead of manually entering every address. The backend expands selected group IDs or group paths into email recipients before sending or queueing the message.

Operational notes:

- Keycloak remains the source of truth for group membership.
- Run RBAC sync after group membership changes before sending group-targeted announcements or email. Portal group member add/remove actions perform a group-level sync automatically after Keycloak accepts the change.
- Group targeting uses a point-in-time snapshot at publish/send time; later group changes do not rewrite already queued delivery records.
- If a group has no enabled users with email addresses, the email request is rejected with a validation error rather than sending an empty message.
