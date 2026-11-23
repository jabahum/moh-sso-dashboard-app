# Identity-sync production pilot runbook

This runbook links a small number of existing CLIMS accounts to their MoH
Keycloak identities in production, checks that those people can sign in from
the portal, and rehearses removing access. It follows the design in
[`docs/architecture/identity-sync.md`](../architecture/identity-sync.md) (Phase
3) and the application contract in
[`docs/integrations/identity-sync-contract.md`](../integrations/identity-sync-contract.md).

Every step was rehearsed in the disposable e2e stack first (see
[Rehearsal](#rehearsal-in-the-e2e-stack)). Do the same before each pilot: run
`make -C infra identity-sync-e2e-test` on the exact commits you deploy.

## Scope and people

- **Application:** CLIMS (`clims`), contract 1.1. POES follows the same steps
  with `poesgo` once CLIMS has finished the pilot.
- **First batch:** at most 10 `reuse` rows, chosen by the reviewer. No `create`
  (invitation) rows in the first batch. Invitations come in a second batch after
  the first one has been checked.
- **Removal rehearsal:** on one staff-owned test account that exists in both
  Keycloak and CLIMS, never on a real clinical user.

| Role | Who | Does |
|---|---|---|
| Operator | Platform engineer with dashboard database and Keycloak admin access | Runs every `moh-sso` command, watches logs |
| Reviewer | A named person from the MoH identity team | Decides which rows to approve, recorded by name on each row |
| CLIMS owner | CLIMS technical lead | Confirms CLIMS settings, watches CLIMS logs, answers for local roles |
| Pilot users | The people behind the approved rows | Sign in once from the portal and report what they see |

Write the names into the change ticket before starting.

## Stop conditions

Stop and roll back (see [Rollback](#rollback)) if any of these happens:

- `describe` or `run start` reports an error you cannot explain.
- A proposal looks wrong for a person you know (wrong match, a known account
  missing, an ineligible account listed).
- After apply, a pilot user lands in the wrong CLIMS account or with different
  CLIMS permissions than before.
- A user without `clims_access`, or an unlinked user, gets into CLIMS.
- The removal rehearsal leaves a CLIMS session working after the revoke
  command returns, or affects the user's other applications.

## 0. Before the pilot day

### Deployed versions

| Component | Must include |
|---|---|
| Dashboard backend | `feat/platform-identity-sync` (migrations 61 to 65, the identity-sync registry, batch runs, revocations, and the `moh-sso` CLI in the image) |
| CLIMS | `feat/clims-identity-sync-v1.1` (contract 1.1, migration `000017_session_revocation_marker`) |

Take a database backup of the dashboard and CLIMS databases after deploying and
before step 1. Record the backup names in the ticket.

### Keycloak (production realm)

The realm export carries these, but a live realm does not pick them up from the
file. Check each one in the admin console:

1. **User profile:** Realm settings → User profile → JSON editor contains
   `"unmanagedAttributePolicy": "ADMIN_EDIT"`. Invitations store
   `ihp_identity_sync_origin` on the users they create and fail without it.
2. **Sync client:** `clims-identity-sync` exists, is confidential, has only
   *Service accounts* enabled (no standard or direct-access flow), and its
   service account holds the `clims` client role `identity_sync`. Its tokens
   carry `aud` `clims` (audience mapper).
3. **Access role:** the `clims` client has the role `clims_access`, and the
   CLIMS client's ID token carries `clims_roles`.
4. **Hostname:** Keycloak runs with a fixed public hostname (`KC_HOSTNAME`), so
   tokens requested through the internal URL still carry the public issuer.

Copy the `clims-identity-sync` secret into the secret store. It is used in two
places below and never pasted on a command line.

### CLIMS settings

| Setting | Value |
|---|---|
| `APP_ENV` | `production` |
| `IDENTITY_SYNC_TEST_PROBE` | **unset.** CLIMS refuses to start if it is set in production. |
| `OIDC_ISSUER` | the public issuer, for example `https://sso.health.go.ug/realms/moh` |
| `OIDC_CLIENT_ID` / `OIDC_CLIENT_SECRET` | the interactive `clims` client |
| `OIDC_PUBLIC_ORIGIN` | CLIMS's public origin |
| `IDENTITY_SYNC_CLIENT_ID` | `clims-identity-sync` |

The CLIMS owner confirms `/api/v1/internal/identity-sync` is reachable from the
dashboard's network and, if the proxy allows it, not from the internet.

### Dashboard settings

| Setting | Value |
|---|---|
| `CLIMS_SYNC_CLIENT_SECRET_FILE` | path of the mounted `clims-identity-sync` secret. The entrypoint loads it as `CLIMS_SYNC_CLIENT_SECRET`, which the registry references as `env:CLIMS_SYNC_CLIENT_SECRET`. |
| `IDENTITY_SYNC_ALLOWED_PRIVATE_HOSTS` | empty, unless CLIMS's origin resolves to an internal address from the dashboard (split-horizon DNS). Then list that host name only. |
| `KEYCLOAK_EXTERNAL_URL` | the public Keycloak URL (the issuer is built from it) |

### Service identity binding

The registry only accepts a sync client that is an enabled service identity of
the application. A portal admin with `rbac:write` and `rbac:roles:write`
registers it once:

```text
PUT /api/v1/admin/platform/applications/clims/identities/clims-identity-sync
{"subject": "<Keycloak user ID of service-account-clims-identity-sync>",
 "scopes": [], "enabled": true, "reason": "CHG-<ticket>: identity-sync pilot"}
```

Check it appears under the application's service identities in the portal.
`enabled` must be `true`: the registry refuses a disabled binding.

### Running the CLI

The backend image ships the operator CLI at `/app/moh-sso`. Run it as a one-off
container with the backend's own environment, through the entrypoint so mounted
secrets load. With Compose:

```sh
docker compose run --rm --no-deps \
  --entrypoint /usr/local/bin/sso-entrypoint backend \
  /app/moh-sso identity-sync describe --app clims
```

The rest of this runbook writes that prefix as `moh-sso`.

**In the portal.** Steps 1 to 7 can also be done in the portal by an
administrator with `rbac:write` and `rbac:roles:write`: Admin → RBAC → select
the application → Operations → **Identity sync**. The page registers the
integration and checks its live descriptor, starts runs (as background jobs),
lets the reviewer select and approve rows (recorded under their own portal
identity), applies and resumes runs, and lists access removals with a retry and
a clear action. The CLI remains the reference for this runbook; both write the
same records and audit events. On Kubernetes, run
the same command in a Job or `kubectl exec` into a backend pod with
`/usr/local/bin/sso-entrypoint /app/moh-sso …`. Every command that changes
something takes a reason, a reviewer or an actor, and writes the audit log.

## 1. Register CLIMS and describe it

```sh
moh-sso identity-sync register --app clims \
  --origin https://clims.health.go.ug \
  --sync-client clims-identity-sync \
  --access-role clims_access \
  --credential env:CLIMS_SYNC_CLIENT_SECRET \
  --reason "CHG-<ticket>: identity-sync pilot" \
  --enable

moh-sso identity-sync describe --app clims
```

**Expect:** the registry row shows `contractVersion` `1.1` and `enabled` true.
The live descriptor shows `version` `1.1`, `application` `clims`,
`accountIdFormat` `uuid`, `maxPageSize` 100 and the capabilities
`accounts.list`, `identity.link` and `sessions.revoke`. The registry's own
`accountIdFormat` (default `opaque`) is only used for 1.0 applications; a 1.1
application's descriptor wins.

**If it fails:** a private-origin error means the origin resolves to an internal
address (see `IDENTITY_SYNC_ALLOWED_PRIVATE_HOSTS`). A binding error means step
0's service identity is missing. A 401 or 403 from CLIMS means the sync client,
its role or the audience mapper is wrong; compare with the Keycloak checklist.

## 2. Read-only preview against production

```sh
moh-sso identity-sync run start --app clims
moh-sso identity-sync run show --run <run-id> --summary
```

`run start` reads every eligible CLIMS account and stores one proposal per
account. It changes nothing in CLIMS or Keycloak.

**Expect:** a run in status `previewed`. The summary counts by proposal:

| Proposal | Meaning | Can be approved |
|---|---|---|
| `reuse` | exactly one enabled Keycloak user with this email, and that email is verified | yes |
| `create` | no Keycloak user with this email; apply would invite one | yes (not in the first batch) |
| `review` | no email, an unverified email, a disabled Keycloak user, or a link under another issuer | no; resolve by hand first |
| `linked` | CLIMS already has this link | nothing to do |
| `linked_no_access` | linked, but no `clims_access` | never granted by a run; restore access explicitly |
| `revoked` | the person's CLIMS access was removed on purpose | never granted by a run |

The CLIMS owner checks the total against their own count of active accounts
that hold at least one role.

## 3. Export rows for review

```sh
moh-sso identity-sync run show --run <run-id> --proposal reuse > reuse.json
jq -r '["accountId","email","proposedSubject"],
       (.rows[] | [.accountId, .emailHint, .proposedSubject]) | @csv' \
  reuse.json > reuse.csv
```

Repeat with `--proposal review` and `--proposal create` for the reviewer's
records. Handle the file as personal data: keep it on the operator's machine or
the ticket's restricted attachment area, and delete local copies after the
pilot.

The export has the CLIMS account ID, the email and the proposed Keycloak user
ID, not names. The reviewer looks each Keycloak user ID up in the Keycloak admin
console (Users → search by ID) to see the name.

The reviewer picks at most 10 `reuse` rows of people they can reach, ideally
across two or three CLIMS roles, and checks each email against the person.
Approving any other proposal is refused (`only reuse and create rows can be
approved`).

## 4. Approve a small batch and apply it

```sh
moh-sso identity-sync run approve --run <run-id> \
  --account <id1>,<id2>,... \
  --reviewer "<reviewer name or email>" \
  --note "CHG-<ticket>: pilot batch 1"

moh-sso identity-sync run apply --run <run-id>
moh-sso identity-sync run show --run <run-id> --decision approved
```

Apply re-reads each approved account and its directory match. A row whose CLIMS
email changed, or whose Keycloak match no longer qualifies, goes back to
`review` (`account_changed`, `directory_changed`) and nothing is linked for it.
Otherwise apply links the account in CLIMS and then grants `clims_access`.

**Expect:** every approved row ends `linkStatus=linked`, `grantStatus=granted`,
and the run ends `completed`. Rows that were not approved are untouched.

**If a row fails:** `run show` gives its `lastError`. For a transient error
(`grant_failed`, `link_failed`, `account_lookup_failed`), fix the cause and run:

```sh
moh-sso identity-sync run resume --run <run-id>
```

Resume never relinks a row that is already linked; it only retries the grant.
`link_conflict` means CLIMS already links that account or that Keycloak user to
something else: stop and investigate with the CLIMS owner.

## 5. Check sign-in for those users

Ask each pilot user to sign in at the portal and open the CLIMS tile. For each:

- [ ] No second login prompt appears after the portal sign-in.
- [ ] CLIMS opens in their own account (name in the account menu).
- [ ] Their CLIMS role and pages are the same as before the pilot.

The CLIMS owner confirms in the CLIMS database that each pilot account has one
`user_oidc_identities` row for the production issuer. A second `run start`
should now show those accounts as `linked`.

## 6. Rehearse removal on the pilot test account

Use the staff test account only. First make sure it has a live CLIMS session
(sign in from the portal and keep the tab open) and a live session in another
application.

Remove access the way an admin would, in the portal: open the user's access
page, clear `clims_access` and save. The equivalent command is:

```sh
moh-sso identity-sync revoke --app clims --subject <keycloak-user-id> \
  --revoker "<operator>" --reason "CHG-<ticket>: removal rehearsal"
moh-sso identity-sync revocations --app clims
```

**Expect, at once:**

- [ ] The open CLIMS tab is signed out on its next request.
- [ ] A new CLIMS sign-in from the portal is refused.
- [ ] The other application's tab keeps working.
- [ ] `revocations` shows the row with every step `done`.
- [ ] A new `run start` proposes `revoked` for the account and it cannot be approved.

Then restore the test account, so the pilot ends clean:

```sh
moh-sso identity-sync restore --app clims --subject <keycloak-user-id> \
  --restorer "<operator>" --reason "CHG-<ticket>: end of rehearsal"
```

and grant `clims_access` again in the portal. Granting it in the portal also
records a restore, so either order works. Until access is granted again, a new
run proposes `linked_no_access` for the account: restore never grants access by
itself.

## 7. Check failed rows and retry status are visible

Before closing the day, the operator confirms there is no hidden unfinished
work:

```sh
moh-sso identity-sync run show --app clims                 # recent runs and their status
moh-sso identity-sync run show --run <run-id> --decision approved
moh-sso identity-sync revocations --unfinished
```

**Expect:** the pilot run is `completed`, no approved row has a `lastError`, and
`revocations --unfinished` is empty. If it is not empty,
`moh-sso identity-sync revocations --resume --actor "<operator>"` retries the
pending and failed steps, and never repeats a finished one.

The audit log holds every change: registry updates
(`application.identity_sync.updated`), approvals and applies, and revocations
(`application.identity_sync.revocation_*`), with who did it and why.

## 8. After the pilot

Keep the pilot accounts under watch for a week (sign-in errors in CLIMS logs,
support tickets). Then, with the reviewer's agreement:

1. A second batch with a few `create` rows. Each invited person gets one email
   to verify their address and set a password.
2. Larger `reuse` batches.
3. POES (`poesgo`), from step 0.

Scheduled reconciliation is out of scope for the pilot. If it is added later it
must only re-propose and report; it never approves `create` or `review` rows.

## Rollback

| Situation | Action |
|---|---|
| Stop all syncing for CLIMS | Register again without `--enable` (same flags, a reason). Runs and revocations then refuse to use the integration. Existing links and roles stay. |
| One person linked to the wrong CLIMS account | `revoke` that person (removes `clims_access` and ends sessions), then the CLIMS owner removes the link in CLIMS as a manual, audited change. Never edit the link from the dashboard. |
| Undo the whole first batch | `revoke` each pilot subject, then the CLIMS owner removes their `user_oidc_identities` rows. People keep signing in to CLIMS the old way. |
| Dashboard release must be rolled back | Follow [`docs/rollback.md`](../rollback.md). Migrations 61 to 65 have down migrations; take the backup from step 0 first. |

Disabling the integration does not undo grants already made. Revocation still
works for an application whose integration is disabled: it removes the role in
Keycloak and revokes consents, and records the session step as failed
(`integration_disabled`). After re-enabling, `revocations --resume` ends the
sessions.

## Rehearsal in the e2e stack

The e2e stack (`infra/compose/identity-sync-e2e/`) uses the same image,
migrations and CLI as production, with synthetic users and HTTPS on
`*.e2e.test`. Its suites already rehearse steps 1 to 7:

| Runbook step | Rehearsed by |
|---|---|
| 1 Register and describe | `registry-seed` and `scripts/smoke.sh` (`register --enable`, `describe`) |
| 2 Preview | `02-sync.spec.mjs` (`run start`, every proposal type) |
| 3 Export | `02-sync.spec.mjs` (`run show` JSON) |
| 4 Approve and apply | `02-sync.spec.mjs` (reviewer required, review rows refused, forced grant failure then resume, apply-time rechecks, invitation) |
| 5 Sign-in | `03-browser.spec.mjs` (portal sign-in, CLIMS and POES tiles, refusals) |
| 6 Removal | `04-removal.spec.mjs` (removal from the portal form's request, sessions end at once, other app keeps working, next run proposes `revoked`) |
| 7 Visibility | `02-sync.spec.mjs` and `04-removal.spec.mjs` (`run show`, `revocations`, audit log) |

Run the whole rehearsal with:

```sh
CLIMS_REPO=<path to clims> POES_REPO=<path to poesGo> \
  make -C infra identity-sync-e2e-test
```

### Hand rehearsal, 2026-11-23

The operator commands in this runbook were also run by hand against a fresh
stack (`make -C infra identity-sync-e2e-up`, then
`docker compose -p idsync-e2e -f infra/compose/identity-sync-e2e/compose.yaml run --rm cli identity-sync …`),
with these results:

| Step | Command | Result |
|---|---|---|
| 1 | `register … --enable`, `describe` | Stored 1.1, enabled; descriptor 1.1, `uuid`, three capabilities |
| 2 | `run start`, `run show --summary` | `previewed`; 17 rows: 4 reuse, 1 create, 7 review, 3 linked, 2 linked_no_access |
| 3 | `run show --proposal reuse` + the `jq` line | CSV with 4 rows (account ID, email, Keycloak user ID) |
| 4 | `run approve` (2 rows), `run apply`, `run show --decision approved` | Exit 0, run `completed`, both rows `linked`/`granted`, attempts 1, reviewer recorded |
| 5 | CLIMS database check | One `user_oidc_identities` row with the stack's issuer and the proposed subject |
| 6 | `revoke`, `revocations` | Role, consents and sessions `done` on attempt 1, `complete` true |
| 6 | `run start` after revoke | Account proposed `revoked`; approving it fails with `only reuse and create rows can be approved` |
| 6 | `restore` | Restored, "access was not re-granted"; next run proposes `linked_no_access` |
| 7 | `run show --app clims`, `revocations --unfinished` | Recent runs listed; no unfinished revocations |
| Rollback | `register` without `--enable`, then `run start` | `enabled` false; `run start` fails with `identity sync integration is disabled` |
| Rollback | `revoke` while disabled | Role and consents `done`, sessions `failed` (`integration_disabled`), exit 1 with a resume hint |
| Rollback | `register … --enable`, `revocations --unfinished --resume` | Sessions step `done`, revocation `complete` |

Browser sign-in (step 5's user checks) and the portal removal were not repeated
by hand; `03-browser.spec.mjs` and `04-removal.spec.mjs` cover them in the same
stack.
