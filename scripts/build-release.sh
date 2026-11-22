#!/usr/bin/env bash
# Build the macOS release binaries into dist/. Does not tag or publish.
#
# Usage:
#   scripts/build-release.sh <tag>
#
# <tag> is v1.0.0 or 1.0.0. The leading v is optional. The value injected
# into the binary is the v-prefixed tag. A local dry run can use v0.0.0-dev.

set -euo pipefail

RAW="${1:?usage: build-release.sh <tag> (e.g. v1.0.0 or v0.0.0-dev)}"
VERSION="${RAW#v}"
TAG="v${VERSION}"

if ! printf '%s\n' "$TAG" | grep -Eq '^v(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)(-[0-9A-Za-z-]+(\.[0-9A-Za-z-]+)*)?(\+[0-9A-Za-z-]+(\.[0-9A-Za-z-]+)*)?$'; then
  echo "error: ${RAW} is not a valid semver tag (expected vMAJOR.MINOR.PATCH, optional -prerelease)" >&2
  exit 1
fi

cd "$(dirname "$0")/.."
mkdir -p dist

echo "==> Building macOS binaries into dist/ (${TAG})"
LDFLAGS="-s -w -X github.com/fxricky/mywt/internal/version.version=${TAG}"
CGO_ENABLED=0 GOOS=darwin GOARCH=arm64 go build -trimpath -buildvcs=false -ldflags "${LDFLAGS}" -o dist/mywt_darwin_arm64 .
CGO_ENABLED=0 GOOS=darwin GOARCH=amd64 go build -trimpath -buildvcs=false -ldflags "${LDFLAGS}" -o dist/mywt_darwin_amd64 .

echo "==> Generating checksums"
(
  cd dist
  if command -v shasum >/dev/null 2>&1; then
    shasum -a 256 mywt_darwin_arm64 mywt_darwin_amd64 > mywt_checksums.txt
  else
    sha256sum mywt_darwin_arm64 mywt_darwin_amd64 > mywt_checksums.txt
  fi
)
cat dist/mywt_checksums.txt
