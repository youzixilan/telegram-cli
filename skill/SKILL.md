---
name: telegram-cli
description: Manage Telegram via pure Go client (gotd/td) - login, send messages, list chats, search, view history, manage contacts, listen for messages. Use when user asks to interact with Telegram as a user account (not bot API), send messages via Telegram, list Telegram chats, or search Telegram.
---

# Telegram CLI (tgctl)

User-account-level Telegram management. Pure Go, no CGO.

## First-Time Setup

If `tgctl` is not installed or TOOLS.md has no tgctl config, run the setup flow:

### Step 1: Install binary
```bash
curl -fsSL https://raw.githubusercontent.com/youzixilan/telegram-cli/main/scripts/install.sh | bash
```

### Step 2: Get API credentials
Ask the user to go to https://my.telegram.org → API Development → get `api_id` and `api_hash`.

### Step 3: Login
```bash
TELEGRAM_API_ID=<id> TELEGRAM_API_HASH=<hash> tgctl login
```
User enters phone number, auth code, and optional 2FA password interactively.

**Important:** Auth codes must NOT be sent via Telegram (will be invalidated).

### Step 4: Save config to TOOLS.md
After login succeeds, append to workspace TOOLS.md:
```markdown
### tgctl (Telegram CLI)
- Binary: ~/.local/bin/tgctl
- Env: TELEGRAM_API_ID=<id> TELEGRAM_API_HASH=<hash>
- Session: ~/.tgctl/
```

## Prerequisites (after setup)

- `tgctl` binary installed (check: `which tgctl` or `~/.local/bin/tgctl`)
- `TELEGRAM_API_ID` and `TELEGRAM_API_HASH` from TOOLS.md
- Session persists in `~/.tgctl/<profile>/session.json`

## Commands

```bash
TELEGRAM_API_ID=<id> TELEGRAM_API_HASH=<hash> tgctl [--profile <name>] <command>
```

| Command | Description |
|---------|-------------|
| `login` | Login (phone + code + optional 2FA) |
| `me` | Current user info |
| `send <chat> <msg>` | Send message (user ID, chat ID, or @username) |
| `chats [limit]` | List chats |
| `history <chat> [limit]` | Chat history |
| `search <query>` | Search chats and users |
| `contacts` | List contacts |
| `listen [--user id] [--chat id]` | Real-time message listener |
| `logout` | Logout |

## Multi-Account (--profile)

```bash
tgctl --profile work login
tgctl --profile personal login
tgctl --profile work me
tgctl me                       # uses "default" profile
```

## Chat ID Format

- **User**: Positive ID (e.g. `8568316820`)
- **Group/Channel**: Negative ID (e.g. `-1003805592010`)
- **@username**: Use directly (e.g. `@BotFather`)

## Security

- Credentials in env vars only, never hardcode
- `~/.tgctl/` contains auth session — protect it
- Auth codes must not be sent via Telegram (will be invalidated)
