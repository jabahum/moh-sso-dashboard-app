#!/usr/bin/env bash
# One-time application-server preparation. Run explicitly with sudo on Linux.
set -euo pipefail
[[ $(id -u) = 0 ]] || { echo 'Run this setup script as root.' >&2; exit 1; }
DEPLOY_USER=${DEPLOY_USER:-mohdeploy}
DEPLOY_ROOT=${DEPLOY_ROOT:-/opt/moh-sso}
[[ $DEPLOY_USER =~ ^[a-z_][a-z0-9_-]*$ ]] || exit 2
[[ $DEPLOY_ROOT =~ ^/[a-zA-Z0-9_/-]+$ && $DEPLOY_ROOT != / ]] || exit 2
for command in docker python3 ssh-keygen; do command -v "$command" >/dev/null || { echo "Install $command first." >&2; exit 1; }; done
docker compose version >/dev/null
ROOT=$(cd -- "$(dirname -- "$0")/../.." && pwd)
id "$DEPLOY_USER" >/dev/null 2>&1 || useradd --create-home --shell /bin/bash "$DEPLOY_USER"
usermod -aG docker "$DEPLOY_USER"
for directory in "$DEPLOY_ROOT" "$DEPLOY_ROOT/incoming" "$DEPLOY_ROOT/releases" "$DEPLOY_ROOT/state" /etc/moh-sso /etc/moh-sso/secrets /var/backups/moh-sso; do
  install -d -m 0750 -o "$DEPLOY_USER" -g "$DEPLOY_USER" "$directory"
done
for pair in 'infra/server/deploy.json.example:deploy.json' 'infra/env/prod.env.example:prod.env' 'infra/env/remote-proxy.env.example:remote-proxy.env' 'infra/nginx/trusted-proxies.conf:trusted-proxies.conf'; do
  source=${pair%%:*}; destination=/etc/moh-sso/${pair#*:}
  if [[ ! -e $destination ]]; then
    install -m 0600 -o "$DEPLOY_USER" -g "$DEPLOY_USER" "$ROOT/$source" "$destination"
  fi
done
# Configure the actual root in a newly created/default JSON without touching secrets.
python3 - "$DEPLOY_ROOT" <<'PY'
import json,sys
from pathlib import Path
p=Path('/etc/moh-sso/deploy.json')
c=json.loads(p.read_text())
if c['deploy_root']=='/opt/moh-sso':
 c['deploy_root']=sys.argv[1]
 p.write_text(json.dumps(c,indent=2)+'\n')
PY
home=$(getent passwd "$DEPLOY_USER" | cut -d: -f6)
install -d -m 0700 -o "$DEPLOY_USER" -g "$DEPLOY_USER" "$home/.ssh"
touch "$home/.ssh/authorized_keys"
chmod 0600 "$home/.ssh/authorized_keys"
chown "$DEPLOY_USER:$DEPLOY_USER" "$home/.ssh/authorized_keys"
if [[ -n ${DEPLOY_PUBLIC_KEY_FILE:-} ]]; then
  ssh-keygen -lf "$DEPLOY_PUBLIC_KEY_FILE" >/dev/null
  key=$(cat "$DEPLOY_PUBLIC_KEY_FILE")
  grep -qxF "$key" "$home/.ssh/authorized_keys" || printf '%s\n' "$key" >> "$home/.ssh/authorized_keys"
fi
printf 'Prepared %s for %s. Fill /etc/moh-sso/prod.env, deploy.json and secrets; authorize the deployment SSH public key.\n' "$DEPLOY_ROOT" "$DEPLOY_USER"
