# Infrastructure

See the [Make command reference](../docs/make.md) for every deployment and generation target,
its prerequisites, options, effects and examples.

Deployment files and tooling live here. Run `make help` from the repository root
or `make -C infra` for deployment commands. See the [deployment guide](../docs/deployment.md)
for environment setup, host/remote Nginx, Compose, Kubernetes and migration notes.
For the complete CD pipeline, server bootstrap, GitHub settings, production
promotion and rollback, follow the [CD runbook](../docs/cd-pipeline.md).
See [CI checks](../docs/ci-checks.md) for workflow linting, browser smoke tests,
disposable database migration tests and real container recovery tests.
Use `make scheduler-env COMPOSE_ENV=external` to generate private scheduler
configuration; see [scheduler settings](../docs/scheduler-configuration.md).

Private configuration goes in `env/*.env`; copy the matching `.example` and keep
credentials out of Git. Private Compose secrets go in `secrets/`. Docker build
contexts are always the repository root; use `-f infra/docker/<service>/Dockerfile`.

API contracts remain in `backend/docs` because the backend embeds them. Root
`make contracts-generate`, `make contracts-check` and `make openapi-export` delegate
to the backend generator. `make generate` also refreshes frontend manifests.
