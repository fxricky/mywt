#!/usr/bin/env bash
# install.sh — install the latest mywt release binary on macOS.
#
# Usage (from the internet):
#   curl -fsSL https://raw.githubusercontent.com/fxricky/mywt/main/scripts/install.sh | bash
#   curl -fsSL https://raw.githubusercontent.com/fxricky/mywt/main/scripts/install.sh | bash -s -- v0.1.0
#   curl -fsSL https://raw.githubusercontent.com/fxricky/mywt/main/scripts/install.sh | bash -s -- --to ~/bin
#
# Options:
#   <version>   a specific tag to install (e.g. v0.1.0); defaults to latest.
#   --to <dir>  install directory; overrides MYWT_INSTALL_DIR.
#
# Environment:
#   MYWT_INSTALL_DIR  install directory (default: /usr/local/bin, or
#                      ~/.local/bin if /usr/local/bin is not writable).

set -euo pipefail

OWNER="fxricky"
REPO="mywt"

# --- parse args -----------------------------------------------------------
VERSION=""
INSTALL_DIR="${MYWT_INSTALL_DIR:-}"
while [ $# -gt 0 ]; do
  case "$1" in
    --to) INSTALL_DIR="${2:?--to requires a directory}"; shift 2 ;;
    -h|--help)
      sed -n '2,16p' "$0" 2>/dev/null || true
      exit 0 ;;
    *) VERSION="$1"; shift ;;
  esac
done

# --- platform checks ------------------------------------------------------
OS="$(uname -s)"
if [ "$OS" != "Darwin" ]; then
  echo "error: mywt is macOS-only for now (detected $(uname -s))" >&2
  exit 1
fi

ARCH="$(uname -m)"
case "$ARCH" in
  arm64|aarch64) ARCH="arm64" ;;
  x86_64|amd64)  ARCH="amd64" ;;
  *) echo "error: unsupported architecture: $ARCH" >&2; exit 1 ;;
esac

ASSET="mywt_darwin_${ARCH}"

# --- resolve version ------------------------------------------------------
if [ -z "$VERSION" ]; then
  echo "==> Fetching latest release tag"
  VERSION="$(curl -fsSL "https://api.github.com/repos/${OWNER}/${REPO}/releases/latest" \
    | awk -F'"' '/"tag_name"/{print $4; exit}')"
  if [ -z "$VERSION" ]; then
    echo "error: could not determine latest release tag" >&2
    exit 1
  fi
fi
VERSION="${VERSION#v}"  # normalise; URLs below use the v-prefixed tag
TAG="v${VERSION}"

echo "==> Installing mywt ${TAG} for darwin/${ARCH}"

# --- download binary + checksums -----------------------------------------
TMP="$(mktemp -d)"
trap 'rm -rf "$TMP"' EXIT

BASE="https://github.com/${OWNER}/${REPO}/releases/download/${TAG}"
echo "==> Downloading ${ASSET}"
curl -fsSL -o "${TMP}/${ASSET}" "${BASE}/${ASSET}"

echo "==> Verifying checksum"
curl -fsSL -o "${TMP}/mywt_checksums.txt" "${BASE}/mywt_checksums.txt"
EXPECTED="$(awk -v f="${ASSET}" '$2==f{print $1}' "${TMP}/mywt_checksums.txt")"
if [ -z "$EXPECTED" ]; then
  echo "error: no checksum found for ${ASSET} in mywt_checksums.txt" >&2
  exit 1
fi
ACTUAL="$(shasum -a 256 "${TMP}/${ASSET}" | awk '{print $1}')"
if [ "$ACTUAL" != "$EXPECTED" ]; then
  echo "error: checksum mismatch for ${ASSET}" >&2
  echo "  expected: ${EXPECTED}" >&2
  echo "  actual:   ${ACTUAL}" >&2
  exit 1
fi
echo "    ok (${ACTUAL})"

# --- install --------------------------------------------------------------
if [ -z "$INSTALL_DIR" ]; then
  if [ -w /usr/local/bin ]; then
    INSTALL_DIR="/usr/local/bin"
  else
    INSTALL_DIR="$HOME/.local/bin"
  fi
fi
mkdir -p "$INSTALL_DIR"

INSTALL_PATH="${INSTALL_DIR}/mywt"
echo "==> Installing to ${INSTALL_PATH}"
mv "${TMP}/${ASSET}" "${INSTALL_PATH}"
chmod +x "${INSTALL_PATH}"

# --- report ---------------------------------------------------------------
echo "==> Done: $("${INSTALL_PATH}" version)"
if ! command -v mywt >/dev/null 2>&1; then
  echo
  echo "note: '${INSTALL_DIR}' is not on your PATH. Add it, e.g.:"
  echo "  export PATH=\"${INSTALL_DIR}:\$PATH\""
fi
