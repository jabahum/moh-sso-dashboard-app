# Versioning

For root Make commands that verify and release either component or both together, see [Releasing frontend and backend](releasing.md).

## Backend

The backend follows Semantic Versioning and uses component-scoped Git tags:

```text
backend/v1.0.0
backend/v1.1.0
backend/v2.0.0
```

- PATCH: compatible fixes with no contract or migration break.
- MINOR: backward-compatible functionality.
- MAJOR: breaking API, configuration, database migration, security, or operational contract changes.

Frontend workspace packages and apps have their own npm versions. A backend tag never changes frontend package versions automatically.

## Build Metadata

Every backend binary exposes:

- service name
- normalized version without a leading `v`
- Git commit
- RFC3339 UTC build time
- dirty worktree status
- Go runtime version

Inspect it with:

```bash
./bin/moh-sso-dashboard --version
./bin/moh-sso version --output json
curl http://localhost:9000/version
```

Direct `go build`/`go run` builds default to `dev` and use Go embedded VCS information when available. Make builds use the exact `backend/v*` tag only for a clean tagged checkout; other checkouts use `0.0.0-dev.<commit>`. Release values are injected with linker flags and take precedence over VCS fallbacks.

The Go backend is one module: its API server, management CLI, and internal feature packages share a backend release version. Internal packages do not have separate release tags.

## Frontend

All 15 apps (including the private shell) and eight shared packages have semantic versions in their `package.json`. Shared packages use a fixed Changesets release group; apps release independently. Internal shared dependencies use compatible caret ranges, and app dependencies use tilde ranges. See [frontend versioning](../frontend/docs/versioning.md).

The shell version identifies the frontend service. Its runtime metadata and module manifest use that version by default. CI supplies a development version and commit/build timestamp on branch builds. A `frontend/v<version>` tag must match `frontend/apps/shell/package.json` and builds a semantic frontend image tag. Backend and frontend release tags build only their own image.

Run the consistency checks before release:

```bash
cd frontend
npm run version:audit
npm run audit:packages
node --test scripts/test-versioning.mjs
cd ../backend
./scripts/check-version-consistency.sh
```

The frontend audit checks the shell, module versions, dependency compatibility, shared release group, lockfile, and generated manifest. `make release` automatically bumps every frontend module and the backend, commits the prepared versions, verifies both components, and pushes component tags. `BUMP=patch|minor|major` selects the default change level; stronger pending frontend Changesets are honored. `backend/VERSION` stores the prepared backend version and `.release/plan.json` records the generated versions. See [release commands](releasing.md).

## Image Tags

Backend images use tags without the component prefix:

```text
ghcr.io/mohuganda/moh-sso-dashboard-backend:1.2.3
ghcr.io/mohuganda/moh-sso-dashboard-backend:sha-d3062e2
```

`latest` may exist for discovery on the default branch but is forbidden for production deployment. Production records the immutable tag and image digest.

## Compatibility

Minor and patch releases should preserve API and configuration compatibility. Breaking response shapes, removed endpoints, mandatory configuration changes, destructive migrations, or authorization semantic changes require a major release or a documented compatibility window.

Database migrations must remain forward-safe for rolling deployment whenever possible. A release with an irreversible migration must document backup and rollback constraints before it is tagged.

## Release notes

Automatic preparation creates per-module frontend changelogs and scoped backend notes, committed with the release versions. Component tag workflows publish those notes to GitHub Releases. See [release-note outputs and authoring](releasing.md#release-notes-and-changelogs).
