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
    issuer                TEXT NOT NULL CHECK (issuer ~ '^https://'),
    status                TEXT NOT NULL CHECK (status IN ('previewed','applying','completed','failed','cancelled')),
    created_by            TEXT NOT NULL CHECK (btrim(created_by) <> ''),
    created_at            TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at            TIMESTAMPTZ NOT NULL DEFAULT now()   -- also the apply heartbeat
);

CREATE TABLE ihp_identity_sync_rows (
    id               UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    run_id           UUID NOT NULL REFERENCES ihp_identity_sync_runs(id) ON DELETE CASCADE,
    account_id       TEXT NOT NULL,  -- contract ID pattern
    email_hint       TEXT NOT NULL DEFAULT '',
    proposal         TEXT NOT NULL CHECK (proposal IN ('reuse','create','review','linked','linked_no_access','revoked')),
    proposed_subject TEXT,           -- for create rows, set once the user is invited
    decision         TEXT NOT NULL DEFAULT 'pending' CHECK (decision IN ('pending','approved','rejected')),
    reviewed_by      TEXT,
    reviewed_at      TIMESTAMPTZ,
    review_note      TEXT,           -- at most 1000 characters
    user_status      TEXT NOT NULL DEFAULT 'not_required' CHECK (user_status IN ('not_required','pending','invited','failed')),
    link_status      TEXT NOT NULL DEFAULT 'pending' CHECK (link_status IN ('pending','linked','conflict','ineligible','failed')),
    grant_status     TEXT NOT NULL DEFAULT 'pending' CHECK (grant_status IN ('pending','granted','failed','not_required')),
    attempts         INT  NOT NULL DEFAULT 0,
    last_error       TEXT CHECK (last_error ~ '^[a-z][a-z0-9_]{0,63}$'),  -- stable code only, never upstream bodies
    updated_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (run_id, account_id),
    CONSTRAINT ihp_identity_sync_rows_reviewer CHECK (decision = 'pending' OR (reviewed_by <> '' AND reviewed_at IS NOT NULL)),
    CONSTRAINT ihp_identity_sync_rows_no_grant CHECK (NOT (proposal IN ('linked_no_access','revoked') AND grant_status = 'granted'))
);
```

Runs reference their integration without cascading, so deleting a registry
entry that has runs fails with 409 (`ErrIntegrationInUse`). Deleting a run
removes its rows.

**Proposals.** `run start` walks every eligible account (an account listed twice
is kept once) and proposes:

| Proposal | Situation | Initial statuses |
|---|---|---|
| `reuse` | One verified, enabled Keycloak user matches the email | link `pending`, grant `pending` |
| `create` | No Keycloak user matches | user `pending` |
| `review` | Ambiguous, unverified, no email, or linked under another issuer | |
| `linked` | `link` is set for our issuer and that user holds the access role (effective roles). Nothing to do. | link `linked`, grant `granted` |
| `linked_no_access` | Linked, but the user lacks the access role. Shown, never granted here (see Track C). | link `linked`, grant `not_required` |
| `revoked` | The linked or matched subject is revoked (`RevocationChecker`). Only an explicit restore grants it again. | grant `not_required` |

Starting a run is read-only on both sides. A listing, directory or revocation
error stops it before anything is stored, so an unknown answer never becomes
permission to invite or grant.

**Review.** Only `reuse` and `create` rows can be approved. Approve and reject
require a reviewer (non-blank, at most 200 characters, no control characters)
and take an optional note. The reviewer, time and note are stored on the row. A
decision can change until apply first touches the row (`attempts > 0` or the
user was invited). After that it is part of the record.

**Apply state machine.** `apply` first claims the run (`status=applying`). A
second apply gets `ErrRunBusy`, and a run left `applying` with no row activity
for 15 minutes can be taken over. It then re-reads the application's eligible
accounts once, for the rows that still need a link, and moves each approved row
that was never attempted through the steps below. Each outcome is written in its
own row transaction, which first re-checks that the row is still approved by a
named reviewer.

0. `attempts += 1`. A row no longer in the eligible list becomes `ineligible`
   (`account_ineligible`). A row whose email changed since review goes back to
   `review` (`account_changed`).
1. `create` rows whose `user_status` is not `invited`: `EnsureUser` (see
   Invitations), then `user_status=invited` and `proposed_subject` set to the new
   user. On failure, `user_status=failed`.
2. The subject must not be revoked. Otherwise the row becomes `revoked`
   (`identity_revoked`) and is neither linked nor granted. Then re-run the
   directory lookup. A `reuse` row needs the same single verified user. A
   `create` row needs exactly the user it invited (unverified, or verified as
   that subject). Otherwise the row goes back to `review` (`directory_changed`):
   proposal `review`, decision `pending`, reviewer kept for the record. Only a
   new run can handle such a row.
3. Link. If a 1.1 application already reports exactly this link, no `PUT` is
   sent. Otherwise `PUT` the link. The result is `linked`; `conflict`
   (`link_conflict`) or `ineligible` (`account_ineligible`), both final; or
   `failed` (`link_failed`).
4. Re-check revocation, then grant the access role: `granted` or `failed`
   (`grant_failed`).

`resume` processes approved rows that were attempted but neither finished nor
reached a final state. A row with `link_status=linked` skips steps 0–3: no
account re-read, no directory lookup, no `PUT`. Only the revocation check and
the grant run, so the link is never recreated. Within each call, HTTP retries
follow the contract's backoff rules (the connector does this); across calls,
resume is the retry. The run ends `completed`, or `failed` while an attempted,
approved row still has work left.

Rows proposed `linked_no_access` or `revoked`, and rows that become `revoked`,
are never granted. Apply selects only approved `reuse`/`create` rows, and
migration 63 rejects a `linked_no_access` or `revoked` row marked granted.

Stable error codes: `account_ineligible`, `account_changed`,
`directory_changed`, `directory_lookup_failed`, `invite_failed`,
`invite_email_collision`, `invite_ambiguous`, `invite_origin_mismatch`,
`invite_user_disabled`, `invite_attribute_not_stored`, `invite_email_failed`,
`invalid_identity`, `link_conflict`, `link_failed`, `grant_failed`,
`identity_revoked`, `revocation_check_failed`.

### Invitations

`identityreconcile.KeycloakDirectory` implements `Directory` for one
application, plus the `AccessChecker` that separates `linked` from
`linked_no_access`. `EnsureUser` is idempotent across timeouts:

1. Search users by the attribute `ihp_identity_sync_origin=<app>:<accountId>`
   (exact admin API `q=` query). A single marked user is reused if it is enabled
   and still has the account's email. Otherwise the call fails with
   `invite_origin_mismatch`, so a user who could set the attribute on
   themselves cannot claim another account. If invitation actions are still
   pending, the execute-actions email is sent again. Two marked users give
   `invite_ambiguous`.
2. Any other user with the account's email is unrelated:
   `invite_email_collision`.
3. Create the user: username = lower-cased email, email, enabled,
   `emailVerified=false`, required actions `VERIFY_EMAIL` and `UPDATE_PASSWORD`,
   and the marker attribute. No realm role is added. If the create times out or
   returns 409, search for the marker again. A 409 with no marked user is a
   collision.
4. Read the user back. If the realm dropped the marker (Keycloak 24+ drops
   attributes its user profile does not declare), delete the user just created
   and fail with `invite_attribute_not_stored`. The realm can then be fixed and
   the row resumed.
5. Send the execute-actions email (`VERIFY_EMAIL`, `UPDATE_PASSWORD`). A failure
   gives `invite_email_failed`. Resume reuses the user and sends it again.

**Realm requirement.** The realm user profile must declare
`ihp_identity_sync_origin` as an attribute that only administrators can view
and edit, or enable unmanaged attributes with admin-only editing. Users must not
be able to edit it.

Keycloak admin client additions (`internal/keycloak/identity_sync_users.go`;
their errors carry only the HTTP status):

- `SearchUsers`: exact email and/or one attribute, full representations.
- `CreateUserFromRequest`: returns `ErrUserConflict` on 409.
- `GetUserRepresentation` and `GetEffectiveUserClientRoles`: return
  `ErrUserNotFound` on 404.
- `DeleteUserWithContext`.

### Interfaces

```text
moh-sso identity-sync run start   --app <id>
moh-sso identity-sync run show    --run <id> [--proposal p] [--decision d] [--summary] | --app <id>
moh-sso identity-sync run approve --run <id> --account <id>[,<id>...] --reviewer <name> [--note text]
moh-sso identity-sync run reject  --run <id> --account <id>[,<id>...] --reviewer <name> [--note text]
moh-sso identity-sync run apply   --run <id>
moh-sso identity-sync run resume  --run <id>
```

`apply` and `resume` exit non-zero while rows remain to resume. Every command
calls `identityreconcile.BatchService` (`StartRun`, `GetRun`, `ListRuns`,
`ApproveRows`, `RejectRows`, `Apply`, `Resume`). The service uses a
`BatchRepository` (`PostgresBatchRepository`, or `MemoryBatchRepository` in
tests). Start, apply and resume also take a `BatchTarget`: the registry
connector plus a `KeycloakDirectory`. An admin page can call the same service
later.

**Revocations (Track C).** `BatchService.Revocations` is a
`RevocationChecker { IsRevoked(ctx, application, issuer, subject) (bool, error) }`.
Nil means nothing is revoked (`NoRevocations`). The CLI gets it from the package
variable `identitySyncRevocations(ctx, rt)`, which Track C replaces from
`identity_sync_revoke.go`.

**Owned files:** `backend/internal/identityreconcile/batch*.go`,
`backend/internal/identityreconcile/invite*.go`, migration 63,
`backend/cmd/cli/identity_sync_run*.go`, and
`backend/internal/keycloak/identity_sync_users.go`.

## Track C — access removal

Removing a person's access to one application (`identity-sync revoke`, or
removing `<app>_access` through portal RBAC) runs these idempotent steps:

1. Record `(application, issuer, subject, revoked_by, reason)` in
   `ihp_identity_sync_revocations` (migration 64). Proposals then show
   `revoked`, and no later sync grants access again.
2. Remove the application's access role in Keycloak. The application's OIDC
   callback then refuses new sign-ins, as the contract requires.
3. Revoke the user's consents and offline tokens for the application's
   interactive client (`DELETE /admin/realms/{realm}/users/{id}/consents/{clientId}`).
   The user's other applications and Keycloak SSO session are untouched.
4. Call the application's `POST /sessions/revoke` (contract 1.1), retried with
   backoff. For a 1.0 application, existing sessions last until their local
   expiry. The CLI reports that window instead of claiming success.
5. Record the outcome in the audit log.

**Decision pending:** the agreed window for ending existing sessions. The design
supports "immediately", meaning steps 2–4 run synchronously. The e2e tests
assert whichever window is agreed.

**Owned files:** `backend/internal/identityreconcile/revoke*.go`, migration 64,
and the hook in `internal/features/rbac` (the `UpdateUserAccess` path) that calls
the revoke service when an `<app>_access` role is removed.

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
   session ends within the agreed window, POES keeps working, and the next sync
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
