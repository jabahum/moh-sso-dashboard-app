# Releasing frontend and backend

Run release commands from the repository root. Backend and frontend versions are independent; a combined release can use `backend/v0.0.2` and `frontend/v0.1.1` from the same commit.

## Requirements

- Node.js 22, npm, installed frontend dependencies (`cd frontend && npm ci`), and the Go version specified by `backend/go.mod`.
- A clean, committed checkout containing the intended release changes and frontend version bumps.
- A configured Git author identity and push access to the release remote (default `upstream`, configurable through `RELEASE_REMOTE`). The remote must support atomic pushes, and component tags must be unused.
- For npm publishing, registry credentials and access to the restricted `@moh-sso` scope.

If using nvm, select Node 22 before running Make (`nvm use 22`). You can also supply `NODE=/path/to/node` and `NPM=/path/to/npm` to Make when multiple installations exist.

## Prepare frontend versions

Record changes with `cd frontend && npm run changeset`, then apply pending bumps from the root:

```bash
make frontend-version
```

Review and commit the updated package versions, changelogs, lockfile, and version manifest. Shared packages bump together; apps retain their independent versions. `FRONTEND_VERSION` defaults to the shell package version. An explicit value must match it; release commands never bump versions automatically.

Choose the backend version according to SemVer; `make -C backend next-version BUMP=patch` can suggest the next version from local backend tags. Fetch release tags before relying on that suggestion.

## Verify locally

```bash
make release-check BACKEND_VERSION=0.0.2 FRONTEND_VERSION=0.1.1
make release-backend-check BACKEND_VERSION=0.0.2
make release-frontend-check FRONTEND_VERSION=0.1.1
```

Checks require a clean checkout and reject locally existing release tags. Backend verification runs formatting, vet, tests, architecture checks, and a release binary build. Frontend verification runs versioning regression tests and `npm run release:check`, including typecheck, lint, production build, runtime checks, and publishability checks. Generated source configuration, import map, and manifest are restored after frontend checks, including on failure; build artifacts remain available for inspection.

## Release both components

```bash
make release BACKEND_VERSION=0.0.2 FRONTEND_VERSION=0.1.1 RELEASE_REMOTE=upstream
```

This verifies both components sequentially before creating any tags. It checks that neither tag exists on the remote, creates annotated `backend/v0.0.2` and `frontend/v0.1.1` tags, then pushes both in one atomic Git push. A rejected atomic push cannot publish only one component. It does not make the two subsequent CI runs atomic: inspect both workflows before deploying.

## Release one component

```bash
make release-backend BACKEND_VERSION=0.0.2
make release-frontend FRONTEND_VERSION=0.1.1
```

Backend tags trigger **Backend Release** for binary artifacts/checksums and **Build & Push Images** for the backend image. Frontend tags trigger **Build & Push Images** for the frontend image and embedded module manifest. Images receive their component's semantic version and a commit SHA tag. Watch GitHub Actions for completion; a successful tag push means CI has been queued, not that a release image has finished building.

Use the image tags as independent `BACKEND_TAG` and `FRONTEND_TAG` values in Compose or Helm. Frontend image releases build modules from source and do not require npm publication.

## Publish frontend npm modules

```bash
make frontend-publish
```

This verifies the frontend and runs Changesets publishing for eligible public apps/shared packages. The private shell remains unpublished. This is a separate registry operation from component image tagging. Run it before tagging the frontend image if both operations are needed, since release verification rejects existing component tags.

## Failed pushes and retries

If a push fails, inspect local and remote tags before retrying. The release command retains its local annotated tags so you can diagnose the failure. After confirming that neither tag exists remotely, you can retry the same atomic push manually with the exact refs printed by the release command. Never move or overwrite an already published tag. If CI fails after the push, inspect and rerun the failed workflow for the existing tag rather than creating a new tag at another commit.

Run `make release-help` for the command summary. See [backend release details](releasing-backend.md), [versioning](versioning.md), and [frontend versioning](../frontend/docs/versioning.md).
