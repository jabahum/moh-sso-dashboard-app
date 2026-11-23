# Adding a new app

Use this checklist when adding a new frontend app to the workspace. It is for frontend developers.

For the complete governed onboarding flow, screenshots, and the distinction
between a parent system and a frontend module, see the
[onboarding handbook](../../docs/platform-onboarding.md#8-add-a-hosted-single-spa-module).
Adding registry/navigation metadata alone does not install or mount a new app.

## Create files

```text
frontend/apps/<app>/src/root.component.tsx
frontend/apps/<app>/src/routes.tsx
frontend/apps/<app>/src/single-spa.tsx
frontend/apps/<app>/src/index.ts
frontend/apps/<app>/package.json
frontend/apps/<app>/vite.config.ts
```

## Exports

The app package should expose at least the following (existing apps such as `users` also export `./api` and `./types`):

```json
{
  "exports": {
    ".": {
      "types": "./dist/index.d.ts",
      "import": "./dist/index.js"
    },
    "./single-spa": {
      "types": "./dist/single-spa.d.ts",
      "import": "./dist/single-spa.js"
    },
    "./routes": {
      "types": "./dist/routes.d.ts",
      "import": "./dist/routes.js"
    }
  }
}
```

## App lifecycle

For React apps:

```ts
import { createReactMicrofrontendLifecycle } from "@moh-sso/microfrontend";
import { AppRoot } from "./root.component";

export const { bootstrap, mount, unmount } = createReactMicrofrontendLifecycle(AppRoot);
```

Non-React apps should export the same lifecycle functions directly.

## Route metadata

Add route ownership in `src/routes.tsx`:

```ts
import type { MicrofrontendRoute } from "@moh-sso/microfrontend";

export const exampleRoute: MicrofrontendRoute = {
  appName: "@moh-sso/example",
  path: "/apps/example",
};
```

Use `paths` when an app owns multiple activation paths.

## Register the app

Update:

- `frontend/package.json` scripts: `build:apps`, `dev:mf:<app>` and `preview:mf:<app>` (pick the next free port; `4115` is the highest in use)
- `frontend/scripts/generate-import-map.mjs` (`apps` list)
- `frontend/scripts/stage-microfrontends.mjs` (`apps` list; `stage-from-npm-packages.mjs` reuses it)
- `frontend/scripts/dev-microfrontends.mjs`
- `frontend/scripts/verify-production-runtime.mjs` and `frontend/scripts/architecture-check.mjs` (app lists)
- `frontend/public/import-map.local.json`
- both `versions` maps in `frontend/module-sources.json`: an npm-mode selection must pin exactly every publishable module, so the `production` entry (source `npm`) needs a published version of the new app
- shell microfrontend registry files
- shell lifecycle loader/orchestrator lookup and guarded user/admin routes
- shell `package.json` dependency when importing the new app
- `tsconfig.base.json` only if a new public path alias is needed

## Package metadata

Declare:

- direct `@moh-sso/*` imports as dependencies
- platform/runtime libraries as peer dependencies
- type-only dependencies as dev dependencies

Then run:

```bash
npm run audit:packages
```

## Verification

```bash
npm run audit:packages
npm run typecheck
npm run lint
npm run build -w @moh-sso/<app>
```

Also run the shell build if the app is registered in shell routes:

```bash
npm run build:shell
```
