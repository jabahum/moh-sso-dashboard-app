# Health Workforce Portal OIDC setup

This page is for identity operators who provision the Keycloak web and laptop-test
clients requested for Digital Timesheet and Clinical Training, and for the external
team integrating sign-in. It does not activate sign-in in the external application or
change any live Keycloak instance automatically.

Production uses the existing **`moh-realm`**. The laptop-test client and synthetic
users live separately in **`moh-test`**, following the environment isolation setup.

## Client contract

| Setting | Production | Laptop test |
| --- | --- | --- |
| Client ID | `digital-timesheets` | `digital-timesheets-test` |
| Redirect URI, exact | `https://timesheet.health.go.ug/auth/sso` | `http://127.0.0.1:8081/auth/sso` |
| Post-logout URI, exact | `https://timesheet.health.go.ug/login` | `http://127.0.0.1:8081/login` |
| Web origin | `https://timesheet.health.go.ug` | `http://127.0.0.1:8081` |

The clients belong to different realms. See [identity environments](identity-environments.md)
for fresh test-realm bootstrap and per-environment credentials.

### Flows and logout

Both clients are public: **no client secret**. Authorization code with **S256 PKCE**
is required. Implicit flow, direct password grants, service accounts, authorization
services and front-channel logout are disabled. Use RP-initiated logout with an
`id_token_hint` and the registered post-logout URI.

### Scopes and claims

Request `scope=openid profile email`. Only `profile` and `email` are assigned as
default client scopes; `openid` is the OIDC request scope. Neither `roles` nor
`offline_access` is assigned. Dedicated email mappers include `email` and the
boolean `email_verified` in access tokens and ID tokens. A dedicated subject mapper
includes the stable Keycloak user ID as access-token `sub`, including on Keycloak
25+ where this otherwise comes from the `basic` scope. This also works with the
repository exports, which do not define `basic`.

Access and ID tokens use RS256. Access-token lifetime inherits the actual realm
default; the setup does not assume that the live default is five minutes. Roles
and hospital scope remain the external application's responsibility. These clients
are excluded from portal system enrollment and navigation.

### Issuer and application-side validation

For the expected production host:

- Issuer: `https://auth.health.go.ug/realms/moh-realm`
- Discovery: `https://auth.health.go.ug/realms/moh-realm/.well-known/openid-configuration`

The Timesheet application must use this issuer consistently. Its server must verify
the signature against discovery's JWKS, require RS256, validate issuer, expiry,
access-token type and `azp` against the selected client ID, and reject unverified
emails. First login may link by verified email; subsequent logins must use the
bound immutable `sub`. This repository does not implement that external application.

## Apply to an existing realm

The production manifest is `infra/keycloak/clients/digital-timesheets.json`;
`digital-timesheets-test.json` holds the isolated test client and fixtures. Both legacy realm exports
contain only the production client. The separate test manifest targets `moh-test`. **Startup realm import does
not update an existing realm.** Do not delete/reimport a live realm to add clients.

### Prerequisites

Use the existing private administrative environment, with
`KEYCLOAK_INTERNAL_URL` (or `KEYCLOAK_BASE_URL`), `KEYCLOAK_REALM=moh-realm`,
`KEYCLOAK_ADMIN_CLIENT_ID`, and `KEYCLOAK_ADMIN_CLIENT_SECRET`. Do not commit these
values. The operator needs realm read and client management permissions; creating
the optional users also needs user query and management permissions. This setup
does not grant new administrative permissions or create a service account.

### Preview the plan

Start with a read-only plan:

```sh
python3 infra/scripts/reconcile-digital-timesheets.py
```

The output reports clients to create/update, actual realm security/session settings,
and whether SMTP has configuration. It does not print credentials or test identities.
These readings answer configuration questions without changing shared realm policy.
SMTP configuration alone does not establish successful delivery, and the script does
not assert that existing users' emails are verified.

### Apply with a backup

Review the plan, then use a **new**, private backup path outside the repository:

```sh
python3 infra/scripts/reconcile-digital-timesheets.py \
  --apply --backup /private/operator-backups/timesheets-before.json
```

Create the parent directory privately first. The script creates the backup with mode
`0600` and refuses to overwrite it. It snapshots the managed clients' prior settings,
scope assignments and mappers before writing, rejects a different target realm or an
existing client with a different authentication type, then verifies convergence. Re-running
the plan after successful setup reports no client changes. Unknown client attributes
and unrelated clients, users, roles and realm settings are preserved. Review any
pre-existing dedicated mappers on these client IDs separately; only the named
authentication mappers are managed by this script.

## Synthetic test accounts

The user approved arbitrary test identities. These belong only to `moh-test`. These four fixtures use reserved,
non-deliverable `.invalid` addresses, never real staff addresses:

| Username | Email | Email verified |
| --- | --- | --- |
| `timesheets-test-trainee` | `timesheets-test-trainee@example.invalid` | Yes |
| `timesheets-test-supervisor` | `timesheets-test-supervisor@example.invalid` | Yes |
| `timesheets-test-coordinator` | `timesheets-test-coordinator@example.invalid` | Yes |
| `timesheets-test-unverified` | `timesheets-test-unverified@example.invalid` | No |

Users are optional, disabled initially, and have **no passwords in source control**.
They are not included in either realm export's user list. To include them in the
test plan/apply, load the `moh-test` administrative environment and add
`--environment test --include-test-users` to the commands above. Production setup
contains no synthetic users. Existing accounts with
the same username and fixture email are left untouched; conflicting usernames abort
before any mutation. The backup lists which fixtures were absent before setup.
No custom user-profile attributes or realm user-profile policy changes are needed.

An administrator must set temporary passwords privately and enable the accounts
before interactive testing. `UPDATE_PASSWORD` requires a password change at first
login. Create matching test accounts in the Timesheet application for the pilot
linking checks. The labels trainee/supervisor/coordinator are fixture names, not
Keycloak roles. Do not try sending verification/reset email to these addresses.
The unverified fixture must remain unverified: Keycloak can authenticate it, but the
Timesheet server must refuse to establish its application session.

## Validation and rollback

### Run the tests

```sh
python3 infra/scripts/test-digital-timesheets.py
python3 infra/scripts/test-digital-timesheets-runtime.py
```

The second command requires Docker and starts/removes its own temporary Keycloak
26.0.5 container on a random loopback port. It uses disposable credentials and never
contacts the production or laptop application URLs. It checks real code exchange,
PKCE/redirect restrictions, token claims, both email-verification cases, CORS, SSO,
logout and disabled grant types, plus reconciliation idempotence.

### Roll back a failed setup

If setup fails partway through, retain the private backup and inspect the next
dry run. For newly created clients, disable the selected client ID to stop new logins;
revoke their sessions before deleting them if rollback is required. For previously
existing clients, restore their settings, scope assignments and mapper configuration
from `clients[].before` in the backup using the admin console/API. Restore only these
clients, never the entire realm. Remove test users only after checking the backup's
`createTestUsers` list and confirming that they have no needed activity. There is
no automated destructive rollback.

## Out of scope and references

The external team controls pilot activation and continued email/password sign-in.
Android/offline access, a privileged server service account, portal launcher
enrollment, bulk staff migration and realm-wide policy changes remain out of scope.

References: [Keycloak Admin REST API](https://www.keycloak.org/docs-api/latest/rest-api/),
[Keycloak 25 subject-claim change](https://github.com/keycloak/keycloak/blob/main/docs/documentation/upgrading/topics/changes/changes-25_0_0.adoc).
