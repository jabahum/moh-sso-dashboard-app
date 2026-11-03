# Adding A New App

Use this checklist when adding a new frontend app.

For the complete governed onboarding flow, screenshots, and the distinction
between a parent system and a frontend module, see the
[onboarding handbook](../../docs/platform-onboarding.md#8-add-a-hosted-single-spa-module).
Adding registry/navigation metadata alone does not install or mount a new app.

## Create Files

```text
frontend/apps/<app>/src/root.component.tsx
frontend/apps/<app>/src/routes.tsx
frontend/apps/<app>/src/single-spa.tsx
frontend/apps/<app>/src/index.ts
frontend/apps/<app>/package.json
frontend/apps/<app>/vite.config.ts
```

## Exports

The app package should expose:

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

## App Lifecycle

For React apps:

```ts
import { createReactMicrofrontendLifecycle } from "@moh-sso/microfrontend";
import { AppRoot } from "./root.component";

export const { bootstrap, mount, unmount } = createReactMicrofrontendLifecycle(AppRoot);
```

Non-React apps should export the same lifecycle functions directly.

## Route Metadata

Add route ownership in `src/routes.tsx`:

```ts
import type { MicrofrontendRoute } from "@moh-sso/microfrontend";

export const exampleRoute: MicrofrontendRoute = {
  appName: "@moh-sso/example",
  path: "/apps/example",
};
```

Use `paths` when an app owns multiple activation paths.

## Register The App

Update:

- `frontend/package.json` build/dev scripts
- `frontend/scripts/generate-import-map.mjs`
- `frontend/scripts/stage-microfrontends.mjs` (and npm-module staging when used)
- `frontend/scripts/dev-microfrontends.mjs`
- shell microfrontend registry files
- shell lifecycle loader/orchestrator lookup and guarded user/admin routes
- shell `package.json` dependency when importing the new app
- `tsconfig.base.json` only if a new public path alias is needed

## Package Metadata

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
