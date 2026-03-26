#!/bin/bash
set -e

REPO="youzixilan/telegram-cli"
INSTALL_DIR="${TGCTL_INSTALL_DIR:-$HOME/.local}"
BIN_DIR="$INSTALL_DIR/bin"

# detect platform
OS=$(uname -s | tr '[:upper:]' '[:lower:]')
ARCH=$(uname -m)
case "$ARCH" in
  x86_64|amd64) ARCH="amd64" ;;
  arm64|aarch64) ARCH="arm64" ;;
  *) echo "Unsupported architecture: $ARCH"; exit 1 ;;
esac

case "$OS" in
  darwin) SUFFIX="darwin-$ARCH"; EXT="tar.gz" ;;
  linux)  SUFFIX="linux-$ARCH"; EXT="tar.gz" ;;
  mingw*|msys*|cygwin*) SUFFIX="windows-amd64"; EXT="zip" ;;
  *) echo "Unsupported OS: $OS"; exit 1 ;;
esac

echo "Detected platform: $SUFFIX"
echo "Installing to: $BIN_DIR"

DOWNLOAD_URL="https://github.com/$REPO/releases/latest/download/tgctl-$SUFFIX.$EXT"
echo "Downloading $DOWNLOAD_URL ..."

mkdir -p "$BIN_DIR"
TMP=$(mktemp -d)
trap "rm -rf $TMP" EXIT

if [ "$EXT" = "zip" ]; then
  curl -fSL "$DOWNLOAD_URL" -o "$TMP/tgctl.zip"
  unzip -o "$TMP/tgctl.zip" -d "$TMP"
else
  curl -fSL "$DOWNLOAD_URL" -o "$TMP/tgctl-$SUFFIX"
fi

# install binary
if [ -f "$TMP/tgctl-$SUFFIX" ]; then
  cp "$TMP/tgctl-$SUFFIX" "$BIN_DIR/tgctl"
  chmod +x "$BIN_DIR/tgctl"
elif [ -f "$TMP/tgctl-$SUFFIX.exe" ]; then
  cp "$TMP/tgctl-$SUFFIX.exe" "$BIN_DIR/tgctl.exe"
fi

echo ""
echo "✅ tgctl installed to $BIN_DIR/tgctl"
echo ""

if ! echo "$PATH" | grep -q "$BIN_DIR"; then
  echo "⚠️  $BIN_DIR is not in your PATH. Add it:"
  echo "  export PATH=\"$BIN_DIR:\$PATH\""
  echo ""
fi

echo "Next steps:"
echo "  1. Get API credentials from https://my.telegram.org"
echo "  2. Run: TELEGRAM_API_ID=<id> TELEGRAM_API_HASH=<hash> tgctl login"
