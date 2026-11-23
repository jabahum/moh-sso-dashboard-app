# Backend rollback

This page describes how a backend release is rolled back automatically or by hand, and the limits that database migrations place on rollback. It is for operators. For the managed CD rollback workflow, see the [CD and server setup runbook](cd-pipeline.md).

## Automated rollback

The deployment workflow retains the previously deployed backend and frontend tags. If `/version` or `/health/live` verification fails, it invokes the server restart process with those previous tags.

## Manual rollback

1. Identify the previous immutable tag and digest from `deployment-history.log`.
2. Confirm the image still exists in GHCR.
3. Set the previous tags:

```bash
export BACKEND_TAG=1.2.2
export FRONTEND_TAG=sha-previous
./restart
```

4. Verify:

```bash
curl -fsS http://localhost:9000/version
curl -fsS http://localhost:9000/health/live
```

5. Confirm the reported version and commit match the rollback target.

## Migrations and rollback

Do not roll application code back across an incompatible schema migration. Restore the database backup or apply a tested down/forward-fix migration according to the release runbook.

## Failed release

A failed release workflow does not create the `backend/v<version>` tag. Fix the failure and rerun the same version only when no tag or release artifact exists.
