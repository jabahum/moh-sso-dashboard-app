#!/bin/sh
# Prepares local HTTPS for testing CLIMS against the dashboard dev stack
# (see README.md in this folder). Everything it writes goes to
# LOCAL_HTTPS_DIR (default ~/.moh-dev-https), never into the repository.
#
#   infra/compose/local-https/setup.sh            # CA, certificate, bundle, Caddyfiles
#   infra/compose/local-https/setup.sh --secret   # also save the clims client secret from the dev Keycloak
#
# An existing CA and certificate are kept; delete LOCAL_HTTPS_DIR to start over.
set -eu

here=$(cd "$(dirname "$0")" && pwd)
dir=${LOCAL_HTTPS_DIR:-$HOME/.moh-dev-https}
mkdir -p "$dir"
umask 077

if [ ! -f "$dir/ca.crt" ] || [ ! -f "$dir/ca.key" ]; then
  openssl req -x509 -newkey rsa:2048 -nodes -days 825 -subj "/CN=MOH local dev CA" \
    -keyout "$dir/ca.key" -out "$dir/ca.crt" \
    -addext "basicConstraints=critical,CA:TRUE" -addext "keyUsage=critical,keyCertSign,cRLSign" 2>/dev/null
  rm -f "$dir/leaf.crt"
  echo "created local CA: $dir/ca.crt"
fi

if [ ! -f "$dir/leaf.crt" ] || [ ! -f "$dir/leaf.key" ]; then
  ext=$(mktemp)
  printf '%s\n' "subjectAltName=DNS:localhost,DNS:keycloak,DNS:clims.local.test,IP:127.0.0.1" \
    "basicConstraints=CA:FALSE" "keyUsage=digitalSignature,keyEncipherment" "extendedKeyUsage=serverAuth" > "$ext"
  openssl req -newkey rsa:2048 -nodes -subj "/CN=localhost" -keyout "$dir/leaf.key" -out "$dir/leaf.csr" 2>/dev/null
  openssl x509 -req -in "$dir/leaf.csr" -CA "$dir/ca.crt" -CAkey "$dir/ca.key" -CAcreateserial \
    -days 825 -extfile "$ext" -out "$dir/leaf.crt" 2>/dev/null
  rm -f "$dir/leaf.csr" "$ext"
  echo "created certificate for localhost, keycloak, clims.local.test: $dir/leaf.crt"
fi

# Trust bundle for Go programs in the containers (SSL_CERT_FILE replaces the
# system roots, so it carries them plus the local CA).
if [ -f /System/Library/Keychains/SystemRootCertificates.keychain ]; then
  security find-certificate -a -p /System/Library/Keychains/SystemRootCertificates.keychain > "$dir/bundle.pem"
elif [ -f /etc/ssl/certs/ca-certificates.crt ]; then
  cat /etc/ssl/certs/ca-certificates.crt > "$dir/bundle.pem"
else
  : > "$dir/bundle.pem"
fi
cat "$dir/ca.crt" >> "$dir/bundle.pem"

cp "$here/clims-tls.Caddyfile" "$here/clims-kc-loopback.Caddyfile" "$dir/"

# Containers run as other users (Keycloak uid 1000) and must read these.
# The private key is a throwaway local-only key; the CA key stays private.
chmod 644 "$dir/ca.crt" "$dir/leaf.crt" "$dir/leaf.key" "$dir/bundle.pem" "$dir"/*.Caddyfile
chmod 600 "$dir/ca.key"
chmod 755 "$dir"

if [ "${1:-}" = "--secret" ]; then
  kc=${KEYCLOAK_ADMIN_URL:-http://localhost:8081}
  token=$(curl -fsS -d grant_type=password -d client_id=admin-cli \
      --data-urlencode "username=${KEYCLOAK_ADMIN:-admin}" --data-urlencode "password=${KEYCLOAK_ADMIN_PASSWORD:-admin}" \
      "$kc/realms/master/protocol/openid-connect/token" | python3 -c 'import sys,json;print(json.load(sys.stdin)["access_token"])')
  id=$(curl -fsS -H "Authorization: Bearer $token" "$kc/admin/realms/${KEYCLOAK_REALM:-moh-realm}/clients?clientId=clims" |
    python3 -c 'import sys,json;print(json.load(sys.stdin)[0]["id"])')
  curl -fsS -H "Authorization: Bearer $token" "$kc/admin/realms/${KEYCLOAK_REALM:-moh-realm}/clients/$id/client-secret" |
    python3 -c 'import sys,json;print(json.load(sys.stdin)["value"])' > "$dir/clims-oidc-secret.txt"
  chmod 600 "$dir/clims-oidc-secret.txt"
  echo "saved the clims client secret to $dir/clims-oidc-secret.txt (not displayed)"
fi

echo "ready: $dir"
