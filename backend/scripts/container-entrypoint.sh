#!/bin/sh
set -eu
# File-backed secrets take precedence over env_file placeholders. Never print values.
for key in DB_PASSWORD KEYCLOAK_ADMIN_CLIENT_SECRET KEYCLOAK_WEB_CLIENT_SECRET \
  HEALTH_BI_KEYCLOAK_CLIENT_SECRET TOKEN_SYMMETRIC_KEY REDIS_PASSWORD \
  REMOTE_DB_PASSWORD DWH_PASSWORD DQA_DB_PASSWORD SMTP_PASSWORD SMTP_PASSWORD_GM \
  S3_SECRET_ACCESS_KEY MINIO_SECRET_ACCESS_KEY AFRICASTALKING_API_KEY TWILIO_AUTH_TOKEN; do
  path="$(printenv "${key}_FILE" || true)"
  if [ -n "$path" ]; then
    if [ ! -r "$path" ]; then
      echo "Cannot read mounted secret for $key" >&2
      exit 1
    fi
    value="$(cat "$path")"
    if [ -z "$value" ]; then
      echo "Mounted secret for $key is empty" >&2
      exit 1
    fi
    export "$key=$value"
  fi
done
# Production reads root-owned Compose secret files before dropping privileges.
# Development leaves SSO_RUNTIME_USER unset so Air can build bind-mounted code.
if [ "$(id -u)" = 0 ] && [ -n "${SSO_RUNTIME_USER:-}" ]; then
  exec su-exec "$SSO_RUNTIME_USER" "$@"
fi
exec "$@"
