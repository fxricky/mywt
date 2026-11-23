#!/usr/bin/env bash
# Start the Release workflow on main.
#
# GitHub Actions creates the tag, builds the macOS binaries, and publishes
# the release. This script checks semver and dispatches that workflow. It
# does not create a tag, push a ref, or create a release.
#
# From the Actions tab, Release, then Run workflow, does the same thing.
#
# Usage:
#   scripts/release.sh <version>    # start Release for v<version> on main
#
# <version> is 1.1.0 or v1.1.0. The leading v is optional here.
#
# A local build, with no tag and no GitHub release, is:
#   scripts/build-release.sh v0.0.0-dev
#
# Prerequisites:
#   - gh, authenticated to fxricky/mywt, with permission to dispatch workflows.

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

if ! command -v gh >/dev/null 2>&1; then
  echo "error: gh is required to start the Release workflow" >&2
  exit 1
fi

cd "$(dirname "$0")/.."

echo "==> Starting the Release workflow for ${TAG} on main"
gh workflow run release.yml --ref main -f "version=${TAG}"
echo "    The release environment must be approved before Actions tags, builds, and publishes."
echo "    Install with: curl --proto '=https' --tlsv1.2 -fsSL https://raw.githubusercontent.com/fxricky/mywt/main/scripts/install.sh | bash"
