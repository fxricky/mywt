#!/usr/bin/env bash
# Release mywt.
#
# Creating the GitHub release is what starts the build. This script checks
# semver, requires HEAD to already be on origin/main, pushes the tag, and
# publishes the release with `gh release create --verify-tag`. GitHub Actions
# (.github/workflows/release.yml) then builds the binaries and uploads them
# onto that release. Pushing a tag by itself does not start the workflow.
#
# Usage:
#   scripts/release.sh <version>    # tag v<version>, push it, and publish the release
#
# <version> is 1.0.0 or v1.0.0. The leading v is optional here.
#
# A local build, with no tag and no GitHub release, is:
#   scripts/build-release.sh v0.0.0-dev
#
# Prerequisites:
#   - A clean working tree.
#   - HEAD is an ancestor of origin/main (including the tip of main).
#   - gh, authenticated to fxricky/mywt.
#   - Permission to push tags and create releases.

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
if ! command -v gh >/dev/null 2>&1; then
  echo "error: gh is required to create the GitHub release" >&2
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

create_args=(--verify-tag --title "$TAG" --generate-notes)
ver="${TAG#v}"
ver="${ver%%+*}"
if [[ "$ver" == *-* ]]; then
  echo "==> Publishing ${TAG} as a prerelease."
  create_args+=(--prerelease)
else
  echo "==> Publishing ${TAG}."
fi
gh release create "$TAG" "${create_args[@]}"
echo "    GitHub Actions will test, build the macOS binaries, and upload them to this release."
echo "    Install with: curl --proto '=https' --tlsv1.2 -fsSL https://raw.githubusercontent.com/fxricky/mywt/main/scripts/install.sh | bash"
