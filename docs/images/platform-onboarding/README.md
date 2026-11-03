# Onboarding screenshot index

These are actual, unedited screenshots captured from the local development portal
on 3 November 2026, while reviewing `platform/phase-1-application-registry` after
merge commit `4c223829`. The footer displays the inherited frontend `1.1.0`
version and backend `dev`; it does not mean the platform additions were already
in the published `1.1.0` release. No production system was used.

| File                                                                 | Screen and handbook location                   | Capture state                                                           |
| -------------------------------------------------------------------- | ---------------------------------------------- | ----------------------------------------------------------------------- |
| [01-registry-and-lifecycle.jpg](01-registry-and-lifecycle.jpg)       | RBAC → existing pilot; section 6               | Existing active local fixture; no transition submitted                  |
| [02-external-registration.jpg](02-external-registration.jpg)         | RBAC → Register application; section 4         | Unsaved `pilot` example using reserved example addresses                |
| [03-launch-and-visibility.jpg](03-launch-and-visibility.jpg)         | Same form, lower fields; section 4             | Unsaved external/new-tab example, side navigation off, empty navigation |
| [04-access-and-system-roles.jpg](04-access-and-system-roles.jpg)     | RBAC → existing pilot roles; section 5         | Existing viewer/operator roles; no permissions changed                  |
| [05-module-navigation-preview.jpg](05-module-navigation-preview.jpg) | RBAC → Data & Statistics navigation; section 9 | Unsaved simplified JSON for readability, discarded afterward            |
| [06-identity-client-inventory.jpg](06-identity-client-inventory.jpg) | Administration → Clients; section 3            | Read-only local identity-client inventory; not the Keycloak console     |

## Refresh procedure

1. Run the local development portal with the current platform code and disposable
   test data. Use the existing Make targets documented in `docs/make.md`.
2. Sign in with an authorized local administrator. Avoid screenshots of real user
   lists, message bodies, patient data, credentials, token URLs or secret settings.
3. Use the actual UI and labels. Fill only unsaved forms when illustrating a new
   example; do not activate systems, grant access, or submit lifecycle changes
   just to make an image. Existing local fixtures may be inspected read-only.
4. Capture the relevant panel with readable text and no unrelated browser tabs.
   Keep the image truthful; do not fabricate success or approval states.
5. Inspect each saved image, update the adjacent caption and this index, and
   verify its relative Markdown link. Refresh any stale field values or labels.
6. Discard all unsaved drafts and restore the original browser display afterward.

Keycloak admin-console labels depend on the installed version. The handbook links
to the official administration guide for that console rather than presenting
the portal's Clients page as a Keycloak configuration screenshot.
