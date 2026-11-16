#!/bin/sh

set -eu

usage() {
  echo "usage: $0 <version>" >&2
  echo "example: $0 1.2.3" >&2
}

if [ "$#" -ne 1 ]; then
  usage
  exit 2
fi

version=${1#backend/v}
version=${version#v}

if ! printf '%s' "$version" | grep -Eq '^[0-9]+\.[0-9]+\.[0-9]+(-[0-9A-Za-z.-]+)?$'; then
  echo "invalid backend semantic version: $version" >&2
  exit 1
fi

tag="backend/v$version"

if git rev-parse "refs/tags/$tag" >/dev/null 2>&1; then
  echo "release check failed: tag already exists: $tag" >&2
  exit 1
fi

if [ -n "$(git status --porcelain)" ]; then
  echo "release check failed: worktree must be clean for release verification" >&2
  git status --short
  exit 1
fi

commit=$(git rev-parse HEAD)
build_time=$(date -u +'%Y-%m-%dT%H:%M:%SZ')

echo "Checking backend release $tag at $commit"

./scripts/check-version-consistency.sh

unformatted=$(gofmt -l .)
if [ -n "$unformatted" ]; then
  echo "release check failed: Go files need formatting" >&2
  echo "$unformatted" >&2
  exit 1
fi

go vet ./...
make modules-check
go test ./...
go test ./internal/architecture

make build-release \
  VERSION="$version" \
  COMMIT="$commit" \
  BUILD_TIME="$build_time" \
  DIRTY=false

echo "Backend release $tag is locally verified."
