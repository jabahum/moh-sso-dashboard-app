# Cross-application SSO

This page explains how other applications launch the Integrated Health Portal with an authenticated session, for developers of launching applications and the operators who configure the portal. The portal uses Keycloak for identity and a backend-managed `sso_session` cookie for its application session. Opening `/` directly is intentionally public and does not create that backend session.

## Production route ownership

The Integrated Health Portal has its own origin, `portals.health.go.ug`. Report
Browser stays on `dashboards.health.go.ug`:

| URL | Owner |
| --- | --- |
| `https://portals.health.go.ug/` and other paths not listed here | Integrated Health Portal frontend |
| `https://portals.health.go.ug/ssobackend/*` | Integrated Health Portal backend |
| `https://portals.health.go.ug/portal/*` | 308 redirect to the same path without `/portal` (legacy links) |
| `https://dashboards.health.go.ug/` and `/report-browser/*` | Report Browser |
| `https://dashboards.health.go.ug/portal/*` and `/ssobackend/*` | 308 redirect to `portals.health.go.ug` (legacy links) |

Portal links are root-relative paths such as `/apps/news`. Portal authentication
links must begin with `/ssobackend` after runtime resolution.

## Launch contract

Applications that launch the portal and need an authenticated destination must use the registry's `authenticatedLaunchUrl`:

```text
/api/v1/auth/launch?returnTo=/apps/news
```

The ordinary `launchUrl` remains `/`. The launcher resolves the authenticated backend path against its runtime API base. In production, an API base of `https://portals.health.go.ug/ssobackend` therefore produces:

```text
https://portals.health.go.ug/ssobackend/api/v1/auth/launch?returnTo=%2Fapps%2Fnews
```

The backend redirects to Keycloak. An existing Keycloak SSO session is reused, the fixed callback creates the portal `sso_session`, and the browser returns to the validated portal route. Tokens, authorization codes, and session identifiers are never added to launcher URLs.

An external launcher must not navigate directly to the registry value
`/api/v1/auth/launch?...` against its own origin. From Report Browser that requests
`https://dashboards.health.go.ug/api/v1/...`, which is not a portal route.
It must resolve the value against the portal API base first, producing the
`/ssobackend/api/v1/...` URL shown above.

## Registry metadata

The `dashboard-web` system should define:

```yaml
launchUrl: /
authenticatedLaunchUrl: /api/v1/auth/launch?returnTo=%2Fapps%2Fnews
```

The equivalent Keycloak client attributes are `ui.launchUrl` and `ui.authenticatedLaunchUrl`. Portal administrators can edit both values through Systems or RBAC Management. Report Browser and other external launchers should consume this metadata instead of hardcoding portal paths.

## Redirect security

`returnTo` accepts root-relative portal paths (legacy `/portal/...` paths are normalized) and optional query strings or fragments. Absolute URLs require an exact origin in `AUTH_RETURN_URL_ALLOWED_ORIGINS`. The backend rejects foreign origins, credentials, unsupported schemes, scheme-relative URLs, control characters, encoded separators, double encoding, and dot-segment traversal. The target is bound to OAuth state in a short-lived HttpOnly, SameSite=Lax cookie and is cleared after success or terminal failure.

HTTP `Referer` is never used as a redirect target.

## Environment configuration

Development:

```env
FRONTEND_BASE_URL=http://localhost:3000/
AUTH_RETURN_URL_ALLOWED_ORIGINS=http://localhost:3000
KEYCLOAK_REDIRECT_URI=http://localhost:9000/api/v1/auth/callback
```

Production:

```env
FRONTEND_BASE_URL=https://portals.health.go.ug/
AUTH_RETURN_URL_ALLOWED_ORIGINS=https://portals.health.go.ug
KEYCLOAK_REDIRECT_URI=https://portals.health.go.ug/ssobackend/api/v1/auth/callback
API_BASE_URL=/ssobackend
```

The portal and the launching application must use the same Keycloak realm and browser-visible Keycloak origin. Configure only the exact callback URI. Avoid broad redirect wildcards.

## Verification

1. Sign into Report Browser through Keycloak.
2. Launch Integrated Health Portal using `authenticatedLaunchUrl`.
3. Confirm Keycloak does not ask for credentials again.
4. Confirm the browser arrives at `/apps/news`.
5. Confirm `/ssobackend/api/v1/auth/me` returns the authenticated user.
6. Confirm portal menus reflect that user's effective RBAC permissions.

## Troubleshooting

If the public News page appears unauthenticated, inspect the launcher URL first. A direct `/` link bypasses portal session creation by design.

If the browser returns to Report Browser instead, inspect the final path. Only `/report-browser`
belongs to Report Browser; `/` and other portal paths are portal frontend
destinations.
