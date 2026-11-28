#!/usr/bin/env bash
# Start the platform API conformance stack, run the suite against it, and remove
# the stack (including its tmpfs databases) whatever the outcome.
#   infra/scripts/test-platform-conformance.sh
# Set KEEP_STACK=1 to leave it running for debugging.
set -euo pipefail

ROOT=$(cd "$(dirname "$0")/../.." && pwd)
COMPOSE=(docker compose -p platform-conformance -f "$ROOT/infra/compose/platform-conformance/compose.yaml")

cleanup() {
  status=$?
  if [ "$status" -ne 0 ]; then
    "${COMPOSE[@]}" logs --no-color --tail=200 backend keycloak seed || true
  fi
  if [ "${KEEP_STACK:-}" != "1" ]; then
    "${COMPOSE[@]}" down -v --remove-orphans >/dev/null 2>&1 || true
  fi
  exit "$status"
}
trap cleanup EXIT

# Start the long-running services and wait until they are healthy, then run the
# one-shot seed (a finished job would make --wait fail).
"${COMPOSE[@]}" up -d --build --wait backend
"${COMPOSE[@]}" run --rm seed

cd "$ROOT/backend"
PLATFORM_CONFORMANCE_PORTAL=http://127.0.0.1:19000 \
PLATFORM_CONFORMANCE_KEYCLOAK=http://127.0.0.1:18080 \
PLATFORM_CONFORMANCE_DATABASE_URL='postgres://sso_ci:ci-fake-db-password@127.0.0.1:15432/sso_conformance?sslmode=disable' \
  go test -count=1 -v -run TestPlatformConformanceE2E ./internal/platformconformance/
