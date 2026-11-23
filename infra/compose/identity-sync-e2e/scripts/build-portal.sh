#!/usr/bin/env bash
# Builds the dashboard portal (frontend/) for the e2e stack and assembles it in
# generated/portal-root, which Caddy serves at https://dashboard.e2e.test:18443/portal/.
#
# - Uses the workspace micro-frontends (no published packages).
# - The production import map loads React and friends from esm.sh. The e2e
#   stack must not reach external services, so, like the frontend's own
#   browser tests (infra/scripts/serve-browser-fixture.py), the import map is
#   rewritten to bundles built from the installed lockfile
#   (frontend/scripts/prepare-browser-dependencies.mjs).
# - config.js points the portal at the same origin (/api), as production does.
#
# The build rewrites two tracked files (public/import-map.json and
# public/version-manifest.json); they are restored afterwards.
# Skips the build when generated/portal-root exists, unless IDSYNC_E2E_REBUILD_PORTAL=1.
set -euo pipefail
here="$(cd "$(dirname "$0")/.." && pwd)"
repo="$(cd "$here/../../.." && pwd)"
fe="$repo/frontend"
out="$here/generated/portal-root"

if [ -f "$out/portal/index.html" ] && [ "${IDSYNC_E2E_REBUILD_PORTAL:-}" != "1" ]; then
  echo "portal: reusing $out (IDSYNC_E2E_REBUILD_PORTAL=1 rebuilds it)"
  exit 0
fi
command -v node >/dev/null || { echo "portal: node is required (Node 22)"; exit 1; }

backup="$(mktemp -d)"
restore() {
  cp "$backup/import-map.json" "$fe/public/import-map.json"
  cp "$backup/version-manifest.json" "$fe/public/version-manifest.json"
  rm -rf "$backup"
}
cp "$fe/public/import-map.json" "$fe/public/version-manifest.json" "$backup/"
trap restore EXIT

cd "$fe"
[ -d node_modules ] || npm ci --no-audit --no-fund
FRONTEND_MODULE_SOURCE=workspace FRONTEND_ASSET_BASE_URL=/portal FRONTEND_BASE_PATH=/portal npm run build:docker
node scripts/prepare-browser-dependencies.mjs

rm -rf "$out.tmp"
mkdir -p "$out.tmp"
cp -R dist/portal "$out.tmp/portal"
cp -R node_modules/.cache/browser-dependencies "$out.tmp/__browser_dependencies__"
python3 -I - "$out.tmp" <<'EOF'
import json, re, sys
from pathlib import Path
root = Path(sys.argv[1])
local = json.loads((root / "__browser_dependencies__/import-map.json").read_text())["imports"]
index = root / "portal/index.html"
def swap(match):
    imports = json.loads(match.group(1))
    imports["imports"].update(local)
    return '<script type="importmap">' + json.dumps(imports) + "</script>"
html, n = re.subn(r'<script type="importmap">(.*?)</script>', swap, index.read_text(), flags=re.S)
if n != 1:
    sys.exit("expected exactly one import map in portal/index.html")
if "esm.sh" in html:
    sys.exit("portal/index.html still references esm.sh")
index.write_text(html)
(root / "portal/config.js").write_text(
    "/* idsync-e2e: same-origin API */\nwindow.__APP_CONFIG__ = "
    + json.dumps({"API_BASE_URL": "/api", "microfrontendMode": "remote", "singleSpaOrchestration": True,
                  "microfrontendMountMode": "orchestrated", "sidePanelDocking": True})
    + ";\n")
print("portal: import map rewritten to", len(local), "local dependency bundles")
EOF
rm -rf "$out"
mv "$out.tmp" "$out"
echo "portal: assembled $out"
