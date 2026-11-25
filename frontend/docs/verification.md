# Verification

This page lists the frontend checks to run before handing off work, and the acceptance checklist they satisfy. It is for frontend developers. Run verification from `frontend/`.

## Standard checks

```bash
npm run audit:packages
npm run typecheck
npm run lint
npm run build:shell
npm run build:packages
npm run build:apps
npm run build:all
npm run generate:versions
FRONTEND_MODULE_SOURCE=workspace npm run build:docker
```

`npm run build:docker` without `FRONTEND_MODULE_SOURCE=workspace` stages the published npm releases pinned in `module-sources.json` (`production`) instead of this checkout.

## Manual direct commands

These are the commands used by CI-style checks and are useful when debugging workspace script issues.

```bash
./node_modules/.bin/tsc --noEmit -p tsconfig.app.json
```

```bash
NODE_OPTIONS=--max-old-space-size=8192 ./node_modules/.bin/eslint . --cache --cache-location .eslintcache --max-warnings=0
```

```bash
./node_modules/.bin/vite build --config vite.config.ts
```

## Docker

Run from the repository root:

```bash
docker build -f infra/docker/frontend/Dockerfile.nginx .
docker build -f infra/docker/frontend/Dockerfile.dev .
```

## Compatibility import check

Old aliases should not appear in frontend source or root config:

```bash
rg "@/features|@/store|@/shared|@/lib|@/ui|@/utils" frontend/apps frontend/packages frontend/vite.config.ts frontend/tsconfig.base.json -n
```

Expected result: no matches.

## Staged dist check

After `npm run build:docker`, confirm:

```text
dist/import-map.json
dist/version-manifest.json
dist/mf/*/single-spa.js
dist/packages/*/index.js
```

The Docker images build with `FRONTEND_BASE_PATH=/`, so these root paths are what is served. `npm run runtime:verify` checks that layout and that `dist/portal/` holds only the retired service worker kill switch `sw.js`.

## Acceptance checklist

- dependency audit passes
- typecheck passes
- lint passes
- shell build passes
- all package builds pass
- all app builds pass
- version manifest generation passes
- docker production build passes
- docker dev build passes
- no old compatibility imports exist
- staged `dist` contains shell, import map, app bundles, and package bundles
