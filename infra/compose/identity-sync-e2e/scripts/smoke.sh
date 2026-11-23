#!/usr/bin/env bash
# Health proof for the running idsync-e2e stack. Run from anywhere:
#   CLIMS_REPO=... POES_REPO=... infra/compose/identity-sync-e2e/scripts/smoke.sh
# Exits non-zero on the first failed check. Needs docker, curl and python3.
set -euo pipefail
here="$(cd "$(dirname "$0")/.." && pwd)"
compose=(docker compose -p idsync-e2e -f "$here/compose.yaml")
ca="$here/generated/caddy-root.crt"
port=18443
issuer="https://sso.e2e.test:$port/realms/moh-realm"
# Host access without touching /etc/hosts: pin every e2e host to the published port.
resolve=()
for h in sso dashboard clims poes mail; do resolve+=(--resolve "$h.e2e.test:$port:127.0.0.1"); done
hcurl() { curl -sS --fail-with-body --cacert "$ca" "${resolve[@]}" "$@"; }
json() { python3 -I -c "import json,sys; d=json.load(sys.stdin); print($1)"; }
ok() { printf 'PASS  %s\n' "$*"; }

echo "--- services"
"${compose[@]}" ps -a --format '{{.Service}}\t{{.State}}\t{{.Status}}' | sort
bad=$("${compose[@]}" ps -a --format '{{.Service}} {{.State}} {{.Status}}' |
  awk '($2=="running" && $0 !~ /\(healthy\)/) || ($2=="exited" && $0 !~ /Exited \(0\)/) {print $1}')
[ -z "$bad" ] || { echo "FAIL  not healthy/completed: $bad"; exit 1; }
ok "every long-running service is healthy and every one-shot job exited 0"

echo "--- OIDC discovery inside the compose network (HTTPS through Caddy)"
docker run --rm --network idsync-e2e_edge -v idsync-e2e_e2e_ca:/ca:ro curlimages/curl:8.14.1 \
  -sS --fail-with-body --cacert /ca/root.crt "$issuer/.well-known/openid-configuration" |
  json 'd["issuer"]' | grep -qx "$issuer"
ok "in-network discovery issuer = $issuer"

echo "--- OIDC discovery from the host (published Caddy port $port)"
hcurl "$issuer/.well-known/openid-configuration" | json 'd["issuer"]' | grep -qx "$issuer"
ok "host discovery issuer = $issuer"

echo "--- GET / descriptor with a client-credentials sync token"
for pair in clims:clims-identity-sync:e2e-fake-clims-sync-secret:clims poesgo:poesgo-identity-sync:e2e-fake-poesgo-sync-secret:poes; do
  IFS=: read -r app client secret host <<<"$pair"
  token=$(hcurl -d grant_type=client_credentials -d "client_id=$client" -d "client_secret=$secret" \
    "$issuer/protocol/openid-connect/token" | json 'd["access_token"]')
  descriptor=$(hcurl -H "Authorization: Bearer $token" "https://$host.e2e.test:$port/api/v1/internal/identity-sync")
  echo "$descriptor"
  echo "$descriptor" | json 'd["version"]+" "+d["application"]' | grep -qx "1.1 $app"
  ok "$app descriptor reports contract 1.1 (token from $client)"
done

echo "--- identity-sync describe (dashboard CLI, one-shot container)"
for app in clims poesgo; do
  "${compose[@]}" run --rm --no-deps cli identity-sync describe --app "$app" |
    json 'd["integration"]["contractVersion"]+" "+d["descriptor"]["version"]+" "+str(d["integration"]["enabled"])' |
    grep -qx "1.1 1.1 True"
  ok "identity-sync describe --app $app: registry 1.1, live descriptor 1.1, enabled"
done

echo "--- dashboard, CLIMS SPA, POES and mailpit over HTTPS from the host"
hcurl -o /dev/null "https://dashboard.e2e.test:$port/health/ready" && ok "dashboard /health/ready"
hcurl "https://clims.e2e.test:$port/sign-in" | grep -qi '<html' && ok "CLIMS SPA /sign-in served"
hcurl -o /dev/null "https://clims.e2e.test:$port/health/ready" && ok "CLIMS backend /health/ready"
hcurl -o /dev/null "https://poes.e2e.test:$port/ready" && ok "POES /ready"
hcurl "https://mail.e2e.test:$port/api/v1/info" | json 'd["Version"]' >/dev/null && ok "mailpit API reachable"
echo "ALL SMOKE CHECKS PASSED"
