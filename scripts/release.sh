#!/usr/bin/env bash
# Release mywt.
#
# Pushing a vMAJOR.MINOR.PATCH tag is what publishes a release. GitHub Actions
# (.github/workflows/release.yml) builds the binaries and then publishes the
# draft. This script does not build binaries or create the GitHub release, so
# a local tag push and Actions do not publish twice.
#
# Usage:
#   scripts/release.sh <version>    # tag v<version> and push it
#
# <version> is 1.0.0 or v1.0.0. The leading v is optional here.
#
# A local build, with no tag and no GitHub release, is:
#   scripts/build-release.sh v0.0.0-dev
#
# Tagging is refused unless HEAD is already on origin/main. The workflow
# checks the same thing before it builds or publishes.
#
# Prerequisites for tagging:
#   - A clean working tree.
#   - HEAD is an ancestor of origin/main (including the tip of main).
#   - Permission to push tags to fxricky/mywt.

set -euo pipefail

RAW="${1:?usage: release.sh <version> (e.g. 1.0.0 or v1.0.0)}"
VERSION="${RAW#v}"
TAG="v${VERSION}"

# Semver: MAJOR.MINOR.PATCH, optional pre-release and build metadata.
# v0.0.0-test is valid. v1.2 and v1.0.0.1 are not.
if ! printf '%s\n' "$TAG" | grep -Eq '^v(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)(-[0-9A-Za-z-]+(\.[0-9A-Za-z-]+)*)?(\+[0-9A-Za-z-]+(\.[0-9A-Za-z-]+)*)?$'; then
  echo "error: ${RAW} is not a valid semver tag (expected vMAJOR.MINOR.PATCH, optional -prerelease)" >&2
  exit 1
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
