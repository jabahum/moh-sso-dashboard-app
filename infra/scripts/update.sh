#!/bin/sh
# Run from a checked-out release on the application server; no Git or volume reset.
set -eu
ROOT=$(CDPATH= cd -- "$(dirname -- "$0")/../.." && pwd)
mode=${COMPOSE_ENV:-prod}
case "$mode" in prod|gateway|remote) ;; *) echo 'COMPOSE_ENV must be prod, gateway or remote' >&2; exit 2 ;; esac
compose() { sh "$ROOT/infra/scripts/compose.sh" "$mode" "$@"; }
# Optional registry login. Otherwise reuse the Docker account's existing login.
if [ -n "${GHCR_TOKEN_FILE:-}" ]; then
  docker login ghcr.io -u "${GHCR_USER:-mohuganda}" --password-stdin < "$GHCR_TOKEN_FILE"
fi
compose config --quiet
compose pull
compose up -d
compose ps
if [ "${TAIL_LOGS:-false}" = true ]; then
  compose logs --tail="${LOG_TAIL_LINES:-120}"
fi
