#!/bin/sh
# One location-aware entry point for all deployment modes. No env-file sourcing.
set -eu
ROOT=$(CDPATH='' cd -- "$(dirname -- "$0")/../.." && pwd)
cd "$ROOT"
mode=${1:-prod}
if [ "$#" -gt 0 ]; then shift; fi
case "$mode" in
  prod|remote) file=infra/compose/docker-compose.prod.yml; default_env=infra/env/prod.env ;;
  gateway) file=infra/compose/docker-compose-nginx.yml; default_env=infra/env/prod.env ;;
  dev) file=infra/compose/docker-compose.dev.yml; default_env=infra/env/dev.env ;;
  external) file=infra/compose/docker-compose-without-keycloak.yml; default_env=infra/env/external-dev.env ;;
  *) echo "Usage: $0 prod|dev|gateway|remote|external <compose arguments>" >&2; exit 2 ;;
esac
env_file=${ENV_FILE:-$default_env}
if [ "$mode" = dev ] && [ ! -f "$env_file" ] && [ -z "${ENV_FILE:-}" ]; then
  env_file=infra/env/dev.env.example
fi
if [ ! -f "$env_file" ]; then
  echo "Missing environment file: $env_file. Copy ${default_env}.example to $env_file first." >&2
  exit 2
fi
env_file=$(CDPATH='' cd -- "$(dirname -- "$env_file")" && pwd)/$(basename -- "$env_file")
case "$mode" in
  dev|external) export DEV_ENV_FILE="$env_file" ;;
  *) export APP_ENV_FILE="$env_file" ;;
esac
if [ "$mode" = remote ]; then
  remote_env=${REMOTE_ENV_FILE:-infra/env/remote-proxy.env}
  if [ ! -f "$remote_env" ]; then
    echo "Missing remote settings: $remote_env. Copy infra/env/remote-proxy.env.example first." >&2
    exit 2
  fi
  exec docker compose --env-file "$env_file" --env-file "$remote_env" -f "$file" "$@"
fi
exec docker compose --env-file "$env_file" -f "$file" "$@"
