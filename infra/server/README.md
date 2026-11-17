# Server configuration pack

Use [the CD runbook](../../docs/cd-pipeline.md) for the exact server/GitHub setup
and deployment actions. Nothing here installs or deploys automatically.

- `bootstrap.sh`: one-time Linux deployment-user/directory/config preparation.
- `deploy.json.example`: server-local paths, topology, HTTPS verification and hook.
- `sshd-mohdeploy.conf.example`: optional SSH restrictions for the dedicated user.
- `hooks/backup-primary.sh.example`: optional tested primary-DB backup hook template.

Application credentials remain on the server in `/etc/moh-sso`; never add real
values to these examples or GitHub build artifacts. The CD bundle contains only
public deployment infrastructure and immutable image metadata.
## Modular distribution deployments

Use `deploy.distribution.json.example` for a server that manages the distribution's
core database, isolated Observatory database, Redis, uploads and gateway. It uses
the same validated image-pair bundle, SSH deployment and rollback tools as the
other production modes. Supply four private secret files and the trusted-proxy
configuration. The managed Observatory host/password wiring is supplied by this
topology; warehouse/upload integrations and Keycloak remain explicit settings.

Give staging and production distinct `project_name`, `deploy_root`, environment
files, secret paths and gateway bindings. The project name is captured with each
deployment snapshot and reused during rollback. An existing deployment cannot
silently change project or topology; plan a data migration separately. Existing
records without a project name retain their original default project.

The release manifest includes exact frontend pins, compiled module versions and
Go/npm dependency-file checksums. Runtime verification checks the frontend's
complete published module set as well as the two application versions/commits.
Distribution release Compose files contain no build settings: servers pull the
approved digests and do not require source files or npm/Go toolchains.

`hooks/backup-distribution.sh.example` provides an operator-installed backup hook
for both managed databases and uploads. Set its project and private backup path,
and arrange off-host retention. It expects a running installation; set the hook
after a first installation or provide an equivalent first-install policy.
Database and upload writers need a coordinated maintenance window for a backup
consistent across services. Reverting images does not reverse schema changes;
restore a compatible backup when required. The disposable authenticated check
exercises database and upload restoration with fixture data.

### Build and promote

The existing `Build & Push Images` workflow now uses the pinned npm Dockerfile
for the frontend and runs the distribution installation gate before publishing.
Main builds produce SHA-tagged GHCR images and a `deployment-bundle` containing
their verified digests. Component release tags continue to publish their numeric
image versions through the existing release workflows.

After merging the implementation, use a successful main build's run ID with
`Deploy Verified Image Bundle`, selecting `staging`. Configure that GitHub
environment's `DEPLOY_ROOT`, `DEPLOY_CONFIG`, host and SSH settings for its private
distribution server configuration. Verify the deployed application and identity
provider, then dispatch the same build run ID with `environment=production`.
This promotes the same archived image pair and dependency pins without rebuilding.
Use separate GitHub environment settings for production. Keep automatic production
release deployment disabled when requiring this staging-first process; production
environment protection can require operator approval.

The trusted build selector rejects failed builds, fork builds, unrelated workflows
and non-main builds. Environment and secret values stay on the selected server;
bundles and dependency metadata contain no operator credentials. Automatic release
selection and operator rollback retain their existing behavior.
