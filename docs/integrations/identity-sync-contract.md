# Identity-sync application contract (v1.1)

An application implements this contract so the MoH SSO Dashboard can link the
application's existing local accounts to Keycloak identities, and later end
those accounts' sessions when portal access is removed. Any language works. The
machine-readable shape is
[`backend/core/identitysync/contract/identity-sync.openapi.yaml`](../../backend/core/identitysync/contract/identity-sync.openapi.yaml).
That file and this document together are the contract. Where they differ, this
document wins and the difference is a bug.

The key words MUST, MUST NOT, SHOULD and MAY are used as in RFC 2119.

## Versions

| Version | Adds | Implemented by |
|---|---|---|
| 1.0 | `GET /accounts` (fixed page size, `after` cursor) and `PUT /accounts/{id}/identity` | CLIMS, POES |
| 1.1 | `GET /` descriptor, `limit`, `link` on each account, `POST /sessions/revoke` | none yet |

Minor versions are backwards compatible. A client written for 1.0 ignores new
fields and endpoints. The dashboard reads `GET /` first. A 404 there means the
application implements 1.0, and the dashboard then disables batch resume checks
and session revocation for it. A breaking change would be published as `/v2`
under a new base path.

## Roles of each party

- **The application** owns its accounts: who is eligible, local roles,
  persistence and its own sessions. It never creates accounts through this
  contract.
- **The dashboard** owns matching and review. It finds the Keycloak user for an
  account, records an operator's approval, asks the application to store the
  link, then grants the application's access role in Keycloak.
- **Keycloak** owns identities and the application's access role.

## Base URL and transport

The base URL is `https://<registered origin>/api/v1/internal/identity-sync`.

- The origin MUST use HTTPS and MUST be scheme and host only (an optional port
  is allowed).
- The application MUST NOT redirect these endpoints. The dashboard treats a
  redirect as a failure.
- Every response MUST set `Cache-Control: no-store`.
- Request bodies are JSON and at most 4 KiB. Response bodies MUST be at most
  256 KiB.
- This path SHOULD be reachable only from the dashboard's network, in addition
  to the token checks below.

## Authorization

Every request carries `Authorization: Bearer <token>`. The token is a Keycloak
access token that the dashboard obtains with the client-credentials grant for
the application's dedicated identity-sync client (for example
`clims-identity-sync`). The application MUST reject the request with **401**
unless all of these hold:

1. The signature verifies against the realm's published keys (RS256).
2. `iss` equals the application's configured public issuer exactly.
3. `exp` and `nbf` are valid.
4. `aud` contains the application's own Keycloak client ID.

It MUST then reject with **403** unless all of these hold:

5. `typ` is `Bearer`.
6. `azp` equals the configured identity-sync client ID. A token issued to
   another client, including the application's own interactive client or a user
   token, is refused.
7. `resource_access[<application client ID>].roles` contains `identity_sync`.

The application MUST NOT accept cookies, API keys or a development bypass on
these endpoints.

## `GET /` — contract descriptor (1.1)

Returns what the application implements:

```json
{
  "contract": "moh-identity-sync",
  "version": "1.1",
  "application": "clims",
  "accountIdFormat": "uuid",
  "maxPageSize": 100,
  "capabilities": ["accounts.list", "identity.link", "sessions.revoke"]
}
```

`application` MUST equal the application's Keycloak client ID. The dashboard
refuses to sync an application whose descriptor names a different one.
`accountIdFormat` is `uuid`, `positive-integer` or `opaque`.

## `GET /accounts` — eligible accounts

Query parameters:

- `after` (optional): the `nextCursor` from the previous page, passed back
  unchanged.
- `limit` (1.1, optional): 1–100, default 100.

Response:

```json
{
  "accounts": [
    {"id": "8b0c…", "email": "nurse@example.org", "link": null},
    {"id": "9a1d…", "email": "lab@example.org",
     "link": {"issuer": "https://sso.health.go.ug/realms/moh", "subject": "5f2e…"}}
  ],
  "nextCursor": "9a1d…"
}
```

Rules:

- **Eligibility.** The application lists only accounts that may sign in today:
  active, not locked, and holding at least one local role. Service and system
  accounts MUST be excluded. Already-linked eligible accounts MUST still be
  listed. From 1.1, `link` shows the current link for the configured issuer, or
  `null`.
- **Order and cursor.** Accounts are in a stable ascending order by account ID.
  `nextCursor` is opaque to the client. The client only passes it back. It is
  `""` on the last page. A page MAY be shorter than `limit` without being the
  last page. Only an empty `nextCursor` ends the listing.
- **Consistency.** Accounts created or changed during a listing MAY be missed or
  seen twice. The dashboard tolerates both, because linking is idempotent and
  every apply re-reads the account.
- **IDs.** Each account ID MUST match `^[A-Za-z0-9][A-Za-z0-9._:-]{0,127}$`,
  stay the same for the account's lifetime, and never be reused.
- **Email.** Email is only a matching hint and MAY be empty. The application
  MUST NOT treat it as an identity key.
- **Errors.** An invalid `after` or `limit` returns **400**.

## `PUT /accounts/{id}/identity` — link an identity

Body: `{"issuer": "<public issuer>", "subject": "<Keycloak user ID>"}`.

| Outcome | Status |
|---|---|
| Linked now, or already linked to exactly this identity | **204** |
| `issuer` is not the configured issuer, `subject` is empty or over 255 characters, or the body is malformed | **400** |
| The account does not exist or is not eligible | **404** |
| The account already has a different subject for this issuer, or this subject is linked to another account | **409** |

The application MUST check eligibility and both uniqueness rules in one
transaction. It MUST NOT replace or move an existing link. A conflict changes
nothing. Changing a link is a manual, audited operation inside the application.
Linking MUST NOT grant local roles: the account keeps the permissions it already
had.

## `POST /sessions/revoke` — end local sessions (1.1)

Body: `{"issuer": "<public issuer>", "subject": "<Keycloak user ID>"}`.

The application ends every local session of the account linked to this
identity and invalidates its local refresh tokens. After a **204**, no request
using an earlier session or refresh token may succeed. The call is idempotent,
and it also returns **204** when the identity is not linked or has no sessions.
It MUST NOT remove the link or local roles. An `issuer` other than the
configured one returns **400**.

## Browser sign-in requirements

These are not endpoints, but the dashboard relies on them so that removing
access in Keycloak takes effect:

- The application's OIDC callback MUST use the authorization-code flow with
  PKCE S256, and MUST check `state`, `nonce`, issuer, audience and expiry.
- It MUST sign the user in only when the ID token's application role claim (for
  example `clims_roles`) contains the access role (for example `clims_access`),
  **and** the verified `iss` and `sub` are linked to an eligible local account.
  Otherwise it refuses the sign-in without creating a session.
- It MUST NOT link accounts or create them during sign-in.
- Local sessions SHOULD be no longer than the agreed revocation window, or the
  application MUST implement `POST /sessions/revoke`.

## Errors and retries

The status codes above are normative. A JSON body
`{"error": {"code": "...", "message": "..."}}` is optional. Its `code` is one of
`invalid_request`, `account_ineligible`, `link_conflict`, `rate_limited` or
`unavailable`. Messages MUST NOT contain tokens, secrets or other users' data.

All three operations are idempotent, so the dashboard retries them safely:

- **Retried:** network errors, timeouts, **429**, **502**, **503** and **504**.
  The dashboard uses exponential backoff with jitter and honours `Retry-After`,
  up to 5 attempts.
- **Not retried:** any other 4xx. These are reported for review.
- **Overload:** applications SHOULD return 429 or 503 with `Retry-After` rather
  than fail slowly. The dashboard's per-request timeout is 15 seconds.

## Conformance

`identity-sync conformance --app <id>` (see the
[design](../architecture/identity-sync.md#conformance-suite)) runs the
contract's checks against a registered application in a test environment:
authorization refusals, pagination, idempotent linking, both conflict cases,
session revocation, and the response headers and limits. Run it with synthetic
accounts. Some checks create links.
