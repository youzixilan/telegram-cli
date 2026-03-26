# tgctl Installation Guide

## Install

### Automatic (Recommended)

```bash
curl -fsSL https://raw.githubusercontent.com/youzixilan/go-tdlib/main/skill/scripts/install.sh | bash
```

Auto-detects platform, downloads binary + TDLib library, installs to `~/.local/bin/`.

### Manual Download

Download from [GitHub Releases](https://github.com/youzixilan/go-tdlib/releases/latest):

```bash
# macOS Apple Silicon
curl -L https://github.com/youzixilan/go-tdlib/releases/latest/download/tgctl-darwin-arm64.tar.gz | tar xz
# macOS Intel
curl -L https://github.com/youzixilan/go-tdlib/releases/latest/download/tgctl-darwin-amd64.tar.gz | tar xz
# Linux amd64
curl -L https://github.com/youzixilan/go-tdlib/releases/latest/download/tgctl-linux-amd64.tar.gz | tar xz
# Windows amd64
# Download tgctl-windows-amd64.zip from Releases page
```

Install:
```bash
mkdir -p ~/.local/bin ~/.local/lib
mv tgctl-* ~/.local/bin/tgctl
mv libtdjson* ~/.local/lib/
# macOS
install_name_tool -add_rpath ~/.local/lib ~/.local/bin/tgctl
# Linux: add to ~/.bashrc
export LD_LIBRARY_PATH=$HOME/.local/lib:$LD_LIBRARY_PATH
```

## Setup

1. Get API credentials from https://my.telegram.org → API Development
2. Login: `TELEGRAM_API_ID=<id> TELEGRAM_API_HASH=<hash> tgctl login`
3. Add config to TOOLS.md
