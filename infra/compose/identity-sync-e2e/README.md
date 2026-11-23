# Identity-sync end-to-end stack

A disposable stack for the identity-sync Phase 2 tests
([design](../../../docs/architecture/identity-sync.md#phase-2--end-to-end-environment),
[contract](../../../docs/integrations/identity-sync-contract.md)). It runs the
dashboard, Keycloak, CLIMS and POES together behind HTTPS, seeded with
**synthetic data only**. It never loads a real dump or backup.

- Compose project: `idsync-e2e`. Every database is on tmpfs. `down -v` removes
  everything, including Caddy's CA.
- Host port: only `127.0.0.1:18443` (Caddy). Nothing else is published.
- Every secret and password in this folder is a fake fixture value
  (`e2e-fake-…`, `E2e-Fixture-Only-1`). They are valid only inside this stack.
- The test-only session probe is on here, and only here: CLIMS and POES run
  with `APP_ENV=test` and `IDENTITY_SYNC_TEST_PROBE=1`.

## Start, check, stop

The app images are built from sibling checkouts. `CLIMS_REPO` and `POES_REPO`
default to `../clims` and `../poesGo` next to the dashboard repository. Set
them when you work from a worktree:

```sh
export CLIMS_REPO=/path/to/clims POES_REPO=/path/to/poesGo
make -C infra identity-sync-e2e-up     # build, start, seed, wait, then run scripts/smoke.sh
make -C infra identity-sync-e2e-smoke  # re-run the health proof against a running stack
make -C infra identity-sync-e2e-test   # e2e suites (stage 2; currently smoke only, then exits 1)
make -C infra identity-sync-e2e-down   # docker compose -p idsync-e2e down -v
```

Without make:

```sh
docker compose -p idsync-e2e -f infra/compose/identity-sync-e2e/compose.yaml up -d --build --wait
docker compose -p idsync-e2e -f infra/compose/identity-sync-e2e/compose.yaml wait registry-seed
infra/compose/identity-sync-e2e/scripts/smoke.sh
docker compose -p idsync-e2e -f infra/compose/identity-sync-e2e/compose.yaml run --rm cli identity-sync describe --app clims
docker compose -p idsync-e2e -f infra/compose/identity-sync-e2e/compose.yaml --profile tools down -v
```

If `docker compose build` fails with `stat ~/.docker/buildx/...: permission denied`
(a sandboxed shell, for example), point `BUILDX_CONFIG` at a writable directory.

## Reaching it from the host

Every origin is `https://<name>.e2e.test:18443`. The same URL works inside the
compose network, where Caddy carries the network aliases, and on the host,
where port 18443 is published. So the issuer, the redirect URIs and the
registered origins are identical everywhere.

| Host | Serves |
|---|---|
| `sso.e2e.test` | Keycloak 26.0.5, realm `moh-realm`. Issuer `https://sso.e2e.test:18443/realms/moh-realm` |
| `dashboard.e2e.test` | Dashboard backend (core profile) |
| `clims.e2e.test` | CLIMS: `/api`, `/docs` and `/health` go to the backend, everything else to the Next.js SPA |
| `poes.e2e.test` | POES backend, including its `/api/v1/auth/sso/session` landing page |
| `mail.e2e.test` | Mailpit UI and API (`/api/v1/messages`). It catches all SMTP from Keycloak and the dashboard |

Do not edit `/etc/hosts`. Map the names per command instead:

- **CA:** `up` writes Caddy's root certificate (never its key) to
  `generated/caddy-root.crt`, which git ignores. The file changes on every
  fresh `up`.
- **curl:** `curl --cacert infra/compose/identity-sync-e2e/generated/caddy-root.crt --resolve sso.e2e.test:18443:127.0.0.1 https://sso.e2e.test:18443/realms/moh-realm/.well-known/openid-configuration`
- **Playwright/Chromium:** launch with
  `args: ['--host-resolver-rules=MAP *.e2e.test 127.0.0.1']`, and either trust
  the CA or, for this disposable stack only, use `ignoreHTTPSErrors: true`.
  For Node `fetch` and the APIs, set `NODE_EXTRA_CA_CERTS` to the CA file.
- **Containers:** join `idsync-e2e_edge` and mount the `idsync-e2e_e2e_ca`
  volume (`root.crt`). Go programs pick it up through `SSL_CERT_FILE`.

## What starts

| Service | Notes |
|---|---|
| `caddy` | Internal CA, TLS for every `*.e2e.test` host on 18443 |
| `ca-export` | One-shot: copies Caddy's root certificate to the `e2e_ca` volume and `generated/` |
| `realm-merge` | One-shot: `keycloak/merge-realm.py` merges `infra/keycloak/realm-export.json` with `keycloak/realm-e2e-overlay.json` |
| `keycloak` | `start-dev --import-realm`, H2 on tmpfs, SMTP to mailpit |
| `mailpit` | `axllent/mailpit:v1.27.0` |
| `dashboard-backend` | Built by `dashboard.Dockerfile` (server plus `moh-sso` CLI). Runs all migrations at startup. Applies the RBAC seed and syncs from the merged realm and live Keycloak. `IDENTITY_SYNC_ALLOWED_PRIVATE_HOSTS=clims.e2e.test,poes.e2e.test` |
| `dashboard-postgres`, `redis` | tmpfs and no persistence |
| `dashboard-seed` | One-shot: `seed/dashboard-bindings.sql` makes `clims-identity-sync` and `poesgo-identity-sync` enabled service identities of their apps, with an audit row |
| `registry-seed` | One-shot: `scripts/register-apps.sh` runs `identity-sync register --enable` for both apps, then `describe` |
| `clims-backend` | `$CLIMS_REPO/infra/docker/backend/Dockerfile.prod`. Its own migrations run at startup |
| `clims-seed` | One-shot: `seed/clims-seed.sql`, applied after those migrations |
| `clims-frontend` | `$CLIMS_REPO/infra/docker/frontend/Dockerfile.prod`. Has the `backend` alias on its private network, as its rewrites expect |
| `poes-backend` | `$POES_REPO/infra/docker/backend/Dockerfile.prod`, `AUTH_PROVIDER=oidc` |
| `poes-postgres` | Init scripts: `seed/poes-01-base-schema.sql` (synthetic stand-in for the MySQL-dump schema), POES migrations `000002` and `000003` from `$POES_REPO`, then `seed/poes-04-seed.sql` |
| `cli` | Profile `tools`: `docker compose … run --rm cli identity-sync <subcommand>` |

## Fixtures

Every Keycloak user's password is `E2e-Fixture-Only-1`, and every email is
`<username>@e2e.test`. Keycloak user IDs are fixed, so app links can be seeded:
`e2e00000-0000-4000-8000-<n>`, with `n` zero-padded to 12 digits.

| Keycloak user (n) | Client roles | CLIMS account | POES account | Expected preview |
|---|---|---|---|---|
| `e2e.admin` (1) | `dashboard-web`: `portal_admin` (`*`), realm `admin`, group Platform Administrators | none | none | n/a. Portal admin for the dashboard admin APIs (`rbac:write`, `rbac:roles:write`) |
| `both.apps` (10) | `clims_access`, `poesgo_access` | active, linked | id 1, active, linked | `linked` in both apps |
| `clims.linked` (101) | `clims_access` | active, linked | none | `linked` |
| `clims.unlinked` (102) | none | active, not linked | none | `reuse` |
| `clims.noaccess` (103) | none | active, linked | none | `linked` (batch run: `linked_no_access`) |
| `clims.disabled` (104) | `clims_access` | `inactive`, linked | none | not listed. Sign-in refused |
| `clims.orphan` (105) | `clims_access` | none | none | n/a. Sign-in refused (no link) |
| `clims.kcdisabled` (106) | none, and the Keycloak user is disabled | active, not linked | none | `review` |
| `poes.linked` / `unlinked` / `noaccess` / `disabled` / `orphan` (201 to 205) | as for CLIMS, with `poesgo_access` | none | ids 2, 3, 4, 5 (inactive), and none for orphan | `linked`, `reuse`, `linked`, not listed, n/a |
| no Keycloak user | none | `clims.invitee@e2e.test` | id 6 `poes.invitee@e2e.test` | `create`. Exercises invitations; mail arrives in mailpit |

Each app also has accounts without an email: `conf1`, `conf2`, `filler1` to
`filler3` (eligible, proposal `review`), and `confinelig` (ineligible). These
are for the conformance suite's linkable, ineligible and pagination fixtures.
Each app also has one ineligible account with an email: CLIMS
`clims.roleless`, which has no role; POES `poes.locked` (id 7) and
`poes.roleless` (id 14). CLIMS account IDs are
`c1000000-0000-4000-8000-0000000000NN`. With `NN` in hex, the order follows the
table: 01 `both.apps` … 0e `roleless`. POES IDs are 1 to 14.

Extra realm clients for the conformance tokens:

- `e2e-other-service`: a service account whose token has `aud` clims/poesgo
  but no `identity_sync` role. Secret `e2e-fake-other-service-secret`.
- `e2e-user-token`: a public client with the password grant, for a user access
  token with `aud` clims/poesgo.
- Sync clients: `clims-identity-sync` / `e2e-fake-clims-sync-secret` and
  `poesgo-identity-sync` / `e2e-fake-poesgo-sync-secret`.

## Smoke check (`scripts/smoke.sh`)

The script proves, with real requests, that:

1. every long-running service is healthy and every one-shot job exited 0;
2. OIDC discovery answers over HTTPS inside the network (curl container on
   `idsync-e2e_edge`) and from the host (`--resolve` to 127.0.0.1:18443), with
   the expected issuer;
3. `GET /api/v1/internal/identity-sync` returns contract `1.1` for CLIMS and
   POES, using a client-credentials token from each sync client;
4. `identity-sync describe --app clims|poesgo` shows registry and live
   descriptor 1.1, enabled;
5. the dashboard, the CLIMS SPA, CLIMS and POES health endpoints, and the
   mailpit API all answer over HTTPS.
