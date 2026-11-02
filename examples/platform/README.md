# Independent application pilot

These examples use only supported HTTP contracts or published frontend packages. They never import backend internals or read platform tables. The Python client and pilot require Python 3.10+ and its standard library; no React adoption is required for an external application.

## Prepare one pilot

1. Apply platform migrations 48–50 and configure the dedicated introspection client described in [Identity contracts](../../docs/platform-identity.md). Configure SMTP and the existing notification-delivery/email workers if testing delivery.
2. In Keycloak, create the interactive client `pilot`. Enable authorization code flow and PKCE S256. Set an exact callback (`http://127.0.0.1:8081/callback` for local development), allowed origin, and post-logout URL (`http://127.0.0.1:8081/`). Disable password/direct-access grants. For a confidential client, provide its secret only to the backend example.
3. Give the interactive client the `identity:read` capability scope and platform audience mapper. Include its resource roles in introspection. Create `viewer` and `operator` client roles; assign the pilot user a role. These are application roles, not portal administrator roles. The sample uses its own `operator` role to authorize its send-test action.
4. Create a separate confidential service client `pilot-service`, enable service accounts, disable interactive/password flows, and assign no administrative roles. Configure the platform audience and `identity:read`, `notifications:write`, `notifications:read` capability scopes. Include scopes and preferred username in introspection. Do not expose its secret to a browser.
5. Register `pilot` in `/admin/rbac` as an external application. Set owner/support/environment metadata, integration type `external`, contract version `1`, and the example's public launch URL. Configure `viewer` and `operator` as enabled system roles and access roles. Configure/validate/approve/activate it.
6. Bind `pilot-service` to `pilot` with its actual Keycloak service-account subject using the identity administration endpoint. Grant only the three capability scopes above.
7. Export the configuration listed in [config.env.example](python/config.env.example) into the example process. Keep real secrets out of source control and shell command arguments. The scripts do not automatically load .env files.

Use HTTPS outside loopback development. For production, replace the single-process demonstration HTTP server and memory sessions with a maintained OIDC/web framework and shared encrypted server-side sessions. Never forward arbitrary client-supplied tokens through an unauthenticated authorization proxy.

## Verify the machine integration

From `examples/platform/python`:

```sh
python3 verify_integration.py
python3 verify_integration.py --check-workers
```

This is read-only: it checks issuer discovery, client credentials, platform identity and expected application binding. Tokens and secrets are never printed. To explicitly send one test message and verify worker progress:

```sh
python3 verify_integration.py --recipient your-test-mailbox@example.org --key pilot-test-001 --poll-seconds 60
```

Retry this exact command with the same key/payload to verify idempotency. Use a new key only for a new test message. Polling a pending message without a deadline is allowed, but a requested polling deadline that ends before SENT returns a nonzero exit code. SENT means the existing SMTP worker completed its send; it is not a read receipt.

## Run the external web pilot

```sh
python3 external_app.py
```

Open the configured public URL. The server listens on loopback; place a TLS reverse proxy in front when using a non-loopback URL. Add the pilot to the portal launcher and open it from there to exercise cross-application sign-in. The callback uses one-use state bound to an HttpOnly cookie and PKCE. Access tokens stay in opaque, short-lived backend sessions. The example consumes access tokens only through the platform's introspected access decision, never by decoding an ID token. It does not implement ID-token validation, refresh-token rotation or a production OIDC session manager.

Each protected page/action makes a fresh application access decision. The `operator` access role enables an explicit send-test form; viewer users cannot call that action. Origin and CSRF checks protect mutations. Sign-out clears the local session and redirects through the configured Keycloak end-session endpoint. Token expiry requires signing in again.

Suspend the application or revoke the machine binding and repeat the verifier to confirm denial. Re-activating a suspended registry application requires the documented validation/approval cycle. Restore only the grant you deliberately revoked. Existing accepted messages may still complete.

## Hosted module template

Copy `hosted-module/` to `frontend/apps/platform-pilot/`, then follow [Adding a new app](../../frontend/docs/adding-a-new-app.md) to register its import-map entry, shell lifecycle/route metadata, build scripts and package dependencies. Run `npm install --package-lock-only --ignore-scripts` after intentionally adding the workspace; keep package-lock changes with your actual module integration.

The template exposes the existing single-spa lifecycle and `MicrofrontendRoute` contract, receives shell auth/runtime props and checks system access through `@moh-sso/auth`. Its parent registry system is `pilot-hosted`, with integration type `hosted_module`, system type `platform` and launch URL `/apps/pilot-hosted`. Create/map its access roles and activate the parent system. Backend operations still require their own authorization; hiding a component is not enforcement. Keep machine secrets out of runtime props and frontend bundles.

The template is intentionally not installed in the production shell or registry. Rename the package, component, route and system ID together when using it for another application.

## Example tests

```sh
python3 -m unittest discover -s examples/platform/python -p 'test_*.py' -v
```

The tests use fake identity/platform responses and local HTTP servers. They verify contract paths, URL transport constraints, PKCE/state, replay rejection, server-only sessions, current access denial, Origin and CSRF enforcement. They do not create live Keycloak clients or send mail.
