#!/bin/sh

set -eu

usage() {
  echo "usage: $0 [patch|minor|major]" >&2
}

bump=${1:-patch}

case "$bump" in
  patch|minor|major) ;;
  *)
    usage
    exit 2
    ;;
esac

latest_tag=$(git tag --list 'backend/v[0-9]*.[0-9]*.[0-9]*' --sort=-v:refname | head -n 1)
repo_root=$(CDPATH= cd -- "$(dirname "$0")/../.." && pwd)
prepared_version=$(cat "$repo_root/backend/VERSION")
latest_version=${latest_tag#backend/v}
baseline=$(printf '%s\n%s\n' "$latest_version" "$prepared_version" | awk -F '[.-]' '
  /^[0-9]+\.[0-9]+\.[0-9]+/ {
    if (!found || $1 > major || ($1 == major && $2 > minor) || ($1 == major && $2 == minor && $3 > patch)) {
      major=$1; minor=$2; patch=$3; found=1
    }
  }
  END { printf "%d.%d.%d", major, minor, patch }
')

major=${baseline%%.*}
rest=${baseline#*.}
minor=${rest%%.*}
patch=${rest#*.}

case "$bump" in
  patch)
    patch=$((patch + 1))
    ;;
  minor)
    minor=$((minor + 1))
    patch=0
    ;;
  major)
    major=$((major + 1))
    minor=0
    patch=0
    ;;
esac

printf '%s.%s.%s\n' "$major" "$minor" "$patch"
