#!/usr/bin/env bash
# Publish dist/mywt_darwin_arm64, dist/mywt_darwin_amd64, and
# dist/mywt_checksums.txt. Those files come from scripts/build-release.sh.
#
# Creates a draft with --verify-tag, uploads, checks the uploaded bytes, and
# only then publishes. An existing release is left untouched.

set -euo pipefail

tag="${1:?usage: publish-release.sh <tag>}"

if ! printf '%s\n' "$tag" | grep -Eq '^v(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)(-[0-9A-Za-z-]+(\.[0-9A-Za-z-]+)*)?(\+[0-9A-Za-z-]+(\.[0-9A-Za-z-]+)*)?$'; then
  echo "error: ${tag} is not a valid semver tag; refusing to publish" >&2
  exit 1
fi

cd "$(dirname "$0")/.."

assets=(
  dist/mywt_darwin_arm64
  dist/mywt_darwin_amd64
  dist/mywt_checksums.txt
)
for path in "${assets[@]}"; do
  if [ ! -f "$path" ]; then
    echo "error: missing ${path}; run scripts/build-release.sh first" >&2
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

if gh release view "$tag" >/dev/null 2>&1; then
  echo "error: release ${tag} already exists; refusing to modify it" >&2
  exit 1
fi

version="${tag#v}"
version="${version%%+*}"
create_args=(--verify-tag --draft --title "$tag" --generate-notes)
edit_args=(--draft=false)
if [[ "$version" == *-* ]]; then
  echo "Marking ${tag} as a prerelease so it does not become the latest release."
  create_args+=(--prerelease)
  edit_args+=(--prerelease)
fi

echo "Creating draft release ${tag}."
gh release create "$tag" "${create_args[@]}"
gh release upload "$tag" "${assets[@]}"

tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT
gh release download "$tag" --dir "$tmp" \
  --pattern 'mywt_darwin_arm64' \
  --pattern 'mywt_darwin_amd64' \
  --pattern 'mywt_checksums.txt'
for name in mywt_darwin_arm64 mywt_darwin_amd64 mywt_checksums.txt; do
  if ! cmp -s "dist/${name}" "${tmp}/${name}"; then
    echo "error: uploaded ${name} does not match the built file" >&2
    exit 1
  fi
done
echo "Uploaded assets match the built checksums."
echo "Publishing ${tag}."
gh release edit "$tag" "${edit_args[@]}"
