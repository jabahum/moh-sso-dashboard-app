#!/bin/sh
set -eu
# File-backed secrets take precedence over env_file placeholders. Never print values.
for key in OBSERVATORY_DB_PASSWORD DB_PASSWORD KEYCLOAK_ADMIN_CLIENT_SECRET KEYCLOAK_WEB_CLIENT_SECRET \
  PLATFORM_AUTH_CLIENT_SECRET HEALTH_BI_KEYCLOAK_CLIENT_SECRET TOKEN_SYMMETRIC_KEY REDIS_PASSWORD \
  REMOTE_DB_PASSWORD DWH_PASSWORD DQA_DB_PASSWORD SMTP_PASSWORD SMTP_PASSWORD_GM \
  S3_SECRET_ACCESS_KEY MINIO_SECRET_ACCESS_KEY AFRICASTALKING_API_KEY TWILIO_AUTH_TOKEN; do
  # Core reads only mandatory capability secrets; optional provider mounts are
  # not prerequisites for this profile.
  profile="$(printf '%s' "${PLATFORM_PROFILE:-full}" | tr '[:upper:]' '[:lower:]' | tr -d '[:space:]')"
  if [ "$profile" = core ]; then
    case "$key" in
      DB_PASSWORD|KEYCLOAK_ADMIN_CLIENT_SECRET|KEYCLOAK_WEB_CLIENT_SECRET|PLATFORM_AUTH_CLIENT_SECRET|TOKEN_SYMMETRIC_KEY|REDIS_PASSWORD) ;;
      *) continue ;;
    esac
  fi
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
# Identity-sync client secrets, one per registered application, are referenced
# from the registry as env:<APP>_SYNC_CLIENT_SECRET. Load any mounted
# <APP>_SYNC_CLIENT_SECRET_FILE the same way, so they can be root-owned files.
for key in $(env | sed -n 's/^\([A-Z][A-Z0-9_]*_SYNC_CLIENT_SECRET\)_FILE=.*/\1/p'); do
  path="$(printenv "${key}_FILE")"
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
done
# Production reads root-owned Compose secret files before dropping privileges.
# Development leaves SSO_RUNTIME_USER unset so Air can build bind-mounted code.
if [ "$(id -u)" = 0 ] && [ -n "${SSO_RUNTIME_USER:-}" ]; then
  exec su-exec "$SSO_RUNTIME_USER" "$@"
fi
exec "$@"
