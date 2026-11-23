# Progressive Web App

The MOH Integrated Health Portal is installable as a Progressive Web App from the shell app. The shell owns the service worker and manifest so individual microfrontend apps do not register competing workers. This page describes the PWA scope, caching rules, install and offline behaviour, and how to verify and troubleshoot it. It is for frontend developers and operators.

## Scope

The PWA is scoped to:

```text
/portal/
```

Key assets:

```text
public/manifest.webmanifest
public/offline.html
public/icons/*
apps/shell/src/app/pwa/PwaLifecycle.tsx
vite.config.ts
```

## What is cached

The service worker (`vite-plugin-pwa` in `vite.config.ts`) precaches the shell build files matching `**/*.{js,css,html,ico,png,svg,webmanifest,json}` plus the public files listed in `includeAssets` (which include `config.js`, `import-map.json` and `version-manifest.json`), and uses runtime caching for:

- `/portal/assets/*` script requests: stale-while-revalidate (cache `portal-static-assets`, 80 entries, 30 days).
- `/portal/mf/*`: network-first for staged microfrontend bundles (cache `portal-microfrontends`, 5-second network timeout, 120 entries, 1 day).
- `/portal/packages/*`: network-first for staged shared package bundles (same cache).

Microfrontend and package bundles use a short runtime cache because import maps can change independently from the shell.

## What is not cached

Do not cache authenticated or runtime-sensitive resources. Navigation fallback to `/portal/index.html` is disabled for `/api/`, `/realms/`, `/auth/` and `/visualizer-proxy/` (`/visualizer_proxy/`):

- `/api/*`
- `/api/v1/auth/*`
- Keycloak realm/auth URLs
- cookie/session endpoints
- authenticated JSON responses

The nginx production config also avoids stale runtime configuration:

```text
/portal/config.js
/portal/import-map.json
/portal/version-manifest.json
/portal/sw.js
/portal/workbox-*.js
```

`infra/docker/frontend/nginx.conf` sends `no-store` for HTML, `config*.js`, `import-map*.json` and `version-manifest.json`, and `no-cache` for `sw.js` and `workbox-*.js`. These must stay `no-store` or `no-cache` so production updates can roll forward and back cleanly.

## Installation UX

The shell listens for `beforeinstallprompt` and shows a subtle toast action to install the portal. Browsers that do not support this event can still install the site through browser menu options.

When a new service worker is available, the shell shows a toast:

```text
A new portal version is available
```

Selecting `Refresh` activates the new worker and reloads the page.

## Offline behavior

The app includes:

```text
/portal/offline.html
```

This is a fallback page for disconnected sessions. Authenticated workflows, live dashboards, API calls, Keycloak login, and remote systems still require network access.

## Local development

PWA service worker registration is disabled in Vite development mode. This avoids stale development files and keeps local `/portal` behavior predictable.

Use:

```bash
cd frontend
source ~/.nvm/nvm.sh
nvm use 22
npm run dev:shell
```

If a browser was previously used against a production build, clear the old worker before testing dev:

1. Open DevTools.
2. Go to Application.
3. Unregister service workers for `localhost`.
4. Clear site data.
5. Hard refresh `/portal/`.

## Production verification

Build:

```bash
cd frontend
source ~/.nvm/nvm.sh
nvm use 22
npm run config:prod
FRONTEND_ASSET_BASE_URL=/portal FRONTEND_BASE_PATH=/portal npm run build:docker
```

Verify files exist:

```text
dist/index.html
dist/manifest.webmanifest
dist/offline.html
dist/sw.js
dist/workbox-*.js
dist/icons/icon-192.png
dist/icons/icon-512.png
dist/icons/maskable-192.png
dist/icons/maskable-512.png
```

Browser checks:

1. Open `/portal/`.
2. DevTools → Application → Manifest.
3. Confirm the manifest is valid and scoped to `/portal/`.
4. Confirm one service worker is registered.
5. Confirm `/api/*` requests are not served from service worker cache.
6. Confirm `/portal/config.js` and `/portal/import-map.json` are not stale cached.
7. Switch offline and confirm the fallback behaves cleanly.

## Troubleshooting

If the portal is blank with no obvious console error:

1. Unregister the service worker.
2. Clear site data.
3. Reload `/portal/`.
4. Confirm `/portal/config.js` returns the expected environment config.
5. Confirm `/portal/import-map.json` matches the deployed microfrontend bundles.
6. Confirm `/portal/assets/*`, `/portal/mf/*`, and `/portal/packages/*` return `200`.

Never debug stale production behavior without checking the service worker and import-map cache first.
