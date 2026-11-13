# Pulse Federation Bridges (`pulse-bridges`)

Official standalone federation connector and bridge daemon for the **Pulse Messaging Platform**.

Enables bidirectional synchronization between Pulse channels and third-party messaging networks (**Telegram, Slack, Discord, and Matrix**).

## 🚀 Supported Adapters

- **Telegram**: Bridges Telegram supergroups and direct bot chats via Webhook & Telegram Bot API.
- **Slack**: Connects channels via Slack Events API callbacks and `chat.postMessage`.
- **Discord**: Forwards messages using Discord Gateway webhooks and Bot API.
- **Matrix**: Integrates with Matrix homeservers using Application Service (AS) API.

## 🛠️ Configuration & Run

```bash
# Configure environment
export PULSE_API_URL="http://localhost:32011"
export PULSE_APP_ID="app_production"
export PULSE_APP_SECRET="sec_0123456789abcdef"
export TELEGRAM_BOT_TOKEN="123456:ABC-DEF1234ghIkl-zyx57W2v1u123ew11"
export SLACK_BOT_TOKEN="xoxb-your-token"
export DISCORD_BOT_TOKEN="your-bot-token"

# Run bridge daemon
go run cmd/pulse-bridges/main.go
```

