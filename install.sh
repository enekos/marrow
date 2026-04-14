#!/bin/sh
set -e

REPO="enekos/marrow"
BINARY_NAME="marrow"

# Detect OS
OS=$(uname -s | tr '[:upper:]' '[:lower:]')
ARCH=$(uname -m)

case "$ARCH" in
    x86_64|amd64) ARCH="amd64" ;;
    arm64|aarch64) ARCH="arm64" ;;
    *) echo "Unsupported architecture: $ARCH"; exit 1 ;;
esac

case "$OS" in
    linux) OS="linux" ;;
    darwin) OS="darwin" ;;
    mingw64*|msys*|cygwin*)
        OS="windows"
        BINARY_NAME="marrow.exe"
        ;;
    *) echo "Unsupported OS: $OS"; exit 1 ;;
esac

# On macOS, default to the universal binary for simplicity
if [ "$OS" = "darwin" ]; then
    ARCH="all"
fi

# Fetch latest release version
VERSION=$(curl -fsSL "https://api.github.com/repos/${REPO}/releases/latest" | grep '"tag_name":' | sed -E 's/.*"([^"]+)".*/\1/')
if [ -z "$VERSION" ]; then
    echo "Failed to fetch latest release version."
    exit 1
fi

echo "Installing ${BINARY_NAME} ${VERSION} for ${OS}/${ARCH}..."

if [ "$OS" = "windows" ]; then
    ARCHIVE_NAME="${BINARY_NAME}_${VERSION}_${OS}_${ARCH}.zip"
else
    ARCHIVE_NAME="${BINARY_NAME}_${VERSION}_${OS}_${ARCH}.tar.gz"
fi

DOWNLOAD_URL="https://github.com/${REPO}/releases/download/${VERSION}/${ARCHIVE_NAME}"

TMP_DIR=$(mktemp -d)
trap 'rm -rf "$TMP_DIR"' EXIT

curl -fsSL -o "$TMP_DIR/$ARCHIVE_NAME" "$DOWNLOAD_URL"

if [ "$OS" = "windows" ]; then
    unzip -q "$TMP_DIR/$ARCHIVE_NAME" -d "$TMP_DIR"
else
    tar -xzf "$TMP_DIR/$ARCHIVE_NAME" -C "$TMP_DIR"
fi

# Determine install location
if [ -d "/usr/local/bin" ] && [ -w "/usr/local/bin" ]; then
    INSTALL_DIR="/usr/local/bin"
elif [ -d "$HOME/.local/bin" ]; then
    INSTALL_DIR="$HOME/.local/bin"
else
    INSTALL_DIR="$HOME/.local/bin"
    mkdir -p "$INSTALL_DIR"
fi

cp "$TMP_DIR/$BINARY_NAME" "$INSTALL_DIR/$BINARY_NAME"
chmod +x "$INSTALL_DIR/$BINARY_NAME"

echo ""
echo "✅ ${BINARY_NAME} ${VERSION} installed to ${INSTALL_DIR}/${BINARY_NAME}"
echo ""
echo "Make sure ${INSTALL_DIR} is in your PATH:"
echo "  export PATH=\"${INSTALL_DIR}:\$PATH\""
echo ""
echo "Quick start:"
echo "  marrow sync -dir ./docs"
echo "  marrow serve -db marrow.db"
