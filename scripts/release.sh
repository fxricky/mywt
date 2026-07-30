#!/usr/bin/env bash
# Release mywt: build macOS binaries (arm64 + amd64), tag the repo, push the
# tag, and create a GitHub Release with the binaries and a checksums file.
#
# Usage:
#   scripts/release.sh <version>          # e.g. scripts/release.sh 0.1.0
#
# Prerequisites:
#   - A clean working tree on the main branch of the mywt repo.
#   - Go installed locally.
#   - `gh` (GitHub CLI) authenticated with push/release rights to fxricky/mywt.

set -euo pipefail

VERSION="${1:?usage: release.sh <version> (e.g. 0.1.0)}"
VERSION="${VERSION#v}"   # strip a leading 'v' if the caller passed one
TAG="v${VERSION}"

if ! command -v gh >/dev/null 2>&1; then
  echo "error: 'gh' (GitHub CLI) is required to create the release" >&2
  exit 1
fi

echo "==> Building macOS binaries into dist/"
LDFLAGS="-s -w -X github.com/fxricky/mywt/internal/version.version=${TAG}"
mkdir -p dist
GOOS=darwin GOARCH=arm64 go build -trimpath -ldflags "${LDFLAGS}" -o dist/mywt_darwin_arm64 .
GOOS=darwin GOARCH=amd64 go build -trimpath -ldflags "${LDFLAGS}" -o dist/mywt_darwin_amd64 .

echo "==> Generating checksums"
( cd dist && shasum -a 256 mywt_darwin_arm64 mywt_darwin_amd64 > mywt_checksums.txt )
cat dist/mywt_checksums.txt

echo "==> Tagging ${TAG}"
git tag "${TAG}"
git push origin "${TAG}"

echo "==> Creating GitHub release ${TAG}"
gh release create "${TAG}" \
  dist/mywt_darwin_arm64 \
  dist/mywt_darwin_amd64 \
  dist/mywt_checksums.txt \
  --title "mywt ${TAG}" \
  --generate-notes

echo "==> Done."
echo "    Install with: curl -fsSL https://raw.githubusercontent.com/fxricky/mywt/main/scripts/install.sh | bash"
