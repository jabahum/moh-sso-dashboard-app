# Verification

Run verification from `frontend/`.

## Standard Checks

```bash
npm run audit:packages
npm run typecheck
npm run lint
npm run build:shell
npm run build:packages
npm run build:apps
npm run build:all
npm run generate:versions
npm run build:docker
```

## Manual Direct Commands

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

## Compatibility Import Check

Old aliases should not appear in frontend source or root config:

```bash
rg "@/features|@/store|@/shared|@/lib|@/ui|@/utils" frontend/apps frontend/packages frontend/vite.config.ts frontend/tsconfig.base.json -n
```

Expected result: no matches.

## Staged Dist Check

After `npm run build:docker`, confirm:

```text
dist/import-map.json
dist/mf/*/single-spa.js
dist/packages/*/index.js
```

## Acceptance Checklist

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
