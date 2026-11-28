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
again. Migration 65 lets the integration itself be disabled after its binding
broke: an UPDATE that leaves the row disabled and keeps
`application_client_id`, `sync_client_id` and `access_role` unchanged skips the
binding checks. Inserts, enabling and every binding change are still checked.

**Origins must be public.** A save sends a live sync-client token to the
origin, so the registry resolves the origin's host on every save and refuses
loopback, link-local (including `169.254.169.254`), private (RFC 1918, IPv6
ULA), CGNAT (`100.64.0.0/10`), unspecified, multicast and reserved addresses
(`ErrPrivateOrigin`) before any token is minted. The connector repeats the check
at dial time on the peer address it actually connects to, which defeats DNS
rebinding, and such a refusal is not retried. Hosts listed in
`IDENTITY_SYNC_ALLOWED_PRIVATE_HOSTS` (comma-separated host names or IP
literals; empty by default) are exempt. Dev and e2e stacks whose applications
run on a private network or loopback set it. It is wired into the dev compose
file, both env examples and the Helm chart (`backend.identitySync.allowedPrivateHosts`);
the CLI reads it from its own environment. **Upgrade step:** before deploying,
check whether any registered origin resolves to an internal address
(split-horizon DNS, in-cluster names) and list those hosts, or that app's syncs
and portal session revocations are refused. Connector transports ignore
`HTTP(S)_PROXY`: through a proxy the dial-time check would inspect the proxy, not
the origin. Changing the origin of an enabled integration needs the same
`rbac:write` and `rbac:roles:write` permissions as any registry change.

**Contract version.** An enabled save reads `GET /` again. When the stored
version is 1.1 or newer and the application now answers like 1.0 (404), the
save is refused (`ErrContractDowngrade`, HTTP 409
`IDENTITY_SYNC_CONTRACT_DOWNGRADE`) unless the caller passes
`allowContractDowngrade` (CLI: `--allow-contract-downgrade`), so a transient 404
during a deploy cannot silently downgrade a registration.

**Secrets.** The registry stores a reference, never a secret. `env:NAME` reads
an environment variable, and `file:/path` reads a mounted secret file (one
trailing newline is trimmed; an empty secret is refused; the secret is never
logged or returned). `keycloak:<sync client ID>` reads the sync client's
current secret through the Keycloak admin API each time a connector is built,
so the secret is never copied out of Keycloak and a regenerated secret is used
on the next call without a restart. It must name the integration's own sync
client, and Keycloak only hands out the secret of a service-account-only client
(confidential, no browser or password flows), so it can't read an
application's interactive client secret. It needs the admin client's
`view-clients` role, which `dashboard-admin` already has. The reference
resolves in the process that uses it: the dashboard backend for the admin API's
enable check, the CLI host for `identity-sync` commands (`keycloak:` there uses
`KEYCLOAK_ADMIN_CLIENT_ID` / `_SECRET`). Existing deployments register CLIMS as
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

**No silent downgrade.** The registry's stored `contract_version` is passed to
the connector. When it is 1.1 or newer, a 404 from `GET /` is
`ErrDescriptorMissing` (not cached, so the next call asks again), never a
fallback to 1.0. Only an explicit re-register (`register --enable` or the admin
API PUT), which probes without the stored version, may record a downgrade to
1.0.

### CLI

```text
moh-sso identity-sync register   --app <id> --origin <https> --sync-client <id> --access-role <role> --credential env:NAME
                                 --reason <text> [--account-id-format uuid|positive-integer|opaque] [--enable]
moh-sso identity-sync describe   --app <id>                     # registry entry + live GET / descriptor
moh-sso identity-sync preview    --app <id> [--after c] [--limit n]
moh-sso identity-sync apply      --app <id> --account <id> --approved-subject <sub> [--after c]
moh-sso identity-sync conformance --app <id>                    # Track D
moh-sso identity-sync check-network [--app <id> | --origin <https>] # DNS, address policy, TCP, TLS, HTTP; no token
```

`clims-identity-sync` and `poes-identity-sync` remain as deprecated aliases for
one release (`identity-sync … --app clims` / `--app poesgo`). They print a
deprecation note to stderr. When the application has an enabled registry entry
they use it. They fall back to the `<APP>_SYNC_*` environment configuration
only when no registry is configured (no database settings or Keycloak base URL)
or the application has no registry row. A registry that is configured but
cannot be opened, a failed lookup, or a row whose revocation list cannot be
opened stops the command with a clear error (fail closed). For a disabled row
the environment run still refuses revoked identities. The connector validates
account IDs using the format in the descriptor. A 1.0 application gets the
existing per-app validation, which comes from a registry field and not from code.

`preview` (and the aliases) show an account the application already reports as
linked for our issuer as `linked`, and an identity with an active revocation as
`revoked` instead of `reuse`; preview opens the revocation list and fails closed
without it. Single-account `apply` refuses any account that is already linked
(`ErrAccountAlreadyLinked`): a linked account without access is
`linked_no_access`, and access removed on purpose must be restored explicitly
(grant the access role in the portal, which also restores an active revocation,
or `identity-sync restore` followed by a reviewed grant). For a 1.1 application
this also means a single apply whose grant failed after the link cannot simply
be re-run; use the portal grant or a batch run's `resume`.

Registry commands read the database from the usual `app.env`/environment
configuration and Keycloak settings from `KEYCLOAK_BASE_URL`, `KEYCLOAK_REALM`,
`KEYCLOAK_EXTERNAL_URL` (public issuer) and the operator credentials. `apply`
walks the eligible accounts to find the reviewed account, then re-checks that
the directory still matches exactly the approved subject before linking.
Database-only subcommands (`run show`, `run approve`, `run reject`, and
`revocations` without `--resume`) carry the `moh-sso/no-keycloak-admin`
annotation, so the root command does not demand the Keycloak admin flags for
them; `revocations` listing opens only the database.

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

0. `attempts += 1`. A row the re-read walk did not return gets
   `link_status=failed` (`account_lookup_failed`) and resume retries it: a
   moving page boundary is not proof of ineligibility. After 3 attempts that
   each found the account missing, the row becomes final as `ineligible`
   (`account_missing`), so a run with a deleted account can still complete.
   Otherwise only a 404 from `PUT /accounts/{id}/identity` makes a row
   `ineligible`. A row whose email
   changed since review goes back to `review` (`account_changed`).
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
4. Under the per-(application, subject) access lock (see Track C), re-check
   revocation, then grant the access role: `granted` or `failed`
   (`grant_failed`). If the lock cannot be taken the row fails with
   `revocation_check_failed`. After the grant, `RevokeService.EnforceRevocation`
   checks again; if a revocation exists anyway, its steps are re-run (the role
   is removed again) and the row becomes `revoked` (`identity_revoked`).

`resume` processes approved rows that were attempted but neither finished nor
reached a final state. A row with `link_status=linked` skips steps 0–3: no
account re-read, no directory lookup, no `PUT`. Only the revocation check and
the grant run, so the link is never recreated. Every apply or resume call that
processes a row still adds one to its `attempts`, so a row granted on its
first resume shows `attempts=2`. Within each call, HTTP retries
follow the contract's backoff rules (the connector does this); across calls,
resume is the retry. The run ends `completed`, or `failed` while an attempted,
approved row still has work left.

Rows proposed `linked_no_access` or `revoked`, and rows that become `revoked`,
are never granted. Apply selects only approved `reuse`/`create` rows, and
migration 63 rejects a `linked_no_access` or `revoked` row marked granted.

Stable error codes: `account_ineligible`, `account_lookup_failed`, `account_missing`, `account_changed`,
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
be able to edit it. `infra/keycloak/realm-export.json` and
`realm-export-prod.json` (Keycloak 26.0.5) declare it in their
`org.keycloak.userprofile.UserProfileProvider` component
(`declarative-user-profile`: Keycloak's default attributes plus
`ihp_identity_sync_origin` with `view`/`edit` = `["admin"]`); the unmanaged
attribute policy is not set. `prepare-keycloak-realm.py` keeps that component
in generated environment realms. An existing realm is not changed by an import:
add the attribute in Realm settings → User profile once.

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
variable `identitySyncRevocations(ctx, rt)`, which `identity_sync_wiring.go`
sets to Track C's `RevokeService.IsRevoked` on the registry database (it fails
closed when that database is unavailable). The same checker guards the
single-account `identity-sync apply` and the deprecated aliases' registry
path (`Service.Application`/`Service.Revocations`), checked before the link and
before the grant. The checker the CLI passes is a `*RevokeService`, so batch and
single apply also get its access lock (`AccessLocker`) and post-grant
re-assertion (`RevocationEnforcer`); with the Keycloak operator settings present
it is the full revoke service, so a re-asserted revocation removes the role and
ends sessions at once.

**Owned files:** `backend/internal/identityreconcile/batch*.go`,
`backend/internal/identityreconcile/invite*.go`, migration 63,
`backend/cmd/cli/identity_sync_run*.go`, and
`backend/internal/keycloak/identity_sync_users.go`.

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
   and never call it success. `unsupported` is recorded only when the registry's
   stored `contract_version` is `1.0`; for any other stored version an
   "unsupported" answer is `failed` (`sessions_unsupported_unexpected`) and resume
   retries it (see Track A, "No silent downgrade").
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

**Grants and revocations are serialised.** Without coordination, a revocation
landing between an apply's revocation check and its grant would be undone by
that grant. Both paths take one lock per (application, subject):
`PostgresRevocations.WithAccessLock` holds the transaction-scoped advisory lock
`pg_advisory_xact_lock(0x1d5c, hashtext('<application>/<subject>'))` while its
function runs. Batch and single apply hold it across "check revoked → grant";
`Revoke` holds it across "record revocation → remove role", and resume and
re-assertion hold it around the role step. Either the grant finishes first and
the revocation then removes the role, or the revocation commits first and the
grant's check sees it. As a second line of defence apply calls
`EnforceRevocation` after granting: if a revocation exists anyway (written by a
path that did not take the lock), its steps are marked `pending` again (so
resume handles them even if the process stops) and, with a full revoke service,
run at once.

**Portal re-grant restores.** When a portal admin grants an application's
registered access role (users API or RBAC user-access update) to a person who
has an active revocation for that application, the portal hook records an
explicit restore by that admin: `restored_by=portal:<admin id>`,
`restore_reason="portal re-grant"`, audited as `…revocation_restored`. The
restore is recorded only after the grant succeeded, so the revocation list
matches reality. A failed restore never fails the grant; it is returned as a
warning (`identity_sync_restore_failed`).

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
  removal always stands. An incomplete revocation, or a 1.0 application that
  cannot end sessions, comes back as `IncompleteRevocationError` with a warning
  code (`identity_sync_revocation_incomplete` or
  `identity_sync_sessions_not_ended`). The users service then raises a critical
  admin notification (`IDENTITY_SYNC_REVOCATION_INCOMPLETE`) and adds a warning
  to the request (`internal/accesswarnings`, nil-safe). `PUT` and `DELETE`
  `/users/:id/client-roles` answer 204 when there is nothing to report and 200
  with `{"warnings": [...]}` otherwise; the RBAC user-access update keeps 200 and
  adds a `warnings` field. The hook also implements
  `users.AccessRoleGrantHook` (portal re-grant restore, above). Wired in
  `internal/bootstrap` (`identity_sync_revocation.go`, both profiles).

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
| Retries | The application is safe to retry: a page read again with the same cursor is identical; five identical link requests sent at once all return 204 and leave one link; five identical session revokes sent at once all return 204; every 429 or 503 carries `Retry-After`. The suite retries 429/502/503/504 like the dashboard does. The dashboard's own retry policy (backoff, attempts, `Retry-After`) stays covered by the connector unit tests. |

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
