# Application manifests

Import a YAML or JSON application document from **Systems → Import manifest**.
The importing account needs administrator access and `rbac:read`, `rbac:write`
and `rbac:roles:write`. Uploads are limited to 128 KiB.

Use `external.yaml`, `internal.yaml` or `microfrontend.yaml` as a starting point.
Their example.org URLs, owners and role names are illustrative. Set the
environment, ownership, URLs and access roles for the target deployment before
previewing. The internal example requires the compiled metrics module.

| Field | Contract |
| --- | --- |
| `apiVersion` | `portal.moh.go.ug/v1alpha1` |
| `kind` | `Application` |
| `metadata.id` | Stable application client ID, 2–100 lowercase letters, digits or dashes, beginning with a letter |
| `metadata.name` | Application display name |
| `metadata.version` | Full semantic application version, optionally prefixed with `v` |
| `metadata.description` | Optional display description |
| `spec.environment` | Must match the backend deployment environment |
| `spec.owner` | Required team, name and email address |
| `spec.supportUrl` | Required support URL |
| `spec.icon`, `spec.category` | Optional portal display metadata |
| `spec.launch` | One of the launch configurations below |
| `spec.access.roles` | Any-of access gate using normalized lowercase client role names belonging to `metadata.id` |
| `spec.navigation` | Optional list of internal navigation entries, each with a unique ID, label and path |

Unknown fields, duplicate YAML/JSON keys, multiple documents and duplicate role
or navigation IDs are rejected. URLs require HTTPS. Loopback HTTP is accepted
only for development and test deployments. URL credentials, wildcard hosts,
queries and fragments are rejected. Documents carry configuration and role names.
Credentials remain in the existing server and identity configuration.

| Launch mode | Required fields | Portal behavior |
| --- | --- | --- |
| `external` | `url` | Opens the hosted application in a new tab |
| `internal` | `route`, `module` | Opens an existing portal route associated with an enabled compiled backend module |
| `microfrontend` | `route`, `appName`, `entryUrl` | Loads a separately hosted ES module in the portal shell |

Launch fields from different modes cannot be combined. External applications
use a launcher tile. The existing registry reserves portal side navigation for
internal applications and microfrontends.

Microfrontend routes use `/apps/extensions/<metadata.id>`. Their navigation
entries must stay under that route. A microfrontend name must be scoped, for
example `@moh-extensions/example-reports`. The `@moh-sso/` namespace belongs to
the applications shipped with the portal. Names cannot be shared by different
manifest applications.

An entry URL must serve a browser-compatible ES module exporting asynchronous
`bootstrap`, `mount` and `unmount` functions. Cross-origin hosting needs browser
module CORS support. Use an immutable, versioned entry URL for each release.
The shell supplies its existing basename, API, authentication, health context,
event bus and DOM container props. It mounts these applications through the
existing single-spa host component. Direct links and refreshes resolve through
the same authenticated application endpoint.

Navigation entries may specify `icon` and `requiredPermissions`. Every permission
must already be registered and active. These permissions continue to govern
navigation visibility alongside the application's access-role gate. Existing
backend resource permission checks continue to govern API access.

## Review and import

Preview shows the current and proposed registry configuration, access roles,
new local role definitions and activation requirements. Editing the document
clears the previous review. Import sends the document and its preview token.
A changed registry state produces HTTP 409 and requires a fresh preview.

Application metadata, navigation, access-role mappings and audit changes commit
in one serializable database transaction. An import failure rolls back the full
change. Identical repeated imports return `noop`, including a retry after a lost
successful response. Existing role permission assignments remain owned by RBAC
administration. Role names in a manifest do not assign roles to users.

New applications enter the existing `registered` lifecycle state. Complete the
configuration, validation and approval workflow before activation. Suspend an
active application before changing its manifest. Retired applications cannot be
updated. An unchanged import preserves the current lifecycle state.

## Identity and configuration ownership

The approved Postgres system registry drives portal navigation and launch access.
The imported document is recorded in the existing system metadata. A separate
manifest registry table is unnecessary. Startup seed and identity discovery
preserve manifest-managed application configuration and access definitions.

Keycloak remains responsible for authentication and user/client role assignments.
Use the existing client onboarding and identity synchronization administration
when a new OAuth client or Keycloak roles are needed. Manifest import is a portal
registration operation. It creates missing local access-role definitions and
preserves their existing permission mappings.

Explicit administrative edits and sync operations remain available. A subsequent
manifest preview shows registry drift. Re-import the intended manifest after
reviewing that drift. Runtime loading requires an active, enabled application
whose launch URL still matches its recorded manifest.

## API

All paths below are relative to `/api/v1`.

| Method and path | Request or result |
| --- | --- |
| `POST /admin/rbac/application-manifests/validate` | `{ "document": "YAML or JSON text" }` returns the normalized document |
| `POST /admin/rbac/application-manifests/preview` | Same request returns the proposal and `reviewToken` |
| `POST /admin/rbac/application-manifests/import` | Adds `reviewToken` and applies the reviewed proposal |
| `GET /applications/:clientId/manifest` | Returns launch metadata for an active application accessible to the current account |

Responses use the portal's standard success and error envelopes. Import records
`application.manifest.imported` in the existing RBAC audit log in the same
transaction as the registry change.
