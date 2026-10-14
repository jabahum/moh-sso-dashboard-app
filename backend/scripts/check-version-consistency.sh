#!/bin/sh

set -eu

repo_root=$(CDPATH= cd -- "$(dirname "$0")/../.." && pwd)
backend_dir="$repo_root/backend"
module=$(awk '$1 == "module" { print $2; exit }' "$backend_dir/go.mod")
go_version=$(awk '$1 == "go" { print $2; exit }' "$backend_dir/go.mod")
version_package="$module/internal/version"
prepared_version=$(cat "$backend_dir/VERSION")
if ! printf '%s' "$prepared_version" | grep -Eq '^(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)(-[0-9A-Za-z.-]+)?$'; then
  echo "backend/VERSION must contain a semantic version" >&2
  exit 1
fi

require_text() {
  file=$1
  text=$2
  if ! grep -F "$text" "$file" >/dev/null; then
    echo "version consistency check failed: $file does not contain $text" >&2
    exit 1
  fi
}

require_text "$backend_dir/Makefile" "VERSION_PACKAGE = $version_package"
require_text "$backend_dir/Makefile" 'sh ./scripts/build-version.sh'
require_text "$backend_dir/Dockerfile" "$version_package.Version="
require_text "$repo_root/.github/workflows/backend-release.yml" "VERSION_PACKAGE: $version_package"
require_text "$repo_root/.github/workflows/backend-release-pr.yml" "VERSION_PACKAGE: $version_package"
require_text "$backend_dir/Dockerfile" "golang:$go_version-alpine"
require_text "$backend_dir/Dockerfile.dev" "golang:$go_version-alpine"
require_text "$repo_root/.github/workflows/build.yml" 'backend/v*.*.*'
require_text "$repo_root/.github/workflows/build.yml" 'frontend/v*.*.*'
require_text "$repo_root/.github/workflows/backend-release.yml" 'tag_name: backend/v${{ steps.version.outputs.version }}'
require_text "$repo_root/.github/workflows/backend-release.yml" 'TAG="backend/v${{ steps.version.outputs.version }}"'
require_text "$repo_root/.github/workflows/backend-release-pr.yml" 'tag_name: backend/v${{ steps.version.outputs.version }}'
require_text "$repo_root/.github/workflows/backend-release-pr.yml" 'TAG="backend/v${{ steps.version.outputs.version }}"'

if grep -F 'moh-sso-dashboard-backend:${BACKEND_TAG:-latest}' \
  "$repo_root/docker-compose.yml" "$repo_root/docker-compose-nginx.yml" >/dev/null; then
  echo "version consistency check failed: production backend falls back to latest" >&2
  exit 1
fi

echo "Backend version configuration is consistent ($version_package, Go $go_version)."
