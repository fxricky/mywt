#!/usr/bin/env bash
# Publish dist/ from `goreleaser release --clean --skip=publish`.
#
# GoReleaser names the release assets mywt_darwin_arm64 and mywt_darwin_amd64,
# but the built files stay under dist/<target>/mywt. This copies them to those
# asset names, creates a draft with --verify-tag, uploads, checks the uploaded
# bytes, and only then publishes.
#
# An existing release is left untouched. Asset names that already exist are
# not replaced, because this script never uploads onto an existing release.

set -euo pipefail

tag="${1:?usage: publish-release.sh <tag>}"

if ! printf '%s\n' "$tag" | grep -Eq '^v(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)(-[0-9A-Za-z-]+(\.[0-9A-Za-z-]+)*)?(\+[0-9A-Za-z-]+(\.[0-9A-Za-z-]+)*)?$'; then
  echo "error: ${tag} is not a valid semver tag; refusing to publish" >&2
  exit 1
fi

cd "$(dirname "$0")/.."

stage="$(mktemp -d)"
tmp=""
cleanup() {
  rm -rf "$stage"
  if [ -n "$tmp" ]; then
    rm -rf "$tmp"
  fi
}
trap cleanup EXIT

python3 - "$stage" <<'PY'
import json, os, shutil, sys
stage = sys.argv[1]
arts = json.load(open("dist/artifacts.json"))
wanted = {"mywt_darwin_arm64", "mywt_darwin_amd64"}
found = set()
for art in arts:
    extra = art.get("extra") or {}
    if extra.get("Format") != "binary":
        continue
    name = art["name"]
    path = art["path"]
    if not os.path.isfile(path):
        alt = os.path.join("dist", path.removeprefix("dist/"))
        if os.path.isfile(alt):
            path = alt
        else:
            sys.exit(f"error: missing built binary {art['path']}")
    if name not in wanted:
        sys.exit(f"error: unexpected release binary {name}")
    shutil.copy2(path, os.path.join(stage, name))
    found.add(name)
missing = wanted - found
if missing:
    sys.exit("error: missing release binaries: " + ", ".join(sorted(missing)))
checksum = "dist/mywt_checksums.txt"
if not os.path.isfile(checksum):
    sys.exit("error: missing dist/mywt_checksums.txt")
shutil.copy2(checksum, os.path.join(stage, "mywt_checksums.txt"))
PY

(
  cd "$stage"
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
gh release upload "$tag" \
  "$stage/mywt_darwin_arm64" \
  "$stage/mywt_darwin_amd64" \
  "$stage/mywt_checksums.txt"

tmp="$(mktemp -d)"
gh release download "$tag" --dir "$tmp" \
  --pattern 'mywt_darwin_arm64' \
  --pattern 'mywt_darwin_amd64' \
  --pattern 'mywt_checksums.txt'
for name in mywt_darwin_arm64 mywt_darwin_amd64 mywt_checksums.txt; do
  if ! cmp -s "$stage/$name" "$tmp/$name"; then
    echo "error: uploaded ${name} does not match the built file" >&2
    exit 1
  fi
done
echo "Uploaded assets match the built checksums."
echo "Publishing ${tag}."
gh release edit "$tag" "${edit_args[@]}"
