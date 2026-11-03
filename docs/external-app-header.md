# Shared MOH header for external applications

External applications can keep their own domain, backend, deployment and OIDC
session while displaying the same MOH header as the portal. The header reuses
the portal's `UserHeader`, coat of arms, Carbon actions and application switcher.
Set `appName` to the application's display name: for example, **MOH Logistics
Management** replaces **MOH Integrated Health Portal**. Supply the name without
the `MOH` prefix, which is added by the component.

This is an opt-in integration in each external application's frontend. Registering
an application in the portal does not insert a header into its website. This
implementation supports React applications using the shared UI package; Angular,
Vue and server-rendered applications without React will need a separate adapter.
Hosted single-spa modules continue to use the shell's header and must not render
another one.

## Install and render

Use a released `@moh-sso/ui` version containing `ExternalAppHeader` and its
`styles.css` export. These changes must be released before external projects can
install them from the package registry; version 1.1.0 does not contain this API.
For local development, build the UI workspace and use a locally packed package.
Use the package's declared React, Carbon and router peer dependency versions.

Import Carbon's global stylesheet once, followed by the shared UI stylesheet:

```tsx
import "@carbon/styles/css/styles.css";
import "@moh-sso/ui/styles.css";
import { Content } from "@carbon/react";
import { ExternalAppHeader, MohThemeProvider } from "@moh-sso/ui";

export function App({ session, permittedApplications, logout }) {
  return (
    <MohThemeProvider>
      <ExternalAppHeader
        appName="Logistics Management"
        username={session.username}
        portalUrl="https://dashboards.health.go.ug/portal/apps"
        applications={permittedApplications}
        onProfile={() => window.location.assign("/account")}
        onLogout={logout}
      />
      <Content id="main-content">
        {/* The external application's existing routes/pages go here. */}
      </Content>
    </MohThemeProvider>
  );
}
```

`Content` reserves space below Carbon's fixed header. Keep exactly one
`main-content` target so the skip link works. If your layout already reserves
header space, use its existing main element with that ID instead.

The authenticated application's own backend supplies a launch list such as:

```json
[
  {
    "id": "reports",
    "name": "Health Reports",
    "href": "https://reports.example.org"
  }
]
```

Use absolute launch URLs because relative URLs resolve against the external app's
origin. The component omits non-HTTP(S) links and credential-bearing URLs. Launch
links open in the current tab; each destination performs its normal OIDC flow,
which can reuse the Keycloak sign-in session. The app switcher also always includes
an **All portal applications** link to the configured portal launcher. Use this
link as the initial integration when you do not yet have a per-user launch list.
The application-name link returns to that same launcher.

## Session and access ownership

- `username` comes from the application's verified session, not an arbitrary URL
  parameter.
- `applications` contains only links approved for the current user. The header
  does not fetch the portal's cookie-authenticated APIs across origins, and does
  not currently provide automatic registry synchronization. To keep a shared
  launch list current, implement a backend integration with a supported user
  authorization contract; do not expose a global administrative catalog.
- Use `isLoading`, `isError` and `onRetry` while fetching that list. Keep the
  loading/error state separate from a successfully loaded empty list.
- `onProfile` opens the external application's profile/account screen.
- `onLogout` must end the local session and initiate that application's configured
  OIDC logout flow. The component deliberately delegates this action; redirecting
  only to the portal's logout endpoint cannot clear the external app's session.
- Never place access tokens or client secrets in application links or header
  props. UI visibility does not replace backend authorization.

## Preview and acceptance

From `frontend`, run `npm run ui:ladle` and open **Shell / External application
header / Logistics**. This preview uses example links; its profile/logout actions
show status text rather than changing any session.

Before onboarding an application, verify its configured title, narrow-screen
layout, keyboard access to the switcher (including Escape and focus return),
skip-to-content, profile action and complete logout flow. Confirm users see only
their permitted launch links and that switching apps reuses SSO correctly. Test
with both permitted and denied users. Install a tested package version in each
app, and upgrade it deliberately when the shared header changes.

See the [onboarding handbook](platform-onboarding.md) for identity, registration,
roles and application lifecycle setup.
