# Identity environments and client onboarding

This page is for operators and identity administrators who set up Keycloak realms for
each portal environment and onboard or promote OIDC clients between them. It covers
the realm model, local development, creating realms on an existing Keycloak instance,
an optional separate identity stack, and the client onboarding workflow.

## Environments and realms

The portal manages one configured realm per deployment. Its credentials never select
an arbitrary realm from a browser request. Onboarding is blocked when the deployment
and realm do not match `backend/internal/keycloak/identity-environments.json`:

| Portal ENVIRONMENT | KEYCLOAK_REALM | Target hosting |
| --- | --- | --- |
| development | moh-dev | Existing shared Keycloak instance |
| test | moh-test | Existing instance for manual testing; disposable container in CI |
| staging | moh-staging | Existing shared Keycloak instance |
| production | moh-realm | Existing production realm, unchanged |

If an environment's existing realm has a different name, for example a production
realm called `MoH`, set `IDENTITY_ENVIRONMENT_REALM` to that name alongside
`KEYCLOAK_REALM` in the deployment's private environment file. The two must match,
and the override may not name another environment's realm (a development deployment
cannot declare `moh-realm`). Without it, client onboarding stops with
"connected to a realm for a different environment".

One Keycloak instance is sufficient for this setup. All realms use your existing
identity hostname and certificate; their issuer paths differ. Users, clients,
credentials, signing keys and sessions are separate per realm. Single sign-on works
between applications in the same realm, not across environments. The realms share
the Keycloak server, database infrastructure, server administrators and maintenance
window. A separate production instance can be introduced later; it is optional here.

## Local Docker development

The standalone dev stack reads `KEYCLOAK_REALM` from the private
`infra/env/dev.env`; Compose must not override that setting. Set it to `moh-dev`
after creating the development realm with the realm preparation/bootstrap tools
below. Configure its own `dashboard-admin` and `dashboard-web` credentials in that
same private file. Do not reuse credentials from a remote production instance.
Recreate only the development backend after changing those settings:

```sh
sh infra/scripts/compose.sh dev up -d --no-deps backend
```

Sign in again with an account in `moh-dev`; sessions and users from `moh-realm`
are not transferred. The old realm and its users remain available. The legacy dev
example still uses `moh-realm` for existing setups; onboarding remains blocked
there until the isolated realm is configured.

## Create missing realms on your existing instance

This is the recommended path for the current server. It does not start containers,
change the existing Keycloak database configuration, restart Keycloak or replace any
realm. It uses the Admin API to create only missing `moh-dev`, `moh-test` and
`moh-staging` realms. `moh-realm` and all other existing realms are outside its write
scope. Existing environment realms are skipped without importing or updating them.

### Prepare the plan and operator credentials

Copy `infra/keycloak/environments.shared.example.json` to a private working file
and replace the example application URLs with those you will use. Each entry has
`portalUrl` and `apiUrl` (including `/api/v1`). A subset of development/test/staging
is allowed; `production` and arbitrary realm names are rejected. These URLs configure
the portal clients; this command does not deploy those application environments.

Copy `infra/env/identity-shared.env.example` to a private file outside source control
and load/export it using your normal operator secret-management process. Set
`KEYCLOAK_BASE_URL` to the existing HTTPS identity URL, for example
`https://auth.health.go.ug`. Choose **one** authentication mode:

- An existing administrative client in the `master` realm using
  `KEYCLOAK_BOOTSTRAP_CLIENT_ID` and `KEYCLOAK_BOOTSTRAP_CLIENT_SECRET`.
- Your existing `master` administrator using `KEYCLOAK_BOOTSTRAP_USERNAME` and
  `KEYCLOAK_BOOTSTRAP_PASSWORD` through `admin-cli`, if password authentication is
  permitted for that account. For an account requiring additional authentication,
  use the administrative-client option instead.

The operator identity needs realm creation and read access to the target environment
realms. The normal portal's realm-scoped administrative client is not sufficient.
The command does not create a privileged operator identity or grant permissions.
Keep these bootstrap credentials out of portal deployments; each portal continues
using its own realm-scoped credentials. Credentials are read only from environment
variables and are not printed, written to the plan, or passed as command arguments.

### Preview and apply

Preview first:

```sh
python3 infra/scripts/bootstrap-keycloak-realms.py \
  --plan /private/identity/shared-environments.json
```

Review the realm names and `create`/`skip-existing` actions. Apply explicitly with a
**new** private creation record (create its parent directory first):

```sh
python3 infra/scripts/bootstrap-keycloak-realms.py \
  --plan /private/identity/shared-environments.json \
  --apply --record /private/identity/realm-creation.json
```

The record is created with mode `0600`; an existing record is never overwritten.
The command rechecks before creating and never uses realm update, delete or partial
import. A concurrent realm creation is skipped. It records each successful creation
before verifying it, so an interrupted run remains inspectable. Re-running with a
new record skips existing realms. Review existing realm configuration separately if
it differs from the requested URLs; this bootstrap never reconciles existing realms.
If a request was interrupted, inspect the record and admin console before cleanup.
There is no automatic realm deletion or rollback, and no migration of production
users or credentials. Retire bootstrap access after the one-time setup as appropriate.

### Configure each portal deployment

All four portal deployments can use the **same** external identity base URL:

| Environment | Example issuer |
| --- | --- |
| development | `https://auth.health.go.ug/realms/moh-dev` |
| test | `https://auth.health.go.ug/realms/moh-test` |
| staging | `https://auth.health.go.ug/realms/moh-staging` |
| production | `https://auth.health.go.ug/realms/moh-realm` |

Set each portal's `ENVIRONMENT`, `KEYCLOAK_REALM`, internal/external identity URLs,
callback and client credentials consistently. Obtain the newly generated client
secrets privately through the appropriate realm's administration. Create separate
non-production administrators/test users and configure staging/test SMTP to a test
inbox or sink. Production users, clients, policies and signing keys stay untouched.

## Optional: bootstrap a separate identity stack

### Generate the realm import file

Generate an environment-specific import file. Substitute the real staging portal
and API URLs; the API URL includes `/api/v1`:

```sh
python3 infra/scripts/prepare-keycloak-realm.py \
  --environment staging \
  --portal-url https://portal-staging.example.org \
  --api-url https://portal-staging.example.org/api/v1 \
  --output /private/identity/moh-staging.json
```

Create the output directory first. The generator refuses to overwrite an existing
file. It preserves platform role/scope definitions and service-account role bindings,
removes human accounts and all credentials/IDs, and excludes external application
clients. Keycloak generates fresh client credentials and signing keys. External apps
must be onboarded independently in each environment. Only core portal clients start
enabled. Other retained platform client definitions remain disabled and have their
old browser URLs removed until configured for this environment.

### Start the stack

Copy `infra/env/identity.env.example` to a private file outside the repository. Set a
unique project, identity hostname, loopback port, generated import file path and
private database/bootstrap passwords. Then:

```sh
docker compose --env-file /private/identity/staging.env \
  -f infra/compose/docker-compose.identity.yml config --quiet
docker compose --env-file /private/identity/staging.env \
  -f infra/compose/docker-compose.identity.yml up -d
```

The stack uses Keycloak production mode and PostgreSQL, binds HTTP to loopback and
expects a trusted HTTPS reverse proxy at `IDENTITY_HOSTNAME`. Route that environment's
hostname to its loopback port; forward the configured proxy headers. Avoid routing
untrusted traffic directly to the internal listener. For local development use a
loopback HTTP hostname and generate the realm with loopback portal/API URLs.

### Configure the portal after first boot

After first boot, set environment-specific secrets for the portal's existing
administrative and confidential web clients in the Keycloak admin console. Store
these in that portal deployment's private environment. Set its `ENVIRONMENT`,
`KEYCLOAK_REALM`, internal URL, external URL, callback and web/admin client credentials
consistently. Each application must validate its own realm's exact issuer. Configure
SMTP separately; staging email should go to a test inbox/sink. Provision human
administrators explicitly and grant realm-specific administration permissions.

These commands prepare fresh installations. They do not replace live realms or
migrate users. Startup import skips existing realms. The existing local dev Compose
stack is retained for compatibility; opting into `moh-dev` requires configuring that
portal deployment to use its `moh-dev` realm on the existing or optional new stack. Do not point both realm names at
the same application's configuration or reuse old cookies/sessions after switching.

## Onboard a client from the Clients page

Open **Clients → Onboard application** (requires `clients:write`). Register the
application name, owner, support email and client profile. Review callbacks, logout
addresses, browser origins and the optional external launcher entry. Select
**Validate & preview**, inspect the environment and configuration, confirm review,
then provision the disabled client. The result provides its issuer/client ID and
next steps. **Export onboarding draft** produces a credential-free JSON document.
For managed disabled clients, **Edit** opens the same reviewed update workflow;
existing unmanaged clients retain their normal editor.

## Client lifecycle and promotion

The Clients administration workflow uses the same public-web, confidential-web and
service profiles as the operator CLI. Browser profiles use authorization code/S256,
RS256, `profile email`, stable subject/email claims, and exact callbacks/logout/origins.
Service profiles use client credentials with no browser flow or automatic role grants.
All new or updated onboarded clients stay disabled. Access mapping and activation
are explicit follow-up steps; configuring a launcher entry does not grant access.

Export an onboarding draft from the source deployment, import it into the target,
review/change its environment and all URLs, preview the result, then provision it.
Only the configuration is portable: users, credentials, signing keys and sessions
are never included. An existing target client must be owned by the same onboarding
profile and be disabled before an update. The preview shows its previous settings;
changes to the draft or target state require a new review. Audit records contain the
client ID, environment, realm, profile and operation, never credentials.

### Promote with the operator CLI

For an operator-assisted promotion, use the exported version-1 document:

```sh
python3 infra/scripts/reconcile-keycloak-clients.py \
  --environment staging --spec /private/identity/reviewed-app.json
python3 infra/scripts/reconcile-keycloak-clients.py \
  --environment staging --spec /private/identity/reviewed-app.json \
  --apply --backup /private/identity/app-before.json
```

Load only that environment's existing `KEYCLOAK_*` administrative credentials. The
CLI refuses realm/environment mismatches, cross-type client conversion and updates
to enabled or unmanaged clients. Apply requires a new mode-0600 backup. Provisioning
is repeatable; managed scopes/mappers are checked after apply. If a partial create
fails, leave it disabled, inspect the next preview and retry. For recovery restore
only the affected client's saved settings/scopes/mappers, never an entire realm.

### Checks before enabling a client

Before enabling: test code exchange, wrong/missing PKCE, logout, issuer/azp validation,
email-verification refusal and portal access mapping where applicable. Confidential
client secrets are retrieved/rotated privately through the realm's administration;
this workflow never exports them. Promote configuration through development → test
→ staging → production with the existing production review/release process.
