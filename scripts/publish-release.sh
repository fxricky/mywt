#!/usr/bin/env bash
# Upload dist/mywt_darwin_arm64, dist/mywt_darwin_amd64, and
# dist/mywt_checksums.txt onto a GitHub release that already exists.
# Those files come from scripts/build-release.sh.
#
# Does not create, edit, or delete the release, so a prerelease flag, title,
# and notes stay as they are. Fails if one of the asset names is already
# present. After upload, downloads the assets and compares them to dist/.

set -euo pipefail

tag="${1:?usage: publish-release.sh <tag>}"

if ! printf '%s\n' "$tag" | grep -Eq '^v(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)(-[0-9A-Za-z-]+(\.[0-9A-Za-z-]+)*)?(\+[0-9A-Za-z-]+(\.[0-9A-Za-z-]+)*)?$'; then
  echo "error: ${tag} is not a valid semver tag; refusing to upload" >&2
  exit 1
fi

cd "$(dirname "$0")/.."

names=(
  mywt_darwin_arm64
  mywt_darwin_amd64
  mywt_checksums.txt
)
assets=()
for name in "${names[@]}"; do
  assets+=("dist/${name}")
  if [ ! -f "dist/${name}" ]; then
    echo "error: missing dist/${name}; run scripts/build-release.sh first" >&2
    exit 1
  fi
done

(
  cd dist
  if command -v shasum >/dev/null 2>&1; then
    shasum -a 256 -c mywt_checksums.txt
  else
    sha256sum -c mywt_checksums.txt
  fi
)

if ! gh release view "$tag" >/dev/null 2>&1; then
  echo "error: release ${tag} does not exist; refusing to create one" >&2
  exit 1
fi

existing="$(gh release view "$tag" --json assets --jq '.assets[].name')"
for name in "${names[@]}"; do
  if printf '%s\n' "$existing" | grep -Fxq "$name"; then
    echo "error: asset ${name} already exists on release ${tag}; refusing to overwrite it" >&2
    exit 1
  fi
done

echo "Uploading assets to existing release ${tag}."
gh release upload "$tag" "${assets[@]}"

tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT
gh release download "$tag" --dir "$tmp" \
  --pattern 'mywt_darwin_arm64' \
  --pattern 'mywt_darwin_amd64' \
  --pattern 'mywt_checksums.txt'
for name in "${names[@]}"; do
  if ! cmp -s "dist/${name}" "${tmp}/${name}"; then
    echo "error: uploaded ${name} does not match the built file" >&2
    exit 1
  fi
done
echo "Uploaded assets match the built checksums."
