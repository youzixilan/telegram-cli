---
name: telegram-cli
description: Manage Telegram via TDLib - login, send messages, create bots, list chats, search, view history, manage contacts. Use when user asks to interact with Telegram as a user account (not bot API), create Telegram bots, send messages via Telegram, list Telegram chats, or search Telegram.
---

# Telegram CLI (TDLib)

User-account-level Telegram management via TDLib.

## First-Time Setup

If `tgctl` is not installed or TOOLS.md has no tgctl config, run the setup flow:

### Step 1: Install binary
```bash
curl -fsSL https://raw.githubusercontent.com/youzixilan/go-tdlib/main/skill/scripts/install.sh | bash
```

### Step 2: Get API credentials
Ask the user to go to https://my.telegram.org → API Development → get `api_id` and `api_hash`.

### Step 3: Login
```bash
TELEGRAM_API_ID=<id> TELEGRAM_API_HASH=<hash> tgctl login
```
User enters phone number, auth code (NOT via Telegram!), and optional 2FA password.

### Step 4: Save config to TOOLS.md
After login succeeds, append to workspace TOOLS.md:
```markdown
### tgctl
- Binary: ~/.local/bin/tgctl
- Env: TELEGRAM_API_ID=<id> TELEGRAM_API_HASH=<hash>
- Profile: default
```

## Prerequisites (after setup)

- `tgctl` binary installed (check: `which tgctl` or `~/.local/bin/tgctl`)
- `TELEGRAM_API_ID` and `TELEGRAM_API_HASH` from TOOLS.md
- Session persists in `~/.tgctl/<profile>/`

## Commands

```bash
TELEGRAM_API_ID=<id> TELEGRAM_API_HASH=<hash> tgctl [--profile <name>] <command>
```

| Command | Description |
|---------|-------------|
| `me` | Current user info |
| `send <chat> <msg>` | Send message (ID or @username) |
| `chats [limit]` | List chats |
| `create-bot <name> <username>` | Create bot via BotFather, returns token |
| `history <chat> [limit]` | Chat history |
| `search <query>` | Search public chats |
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

- **Private chat**: User ID directly (auto-creates via `createPrivateChat`)
- **Group/Channel**: Add `-100` prefix (e.g. `3805592010` → `-1003805592010`)
- **@username**: Use directly (e.g. `@BotFather`)

## Security

- Credentials in env vars only, never hardcode
- `~/.tgctl/` contains auth session — protect it
- Auth codes must not be sent via Telegram (will be blocked)
