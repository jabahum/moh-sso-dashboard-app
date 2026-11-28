# Microfrontends

This page describes how frontend apps run as single-spa microfrontends: the loading modes, the app contract, route ownership, shell integration and the root page layout. It is for frontend developers.

The frontend retains single-spa lifecycles. Local development defaults to hybrid
mode, where apps resolve through workspace imports. The checked-in production
configuration selects remote/import-map loading with single-spa orchestration.
See the [onboarding handbook](../../docs/platform-onboarding.md#8-add-a-hosted-single-spa-module)
for the complete integration and verification checklist.

## Current modes

- Hybrid local mode: default. Shell uses local workspace app lifecycles.
- Remote mode: enabled by runtime/env config. Shell can load app bundles through import maps.
- Orchestrated mode: explicitly gated by runtime configuration and enabled in
  `public/config.production.js`. React Router retains shell route guards/layout;
  `SingleSpaApp` coordinates with the orchestrator instead of mounting locally
  when orchestration starts successfully. Failure to load single-spa permits the
  local lifecycle fallback; investigate that state rather than treating a visible
  screen alone as proof that orchestration ran.

Hybrid mode is the safe default. In hybrid mode, React Router renders `SingleSpaApp` wrappers and `startMicrofrontendOrchestration()` does not call `registerApplication`.

Full orchestration requires:

```text
VITE_SINGLE_SPA_ORCHESTRATION=true
VITE_MICROFRONTEND_MODE=remote
VITE_MICROFRONTEND_MOUNT_MODE=orchestrated
```

or equivalent runtime config (which takes precedence over the `VITE_*` variables when present; every checked-in `public/config*.js` sets all three keys):

```js
window.__APP_CONFIG__.singleSpaOrchestration = true;
window.__APP_CONFIG__.microfrontendMode = "remote";
window.__APP_CONFIG__.microfrontendMountMode = "orchestrated";
```

## Local vs remote

Local mode uses workspace imports and local lifecycle modules.

Remote mode uses import maps. The shell loads `@moh-sso/<app>` bundle URLs from the import map.

For true `registerApplication` mode, the shell creates one DOM container per app with the id `single-spa-application-<app>` (`containers.ts`) and attaches it inside the content area of the route that hosts the app.

## App contract

Every standalone app exposes:

```text
src/root.component.tsx
src/routes.tsx
src/single-spa.tsx
```

Every app lifecycle exports:

```ts
export async function bootstrap(props) {}
export async function mount(props) {}
export async function unmount(props) {}
```

React apps use `createReactMicrofrontendLifecycle` from `@moh-sso/microfrontend`.

## Runtime props

Apps accept runtime props from `@moh-sso/microfrontend`:

- `basename`
- `auth`
- `apiBaseUrl`
- `eventBus`
- `healthContext`
- `domElement` during mount (`MicrofrontendMountProps`)

`basename` is important for nested routes inside each app.

## Route ownership

Examples:

- users: `/admin/users`
- clients: `/admin/clients`
- announcements: `/admin/announcements`
- audit: `/admin/audit-logs`
- email: `/admin/emails`
- rbac: `/admin/rbac`
- data-visualizer: `/apps/dwh/data-visualizer`
- data-validation: `/apps/dwh/data-validation`
- report-browser: `/apps/dwh/dashboards`
- report-scheduler: `/apps/dwh/report-scheduler` (`/apps/report-scheduler` redirects there)
- issue-tracker: `/apps/dwh/issue-tracker`
- e-services: `/apps/eservices`

Documents owns:

- `/apps/dwh/documents`
- `/apps/dwh/documents/:id`
- `/apps/utilities/self-service/eservice/document-upload`
- `/apps/utilities/self-service/eservice/document-upload/:id`

The documents root still accepts the legacy `/apps/dwh/filesvr` basename, but the shell no longer routes that path.

Surveillance owns:

- `/apps/dwh/surveillance`
- `/apps/dwh/surveillance/:diseaseName`

Utilities owns:

- `/apps/utilities/self-service/timesheet`
- `/apps/utilities/self-service/elearning`
- `/apps/utilities/self-service/leave-plan`
- `/apps/utilities/self-service/absence-requests`
- `/apps/utilities/self-service/absence-dashboard`
- `/apps/utilities/self-service/eservice/service-access`
- `/apps/utilities/self-service/eservice/equipment-request`

The shell routes are in `apps/shell/src/app/routes/admin.routes.tsx` and `user.routes.tsx`; base paths are listed in `apps/shell/src/app/microfrontends/routes.ts`.

## Shell integration

`SingleSpaApp` lives at:

```text
frontend/apps/shell/src/app/microfrontends/SingleSpaApp.tsx
```

It mounts lifecycle objects inside shell-owned React Router routes.

The orchestrator lives at:

```text
frontend/apps/shell/src/app/microfrontends/orchestrator.ts
```

Related files:

- `registry.ts`: app route registry (`MicrofrontendRoute` objects with required permissions and systems)
- `registerApps.ts`: re-exports the registry for orchestration
- `routes.ts`: route constants
- `lifecycles.ts`: local lifecycle imports
- `containers.ts`: shell-owned DOM containers for orchestrated mode

## Launcher entries and system ownership

A deployable microfrontend and a Keycloak client are related but not identical concepts. One registered platform system may own several route-level microfrontends or modules. Its `ui.navigation` metadata can mark selected items with `displayInLauncher: true`, allowing those modules to appear as separate launcher cards without changing the system registry or Keycloak model.

Module launcher entries:

- use `<parentClientId>:<moduleId>` as a stable UI identity;
- retain the parent system as the active client for side navigation and authorization;
- inherit system metadata unless the item supplies its own icon, order, description, or launch mode;
- are filtered by the item's permission requirements;
- do not replace route guards or create a new client role boundary.

Existing systems are unchanged unless navigation metadata explicitly opts a module into the launcher.

## Non-React apps

Future Vue, Angular, Svelte, or plain JavaScript apps can participate by exposing the same lifecycle contract:

- `bootstrap`
- `mount`
- `unmount`

They should accept the same runtime props and avoid importing shell internals.

## Root page layout

Root screens use `PageShell` from `@moh-sso/ui` for a single page title,
description, divider and optional header actions. Keep routing and permissions
at their existing application boundaries; render route-specific headers inside
multi-page applications instead of adding a second module heading.

All 15 application roots follow this pattern:

| Applications | Header owner |
| --- | --- |
| Users, Clients, Audit Logs | Shared `DataTableShell`, which uses `PageShell` |
| Announcements, Email Outbox, RBAC Management | Their root page component |
| Data Visualizer, Data Validation, Report Browser | Application root |
| Document Management, Issue Tracker, Report Scheduler, Surveillance | Their workspace/root page |
| E-services | `EservicePageShell` and individual placeholder routes |
| Utilities | Route-specific `UtilityPage` |

The portal layout supplies outer page padding. Page components avoid adding a
second outer gutter. `PageShell` spaces its content sections and keeps header
actions responsive. Dedicated detail-page and access-denied headings remain
local to those screens.

Until its first data load finishes, an app that loads data on start returns
`<PageLoading>` from `@moh-sso/ui` instead of its page, so the page and header
appear once, complete. Gate on the query's `isLoading`. If a later query (a new
filter, for example) starts its own first load, wrap the flag in
`useFirstLoad(isLoading)` so the page stays mounted. Apps that load nothing on
start render their page immediately.

`tests/browser/root-pages.spec.mjs` visits every application root at desktop and
mobile widths and checks for a single visible header aligned with the portal's
content gutter. It uses disposable local API responses and a report iframe stub.
