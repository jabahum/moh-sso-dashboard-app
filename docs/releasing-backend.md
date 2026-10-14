# Releasing The Backend

The root Makefile supports backend-only and combined frontend/backend releases. See [Releasing frontend and backend](releasing.md) for `make release`, `make release-backend`, and independent version arguments. The commands below run from `backend/` and retain the existing backend-specific workflow.

## Automated Release

The `Backend Release` GitHub Actions workflow runs in two modes:

1. Manual dispatch: provide a version such as `1.2.3`. Do not include `backend/v` in the input.
2. Tag push: pushing `backend/v<version>` runs the same verification and publishes the GitHub release automatically.

## Release PR Flow

Use this flow when you want a reviewed release branch before the tag and GitHub Release are created.

### Command-Assisted Flow

The backend Makefile includes release helpers so the common path does not require memorizing `gh` command syntax.

From `backend/`:

```bash
make next-version BUMP=patch
make release-check VERSION=1.2.3
make release-pr VERSION=1.2.3
make release-status VERSION=1.2.3
```

What those commands do:

| Command | Purpose |
| --- | --- |
| `make next-version BUMP=patch` | Prints the next backend SemVer from existing `backend/v*` tags. `BUMP` can be `patch`, `minor`, or `major`. |
| `make release-check VERSION=1.2.3` | Runs local release verification: version consistency, formatting check, vet, tests, architecture test, release binary build, and binary metadata verification. Requires a clean worktree. |
| `make release-pr VERSION=1.2.3` | Creates or checks out `release/backend-v1.2.3`, adds an empty release marker commit when the branch has no commits ahead of `main`, pushes it, and opens a GitHub release PR. |
| `make release-dispatch VERSION=1.2.3` | Runs the one-shot `backend-release.yml` workflow manually. Use this when you intentionally do not need a release PR. |
| `make release-status VERSION=1.2.3` | Shows the GitHub release, recent release workflows, and Docker image manifest when Docker is available. |
| `make verify-image IMAGE=ghcr.io/mohuganda/moh-sso-dashboard-backend:1.2.3 VERSION=1.2.3 COMMIT=<sha>` | Runs the backend image with `--version` and checks the embedded metadata. |

The helper commands default to:

```text
REPO=mohuganda/moh-sso-dashboard
BASE=main
RELEASE_REMOTE=upstream
BACKEND_IMAGE=ghcr.io/mohuganda/moh-sso-dashboard-backend
```

Override them when needed:

```bash
make release-pr VERSION=1.2.3 REPO=my-org/my-fork BASE=main
make release-pr VERSION=1.2.3 RELEASE_REMOTE=origin
make release-status VERSION=1.2.3 BACKEND_IMAGE=registry.example.go.ug/moh/backend
```

For release PRs that should create the `backend/v<version>` tag automatically, push the release branch to `upstream`, not a fork. GitHub does not allow a fork PR workflow token to create tags in `mohuganda/moh-sso-dashboard`.

When `RELEASE_REMOTE=origin` points to a fork, the helper opens the PR with a fork-qualified head such as `jabahum:release/backend-v1.2.3` against `mohuganda/moh-sso-dashboard`. That is useful for review, but it cannot create the release tag after merge. After that fork PR is merged, run:

```bash
cd backend
make release-dispatch VERSION=1.2.3
```

Fork-based release PRs skip artifact provenance attestation, tag creation, GitHub Release publication, and release metadata upload because GitHub does not expose write-capable tokens or OIDC ID tokens to that workflow context. They still run formatting, vet, tests, architecture checks, version verification, SBOM generation, and checksums. Manual dispatch and direct tag releases still run the full release publication path.

GitHub artifact attestations are controlled by `ENABLE_GITHUB_ARTIFACT_ATTESTATIONS` in the backend release workflows. It is currently disabled because GitHub returned `Feature not available for the mohuganda organization`. Keep checksums and SBOMs enabled; set `ENABLE_GITHUB_ARTIFACT_ATTESTATIONS=true` only after the repository/org supports GitHub artifact attestations.

### Manual Commands

1. Pick the next backend SemVer.

   Use the backend component tag format:

   ```text
   backend/v1.2.3
   ```

   The branch name must be:

   ```text
   release/backend-v1.2.3
   ```

2. Make sure `main` is current.

   ```bash
   git checkout main
   git pull upstream main
   ```

3. Create the release branch.

   ```bash
   git checkout -b release/backend-v1.2.3
   ```

4. Run local release checks.

   ```bash
   cd backend
   gofmt -w $(find . -name '*.go')
   GOCACHE=/private/tmp/moh-sso-go-build go test ./...
   ./scripts/check-version-consistency.sh
   cd ..
   ```

5. Commit any release-prep changes.

   If there are no file changes, the branch can still be used as a release marker. If there are formatting, docs, changelog, or version-prep changes:

   ```bash
   git add .
   git commit -m "Prepare backend v1.2.3 release"
   ```

6. Push the release branch.

   ```bash
   git push upstream release/backend-v1.2.3
   ```

7. Open the PR into `main`.

   ```bash
   gh pr create \
     --repo mohuganda/moh-sso-dashboard \
     --base main \
     --head release/backend-v1.2.3 \
     --title "Release backend v1.2.3" \
     --body "Prepares backend release backend/v1.2.3."
   ```

8. Wait for PR checks to pass, then merge the PR.

   ```bash
   gh pr merge --repo mohuganda/moh-sso-dashboard --merge --delete-branch
   ```

9. After the PR is merged, GitHub Actions runs `Backend Release From PR`.

   The workflow only runs when:

   - the PR is merged
   - the PR target branch is `main`
   - the source branch starts with `release/backend-v`
   - the suffix is a valid SemVer, for example `1.2.3`

   It then verifies the merge commit, creates the tag, and publishes the release.

10. Confirm the release.

    ```bash
    gh run list --repo mohuganda/moh-sso-dashboard --workflow backend-release-pr.yml --limit 5
    gh release view backend/v1.2.3 --repo mohuganda/moh-sso-dashboard
    git fetch upstream --tags
    git tag --list "backend/v1.2.3"
    ```

11. Confirm the Docker image is built.

    Creating the component tag triggers the image build workflow. Expected backend image tags:

    ```text
    ghcr.io/mohuganda/moh-sso-dashboard-backend:1.2.3
    ghcr.io/mohuganda/moh-sso-dashboard-backend:sha-<commit>
    ```

    Check the build workflow:

    ```bash
    gh run list --repo mohuganda/moh-sso-dashboard --workflow build.yml --limit 5
    ```

12. Deploy using immutable tags.

    ```bash
    BACKEND_TAG=1.2.3 FRONTEND_TAG=<frontend-tag> \
      docker compose -f docker-compose-nginx.yml pull

    BACKEND_TAG=1.2.3 FRONTEND_TAG=<frontend-tag> \
      docker compose -f docker-compose-nginx.yml up -d
    ```

If you prefer a one-shot release without a PR, use the manual dispatch flow below.

## Manual Dispatch Flow

Run the `Backend Release` workflow directly:

```bash
cd backend
make release-dispatch VERSION=1.2.3
```

Equivalent raw `gh` command:

```bash
gh workflow run backend-release.yml \
  --repo mohuganda/moh-sso-dashboard \
  -f version=1.2.3
```

Do not include `backend/v` in the `version` input.

The workflow performs these operations before publishing the release:

1. Validate SemVer and confirm the tag is unused.
2. Verify formatting, vet, tests, and architecture boundaries.
3. Verify all version package/build paths are consistent.
4. Build Linux and macOS artifacts for amd64 and arm64.
5. Execute the Linux binary and verify version, commit, and clean state.
6. Generate SHA-256 checksums, an SPDX SBOM, and provenance attestations.
7. Create and push `backend/v<version>` only for manual dispatch.
8. Publish the GitHub release artifacts for manual dispatch, tag-push runs, and merged release PRs.

Pushing the component tag triggers the backend Docker image build. The image build embeds the same metadata, adds OCI labels, publishes provenance/SBOM data, and runs the image with `--version` as a post-build check.

The release workflow emits a metadata artifact with both:

- `release_tag`: `backend/v<version>` for the Git tag and GitHub Release
- `image_tag`: `<version>` for the Docker image tag

## Local Release Build

```bash
cd backend
GOCACHE=/private/tmp/moh-sso-go-build make build-release \
  VERSION=1.2.3 \
  COMMIT="$(git rev-parse HEAD)" \
  BUILD_TIME="$(date -u +'%Y-%m-%dT%H:%M:%SZ')" \
  DIRTY=false
```

Release builds reject `VERSION=dev` and `DIRTY=true`.

## Hotfixes

Branch from the deployed release commit, apply only the required compatible fix, run full verification, and release the next PATCH version. Do not overwrite or move an existing release tag.

## Signing

OCI provenance and SBOM generation are enabled. Cosign signing can be added through GitHub OIDC without storing private signing keys in the repository. Production policy may require signature verification before deployment.
