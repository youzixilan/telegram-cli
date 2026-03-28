# Security Policy

## Credential Handling

- tgctl **never** stores API credentials in code or config files
- All credentials (`TELEGRAM_API_ID`, `TELEGRAM_API_HASH`) must be passed via environment variables at runtime
- Auth sessions are stored locally in `~/.tgctl/` and are never transmitted to third parties
- No credentials are logged, cached, or written to disk by the application

## Network Communication

- tgctl only communicates with official Telegram servers (Telegram DC endpoints)
- No telemetry, no analytics, no third-party API calls
- All connections use Telegram's MTProto encryption protocol

## Authentication

- Login requires interactive user input (phone number + SMS verification code)
- Optional 2FA password support for accounts with two-step verification enabled
- The tool cannot authenticate without explicit user participation
- Auth codes must NOT be sent via Telegram messages (Telegram will invalidate them)

## Installation Safety

- Pre-built binaries are available on the [Releases](https://github.com/youzixilan/telegram-cli/releases) page
- The install script downloads binaries from GitHub Releases only — no external sources
- Users are encouraged to review the install script before running it
- Build from source is supported via `go install`

## Reporting Vulnerabilities

If you discover a security vulnerability, please open a GitHub issue or contact the maintainer directly.
