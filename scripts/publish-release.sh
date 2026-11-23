#!/usr/bin/env bash
# Create a draft GitHub release for a tag that already exists, upload
# dist/mywt_darwin_arm64, dist/mywt_darwin_amd64, and dist/mywt_checksums.txt,
# check those uploads, then publish the draft.
#
# The tag is created before this script runs. The Release workflow pushes it
# with the workflow GITHUB_TOKEN. This script does not create, move, or
# delete tags.
#
# Fails if a release for the tag already exists, including a draft. It does
# not edit or delete that release, and the upload does not pass --clobber.
# A semver pre-release tag is created with --prerelease. The draft is
# published only after the downloaded assets match dist/.

set -euo pipefail

tag="${1:?usage: publish-release.sh <tag>}"

if ! printf '%s\n' "$tag" | grep -Eq '^v(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)(-[0-9A-Za-z-]+(\.[0-9A-Za-z-]+)*)?(\+[0-9A-Za-z-]+(\.[0-9A-Za-z-]+)*)?$'; then
  echo "error: ${tag} is not a valid semver tag; refusing to publish" >&2
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

work="$(mktemp -d)"
trap 'rm -rf "$work"' EXIT

if gh release view "$tag" --json tagName >/dev/null 2>"${work}/view.err"; then
  echo "error: release ${tag} already exists; refusing to modify it" >&2
  exit 1
fi
if ! grep -Eqi 'release not found' "${work}/view.err"; then
  echo "error: could not confirm release ${tag} is absent; refusing to publish" >&2
  cat "${work}/view.err" >&2
  exit 1
fi

create_args=(--draft --verify-tag --title "$tag" --generate-notes)
ver="${tag#v}"
ver="${ver%%+*}"
if [[ "$ver" == *-* ]]; then
  echo "Creating draft prerelease ${tag}."
  create_args+=(--prerelease)
else
  echo "Creating draft release ${tag}."
fi
gh release create "$tag" "${create_args[@]}"

gh release view "$tag" --json assets --jq '.assets[].name' >"${work}/assets"
for name in "${names[@]}"; do
  if grep -Fxq "$name" "${work}/assets"; then
    echo "error: asset ${name} already exists on release ${tag}; refusing to overwrite it" >&2
    exit 1
  fi
done

echo "Uploading assets to draft release ${tag}."
gh release upload "$tag" "${assets[@]}"

mkdir -p "${work}/download"
gh release download "$tag" --dir "${work}/download" \
  --pattern 'mywt_darwin_arm64' \
  --pattern 'mywt_darwin_amd64' \
  --pattern 'mywt_checksums.txt'
for name in "${names[@]}"; do
  if [ ! -f "${work}/download/${name}" ]; then
    echo "error: downloaded release is missing ${name}" >&2
    exit 1
  fi
  if ! cmp -s "dist/${name}" "${work}/download/${name}"; then
    echo "error: uploaded ${name} does not match the built file" >&2
    exit 1
  fi
done

echo "Publishing ${tag}."
gh release edit "$tag" --draft=false
echo "Published ${tag}. Uploaded assets match the built files."
