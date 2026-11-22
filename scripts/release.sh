#!/usr/bin/env bash
# Tag a commit that is already on origin/main and push the tag.
#
# Pushing vX.Y.Z starts GitHub Actions (.github/workflows/release.yml).
# That workflow builds the macOS binaries and publishes the GitHub release.
# This script does not call gh and does not create or edit a release.
#
# Run workflow from the Actions tab is the other way to start that same
# workflow. It creates the tag itself. Use one of the two, not both, for a
# given version.
#
# Usage:
#   scripts/release.sh <version>    # tag v<version> and push it
#
# <version> is 1.0.0 or v1.0.0. The leading v is optional here.
#
# A local build, with no tag and no GitHub release, is:
#   scripts/build-release.sh v0.0.0-dev
#
# Prerequisites:
#   - A clean working tree.
#   - HEAD is an ancestor of origin/main (including the tip of main).
#   - Permission to push the tag.

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
echo "    GitHub Actions will test, build the macOS binaries, and publish the release."
echo "    Install with: curl --proto '=https' --tlsv1.2 -fsSL https://raw.githubusercontent.com/fxricky/mywt/main/scripts/install.sh | bash"
