#!/bin/sh
set -eu

# Only a clean checkout at a backend release tag is a release build.
repo_root=$(CDPATH= cd -- "$(dirname "$0")/../.." && pwd)
cd "$repo_root"
tag=$(git describe --tags --match 'backend/v[0-9]*' --exact-match HEAD 2>/dev/null || true)
version=${tag#backend/v}
if [ -n "$tag" ] && [ -z "$(git status --porcelain)" ] &&
  printf '%s' "$version" | grep -Eq '^(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)(-[0-9A-Za-z.-]+)?$'; then
  printf '%s\n' "$version"
else
  commit=$(git rev-parse --short=7 HEAD 2>/dev/null || printf 'unknown')
  printf '0.0.0-dev.%s\n' "$commit"
fi
