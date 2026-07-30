#!/usr/bin/env bash
# Release mywt: tag the source repo, push the tag, and update the Homebrew tap
# formula with the new url + source-tarball sha256.
#
# Usage:
#   scripts/release.sh <version>          # e.g. scripts/release.sh 0.1.0
#   TAP_PATH=/path/to/homebrew-tap scripts/release.sh 0.1.0
#
# Prerequisites:
#   - A clean working tree on the main branch of the mywt repo.
#   - A local checkout of github.com/fxricky/homebrew-tap (default: ../homebrew-tap).
#   - `gh` or push access to both repos.

set -euo pipefail

VERSION="${1:?usage: release.sh <version> (e.g. 0.1.0)}"
# strip a leading 'v' if the caller passed one
VERSION="${VERSION#v}"
TAG="v${VERSION}"
TAP_PATH="${TAP_PATH:-../homebrew-tap}"

echo "==> Tagging ${TAG}"
git tag "${TAG}"
git push origin "${TAG}"

echo "==> Computing source tarball sha256"
URL="https://github.com/fxricky/mywt/archive/refs/tags/${TAG}.tar.gz"
SHA="$(curl -fsSL "${URL}" | shasum -a 256 | awk '{print $1}')"
echo "    sha256 = ${SHA}"

echo "==> Updating formula at ${TAP_PATH}/Formula/mywt.rb"
FORMULA="${TAP_PATH}/Formula/mywt.rb"
if [ ! -f "${FORMULA}" ]; then
  echo "error: formula not found at ${FORMULA}" >&2
  exit 1
fi

# macOS sed requires -i '' (empty backup suffix)
sed -i '' -E "s#url \".*\"#url \"${URL}\"#" "${FORMULA}"
sed -i '' -E "s#sha256 \"[a-f0-9]{64}\"#sha256 \"${SHA}\"#" "${FORMULA}"

echo "==> Committing and pushing the tap"
( cd "${TAP_PATH}" \
  && git add Formula/mywt.rb \
  && git commit -m "mywt ${TAG}" \
  && git push )

echo "==> Done. Install with: brew install fxricky/tap/mywt"
