#!/bin/sh
set -e

# EZKeel installer — detects OS/arch and downloads the right binary from GitHub.

REPO="ferax564/ezkeel-cli"
INSTALL_DIR="/usr/local/bin"
BINARY_NAME="ezkeel"

# Detect OS
OS=$(uname -s | tr '[:upper:]' '[:lower:]')
case "$OS" in
  linux)  OS="linux" ;;
  darwin) OS="darwin" ;;
  *)
    echo "Error: unsupported operating system: $OS"
    exit 1
    ;;
esac

# Detect architecture
ARCH=$(uname -m)
case "$ARCH" in
  x86_64|amd64)  ARCH="amd64" ;;
  arm64|aarch64) ARCH="arm64" ;;
  *)
    echo "Error: unsupported architecture: $ARCH"
    exit 1
    ;;
esac

# Get latest release tag
echo "Detecting latest EZKeel release..."
TAG=$(curl -fsSL "https://api.github.com/repos/${REPO}/releases/latest" | grep '"tag_name"' | sed 's/.*"tag_name": *"\([^"]*\)".*/\1/')

if [ -z "$TAG" ]; then
  echo "Error: could not determine latest release"
  exit 1
fi

ASSET="${BINARY_NAME}-${OS}-${ARCH}"
URL="https://github.com/${REPO}/releases/download/${TAG}/${ASSET}"

echo "Downloading EZKeel ${TAG} for ${OS}/${ARCH}..."
TMP=$(mktemp)
if ! curl -fsSL -o "$TMP" "$URL"; then
  echo "Error: failed to download ${URL}"
  rm -f "$TMP"
  exit 1
fi

chmod +x "$TMP"

# Install — use sudo if needed
if [ -w "$INSTALL_DIR" ]; then
  mv "$TMP" "${INSTALL_DIR}/${BINARY_NAME}"
else
  echo "Installing to ${INSTALL_DIR} (requires sudo)..."
  sudo mv "$TMP" "${INSTALL_DIR}/${BINARY_NAME}"
fi

echo ""
echo "EZKeel ${TAG} installed to ${INSTALL_DIR}/${BINARY_NAME}"
echo ""
echo "Get started:"
echo "  ezkeel version"
echo "  ezkeel init my-project"
echo "  ezkeel --help"
