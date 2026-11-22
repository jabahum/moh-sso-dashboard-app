# Identity sync: dashboard design

This is the shared design for the identity-sync work. It covers the app registry,
batch review, access removal, the published Go module, conformance and
end-to-end testing. The application side is specified in the
[identity-sync contract](../integrations/identity-sync-contract.md). Each section
names the track that builds it and the files that track owns, so tracks can work
in parallel without editing the same files.

## Current state (baseline)

- **CLI.** `moh-sso clims-identity-sync` and `poes-identity-sync` each hard-code
  one application (`backend/cmd/cli/identity_sync.go`). They preview a page and
  apply one reviewed link at a time.
- **Connector.** `backend/internal/identityreconcile` holds the HTTP connector
  for contract 1.0 and the review service: `Preview` and `Apply` for the `Reuse`
  action. `Create` stops because `EnsureUser` is not implemented.
- **Service accounts.** Each application has a dedicated sync client
  (`clims-identity-sync`, `poesgo-identity-sync`) holding the service-only
  `identity_sync` role (migration 61).
- **Applications.** CLIMS and POES implement contract 1.0 and an OIDC callback
  with a session handoff.

## Track A — app registry and generic CLI

### Registry table (migration 62)

```sql
CREATE TABLE ihp_identity_sync_integrations (
    application_client_id TEXT PRIMARY KEY REFERENCES ihp_systems(client_id),
    origin                TEXT NOT NULL CHECK (origin ~ '^https://[A-Za-z0-9.-]+(:[0-9]{1,5})?$'),
    sync_client_id        TEXT NOT NULL UNIQUE REFERENCES ihp_application_identities(oauth_client_id),
    access_role           TEXT NOT NULL,
    credential_ref        TEXT NOT NULL CHECK (credential_ref ~ '^(env:[A-Z][A-Z0-9_]*|file:/[^\s]+)$'),
    account_id_format     TEXT NOT NULL CHECK (account_id_format IN ('uuid','positive-integer','opaque')),
                                         -- used for 1.0 apps; a 1.1 descriptor's format wins
    contract_version      TEXT CHECK (contract_version ~ '^1\.[0-9]+$'),
                                         -- last version read from GET /; NULL until checked
    enabled               BOOLEAN NOT NULL DEFAULT FALSE,
    updated_by            TEXT,
    created_at            TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at            TIMESTAMPTZ NOT NULL DEFAULT now()
);
```

Enforced by trigger and in the service:

- `sync_client_id` must be an enabled `ihp_application_identities` binding for
  the same `application_client_id`. This reuses the existing, audited
  service-identity registration (`PUT /api/v1/admin/platform/applications/:clientId/identities/:oauthClientId`).
- `access_role` must be one of the system's `ihp_system_access_roles` and must
  not be service-only.

The trigger (`ihp_identity_sync_integration_guard`) raises `check_violation`
with constraint `ihp_identity_sync_sync_client` or
`ihp_identity_sync_access_role`. Disabling the service identity or removing the
access role later is not blocked (an operator must be able to disable a
compromised sync client at once); instead the registry service re-checks both
bindings every time it builds a connector, and refuses to sync until they hold
again.

**Secrets.** The registry stores a reference, never a secret. `env:NAME` reads
an environment variable, and `file:/path` reads a mounted secret file (one
trailing newline is trimmed; an empty secret is refused; the secret is never
logged or returned). The reference resolves in the process that uses it: the
dashboard backend for the admin API's enable check, the CLI host for
`identity-sync` commands. Existing deployments register CLIMS as
`env:CLIMS_SYNC_CLIENT_SECRET`, so nothing has to move.

**Admin API.** `PUT/GET/DELETE /api/v1/admin/platform/applications/:clientId/identity-sync`
requires both `rbac:write` and `rbac:roles:write`, records each change and its
reason in the audit log (`application.identity_sync.updated|deleted`) in the
same transaction, and refuses to enable an integration until `GET /` succeeds
or returns 404 (a 1.0 application); the version read is stored in
`contract_version`. PUT and DELETE take a JSON body with a required `reason`.
Errors: 400 invalid input or binding, 404 unknown application or entry, 409
still referenced, 422 the application could not be described.

**Connector.** `identityreconcile.HTTPAccounts` implements both versions. It
reads `GET /api/v1/internal/identity-sync` first and caches the descriptor
(404 means 1.0 with the registry's `account_id_format`); it sends `limit` only
to 1.1 applications, decodes `link`, validates IDs with the descriptor's format
and cursors with the contract pattern (1.1) or the ID format (1.0), and offers
`RevokeSessions`, which returns `identitysync.ErrUnsupported` unless the
descriptor lists `sessions.revoke`. Retries follow the contract: network errors,
timeouts, 429, 502, 503 and 504; up to 5 attempts; exponential backoff from
500 ms (capped at 10 s) with jitter; `Retry-After` honoured up to 60 s (a longer
one ends the call). Redirects are refused, responses are capped at 256 KiB and
each request times out after 15 s.

### CLI

```text
moh-sso identity-sync register   --app <id> --origin <https> --sync-client <id> --access-role <role> --credential env:NAME
                                 --reason <text> [--account-id-format uuid|positive-integer|opaque] [--enable]
moh-sso identity-sync describe   --app <id>                     # registry entry + live GET / descriptor
moh-sso identity-sync preview    --app <id> [--after c] [--limit n]
moh-sso identity-sync apply      --app <id> --account <id> --approved-subject <sub> [--after c]
moh-sso identity-sync conformance --app <id>                    # Track D
```

`clims-identity-sync` and `poes-identity-sync` remain as deprecated aliases for
one release (`identity-sync … --app clims` / `--app poesgo`). They print a
deprecation note to stderr. When the application has an enabled registry entry
they use it; otherwise they keep the `<APP>_SYNC_*` environment configuration
exactly as before. The connector validates account IDs using the format in the
descriptor. A 1.0 application gets the existing per-app validation, which comes
from a registry field and not from code.

Registry commands read the database from the usual `app.env`/environment
configuration and Keycloak settings from `KEYCLOAK_BASE_URL`, `KEYCLOAK_REALM`,
`KEYCLOAK_EXTERNAL_URL` (public issuer) and the operator credentials. `apply`
walks the eligible accounts to find the reviewed account, then re-checks that
the directory still matches exactly the approved subject before linking.

**Extension points for other tracks.** `newIdentitySyncCommand()` returns the
parent `*cobra.Command`; a track adds its subcommand from its own file with
`func init() { registerIdentitySyncSubcommand(newIdentitySyncRunCommand) }`.
Subcommands open the registry with `openIdentitySyncRuntime(ctx)` and use
`rt.Service(ctx, app)` (review service, connector and registry row) or
`rt.Connector(ctx, app)`. In the server, `identityreconcile.RegistryService`
(`NewRegistryService(&PostgresRegistry{DB: db}, tokenEndpoint)`) offers `Get`,
`Save`, `Delete`, `Probe`, `Connector(ctx, app)` and `ConnectorFor(row)`.

**Done when:** a new application such as Alerts Manager can be registered and
previewed with no dashboard code change.

**Owned files:** `backend/cmd/cli/identity_sync*.go` (except
`identity_sync_run.go`, `identity_sync_revoke.go` and
`identity_sync_conformance.go`), `backend/internal/identityreconcile/registry*.go`,
`backend/internal/identityreconcile/http_accounts*.go`, migration 62, and the
registry admin handler in `backend/internal/features/platform/identity_sync*.go`.

## Track B — reviewed batch sync and invitations

### Tables (migration 63)

```sql
CREATE TABLE ihp_identity_sync_runs (
    id                    UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    application_client_id TEXT NOT NULL REFERENCES ihp_identity_sync_integrations(application_client_id),
    issuer                TEXT NOT NULL,
    status                TEXT NOT NULL CHECK (status IN ('previewed','applying','completed','failed','cancelled')),
    created_by            TEXT NOT NULL,
    created_at            TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at            TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE ihp_identity_sync_rows (
    id               UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    run_id           UUID NOT NULL REFERENCES ihp_identity_sync_runs(id) ON DELETE CASCADE,
    account_id       TEXT NOT NULL,
    email_hint       TEXT NOT NULL DEFAULT '',
    proposal         TEXT NOT NULL CHECK (proposal IN ('reuse','create','review','linked','linked_no_access','revoked')),
    proposed_subject TEXT,
    decision         TEXT NOT NULL DEFAULT 'pending' CHECK (decision IN ('pending','approved','rejected')),
    reviewed_by      TEXT,
    reviewed_at      TIMESTAMPTZ,
    review_note      TEXT,
    user_status      TEXT NOT NULL DEFAULT 'not_required' CHECK (user_status IN ('not_required','pending','invited','failed')),
    link_status      TEXT NOT NULL DEFAULT 'pending' CHECK (link_status IN ('pending','linked','conflict','ineligible','failed')),
    grant_status     TEXT NOT NULL DEFAULT 'pending' CHECK (grant_status IN ('pending','granted','failed','not_required')),
    attempts         INT  NOT NULL DEFAULT 0,
    last_error       TEXT,           -- stable code only, never upstream bodies
    updated_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (run_id, account_id)
);
```

**Proposals.**

| Proposal | Situation |
|---|---|
| `reuse` | One verified, enabled Keycloak user matches the email |
| `create` | No Keycloak user matches |
| `review` | Ambiguous, unverified or no email |
| `linked` | The descriptor's `link` is already set and that Keycloak user holds the access role. Nothing to do. |
| `linked_no_access` | The account is linked but its Keycloak user lacks the access role. Shown, never auto-granted (see Track C). |
| `revoked` | The subject is in the revocation table. It can only be restored explicitly. |

**Apply state machine.** Each approved row moves through these steps:

1. `create` rows only: `EnsureUser`, then `user_status=invited`. This creates
   the Keycloak user with `VERIFY_EMAIL` and `UPDATE_PASSWORD` required actions,
   sends the execute-actions email, and stores the attribute
   `ihp_identity_sync_origin=<app>:<accountId>` so a retry reuses the same user.
2. Re-run the directory lookup. The match must equal the reviewed subject,
   otherwise the row goes back to `review`.
3. `PUT` the link: `linked`, or `conflict` / `ineligible` / `failed`.
4. Grant the access role: `granted` or `failed`.

Every step is idempotent. **Resume** re-processes rows that aren't finished:
when `link_status=linked` and `grant_status=failed`, only the grant is retried
and the link is never recreated. Retries follow the contract's backoff rules.
The run records who approved each row and its final outcome. Approval and apply
are separate actions, and apply refuses rows with no reviewer.

**Interfaces.** The same review flow is available from the CLI
(`identity-sync run start|show|approve|reject|apply|resume`) and, later, from an
admin page. Both call one service in `internal/identityreconcile/batch*.go`.

**Owned files:** `backend/internal/identityreconcile/batch*.go`,
`backend/internal/identityreconcile/invite*.go`, migration 63, and the
`identity-sync run` CLI subcommands.

## Track C — access removal

**Window: immediately.** When a person's access to one application is removed,
their existing sessions in that application end at once: every step below runs
synchronously in the request (portal) or command (CLI) that removes access.

Removing a person's access to one application (`identity-sync revoke`, or
removing the application's registered access role, normally `<app>_access`,
through portal RBAC or the users API) runs these idempotent steps:

1. Record `(application, issuer, subject, revoked_by, reason)` in
   `ihp_identity_sync_revocations` (migration 64). Proposals then show
   `revoked`, and no later sync grants access again until an explicit restore.
2. Remove the application's access role (the direct client-role mapping) in
   Keycloak. The application's OIDC callback then refuses new sign-ins, as the
   contract requires. If the person still holds the role through a group or a
   composite role, the step is recorded as `failed` with `access_role_inherited`.
   A Keycloak user that no longer exists counts as removed.
3. Revoke the user's consents and offline tokens for the application's
   interactive client only (`DELETE /admin/realms/{realm}/users/{id}/consents/{clientId}`;
   a 404 counts as done). Nothing calls a user-wide logout or deletes SSO
   sessions, so the user's other applications and Keycloak SSO session are
   untouched.
4. Call the application's `POST /sessions/revoke` through Track A's connector,
   which retries transient failures under the contract policy (up to 5 attempts).
   For a 1.0 application the step is `unsupported`: existing sessions last until
   their local expiry. The CLI and the portal hook report that residual window
   and never call it success.
5. Record the outcome in the audit log (`ihp_rbac_audit_events`, resource type
   `identity_sync_revocation`): `application.identity_sync.revocation_recorded`,
   `…revocation_attempted` (step statuses, attempts, `last_error`) and
   `…revocation_restored`.

**An application outage never blocks removal.** Each step runs even when an
earlier one failed, and its status is stored (`pending`, `done`, `failed`,
`unsupported`) with `attempts` and a stable `last_error` code
(`role_removal_failed`, `access_role_inherited`, `consent_revocation_failed`,
`integration_disabled`, `connector_unavailable`, `sessions_app_unavailable`,
`sessions_app_rejected`). `identity-sync revocations --resume` retries `pending`
and `failed` steps of active revocations and never repeats a finished one; it
does not retry `unsupported`. Repeating `revoke` for an active revocation reuses
the row and re-asserts every step (for example, removes a role granted again by
hand).

### Table (migration 64)

```sql
CREATE TABLE ihp_identity_sync_revocations (
    id                    UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    application_client_id TEXT NOT NULL REFERENCES ihp_identity_sync_integrations(application_client_id),
    issuer                TEXT NOT NULL,
    subject               TEXT NOT NULL,
    revoked_by            TEXT NOT NULL,
    reason                TEXT NOT NULL,
    revoked_at            TIMESTAMPTZ NOT NULL DEFAULT now(),
    restored_by           TEXT,
    restore_reason        TEXT,
    restored_at           TIMESTAMPTZ,
    role_removed          TEXT NOT NULL DEFAULT 'pending',  -- pending|done|failed|unsupported
    consents_revoked      TEXT NOT NULL DEFAULT 'pending',
    sessions_revoked      TEXT NOT NULL DEFAULT 'pending',
    attempts              INT  NOT NULL DEFAULT 0,
    last_error            TEXT,                             -- stable code only
    updated_at            TIMESTAMPTZ NOT NULL DEFAULT now()
);
-- one active revocation per (application, issuer, subject)
CREATE UNIQUE INDEX ihp_identity_sync_revocations_active
    ON ihp_identity_sync_revocations (application_client_id, issuer, subject) WHERE restored_at IS NULL;
```

Restore fills `restored_by`, `restore_reason` and `restored_at`; it never grants
the access role. Rows are kept as history, so an integration with revocations
cannot be deleted (the registry reports it as still referenced).

### Interfaces

- `identityreconcile.RevokeService` (`revoke.go`): `Revoke(ctx, app, subject,
  revokedBy, reason)`, `Resume(ctx, app, actor)`, `Restore(ctx, app, subject,
  restoredBy, reason)`, `List(ctx, filter)` and `IsRevoked(ctx, app, issuer,
  subject) (bool, error)`, which matches Track B's `RevocationChecker`. Built with
  `NewRevokeService(registry, &PostgresRevocations{DB: db},
  KeycloakAccessDirectory{Admin: adminClient}, issuer)`.
- Portal hook: `users.Service.SetAccessRoleRemovalHook` (nil-safe). The users
  service calls it after `UpdateUserClientRoles` removes roles (the path portal
  RBAC `UpdateUserAccess` uses for every system) and after
  `RemoveUserClientRoles`. `identityreconcile.PortalRevocationHook` revokes only
  when the client has an enabled integration, the removed roles include its
  access role, and the person no longer holds it through a group. The role
  removal always stands; an incomplete revocation is logged and visible in the
  revocation status and audit log. Wired in `internal/bootstrap`
  (`identity_sync_revocation.go`, both profiles).

### CLI

```text
moh-sso identity-sync revoke      --app <id> --subject <keycloak user id> --reason <text> --revoker <who>
moh-sso identity-sync restore     --app <id> --subject <keycloak user id> --reason <text> --restorer <who>
moh-sso identity-sync revocations [--app <id>] [--all] [--unfinished] [--limit n]
moh-sso identity-sync revocations --resume [--app <id>] [--actor <who>]
```

`revoke` and `revocations --resume` exit non-zero when a step is still pending
or failed, and also when the application could not end sessions (1.0), so a
script never mistakes a partial removal for success.

**Owned files:** `backend/internal/identityreconcile/revoke*.go`, migration 64,
`backend/cmd/cli/identity_sync_revoke.go`, the access-removal helpers in
`backend/internal/keycloak/user_access_revocation.go`, the hook setter in
`internal/features/users`, and `internal/bootstrap/identity_sync_revocation.go`.

## Track D — published Go module and conformance suite

**Contract types and HTTP client.** These move into the standard-library-only
core module at `backend/core/identitysync`:

- `contract/` — the OpenAPI file, embedded with `go:embed`.
- `client/` — the 1.1 HTTP client (moved from `internal/identityreconcile/http_accounts.go`).
- `adapter/` — a `net/http` handler implementing the contract on top of
  `AccountStore` plus a `TokenVerifier` interface. The application supplies the
  OIDC verification, so core stays dependency-free.
- `conformance/` — a runner that takes a base URL and token sources and returns
  structured results. The CLI and CI use it.

The module is published by tagging `backend/core/v0.1.0` (pre-1.0 while the
contract settles). The release workflow runs `infra/scripts/test-platform-core.py`
first.

### Conformance suite

The suite runs against a disposable environment with synthetic accounts.

| Area | Checks |
|---|---|
| Authorization | No token → 401. Expired token → 401. Wrong `aud` → 401. User token → 403. Another client's token → 403. Missing `identity_sync` role → 403. |
| Descriptor | Shape, `application` matches, capabilities match the endpoints actually present |
| Pagination | Full walk with `limit` 1, 7 and 100 covers every eligible account exactly once. Rising order. Empty cursor ends the walk. Bad cursor → 400. |
| Linking | Link → 204. Repeat → 204. Other subject → 409. Same subject on another account → 409. Ineligible account → 404. Wrong issuer → 400. |
| Sessions | Revoke → 204. Revoke again → 204. Unknown subject → 204. A session created before the revoke is rejected afterwards (via an app-provided probe hook in the test environment). |
| Transport | `Cache-Control: no-store` on every response. No redirects. Body size limits. |
| Retries | Covered by connector unit tests with a fault-injecting fake server |

**Owned files:** `backend/core/identitysync/**` (except `accounts.go`, which
Phase 0 owns) and `docs/integrations/identity-sync-adapter.md`.

## Track E — application changes (CLIMS, POES)

Each application, in its own repository:

1. Implements 1.1: `GET /`, `limit`, `link` on accounts, `POST /sessions/revoke`.
2. Adds the session probe hook used by the conformance suite. It is enabled only
   in test builds or by an explicit test-environment flag.
3. Verifies its OIDC callback against the contract's browser sign-in
   requirements. It adds tests for: an account that isn't linked, a missing
   access-role claim, a disabled local account, and a replayed handoff.
4. Passes the conformance suite in its own CI.

CLIMS's callback and handoff already exist (`backend/internal/features/auth/sso.go`).
For CLIMS this track is about verification and tests, not new sign-in code.

## Phase 2 — end-to-end environment

`infra/compose/identity-sync-e2e/` starts a disposable stack:

- **Keycloak:** imports `infra/keycloak/realm-export.json` plus synthetic test
  users.
- **Dashboard:** backend, Postgres (all migrations) and Redis.
- **Applications:** CLIMS and POES backends, each with its own Postgres seeded
  with synthetic accounts, plus their frontends for the browser flow.
- **TLS:** Caddy with an internal CA, so every origin is HTTPS as the contract
  requires. Test runners trust the CA.

Test suites, run by `make identity-sync-e2e`:

1. **Contract:** the conformance suite against CLIMS and POES.
2. **Sync:** batch preview, approval of selected rows, a forced grant failure
   followed by resume (the link is not recreated), and an invitation for a user
   who has no Keycloak account.
3. **Browser (Playwright):** sign in once and open both tiles, landing in the
   right local accounts with existing permissions. An unlinked user is refused,
   and so is a user without the access role.
4. **Removal:** revoke CLIMS. New CLIMS sign-ins are refused, the existing CLIMS
   session ends immediately, POES keeps working, and the next sync
   shows the account as `revoked` and doesn't re-grant.
5. **Migrations:** `make migrations-check` (up, down and up again for 61–64).

## Phase 3 — pilot runbook

`docs/operations/identity-sync-pilot.md` (written after Phase 2) covers each step
and rehearses it in the e2e environment first:

1. Register the application and run `describe`.
2. Read-only preview against production.
3. Export rows for review.
4. Approve a small batch and apply it.
5. Check sign-in for those users.
6. Rehearse removal on a pilot account.
7. Check that failed rows and retry status are visible.
8. Only then consider scheduled reconciliation, which will never auto-approve
   `create` or `review` rows.

## Track dependencies

```text
Phase 0 (contract + this design)
  ├── A registry/CLI ──┬── B batch ──┐
  │                    └── C removal ┤
  ├── D module/conformance ──────────┼── Phase 2 e2e ── Phase 3 pilot
  └── E apps (CLIMS, POES) ──────────┘
```

B and C build on A's registry. D and E depend only on the contract. Phase 2
needs all five tracks.
