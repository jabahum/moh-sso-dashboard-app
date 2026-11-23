# Identity-sync adapter and conformance suite

This guide is for application teams implementing the
[identity-sync contract](identity-sync-contract.md) (v1.1). It covers:

- the Go adapter in `backend/core/identitysync/adapter`, which implements the
  whole HTTP side of the contract for a Go application;
- the conformance suite in `backend/core/identitysync/conformance`, which any
  application, in any language, runs against its test environment;
- the release steps for the `backend/core` module.

Both packages are part of the standard-library-only core module
`github.com/mohuganda/moh-sso-dashboard/backend/core`. Adding it to an
application pulls in no third-party code.

## Go applications: the adapter

The adapter is an `http.Handler`. You supply three things:

| You supply | What it does |
|---|---|
| `adapter.Store` | Lists eligible accounts with their current link, links an identity in one transaction, and ends every session of a linked account. |
| `adapter.TokenVerifier` | Verifies the bearer token's RS256 signature (and normally `exp`/`nbf`) with your own OIDC library, and returns the claims. |
| `adapter.Config` | Application client ID, public issuer, identity-sync client ID, account ID format and, in test environments only, the session probe. |

The adapter does the rest: the contract's authorization checks in order
(issuer, expiry, audience give **401**; `typ`, `azp` and the `identity_sync`
role give **403**), query and body validation, the 4 KiB request and 256 KiB
response limits, status codes, `Cache-Control: no-store` on every response and
the JSON error body. It never accepts cookies or API keys. It also refuses to
publish a page from your store that would break the contract (wrong order, an
ID in the wrong format, a link for another issuer) and returns 500 instead, so
a store bug can't send wrong data to the dashboard.

### Example

```go
import (
	"context"
	"net/http"
	"os"
	"time"

	"github.com/coreos/go-oidc/v3/oidc" // your app's choice; core does not import it

	"github.com/mohuganda/moh-sso-dashboard/backend/core/identitysync"
	"github.com/mohuganda/moh-sso-dashboard/backend/core/identitysync/adapter"
)

type keycloakVerifier struct{ v *oidc.IDTokenVerifier }

func (k keycloakVerifier) VerifyAccessToken(ctx context.Context, raw string) (adapter.Claims, error) {
	tok, err := k.v.Verify(ctx, raw) // RS256 signature, iss, exp
	if err != nil {
		return adapter.Claims{}, err // 401 (return adapter.ErrUnavailable for a JWKS outage: 503)
	}
	var c struct {
		Typ            string `json:"typ"`
		Azp            string `json:"azp"`
		Nbf            int64  `json:"nbf"`
		ResourceAccess map[string]struct {
			Roles []string `json:"roles"`
		} `json:"resource_access"`
	}
	if err := tok.Claims(&c); err != nil {
		return adapter.Claims{}, err
	}
	claims := adapter.Claims{
		Issuer: tok.Issuer, Audience: tok.Audience, AuthorizedParty: c.Azp, Type: c.Typ,
		ExpiresAt: tok.Expiry, ResourceAccess: map[string][]string{},
	}
	if c.Nbf != 0 {
		claims.NotBefore = time.Unix(c.Nbf, 0)
	}
	for client, access := range c.ResourceAccess {
		claims.ResourceAccess[client] = access.Roles
	}
	return claims, nil
}

func mountIdentitySync(mux *http.ServeMux, provider *oidc.Provider, store adapter.Store, probe adapter.SessionProbe) error {
	cfg := adapter.Config{
		Application:     "clims",
		Issuer:          "https://sso.health.go.ug/realms/moh", // the public issuer, exactly
		SyncClientID:    "clims-identity-sync",
		AccountIDFormat: identitysync.AccountIDUUID,
		Store:           store,
		Verifier: keycloakVerifier{provider.Verifier(&oidc.Config{
			SkipClientIDCheck:    true, // the adapter checks aud itself
			SupportedSigningAlgs: []string{oidc.RS256},
		})},
		ProbeEnabled: adapter.ProbeEnabledFromEnv(os.Getenv), // IDENTITY_SYNC_TEST_PROBE=1 in test only
		Probe:        probe,
	}
	h, err := adapter.New(cfg)
	if err != nil {
		return err
	}
	// Register both forms so neither redirects (the contract forbids redirects).
	mux.Handle(adapter.BasePath, h)
	mux.Handle(adapter.BasePath+"/", h)
	return nil
}
```

The handler accepts paths with or without the `/api/v1/internal/identity-sync`
prefix, so it also works behind `http.StripPrefix` or a router's wildcard
route. With another router (Gin, Chi, Echo), route both the base path and
everything under it to the handler, and turn off trailing-slash redirects for
these routes.

### Implementing the Store

- **`ListEligibleAccounts(ctx, cursor, limit)`**: return at most `limit`
  accounts that may sign in today (active, not locked, at least one local role,
  not a service account), in ascending account-ID order, starting after
  `cursor`. Set each account's `Link` to its link for the configured issuer, or
  leave it zero. Return an empty `NextCursor` only on the last page. Use the
  last ID as the cursor, or any opaque value matching `^[A-Za-z0-9._:~-]{1,256}$`.
  Return `adapter.ErrInvalidCursor` for a cursor you cannot resume from.
  Positive-integer IDs are ordered numerically, others bytewise.
- **`LinkIdentity(ctx, accountID, identity)`**: in one transaction, check
  eligibility and both uniqueness rules (one subject per account and issuer,
  one account per issuer and subject). Return `nil` when linked now or already
  linked to exactly this identity, `identitysync.ErrAccountNotFound` for a
  missing or ineligible account, and `identitysync.ErrLinkConflict` otherwise.
  Never replace a link, never grant roles. A unique index on
  `(issuer, subject)` and on `(account_id, issuer)` makes this safe under
  concurrency.
- **`RevokeSessions(ctx, identity)`**: delete every local session and refresh
  token of the linked account before returning, so the next request with any
  of them fails. Return `nil` when the identity is not linked. Keep the link
  and roles.

Return `adapter.ErrRateLimited` (429) or `adapter.ErrUnavailable` (503) for
overload; both set `Retry-After`. Any other error becomes a 500 and is passed
to `Config.OnError`. Error details are never sent to the client.

`adapter/memory` is a complete in-memory Store and SessionProbe. Use it in
your own tests as a reference for the rules above.

### The test-only session probe

Implement `adapter.SessionProbe` so the conformance suite can prove that
revocation ends sessions:

- `CreateTestSession` creates a real session through the same code path your
  OIDC callback uses after a successful sign-in and returns an opaque token
  for it.
- `CheckTestSession` answers exactly what your session middleware would
  decide for that session.

The routes are mounted only when `Config.ProbeEnabled` is true. Set it only
from `adapter.ProbeEnabledFromEnv(os.Getenv)`, and never set
`IDENTITY_SYNC_TEST_PROBE=1` outside test environments. The routes are
specified in the contract appendix
[Test-only session probe](identity-sync-contract.md#appendix-test-only-session-probe).

## Any language: running the conformance suite

The suite talks to your application over HTTPS only, so it works the same for
Go, Java, Node, Python or PHP applications. Run it in CI against a disposable
deployment seeded with synthetic accounts. **It creates links** for the two
linkable fixture accounts, so reset the data before each run.

### 1. Prepare the environment

- Seed at least two eligible, unlinked accounts and one account that exists
  but is ineligible (for example disabled). More eligible accounts make the
  pagination walks (limit 1, 7 and 100) more meaningful; use at least 8.
- Start the application with `IDENTITY_SYNC_TEST_PROBE=1` and implement the
  probe routes from the contract appendix.
- In the test realm, create the clients used to mint tokens (see step 3).

### 2. Write a fixture file

```json
{
  "application": "clims",
  "issuer": "https://sso.test.example/realms/moh",
  "linkableAccounts": ["0b6f…", "4c2e…"],
  "ineligibleAccount": "9d71…",
  "subjects": ["synthetic-sub-1", "synthetic-sub-2", "synthetic-sub-3"]
}
```

`subjects` is optional. Without it the suite generates three random synthetic
subjects. They never need to exist in Keycloak, because the application only
stores them.

### 3. Provide tokens through environment variables

| Variable | Token | Expected |
|---|---|---|
| `IDENTITY_SYNC_TOKEN` (required) | client-credentials token of `<app>-identity-sync` | accepted |
| `IDENTITY_SYNC_EXPIRED_TOKEN` | an expired token, or any unverifiable string | 401 |
| `IDENTITY_SYNC_WRONG_AUDIENCE_TOKEN` | a realm token whose `aud` lacks the application | 401 |
| `IDENTITY_SYNC_USER_TOKEN` | a test user's access token for the application | 403 |
| `IDENTITY_SYNC_OTHER_CLIENT_TOKEN` | another service client's token with the application in `aud` | 403 |
| `IDENTITY_SYNC_MISSING_ROLE_TOKEN` | a sync-style client token without the `identity_sync` role | 403 |

Any optional variable left unset makes its check report **skip**, never pass.
A run that should prove full conformance must set all six. The flags
`--sync-token-env` and the like choose other variable names.

### 4. Run it

```sh
moh-sso identity-sync conformance \
  --origin https://clims.test.example \
  --fixtures conformance-fixtures.json \
  --probe \
  --ca-file /etc/test-ca/root.pem   # only for a private test CA
```

Until the `identity-sync` parent command is registered, Go users can call
`conformance.Run` directly. The CLI is a thin wrapper around it.

Each line shows `PASS`, `FAIL` or `SKIP`, a stable check ID and a detail. The
command exits non-zero when any check fails. `--json` prints a
machine-readable report. A skip always says why: a missing token, the probe
not enabled, or a failed earlier check that it depends on (for example
`blocked by linking.link`).

### What is checked

| Area | Check IDs |
|---|---|
| Descriptor | `descriptor.shape`, `descriptor.application`, `descriptor.capabilities` |
| Authorization | `authorization.no-token`, `.non-bearer`, `.garbage-token`, `.expired-token`, `.wrong-audience`, `.user-token`, `.other-client`, `.missing-role`, `.sync-token`, on every endpoint, including probe routes when enabled |
| Pagination | `pagination.walk-limit-1`, `-7`, `-100` (each account exactly once, strictly ascending, ending on an empty cursor), `pagination.consistent`, `pagination.fixtures`, `pagination.bad-cursor`, `pagination.bad-limit` |
| Linking | `linking.link`, `.repeat`, `.other-subject` (409), `.subject-taken` (409), `.ineligible` (404), `.wrong-issuer` (400), `.malformed` (400), `.visible` (the new link is listed and refused requests changed nothing) |
| Sessions | `sessions.supported`, `.probe-create`, `.probe-active`, `.probe-unlinked`, `.revoke`, `.revoked-session-rejected`, `.revoke-repeat`, `.revoke-unknown`, `.wrong-issuer`, `.link-kept` |
| Retries | `retries.page-replay` (a page read again with the same cursor is identical), `.link-concurrent` (5 identical link requests at once all return 204 and leave one link: a retry racing its original must not conflict), `.revoke-concurrent` (5 identical revokes at once all return 204), `.retry-after` (every 429 or 503 carries `Retry-After`). Retryable answers are retried as the dashboard would. |
| Transport | `transport.request-body-limit`, `.no-store`, `.no-redirects`, `.response-size`, `.error-bodies`, `.https` |

`sessions.supported` fails when the descriptor lacks `sessions.revoke`,
because access removal must end an application's sessions immediately.

## Releasing `backend/core`

The module is versioned with tags prefixed `backend/core/`. It stays below
1.0 while the contract settles, so a minor version may contain breaking
changes. These steps are documentation only. A maintainer performs them after
the change is merged.

1. On the merged `main` commit, check that the module is still
   standard-library only and green:

   ```sh
   python3 infra/scripts/test-platform-core.py
   (cd backend/core && go test -race -count=1 ./...)
   docker run --rm -v "$PWD/backend/core/identitysync/contract":/spec:ro \
     redocly/cli:1.25.11 lint /spec/identity-sync.openapi.yaml
   ```

2. Check that `backend/core/go.mod` still declares
   `module github.com/mohuganda/moh-sso-dashboard/backend/core` with no
   `require` lines, and that `backend/core/README.md` describes the release.
3. Create an annotated tag on that commit and push only the tag:

   ```sh
   git tag -a backend/core/v0.1.0 -m "backend/core v0.1.0: identity-sync contract 1.1 types, adapter and conformance suite"
   git push origin backend/core/v0.1.0
   ```

4. Check resolution from a scratch module outside the repository:

   ```sh
   go mod init example.test/check && \
   go get github.com/mohuganda/moh-sso-dashboard/backend/core@v0.1.0
   ```

   If the repository is private, set
   `GOPRIVATE=github.com/mohuganda/*` for consumers.

5. Applications then depend on `backend/core@v0.1.0`. The dashboard keeps its
   local `replace ./core` so it always builds against the same tree.
