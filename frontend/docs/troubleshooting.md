# Troubleshooting

## Dev Login Opens External Keycloak

The local stack should navigate from `http://localhost:3000/portal/` to
`http://localhost:9000/api/v1/auth/login`, then to Keycloak at
`http://localhost:8081/realms/moh-realm/`.

Check that `/portal/config.js` loads successfully and that
`window.__APP_CONFIG__.API_BASE_URL` is `http://localhost:9000`. A missing runtime
config makes the frontend use its fallback API URL. In the shell HTML, keep the
script source as `/config.js`: Vite adds the `/portal/` base itself. Using
`%BASE_URL%config.js` produced `/portal/portal/config.js` in development, returning
404 and sending login to the production API instead.

Reload the portal after correcting the HTML; use a hard refresh if an older tab
still loads stale assets. Run `node --test scripts/runtime-config-path.test.mjs`
from `frontend/` to verify the script URL in development and production output.

## Vite Warns About Node Version

Vite 7 expects Node `20.19+` or `22.12+`.

Fix:

```bash
node --version
nvm install 20
nvm use 20
```

Switch to Node 20.19+ or 22.12+.

If the `20` alias is unavailable in your local `nvm`, use an exact installed version such as:

```bash
nvm use v20.20.1
```

## Failed To Resolve `single-spa-react`

Fix:

```bash
npm install
```

The React lifecycle helper has a manual fallback, but the package should still be installed for normal single-spa React behavior.

## `npm ci` Fails

Cause: `package.json` and `package-lock.json` are out of sync.

Fix:

```bash
cd frontend
npm install
```

Commit the updated lockfile.

## Docker Build Fails At `npm ci`

Same cause as local `npm ci` failure.

Fix:

```bash
cd frontend
npm install
```

Commit `package-lock.json`, then rerun Docker.

## App Build Fails Due Missing Dependency

Run:

```bash
npm run audit:packages
```

Add the missing dependency or peer dependency to the app/package manifest.

## Remote Mode Loads A Blank App

Check:

- import map URL
- app `single-spa.js` is served
- related chunks such as `root.component-*.js` are served
- app `basename`
- browser console
- network tab for failed module requests

Run:

```bash
npm run audit:import-map
```

## Local Dev Looks Like Production Mode

Cause: `public/config.js` may still contain production or local-remote settings.

Fix:

```bash
npm run config:dev
```

Then restart the dev server.

## Duplicate React Or Invalid Hook Call

Likely cause: remote bundles are loading more than one React instance.

Fix:

- keep React as a peer dependency
- share React through the import map
- avoid bundling duplicate React into remote apps

## Route Works In Shell But Fails In Remote App

Check:

- app root `basename`
- nested app routes
- shell route path
- `MicrofrontendRuntimeProps.basename`

## Docker Dev Only Starts Shell

Default Docker dev runs shell-only mode.

Use:

```bash
FRONTEND_DEV_COMMAND="npm run dev:remote:all" docker compose -f infra/compose/docker-compose.dev.yml up frontend
```

## Import Map Points To Missing Vendor Files

The current production import map uses pinned CDN ESM URLs. If using self-hosted vendor files, make sure those files exist and are served.

## Local Import Map Cannot Load `/single-spa.js`

Vite dev mode may not serve library build output at `/single-spa.js`.

Use preview mode for local import-map testing:

```bash
npm run remote:preview:users
npm run remote:preview:all
```
