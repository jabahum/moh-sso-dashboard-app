#!/usr/bin/env bash
# Runs every identity-sync e2e suite against a freshly started stack:
#   down -v -> up (build, seed, smoke) -> Playwright suites 01-04 -> migrations-check -> down -v
# Exits non-zero when any step fails. Used by "make -C infra identity-sync-e2e-test".
#
#   CLIMS_REPO=/path/to/clims POES_REPO=/path/to/poesGo e2e/identity-sync/run-all.sh
#   IDSYNC_E2E_KEEP=1 leaves the stack running afterwards.
set -uo pipefail
here="$(cd "$(dirname "$0")" && pwd)"
repo="$(cd "$here/../.." && pwd)"
: "${CLIMS_REPO:?set CLIMS_REPO to the CLIMS checkout}" "${POES_REPO:?set POES_REPO to the poesGo checkout}"
export CLIMS_REPO POES_REPO
mk() { make --no-print-directory -C "$repo/infra" "$@"; }

declare -a summary
status=0
step() {
  local name="$1"; shift
  echo "=== $name"
  if "$@"; then summary+=("PASS  $name"); else summary+=("FAIL  $name"); status=1; return 1; fi
}

finish() {
  if [ "${IDSYNC_E2E_KEEP:-}" = "1" ]; then
    echo "=== leaving the idsync-e2e stack running (IDSYNC_E2E_KEEP=1)"
  else
    step "stack down -v" mk identity-sync-e2e-down || true
  fi
  echo
  echo "=== identity-sync e2e summary"
  printf '%s\n' "${summary[@]}"
  exit "$status"
}
trap finish EXIT

step "stack down -v (clean start)" mk identity-sync-e2e-down || exit 1
step "stack up + smoke" mk identity-sync-e2e-up || exit 1

cd "$here"
step "npm ci (Playwright 1.64.1)" npm ci --no-audit --no-fund || exit 1
step "Playwright chromium present" npx playwright install chromium || exit 1
# One Playwright run, suites in file order; each suite reports its own tests.
step "suites: contract, sync, browser, removal" npx playwright test || true
# Keeps every host port in the e2e range (infra/scripts/test-migrations.py).
step "migrations-check" env MIGRATIONS_CHECK_HOST_PORT="${MIGRATIONS_CHECK_HOST_PORT:-18432}" make --no-print-directory -C "$repo/infra" migrations-check || true
