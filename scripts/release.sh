#!/usr/bin/env bash
# Release mywt.
#
# Pushing a vMAJOR.MINOR.PATCH tag is what publishes a release. GitHub Actions
# (.github/workflows/release.yml) builds the macOS binaries and creates or
# updates the GitHub release. This script does not create the release, so a
# local tag push and Actions do not publish twice.
#
# Usage:
#   scripts/release.sh <version>                 # tag v<version> and push it
#   scripts/release.sh --build-only <version>    # write dist/ only
#
# <version> is 1.0.0 or v1.0.0. The leading v is optional here.
#
# --build-only is what Actions runs after the tag push. It uses the same
# ldflags, binary names, and checksum file as a local build, and it does not
# create a tag or a release. CGO is disabled and -buildvcs=false so the same
# command cross-compiles on the Ubuntu runner; mywt has no cgo code.
#
# Tagging is refused unless HEAD is already on origin/main. The workflow
# checks the same thing before it builds or publishes.
#
# Prerequisites for tagging:
#   - A clean working tree.
#   - HEAD is an ancestor of origin/main (including the tip of main).
#   - Permission to push tags to fxricky/mywt.

set -euo pipefail

BUILD_ONLY=0
if [ "${1:-}" = "--build-only" ]; then
  BUILD_ONLY=1
  shift
fi

RAW="${1:?usage: release.sh [--build-only] <version> (e.g. 1.0.0 or v1.0.0)}"
VERSION="${RAW#v}"
TAG="v${VERSION}"

# Semver: MAJOR.MINOR.PATCH, optional pre-release and build metadata.
# v0.0.0-test is valid. v1.2 and v1.0.0.1 are not.
if ! printf '%s\n' "$TAG" | grep -Eq '^v(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)(-[0-9A-Za-z-]+(\.[0-9A-Za-z-]+)*)?(\+[0-9A-Za-z-]+(\.[0-9A-Za-z-]+)*)?$'; then
  echo "error: ${RAW} is not a valid semver tag (expected vMAJOR.MINOR.PATCH, optional -prerelease)" >&2
  exit 1
fi

build_release() {
  echo "==> Building macOS binaries into dist/ (${TAG})"
  local ldflags="-s -w -X github.com/fxricky/mywt/internal/version.version=${TAG}"
  mkdir -p dist
  CGO_ENABLED=0 GOOS=darwin GOARCH=arm64 go build -trimpath -buildvcs=false -ldflags "${ldflags}" -o dist/mywt_darwin_arm64 .
  CGO_ENABLED=0 GOOS=darwin GOARCH=amd64 go build -trimpath -buildvcs=false -ldflags "${ldflags}" -o dist/mywt_darwin_amd64 .

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
}

if [ "$BUILD_ONLY" -eq 1 ]; then
  build_release
  exit 0
fi

if ! git diff --quiet || ! git diff --cached --quiet; then
  echo "error: working tree is not clean; commit or stash before tagging ${TAG}" >&2
  exit 1
fi
if git rev-parse -q --verify "refs/tags/${TAG}" >/dev/null; then
  echo "error: tag ${TAG} already exists" >&2
  exit 1
fi

echo "==> Checking HEAD is on origin/main"
git fetch origin main
if ! git merge-base --is-ancestor HEAD origin/main; then
  echo "error: HEAD $(git rev-parse HEAD) is not on origin/main; refusing to tag ${TAG}" >&2
  exit 1
fi

echo "==> Tagging ${TAG}"
git tag "${TAG}"
git push origin "${TAG}"
echo "==> Pushed ${TAG}."
echo "    GitHub Actions will test, build the macOS binaries, and publish the release."
echo "    Install with: curl --proto '=https' --tlsv1.2 -fsSL https://raw.githubusercontent.com/fxricky/mywt/main/scripts/install.sh | bash"
