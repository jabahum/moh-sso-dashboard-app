# Identity-sync end-to-end suites

The Phase 2 suites from the
[identity-sync design](../../docs/architecture/identity-sync.md#phase-2--end-to-end-environment).
They run against the disposable `idsync-e2e` stack in
[`infra/compose/identity-sync-e2e`](../../infra/compose/identity-sync-e2e/README.md),
which holds synthetic data only.

```sh
export CLIMS_REPO=/path/to/clims POES_REPO=/path/to/poesGo
make -C infra identity-sync-e2e-test   # down -v, up, all suites, migrations-check, down -v
IDSYNC_E2E_KEEP=1 make -C infra identity-sync-e2e-test   # leave the stack up afterwards
```

`run-all.sh` does the work and exits non-zero if any step fails. To iterate
against a stack that is already running, run `npm ci`, then
`npx playwright test tests/02-sync.spec.mjs`. Suites 01, 02 and 04 change
state (links, grants, a revocation), so a second run needs a fresh stack.

This is its own npm package, pinned to `@playwright/test` 1.64.1 (the
frontend's version), and it uses Chromium only. It needs Node 22, docker and
the stack's CA at `infra/compose/identity-sync-e2e/generated/caddy-root.crt`.

## Suites and their users

Each suite has its own synthetic users, so suites do not interfere with each
other. Rows a suite only reads (proposals for other fixtures) are not changed.

| Suite | File | Users and accounts it changes | What it proves |
|---|---|---|---|
| Contract | `01-contract.spec.mjs` | `contract.user` (user token); CLIMS `conf1`/`conf2`, POES 8/9 get links to random subjects | `moh-sso identity-sync conformance --probe` passes against CLIMS and POES, with real Keycloak tokens: a valid sync token, wrong audience (`dashboard-admin`), a user token, another client (`e2e-other-service`), the sync client without `identity_sync` (role removed for the mint), and a genuinely expired sync token (2 s lifespan for the mint). The only allowed skip is `transport.retries`, which the design leaves to the connector unit tests. Garbage, tampered, edited-payload and `alg:none` tokens get 401. |
| Sync | `02-sync.spec.mjs` | CLIMS `clims.unlinked`, `clims.recheckdir`, `clims.recheckacct`; POES `poes.unlinked`, `poes.invitee` | `run start` proposals match the seed (reuse/create/review/linked/linked_no_access, ineligible accounts not listed). Approval needs a reviewer and is refused for `review` rows. A forced grant failure (the dashboard's admin client loses `realm-admin` and `manage-users`) leaves the row `linked`/`failed` (`grant_failed`). `resume` grants the role without touching the app's link row (same id, `created_at` and `xmin`). An invitation creates one unverified Keycloak user with `VERIFY_EMAIL` and `UPDATE_PASSWORD` and `ihp_identity_sync_origin=poesgo:6`, and sends one execute-actions email to mailpit. Re-runs change nothing and send nothing. An unverified Keycloak email (`clims.unverified`) is proposed `review` and cannot be approved. Approved rows whose directory match stops qualifying, or whose app email changes, before apply go back to `review` (`directory_changed`, `account_changed`) with nothing linked or granted. |
| Browser | `03-browser.spec.mjs` | `both.apps`, `browser.unlinked`, `browser.noaccess`, `clims.disabled`/`poes.disabled`, `clims.orphan`/`poes.orphan`, `clims.kcdisabled` (read only) | One portal sign-in, then the CLIMS and POES tiles from the portal's application menu, with no second login. Each app lands in the right local account and role: the CLIMS `moh` dashboard and account menu for "Both Apps", and the POES `/auth/me` user `both.apps` with `poe_officer` and `POE-E2E-ALPHA`. An unlinked user with the access roles is refused by both apps. A user without the access roles gets no tiles and is refused by both apps. A disabled local account and an orphan (access role, no local account) are refused by each app with no session. A user disabled in Keycloak cannot sign in at all. |
| Removal | `04-removal.spec.mjs` | `removal.both` | With live browser and probe sessions in CLIMS and POES, a portal admin removes `clims_access` with the request the portal access form sends: the profile's direct access with CLIMS deselected, to `PUT /api/v1/admin/rbac/user-access/users/:id` (the users app that hosts the form is not served by the core profile). Within 5 s: the CLIMS access token and probe session get 401, a new CLIMS sign-in is refused, POES still works, the response has no warnings, and the revocation row shows every step `done`. Afterwards: Keycloak, the CLI listing and the audit log agree, the next batch run proposes `revoked`, it cannot be approved, and apply does not re-grant. |
| Migrations | `run-all.sh` | none (its own throwaway Postgres on `127.0.0.1:18432`) | `make -C infra migrations-check` |

The removal suite also checks that the RBAC user-access profile, which fills
the portal's access form, shows the user's current Keycloak roles.

## Trust and host names

Nothing edits `/etc/hosts` or the system trust store.

- **Node:** `lib/stack.mjs` pins `*.e2e.test` to `127.0.0.1:18443` and trusts
  only Caddy's root CA.
- **Chromium:** launched with `--host-resolver-rules`. Chromium cannot take a CA
  file, so Node first verifies the served chain against Caddy's root, then
  passes that chain's SPKI pins to `--ignore-certificate-errors-spki-list`.
  There is no `ignoreHTTPSErrors`.
- **CLI:** runs in the stack's `cli` container (`docker compose run`). Tokens go
  in as inherited environment variables, never on the command line.
