# Application identity and authorization contract (v1)

This page defines how external applications, hosted modules and machine identities
authenticate to the portal's platform APIs, for application developers and the
operators who configure Keycloak and the platform server. For the step-by-step
procedure, see the [onboarding handbook](platform-onboarding.md).

## Responsibilities

The portal remains the identity/governance entry point. External backends enforce their own business authorization. Hosted modules use the shell's existing `@moh-sso/auth` context and route guards; their backend endpoints also require permissions. No browser receives a service-account secret or the portal's administrative credentials.

## External applications

### Interactive client

Register one Keycloak interactive client per external application, with its client ID matching the application registry ID. Use authorization code flow with PKCE S256, exact redirect URI allowlists, state and nonce checks, and issuer/audience/signature/time validation in the application's OIDC library. Keep tokens in a backend session using Secure/HttpOnly/SameSite cookies where a backend is available. Use the realm's discovery document; do not copy another application's session cookie or access token storage.

### Access decisions

External backends can obtain a current application-specific access decision with:

`GET /api/v1/platform/applications/:clientId/access`

Supply the user's access token as a Bearer header. The token must:

- be active at Keycloak;
- have the configured issuer and platform API audience;
- contain `identity:read` in its scope;
- have been issued to this application's interactive client.

Configure the application's Keycloak scope mappers to include a nonempty `sub` plus its resource roles/realm roles in introspection. Retain the default `basic` scope on current Keycloak realms, or add an explicit subject mapper when integrating an older/custom realm that lacks it. A valid issuer/audience alone is insufficient without the user subject.

The response contains the subject, access roles and permissions for this application only. Unknown, inactive, unassigned or machine identities are rejected. A realm-wide administrative permission does not automatically bypass application access roles here.

Make this decision for each protected operation when immediate portal suspension enforcement is needed. Fail closed when the decision service is unavailable. This endpoint is an authorization dependency, not an authentication callback or a token exchange service. Do not expose it as an unauthenticated proxy in your application.

### Logout and role changes

Log out through the OIDC library and end the application's local session. Portal logout alone cannot delete independently owned external sessions. Use back-channel logout where supported, or require fresh access decisions for operations. Changes to Keycloak role claims can remain visible in already issued tokens until token expiry/revocation; portal-side role mappings, identity grants and lifecycle status are read on every request. Do not promise that editing an identity-provider role rewrites an existing token.

## Machine identities

### Create the machine client

Create a separate confidential Keycloak client with service accounts enabled and browser/direct-access flows disabled. Its service-account subject must match the UUID assigned by Keycloak, and its username must be `service-account-<clientId>`. Do not grant it portal/admin roles. Obtain access tokens directly from Keycloak with client credentials, exclusively from a backend.

### Bind the client to an application

An operator with both `rbac:write` and `rbac:roles:write` binds that client to a registered application:

`PUT /api/v1/admin/platform/applications/:clientId/identities/:oauthClientId`

```json
{
  "subject": "00000000-0000-4000-8000-000000000001",
  "scopes": ["identity:read", "notifications:write", "notifications:read"],
  "enabled": true,
  "reason": "Approve the application's dedicated integration service account."
}
```

Use the actual Keycloak service-account subject, not the example UUID. A client binding cannot be reassigned to another application. Update the same endpoint with `enabled:false` to revoke it; empty scopes also remove all usable capabilities. Every mutation and its reason are audited in the same transaction. The registry never stores a client secret.

### Check identity and effective scopes

`GET /api/v1/platform/identity` returns the caller's application, service client, subject and effective scopes. Effective scopes are the intersection of the token's scopes and the registry grant. Applications must be enabled and `active`; suspended/retired applications and revoked bindings fail closed on the next request. There is no local positive authorization cache. In-flight operations can finish before a concurrent revocation.

### Authentication rules for platform endpoints

All platform service endpoints accept only Bearer access tokens. Browser cookies, development auth bypass, refresh tokens and arbitrary JWT claims are not authentication mechanisms for these endpoints. Token introspection validates active state, token type, issuer, audience, subject, client identity, expiry and not-before. Timeouts, oversized/malformed responses and redirects fail closed. Machine identities never gain portal permissions from these endpoints.

## Identity reconciliation (CLIMS, POES)

The backend CLI links accounts that already exist in an external application to their Keycloak users. `clims-identity-sync` and `poes-identity-sync` preview matches page by page and apply one reviewed link at a time; they never create Keycloak users.

Each application has a dedicated machine client in the realm exports: `clims-identity-sync` and `poesgo-identity-sync`. These are confidential clients with only service accounts enabled and `fullScopeAllowed: false`. Each service account holds only its application's `identity_sync` client role, and an audience mapper adds the application's client ID (`clims` or `poesgo`) to the token's `aud`. The application's `/api/v1/internal/identity-sync` endpoints must require that audience and role. The role has no portal permissions and must not be assigned to people.

Configure the CLI with:

- `KEYCLOAK_BASE_URL` (HTTPS), `KEYCLOAK_REALM`, `KEYCLOAK_ADMIN_CLIENT_ID`, `KEYCLOAK_ADMIN_CLIENT_SECRET`: operator credentials used to look up users and grant `<clientId>_access`. `KEYCLOAK_BASE_URL` can be an internal address; admin API and token requests go there.
- `KEYCLOAK_EXTERNAL_URL` (HTTPS): the public Keycloak URL. The issuer written into each identity link is `<KEYCLOAK_EXTERNAL_URL>/realms/<realm>`, which must equal the `iss` claim in users' ID tokens at the application. When it is unset the CLI falls back to `KEYCLOAK_BASE_URL`, which is only correct when that is the public URL. The CLI prints the issuer it uses on stderr; check it before applying links.
- `CLIMS_SYNC_ORIGIN` / `POES_SYNC_ORIGIN`: the application's HTTPS origin, for example `https://clims.health.go.ug`.
- `CLIMS_SYNC_CLIENT_ID` + `CLIMS_SYNC_CLIENT_SECRET` / `POES_SYNC_CLIENT_ID` + `POES_SYNC_CLIENT_SECRET`: the identity-sync client's credentials. The CLI requests tokens from `<KEYCLOAK_BASE_URL>/realms/<realm>/protocol/openid-connect/token` with the client-credentials grant and caches each one until 30 seconds before it expires. Run Keycloak with a fixed public hostname (`KC_HOSTNAME`) so these tokens carry the public issuer even when requested through the internal URL.
- `CLIMS_SYNC_ACCESS_TOKEN` / `POES_SYNC_ACCESS_TOKEN` (optional fallback): a pre-minted token. It is used only when no sync client ID or secret is set.

```sh
moh-sso clims-identity-sync --limit 25                 # preview
moh-sso clims-identity-sync --after <cursor>           # next page
moh-sso clims-identity-sync --apply-account <id> --approved-subject <keycloak-user-id>
```

Apply re-runs the preview and refuses if the current match no longer equals the approved subject. Matching uses enabled Keycloak users with a verified email only. Rotate the sync client secret in Keycloak, outside this repository.

## Platform server setup

Configure these additional environment variables:

- `PLATFORM_AUTH_CLIENT_ID`: a dedicated confidential introspection client, normally `moh-platform-api`.
- `PLATFORM_AUTH_CLIENT_SECRET`: its secret, supplied through the deployment's secret mechanism.
- `PLATFORM_AUTH_AUDIENCE`: required audience, defaults to `moh-platform-api`.

The issuer comes from `KEYCLOAK_EXTERNAL_URL` and `KEYCLOAK_REALM`. Introspection uses `KEYCLOAK_BASE_URL`/`KEYCLOAK_INTERNAL_URL` and the realm's `/protocol/openid-connect/token/introspect` endpoint. Use HTTPS or a protected internal transport consistent with the existing deployment. Never redirect the introspection endpoint. Configure the platform audience mapper and include the introspecting client's audience as required by your deployed Keycloak version. Configure the capability client scopes with **Include in token scope** enabled. Configure role mappers for external interactive access decisions.

Missing introspection credentials disable the new platform endpoints with 503; they do not prevent existing portal routes from starting. Migration 49 adds bindings without changing existing sessions or role assignments. Roll back new service consumers before dropping bindings.

Keycloak endpoint reference: https://www.keycloak.org/securing-apps/oidc-layers
