#!/bin/bash
set -e

REPO="youzixilan/go-tdlib"
INSTALL_DIR="${TGCTL_INSTALL_DIR:-$HOME/.local}"
BIN_DIR="$INSTALL_DIR/bin"
LIB_DIR="$INSTALL_DIR/lib"

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
  mingw*|msys*|cygwin*) SUFFIX="windows-$ARCH"; EXT="zip" ;;
  *) echo "Unsupported OS: $OS"; exit 1 ;;
esac

echo "Detected platform: $SUFFIX"
echo "Installing to: $BIN_DIR"

# get latest release URL
DOWNLOAD_URL="https://github.com/$REPO/releases/latest/download/tgctl-$SUFFIX.$EXT"
echo "Downloading $DOWNLOAD_URL ..."

mkdir -p "$BIN_DIR" "$LIB_DIR"
TMP=$(mktemp -d)
trap "rm -rf $TMP" EXIT

if [ "$EXT" = "zip" ]; then
  curl -fSL "$DOWNLOAD_URL" -o "$TMP/tgctl.zip"
  unzip -o "$TMP/tgctl.zip" -d "$TMP/dist"
else
  curl -fSL "$DOWNLOAD_URL" | tar xz -C "$TMP"
fi

# install binary
if [ -f "$TMP/tgctl-$SUFFIX" ]; then
  cp "$TMP/tgctl-$SUFFIX" "$BIN_DIR/tgctl"
elif [ -f "$TMP/tgctl-$SUFFIX.exe" ]; then
  cp "$TMP/tgctl-$SUFFIX.exe" "$BIN_DIR/tgctl.exe"
elif [ -f "$TMP/dist/tgctl-$SUFFIX.exe" ]; then
  cp "$TMP/dist/tgctl-$SUFFIX.exe" "$BIN_DIR/tgctl.exe"
fi
chmod +x "$BIN_DIR/tgctl" 2>/dev/null || true

# install shared library
for f in "$TMP"/libtdjson* "$TMP"/dist/tdjson*; do
  [ -f "$f" ] && cp "$f" "$LIB_DIR/"
done

# fix rpath on macOS
if [ "$OS" = "darwin" ] && [ -f "$BIN_DIR/tgctl" ]; then
  install_name_tool -add_rpath "$LIB_DIR" "$BIN_DIR/tgctl" 2>/dev/null || true
fi

echo ""
echo "✅ tgctl installed to $BIN_DIR/tgctl"
echo ""

# check PATH
if ! echo "$PATH" | grep -q "$BIN_DIR"; then
  echo "⚠️  $BIN_DIR is not in your PATH. Add it:"
  echo "  export PATH=\"$BIN_DIR:\$PATH\""
  echo ""
fi

if [ "$OS" = "linux" ]; then
  echo "⚠️  On Linux, add library path:"
  echo "  export LD_LIBRARY_PATH=\"$LIB_DIR:\$LD_LIBRARY_PATH\""
  echo ""
fi

echo "Next steps:"
echo "  1. Get API credentials from https://my.telegram.org"
echo "  2. Run: TELEGRAM_API_ID=<id> TELEGRAM_API_HASH=<hash> tgctl login"
echo "  3. Add config to your TOOLS.md"
