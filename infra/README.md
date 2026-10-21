# Infrastructure

Deployment files and tooling live here. Run `make help` from the repository root
or `make -C infra` for deployment commands. See the [deployment guide](../docs/deployment.md)
for environment setup, host/remote Nginx, Compose, Kubernetes and migration notes.

Private configuration goes in `env/*.env`; copy the matching `.example` and keep
credentials out of Git. Private Compose secrets go in `secrets/`. Docker build
contexts are always the repository root; use `-f infra/docker/<service>/Dockerfile`.

API contracts remain in `backend/docs` because the backend embeds them. Root
`make contracts-generate`, `make contracts-check` and `make openapi-export` delegate
to the backend generator. `make generate` also refreshes frontend manifests.
