# Backend rollback

This page describes how a backend release is rolled back automatically or by hand, and the limits that database migrations place on rollback. It is for operators. For the full CD transaction, server layout and GitHub setup, see the [CD and server setup runbook](cd-pipeline.md).

Rollback always restores a whole verified release recorded by `infra/scripts/cd_server.py`: the backend and frontend image digests together with the infrastructure files and private env snapshot they were deployed with. There is no backend-only rollback and no server-side `restart` script.

## Automated rollback

`cd_server.py deploy`, run by the **Deploy Verified Image Bundle** workflow (`.github/workflows/deploy-to-server.yaml`), writes `state/pending.json` before starting the candidate. If starting or verifying the candidate fails, it records a `deployment-failed` event, restarts the previous verified release from `state/deployment.json`, verifies it, points the `current` symlink back at it and records an `automatic-rollback` event in `state/history.jsonl`.

Verification checks, inside the application network, the backend `/health/ready` and `/version` endpoints (version and commit must match the manifest), the frontend `/portal/version-manifest.json`, and the frontend verifier script. When `verify_public_edge` is true (the default) it also checks the public `/portal/`, `/ssobackend/health/ready`, `/ssobackend/swagger/openapi.json` and `/ssobackend/version` URLs.

The first managed deployment has no previous verified release, so its failure is reported without a rollback. If the automatic rollback itself fails, `state/pending.json` is kept and new deployments are refused until an operator rollback succeeds.

## Manual rollback

Run the **Roll Back Verified Deployment** workflow (`.github/workflows/rollback-server.yml`) and choose `staging` or `production`. It checks out the default branch, uses the same protected environment and concurrency group as deployment, uploads `cd_server.py`, `cd_bundle.py` and `verify-frontend-deployment.mjs` to `DEPLOY_ROOT/incoming/`, and runs `cd_server.py rollback --config DEPLOY_CONFIG --actor <github actor>` there.

Alternatively, as the deploy user on the server:

```bash
python3 /opt/moh-sso/current/infra/scripts/cd_server.py status
python3 /opt/moh-sso/current/infra/scripts/cd_server.py rollback --actor operator
python3 /opt/moh-sso/current/infra/scripts/cd_server.py verify
```

`status` prints the `current` and `previous` release manifests. `rollback` targets the `previous` release in `state/pending.json` when an interrupted deployment left one, otherwise the `previous` release in `state/deployment.json`, and fails with `No verified previous release is available` when there is none. After restarting and verifying the target, it makes it `current`, records the release it replaced as `previous`, and appends an `operator-rollback` event to `state/history.jsonl`. The scripts default to `/etc/moh-sso/deploy.json`; pass `--config` for another path.

The same actions are available as `make cd-status`, `make cd-rollback` and `make cd-verify` with `DEPLOY_CONFIG=...` (see [Make targets](make.md)).

## Migrations and rollback

Rollback restores images and configuration only; database schema and data are not reverted. Do not roll application code back across an incompatible schema migration. Restore the database backup or apply a tested forward-fix migration according to the release runbook.

## Failed release

When the **Backend Release** workflow (`.github/workflows/backend-release.yml`) is started manually, it creates the `backend/v<version>` tag only after the checks and release build pass, so a failed run leaves no tag. When it is started by pushing a `backend/v*.*.*` tag, the tag already exists even if the run fails. Rerun the same version only when no tag or GitHub release exists for it.
