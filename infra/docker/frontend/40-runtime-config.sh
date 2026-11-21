#!/bin/sh
set -eu
# Only these public settings reach the browser. jq escapes all values as JSON.
config="$(jq -cn \
  --arg api "${API_BASE_URL:-/api}" \
  --arg mode "${FRONTEND_MICROFRONTEND_MODE:-remote}" \
  --arg orchestration "${FRONTEND_SINGLE_SPA_ORCHESTRATION:-true}" \
  --arg mount "${FRONTEND_MICROFRONTEND_MOUNT_MODE:-orchestrated}" \
  --arg docking "${FRONTEND_SIDE_PANEL_DOCKING:-true}" \
  '{API_BASE_URL:$api,microfrontendMode:$mode,singleSpaOrchestration:($orchestration == "true"),microfrontendMountMode:$mount,sidePanelDocking:($docking != "false")}')"
write_config() {
  # Helm supplies an existing config.js through a read-only ConfigMap mount.
  if [ -f "$1" ] && [ ! -w "$1" ]; then
    return
  fi
  mkdir -p "$(dirname "$1")"
  printf '/* generated from public container settings */\nwindow.__APP_CONFIG__ = %s;\n' "$config" > "$1"
}
if [ "$#" -gt 0 ]; then
  write_config "$1"
else
  write_config /usr/share/nginx/html/config.js
  write_config /usr/share/nginx/html/portal/config.js
fi
