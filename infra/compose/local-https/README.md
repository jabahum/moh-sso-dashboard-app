# Local HTTPS: test CLIMS against the dev stack

Use this to test a local CLIMS against the dashboard dev stack end to end:
launch from the portal, identity sync (preview, link, revoke, restore) and logout.

Browser sign-in alone works over plain HTTP. Identity sync does not:

- CLIMS turns its identity-sync endpoints on only with an HTTPS issuer
  (`backend/internal/features/auth/identity_sync.go` in the CLIMS repository).
- The dashboard requires HTTPS for the registered application origin and the
  Keycloak token endpoint it mints sync tokens at (`KEYCLOAK_INTERNAL_URL`).

So these overrides move the dev Keycloak issuer to `https://localhost:8443`,
using a local certificate authority, and put an HTTPS front on CLIMS for the
dashboard backend. For a disposable stack with synthetic data and the
automated suites, use [identity-sync-e2e](../identity-sync-e2e/README.md) instead.

| | Address |
| --- | --- |
| Keycloak issuer | `https://localhost:8443/realms/moh-realm` (`http://localhost:8081` still answers) |
| Portal | `http://localhost:3000/portal/` |
| CLIMS in the browser | `http://localhost:8080` (CLIMS Docker stack's nginx) |
| CLIMS for the dashboard (sync origin) | `https://clims.local.test`, only inside the dev network |

Certificates, the trust bundle, the Caddyfiles and the clims client secret live in
`LOCAL_HTTPS_DIR` (default `~/.moh-dev-https`), outside the repository.

## 1. Prepare

```sh
infra/compose/local-https/setup.sh            # local CA, certificate, trust bundle, Caddyfiles
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
`http://localhost:8080/api/v1/auth/sso/callback`.

## 3. Start CLIMS against it

Save the clims client secret from the running Keycloak (not displayed), then, from
the CLIMS repository:

```sh
infra/compose/local-https/setup.sh --secret   # run in the dashboard repository

cd ../clims
OIDC_CLIENT_SECRET=$(cat ~/.moh-dev-https/clims-oidc-secret.txt) docker compose -p clims \
  --env-file infra/env/dev.env -f infra/compose/docker-compose.yml -f infra/compose/docker-compose.dev.yml \
  -f ../moh-sso-dashboard/infra/compose/local-https/clims.override.yml \
  up -d --no-build kc-loopback backend
docker restart clims-nginx-1   # nginx re-resolves "backend"
```

The override sets `APP_ENV=development` and `IDENTITY_SYNC_TEST_PROBE=1`, so the
conformance suite's session probe is available. Never use either outside a test
environment.

## 4. Register CLIMS for identity sync and the launcher

```sh
docker exec -w /app moh-sso-dashboard-dev-backend-1 sh -c 'go build -o /tmp/moh-sso ./cmd/cli'
docker exec -w /app moh-sso-dashboard-dev-backend-1 /tmp/moh-sso identity-sync register --app clims \
  --origin https://clims.local.test --sync-client clims-identity-sync --access-role clims_access \
  --credential keycloak:clims-identity-sync --account-id-format uuid \
  --reason "Local dev: CLIMS Docker stack" --enable
docker exec -w /app moh-sso-dashboard-dev-backend-1 /tmp/moh-sso identity-sync describe --app clims
```

**Expect:** the descriptor shows contract `1.1`, application `clims`, `uuid` and the
capabilities `accounts.list`, `identity.link` and `sessions.revoke`.

The `clims-identity-sync` service identity must be bound to `clims`, which a portal
admin does once (see [Connected applications](../../../docs/integrations/connected-applications.md#5-connect-an-application)).

The seeded CLIMS launcher entry points at production. Change its launch URL to
`http://localhost:8080/api/v1/auth/sso/start` in Admin → Systems. Changing the
launch URL sends an active system back to `configured`, so take it through
validate, approve and activate again.

## 5. Link a user and test

CLIMS opens an account only through a link the dashboard recorded, keyed by issuer
and Keycloak subject. Moving the issuer to HTTPS needs new links, even for users
linked under `http://localhost:8081` before. The portal user needs a CLIMS account
with the same verified email:

```sh
cli="docker exec -w /app moh-sso-dashboard-dev-backend-1 /tmp/moh-sso identity-sync"
$cli preview --app clims --limit 25
$cli apply   --app clims --account <clims-user-id> --approved-subject <keycloak-user-id>
$cli revoke  --app clims --subject <keycloak-user-id> --reason "..." --revoker <you>   # ends live CLIMS sessions
$cli restore --app clims --subject <keycloak-user-id> --reason "..." --restorer <you>  # then grant clims_access again
```

Check, as the linked user:

1. The portal's application menu shows CLIMS, and it opens without a second login.
2. After `revoke`, an open CLIMS session gets 401 and a new sign-in shows
   "Could not sign in from the portal" (`sso_error=denied`).
3. Portal logout returns to the portal, and CLIMS's next launch asks for a login.
   CLIMS's own session keeps working until it expires or you sign out of CLIMS:
   portal logout cannot end independently managed sessions.

If CLIMS data came from a production dump, link only test accounts, and don't run
`identity-sync conformance` against it: that suite creates links on fixture accounts.

## Undo

Re-run each stack's normal `up` for the same services without the override, then
`docker rm -f moh-sso-dashboard-dev-clims-tls-1 clims-kc-loopback-1`.
