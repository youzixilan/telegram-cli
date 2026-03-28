# telegram-cli

Telegram CLI tool powered by [gotd/td](https://github.com/gotd/td) — pure Go, no CGO.

## Install

### Pre-built binary

Download and review the install script before running:

```bash
curl -fsSL https://raw.githubusercontent.com/youzixilan/telegram-cli/main/scripts/install.sh -o /tmp/install-tgctl.sh
cat /tmp/install-tgctl.sh   # Review the script
bash /tmp/install-tgctl.sh
rm /tmp/install-tgctl.sh
```

Or download directly from [Releases](https://github.com/youzixilan/telegram-cli/releases).

### Build from source

```bash
go install github.com/aqin/telegram-cli/cmd/tgctl@latest
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

## Security

See [SECURITY.md](SECURITY.md) for details.

- All credentials are passed via environment variables, never stored in code
- tgctl only communicates with official Telegram servers
- Login requires interactive user input (phone + SMS code + optional 2FA)
- No telemetry, no analytics, no third-party API calls

## License

MIT — see [LICENSE](LICENSE)
