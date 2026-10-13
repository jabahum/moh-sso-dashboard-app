# Releasing frontend and backend

Run release commands from the repository root. Frontend apps/packages and the backend retain independent versions. Version preparation, local commit, verification, and component tagging are automatic.

## Requirements

- Node.js 22, npm, installed frontend dependencies (`cd frontend && npm ci`), and the Go version specified by `backend/go.mod`.
- A clean, committed checkout containing the application changes to release.
- A configured Git author identity and fetch/push access to the release remote (default `upstream`, configurable with `RELEASE_REMOTE`). The remote must support atomic pushes.
- Registry credentials for optional frontend npm publication.

If using nvm, select Node 22 (`nvm use 22`). Alternatively supply `NODE=/path/to/node` and `NPM=/path/to/npm` to Make.

## One-command release

```bash
make release
```

This performs the following steps:

1. Check that the worktree is clean and fetch remote release tags.
2. Calculate the next backend version from the highest prepared `backend/VERSION` or published backend tag.
3. Generate a Changeset for every frontend app and shared package, including the private shell, and apply all pending Changesets. Update dependency ranges, changelogs, lockfile, and version manifest.
4. Generate scoped release notes and changelogs for every frontend module and the backend, record selected versions in `.release/plan.json`, and automatically commit all release metadata locally.
5. Verify both components sequentially, including production frontend artifacts and a backend release binary.
6. Create annotated `backend/v<backend-version>` and `frontend/v<shell-version>` tags, then push both atomically.

The default is a PATCH bump. To select a larger release:

```bash
make release BUMP=minor
make release BUMP=major
```

The command does not infer breaking changes from code. Choose MINOR or MAJOR when the changes require it. Existing frontend Changesets requesting a stronger bump are honored. Shared packages remain in their fixed release group; apps bump from their own current versions. Each frontend module's version is recorded in the manifest; the frontend component tag uses the shell version. Backend internal packages and its API/CLI share the backend version.

Application changes must be committed before running this command. The generated release commit is included in the pushed tags; the command does not push the branch itself. Integrate the version commit back into your development branch according to your normal review process.

## Prepare without publishing

```bash
make release-prepare
```

This automatically generates and commits all versions, but does not run release verification or publish tags. It is useful for reviewing the release diff. Run `make release` afterward with the same `BUMP` value; the prepared plan is reused without another version bump.

The prepared versions can also be verified separately:

```bash
make release-check
```

Backend verification runs formatting, vet, tests, architecture checks, and a release binary build. Frontend verification runs versioning regression tests and `npm run release:check`, including typecheck, lint, production build, runtime checks, and publishability checks. Generated source configuration, import map, and manifest are restored after frontend checks, including on failure. Build artifacts remain available for inspection.

## Release one component

```bash
make release-backend
make release-frontend
```

These perform the same automatic process for the selected component. Frontend-only releases bump every frontend module; backend-only releases leave frontend versions unchanged. Both use `BUMP=patch|minor|major` and `RELEASE_REMOTE`.

Backend tags trigger **Backend Release** for binary artifacts/checksums and **Build & Push Images** for its image. Frontend tags trigger **Build & Push Images** for the source-built frontend image and module manifest. Watch GitHub Actions for completion: a successful tag push queues CI and does not mean images have finished building. The atomic tag push does not make the two CI runs atomic.

Use image versions independently as `BACKEND_TAG` and `FRONTEND_TAG` in Compose or Helm.

## Explicit-version workflow

To manage version preparation yourself, the manual targets remain available:

```bash
make frontend-version
# Review and commit frontend version changes first.
make release-manual BACKEND_VERSION=0.0.2 FRONTEND_VERSION=0.1.1
make release-backend-manual BACKEND_VERSION=0.0.2
make release-frontend-manual FRONTEND_VERSION=0.1.1
```

Manual targets verify and tag exactly the supplied versions. `FRONTEND_VERSION` must match the shell package. Automatic targets calculate versions from `BUMP`; use manual targets when overriding version numbers.

## Publish frontend npm modules

```bash
make release-prepare
make frontend-publish
make release
```

Npm publishing is a separate registry operation. `frontend-publish` verifies prepared frontend versions and runs Changesets publishing for eligible public apps/packages; the private shell remains unpublished. The frontend image release builds modules from source and does not require npm publication. Publish npm modules before component tags if both operations are needed, since release checks reject existing component tags.

## Failures and retries

If verification fails after preparation, fix the problem and keep the prepared versions for review. Rerunning with the same component and bump reuses the plan while the preparation commit is still HEAD and no component tag exists. If you commit a code fix after preparing, use `make release-manual` with the prepared versions to avoid an additional automatic bump.

If version preparation itself fails, inspect the modified files before retrying; it leaves them available for diagnosis. It never stages existing uncommitted application changes because preparation requires an initially clean checkout.

If a push fails after creating local tags, inspect local and remote tags. The automatic command refuses to bump the same prepared release again. After confirming that neither component tag exists remotely, retry the atomic push with those exact refs. Never move or overwrite a published tag. If CI fails after the push, rerun the failed workflow for the existing tag.

Run `make release-help` for the command summary. See [backend release details](releasing-backend.md), [versioning](versioning.md), and [frontend versioning](../frontend/docs/versioning.md).

## Release notes and changelogs

`make release-prepare` and `make release` generate release notes automatically, before the preparation commit. Every frontend app/package gets its own `CHANGELOG.md` and versioned Markdown note. The backend gets `backend/CHANGELOG.md` and a versioned note. Outputs are:

```text
frontend/apps/<app>/CHANGELOG.md
frontend/packages/<package>/CHANGELOG.md
backend/CHANGELOG.md
.release/notes/frontend/apps/<app>/<version>.md
.release/notes/frontend/packages/<package>/<version>.md
.release/notes/backend/<version>.md
.release/notes/frontend.md
.release/notes/backend.md
.release/notes/manifest.json
```

Each component uses the previous reachable component release tag as its history baseline. Frontend commit lists are filtered to each module's directory; backend commit lists are filtered to `backend/`. The current release tag is excluded when regenerating in CI. Initial releases use available source history. Automatically generated preparation commits are omitted. Modules with no direct source changes explicitly describe version/dependency updates rather than inventing changes.

Frontend notes preserve the current version's Changesets changelog text, including authored summaries and dependency updates, and add scoped source commit subjects. For polished frontend notes, record clear descriptions with `cd frontend && npm run changeset` before releasing. Backend notes use scoped commit subjects by default and preserve authored text in the corresponding version's backend changelog entry. Commit subjects are the source of automatic summaries; the generator does not infer undocumented functionality.

To regenerate notes for current versions without bumping or publishing:

```bash
make release-notes
```

This writes notes/changelogs into the worktree for review. Repeated generation replaces the same version entry and keeps older changelog entries. Commit the resulting files before running commands that require a clean checkout.

The **Frontend Release Notes** tag workflow publishes the combined frontend notes as the GitHub Release body and attaches a compressed archive of individual module notes. The backend release workflows publish scoped backend notes as the GitHub Release body instead of repository-wide automatic notes. Both regenerate notes from tagged source, so manual component tags also receive notes. These release-note jobs are separate from frontend image builds; inspect the image workflow before deployment.
