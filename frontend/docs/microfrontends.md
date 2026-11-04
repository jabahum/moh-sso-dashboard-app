# Microfrontends

The frontend retains single-spa lifecycles. Local development defaults to hybrid
mode, where apps resolve through workspace imports. The checked-in production
configuration selects remote/import-map loading with single-spa orchestration.
See the [onboarding handbook](../../docs/platform-onboarding.md#8-add-a-hosted-single-spa-module)
for the complete integration and verification checklist.

## Current Modes

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

or equivalent runtime config:

```js
window.__APP_CONFIG__.singleSpaOrchestration = true;
window.__APP_CONFIG__.microfrontendMode = "remote";
window.__APP_CONFIG__.microfrontendMountMode = "orchestrated";
```

## App Contract

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

## Runtime Props

Apps accept runtime props from `@moh-sso/microfrontend`:

- `basename`
- `auth`
- `apiBaseUrl`
- `eventBus`
- `domElement` during mount

`basename` is important for nested routes inside each app.

## Route Ownership

Examples:

- users: `/admin/users`
- clients: `/admin/clients`
- announcements: `/admin/announcements`
- audit: `/admin/audit-logs`
- email: `/admin/emails`

Documents owns:

- `/apps/dwh/filesvr`
- `/apps/utilities/self-service/eservice/document-upload`
- `/apps/utilities/self-service/eservice/document-upload/:id`

Surveillance owns:

- `/apps/dwh/surveillance`
- `/apps/dwh/surveillance/:diseaseName`

Utilities owns:

- `/apps/utilities/self-service/timesheet`
- `/apps/utilities/self-service/elearning`
- `/apps/utilities/self-service/leave-plan`
- `/apps/utilities/self-service/absence-requests`
- `/apps/utilities/self-service/absence-dashboard`

## Shell Integration

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

- `registerApps.ts`: app route registry
- `routes.ts`: route constants
- `lifecycles.ts`: local lifecycle imports
- `containers.ts`: shell-owned DOM containers for orchestrated mode

## Launcher Entries And System Ownership

A deployable microfrontend and a Keycloak client are related but not identical concepts. One registered platform system may own several route-level microfrontends or modules. Its `ui.navigation` metadata can mark selected items with `displayInLauncher: true`, allowing those modules to appear as separate launcher cards without changing the system registry or Keycloak model.

Module launcher entries:

- use `<parentClientId>:<moduleId>` as a stable UI identity;
- retain the parent system as the active client for side navigation and authorization;
- inherit system metadata unless the item supplies its own icon, order, description, or launch mode;
- are filtered by the item's permission requirements;
- do not replace route guards or create a new client role boundary.

Existing systems are unchanged unless navigation metadata explicitly opts a module into the launcher.

## Local Vs Remote

Local mode uses workspace imports and local lifecycle modules.

Remote mode uses import maps. The shell loads `@moh-sso/<app>` bundle URLs from the import map.

For true `registerApplication` mode, the shell creates one DOM container per app under `#microfrontend-orchestrated-root`.

## Non-React Apps

Future Vue, Angular, Svelte, or plain JavaScript apps can participate by exposing the same lifecycle contract:

- `bootstrap`
- `mount`
- `unmount`

They should accept the same runtime props and avoid importing shell internals.
