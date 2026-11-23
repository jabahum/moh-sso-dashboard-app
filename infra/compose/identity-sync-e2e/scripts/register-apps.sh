#!/bin/sh
# Runs inside the dashboard image (one-shot "registry-seed" service): registers
# CLIMS and POES in the identity-sync registry with --enable, which reads each
# application's GET / descriptor with a client-credentials sync token and stores
# its contract version, then prints "identity-sync describe" for both.
set -eu
cli=/app/moh-sso
$cli identity-sync register --app clims --origin https://clims.e2e.test:18443 \
  --sync-client clims-identity-sync --access-role clims_access \
  --credential env:CLIMS_SYNC_CLIENT_SECRET --account-id-format uuid \
  --reason "identity-sync e2e seed" --enable
$cli identity-sync register --app poesgo --origin https://poes.e2e.test:18443 \
  --sync-client poesgo-identity-sync --access-role poesgo_access \
  --credential env:POESGO_SYNC_CLIENT_SECRET --account-id-format positive-integer \
  --reason "identity-sync e2e seed" --enable
$cli identity-sync describe --app clims
$cli identity-sync describe --app poesgo
