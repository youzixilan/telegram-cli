# go-tdlib

Telegram CLI tool powered by [gotd/td](https://github.com/gotd/td) — pure Go, no TDLib, no CGO.

## Install

### Pre-built binary

```bash
curl -fsSL https://raw.githubusercontent.com/youzixilan/go-tdlib/main/scripts/install.sh | bash
```

Or download from [Releases](https://github.com/youzixilan/go-tdlib/releases).

### Build from source

```bash
go install github.com/aqin/go-tdlib/cmd/tgctl@latest
```

## Usage

```bash
export TELEGRAM_API_ID=your_api_id
export TELEGRAM_API_HASH=your_api_hash

tgctl login                      # Login (phone + code + optional 2FA)
tgctl me                         # Show current user
tgctl send <chat> <message>      # Send message (user ID, chat ID, or @username)
tgctl chats [limit]              # List chats
tgctl history <chat> [limit]     # Chat history
tgctl search <query>             # Search chats/users
tgctl contacts                   # List contacts
tgctl listen [--user id] [--chat id]  # Listen for messages
tgctl logout                     # Logout
```

### Multi-account

```bash
tgctl --profile work login
tgctl --profile work me
```

Sessions stored in `~/.tgctl/<profile>/`.

## Platforms

- macOS (arm64, amd64)
- Linux (amd64, arm64)
- Windows (amd64)
