# Local HTTPS: test CLIMS and POES against the dev stack

Use this to test a local CLIMS and a local POES (Poesgo) against the dashboard
dev stack end to end: launch from the portal, identity sync (preview, link,
revoke, restore) and logout. For the walkthrough, the test checklists and
troubleshooting, see
[Testing connected applications locally](../../../docs/integrations/local-integration-testing.md).

Browser sign-in alone works over plain HTTP. Identity sync does not:

- CLIMS turns its identity-sync endpoints on only with an HTTPS issuer
  (`backend/internal/features/auth/identity_sync.go` in the CLIMS repository).
- The dashboard requires HTTPS for the registered application origin and the
  Keycloak token endpoint it mints sync tokens at (`KEYCLOAK_INTERNAL_URL`).

So these overrides move the dev Keycloak issuer to `https://localhost:8443`,
using a local certificate authority, and put an HTTPS front on CLIMS and POES
for the dashboard backend. For a disposable stack with synthetic data and the
automated suites, use [identity-sync-e2e](../identity-sync-e2e/README.md) instead.

| | Address |
| --- | --- |
| Keycloak issuer | `https://localhost:8443/realms/moh-realm` (`http://localhost:8081` still answers) |
| Portal | `http://localhost:3000/portal/` |
| CLIMS in the browser | `http://localhost:8080` (CLIMS Docker stack's nginx) |
| POES backend / admin in the browser | `http://localhost:8082` / `http://localhost:3001` (poesGo dev stack) |
| Sync origins for the dashboard | `https://clims.local.test`, `https://poes.local.test`, only inside the dev network |

| File | Applied to |
| --- | --- |
| `dashboard.override.yml` | The dashboard dev stack: HTTPS Keycloak, backend trust and sync settings, `clims-tls` (HTTPS front for both applications) |
| `clims.override.yml` | The CLIMS repository's Docker stack (project `clims`) |
| `poes.override.yml` | The poesGo repository's dev stack (project `poesgo-dev`) |
| `setup.sh` | Writes certificates, the trust bundle, Caddyfiles and client secrets to `LOCAL_HTTPS_DIR` |

Everything `setup.sh` writes lives in `LOCAL_HTTPS_DIR` (default
`~/.moh-dev-https`), outside the repository. The commands below assume the
three repositories sit side by side (`moh-sso-dashboard`, `clims`, `poesGo`).

## 1. Prepare

```sh
infra/compose/local-https/setup.sh   # local CA, certificates, trust bundle, Caddyfiles
```

Trust the CA once so your browser accepts `https://localhost:8443`. Chrome and Safari
use the macOS keychain:

```sh
sudo security add-trusted-cert -d -r trustRoot -k /Library/Keychains/System.keychain ~/.moh-dev-https/ca.crt
# later: sudo security delete-certificate -c "MOH local dev CA" /Library/Keychains/System.keychain
```

## 2. Start the dashboard dev stack with HTTPS Keycloak

```sh
docker compose -p moh-sso-dashboard-dev --env-file infra/env/dev.env \
  -f infra/compose/docker-compose.dev.yml -f infra/compose/local-https/dashboard.override.yml \
  up -d keycloak backend clims-tls
```

Pass the same `-f` files to every later `docker compose` command on these services.
`make dev-up` and plain compose bring back the HTTP issuer.

In the dev realm, the `clims` client needs the redirect URI
`http://localhost:8080/api/v1/auth/sso/callback`, and `poesgo` needs
`http://localhost:8082/api/v1/auth/sso/callback`.

Then save both client secrets from the running Keycloak (they are not displayed):

```sh
infra/compose/local-https/setup.sh --secret
```

## 3. Start CLIMS against it

From the CLIMS repository:

```sh
OIDC_CLIENT_SECRET=$(cat ~/.moh-dev-https/clims-oidc-secret.txt) docker compose -p clims \
  --env-file infra/env/dev.env -f infra/compose/docker-compose.yml -f infra/compose/docker-compose.dev.yml \
  -f ../moh-sso-dashboard/infra/compose/local-https/clims.override.yml \
  up -d --no-build kc-loopback backend
docker restart clims-nginx-1   # nginx re-resolves "backend"
```

The override sets `APP_ENV=development` and `IDENTITY_SYNC_TEST_PROBE=1`, so the
conformance suite's session probe is available. Never use either outside a test
environment.

## 4. Start POES against it

From the poesGo repository (`POESGO_DEV_PORT=8082` in its `infra/compose/.env`;
the admin runs on 3001 because the portal uses 3000):

```sh
cd infra/compose
POESGO_DEV_FRONTEND_PORT=3001 OIDC_CLIENT_SECRET=$(cat ~/.moh-dev-https/poes-oidc-secret.txt) docker compose \
  -f docker-compose.dev.yml -f ../../../moh-sso-dashboard/infra/compose/local-https/poes.override.yml \
  up -d --no-build kc-loopback backend frontend
```

The override puts POES in OIDC mode, which refuses password sign-in, and sets
`OIDC_WEB_HANDOFF_URL` so a portal sign-in lands in the Next.js admin (see
"Web handoff" in poesGo's `docs/23-sso-and-identity-sync-onboarding.md`). The admin's login
page offers "Sign in with the MOH Integrated Health Portal". To go back to local
mode, run the same `up` without the override and `docker rm -f poesgo-dev-kc-loopback-1`.

## 5. Register both applications

```sh
docker exec -w /app moh-sso-dashboard-dev-backend-1 sh -c 'go build -o /tmp/moh-sso ./cmd/cli'
cli="docker exec -w /app moh-sso-dashboard-dev-backend-1 /tmp/moh-sso identity-sync"

$cli register --app clims --origin https://clims.local.test --sync-client clims-identity-sync \
  --access-role clims_access --credential keycloak:clims-identity-sync --account-id-format uuid \
  --reason "Local dev: CLIMS Docker stack" --enable
$cli register --app poesgo --origin https://poes.local.test --sync-client poesgo-identity-sync \
  --access-role poesgo_access --credential keycloak:poesgo-identity-sync --account-id-format positive-integer \
  --reason "Local dev: POES dev stack" --enable
$cli describe --app clims
$cli describe --app poesgo
```

**Expect:** each descriptor shows contract `1.1`, its application (`clims` or
`poesgo`), its account ID format (`uuid` or `positive-integer`) and the
capabilities `accounts.list`, `identity.link` and `sessions.revoke`.

Each sync client must first be bound to its application as a service identity,
which a portal admin does once (see
[Connected applications](../../../docs/integrations/connected-applications.md#5-connect-an-application)).

The seeded launcher entries point at production. Change the launch URLs to
`http://localhost:8080/api/v1/auth/sso/start` (CLIMS) and
`http://localhost:8082/api/v1/auth/sso/start` (POES) in Admin → Systems.
Changing a launch URL sends an active system back to `configured`, so take it
through validate, approve and activate again.

## 6. Link a user and test

CLIMS and POES open an account only through a link the dashboard recorded, keyed
by issuer and Keycloak subject. Moving the issuer to HTTPS needs new links, even
for users linked under `http://localhost:8081` before. The portal user needs an
account in the application with the same verified email:

```sh
$cli preview --app clims --limit 25
$cli apply   --app clims --account <account-id> --approved-subject <keycloak-user-id>
$cli revoke  --app clims --subject <keycloak-user-id> --reason "..." --revoker <you>   # ends live sessions
$cli restore --app clims --subject <keycloak-user-id> --reason "..." --restorer <you>  # then grant the access role again
```

Use `--app poesgo` for POES. To link at every backend start without review, set
`IDENTITY_SYNC_AUTO_LINK_APPS` (see the guide). The test checklists are in the
[guide](../../../docs/integrations/local-integration-testing.md#5-test-checklist).

If an application's data came from a production dump, link only test accounts,
and don't run `identity-sync conformance` against it: that suite creates links on
fixture accounts.

## Undo

Re-run each stack's normal `up` for the same services without the override, then
`docker rm -f moh-sso-dashboard-dev-clims-tls-1 clims-kc-loopback-1 poesgo-dev-kc-loopback-1`.
