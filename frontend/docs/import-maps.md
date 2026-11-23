# Import maps

Import maps prepare the shell for independently deployed microfrontends. This page describes the generated maps, how to enable remote mode, and how to audit, cache and roll back import-map deployments. It is for frontend developers and operators.

## Files

```text
frontend/public/import-map.json
frontend/public/import-map.local.json
```

`import-map.json` is the production-style map. `import-map.local.json` maps apps to local dev server ports.

Production Docker builds set `FRONTEND_ASSET_BASE_URL=/`, so the generated production map points to:

```text
/mf/<app>/single-spa.js
/packages/<package>/index.js
```

The staging step mirrors the final static artifact under `dist/` so these URLs resolve with the shell's `/` base path.

## Vendor dependencies

The production import map uses pinned ESM CDN URLs for shared platform dependencies:

- `react`
- `react-dom`
- `react-dom/client`
- `react-redux`
- `react-router-dom`
- `@carbon/react`
- `@carbon/react/icons`
- `@reduxjs/toolkit`
- `single-spa`
- `single-spa-react`

This avoids missing `/vendor/*.js` files and keeps React shared in remote mode.

## App and package entries

App bundles in the default non-versioned map:

```text
/mf/<app>/single-spa.js
```

In production Docker builds, these become `/mf/<app>/single-spa.js`.

The staged app folder also contains related chunks, CSS, route files, and declarations. Remote hosts must serve the whole `/mf/<app>/` folder, not only `single-spa.js`.

Shared packages:

```text
/packages/<package>/index.js
```

In production Docker builds, these become `/packages/<package>/index.js`.

The staged package folder may contain CSS or related files. Remote hosts should serve the whole `/packages/<package>/` folder.

## Versioned import map mode

Default mode writes local/static paths:

```text
/mf/users/single-spa.js
/packages/api/index.js
```

Versioned mode writes CDN/base URLs with package versions:

```bash
FRONTEND_ASSET_BASE_URL=https://cdn.example.com FRONTEND_VERSIONED_IMPORTS=true npm run generate:import-map
```

Example:

```json
{
  "imports": {
    "@moh-sso/users": "https://cdn.example.com/mf/users/0.4.1/single-spa.js",
    "@moh-sso/documents": "https://cdn.example.com/mf/documents/0.8.0/single-spa.js",
    "@moh-sso/api": "https://cdn.example.com/packages/api/0.2.0/index.js"
  }
}
```

Environment variables:

- `FRONTEND_VERSIONED_IMPORTS=true`
- `FRONTEND_ASSET_BASE_URL=https://cdn.example.com`

## Local import map

The local map points apps to dev servers:

- users: `http://localhost:4101/single-spa.js`
- clients: `http://localhost:4102/single-spa.js`
- announcements: `http://localhost:4103/single-spa.js`
- audit: `http://localhost:4104/single-spa.js`
- email: `http://localhost:4105/single-spa.js`
- documents: `http://localhost:4106/single-spa.js`
- surveillance: `http://localhost:4107/single-spa.js`
- data-visualizer: `http://localhost:4108/single-spa.js`
- issue-tracker: `http://localhost:4109/single-spa.js`
- report-browser: `http://localhost:4110/single-spa.js`
- e-services: `http://localhost:4111/single-spa.js`
- utilities: `http://localhost:4112/single-spa.js`

## Enable remote mode

The shell `index.html` loads the import map before the shell module. To enable remote/orchestrated mode:

1. Set `window.__APP_CONFIG__.singleSpaOrchestration = true`.
2. Set `window.__APP_CONFIG__.microfrontendMode = "remote"`.
3. Set `window.__APP_CONFIG__.microfrontendMountMode = "orchestrated"`.
4. Make sure the import map script is present before the shell module.

   Production builds keep `import-map.json` on disk and inline the same map into `dist/index.html`.
   This avoids relying on external import-map loading, which is not consistently supported by browsers.

   Production Docker uses `public/config.production.js`, which sets these runtime flags.

5. Optionally enable import-map-overrides:

```html
<script async src="https://unpkg.com/import-map-overrides@6.1.0/dist/import-map-overrides.js"></script>
```

See [templates/index.remote.html](templates/index.remote.html) for a remote-mode HTML template.

## Import map overrides

`import-map-overrides` can replace one deployed app with a local dev bundle. This is useful when testing a single app against deployed shell/config.

## Import map audit

After staging remote bundles, run:

```bash
npm run audit:import-map
```

This checks that bare imports in `dist/mf` and `dist/packages` are present in `public/import-map.json`.

## Cache busting

Use one of these strategies:

- versioned bundle URLs
- hashed deployment folders
- immutable CDN assets plus mutable import map

## CDN vs self-hosted vendors

The current map uses pinned CDN ESM URLs for simplicity. If an offline or fully self-hosted deployment is required, replace vendor entries with self-hosted files and ensure those files are served with the shell.

## Rollback

Rollback is import-map driven:

1. Point the affected app entry back to a previous versioned bundle URL.
2. Redeploy the import map or shell config.
3. Clear CDN/browser cache if required.
