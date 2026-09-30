#!/usr/bin/env sh
# install.sh — installer for git-release
# Usage:
#   curl -fsSL https://github.com/gronitab/git-release/releases/latest/download/install.sh | sh
#
# Override install directory: INSTALL_DIR=/usr/local/bin sh install.sh
set -e

REPO="gronitab/git-release"
INSTALL_DIR="${INSTALL_DIR:-/usr/local/bin}"
BINARY="git-release"

# ── Detect OS ────────────────────────────────────────────────────────────────
OS="$(uname -s)"
case "$OS" in
  Linux)  os="linux"   ;;
  Darwin) os="darwin"  ;;
  MINGW*|MSYS*|CYGWIN*) os="windows" ;;
  *)
    echo "Unsupported OS: $OS" >&2
    exit 1
    ;;
esac

# ── Detect arch ──────────────────────────────────────────────────────────────
ARCH="$(uname -m)"
case "$ARCH" in
  x86_64|amd64)           arch="amd64" ;;
  aarch64|arm64|armv8*)   arch="arm64" ;;
  *)
    echo "Unsupported architecture: $ARCH" >&2
    exit 1
    ;;
esac

# ── Resolve version ───────────────────────────────────────────────────────────
if [ -z "$VERSION" ]; then
  VERSION="$(curl -fsSL "https://api.github.com/repos/${REPO}/releases/latest" \
    -H "Accept: application/vnd.github+json" \
    | grep '"tag_name"' | head -1 | sed 's/.*"tag_name": *"\([^"]*\)".*/\1/')"
fi

if [ -z "$VERSION" ]; then
  echo "Could not determine latest version" >&2
  exit 1
fi

# Strip leading 'v' for asset name
VER="${VERSION#v}"

# ── Build download URL ────────────────────────────────────────────────────────
if [ "$os" = "windows" ]; then
  EXT="zip"
else
  EXT="tar.gz"
fi

ASSET="${BINARY}_${VER}_${os}_${arch}.${EXT}"
URL="https://github.com/${REPO}/releases/download/${VERSION}/${ASSET}"

echo "Installing git-release ${VERSION} (${os}/${arch}) → ${INSTALL_DIR}/${BINARY}"

# ── Download & extract ────────────────────────────────────────────────────────
TMP="$(mktemp -d)"
trap 'rm -rf "$TMP"' EXIT

curl -fsSL "$URL" -o "$TMP/$ASSET"

if [ "$EXT" = "tar.gz" ]; then
  tar -xzf "$TMP/$ASSET" -C "$TMP" "$BINARY"
else
  unzip -q "$TMP/$ASSET" "$BINARY.exe" -d "$TMP"
  BINARY="${BINARY}.exe"
fi

# ── Install ───────────────────────────────────────────────────────────────────
if [ -w "$INSTALL_DIR" ]; then
  mv "$TMP/$BINARY" "$INSTALL_DIR/$BINARY"
  chmod +x "$INSTALL_DIR/$BINARY"
else
  sudo mv "$TMP/$BINARY" "$INSTALL_DIR/$BINARY"
  sudo chmod +x "$INSTALL_DIR/$BINARY"
fi

echo "Done! Run: git release status"
