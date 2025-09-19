# Uptime Kuma Webhook to Telegram Bot

Lightweight Go service that receives [Uptime Kuma](https://github.com/louislam/uptime-kuma) webhook callbacks and forwards the events to a Telegram bot.

## Quick Start
1. Install Go (1.21 or newer recommended).
2. Copy `.env.example` to `.env` and fill in the required fields, or set the variables directly in your environment (the service automatically reads `.env` on startup and ignores it if not found).
3. Start the server:
   ```bash
   go run ./...
   ```

## Required Environment Variables
| Variable | Description |
| --- | --- |
| `WEBHOOK_AUTH_TOKEN` | Bearer token expected in the webhook request header |
| `TELEGRAM_BOT_TOKEN` | Telegram bot token |
| `TELEGRAM_CHAT_ID` | Chat ID that should receive the notification |

### Optional Environment Variables
| Variable | Default | Description |
| --- | --- | --- |
| `LISTEN_ADDR` | `:8080` | HTTP listen address |
| `TELEGRAM_API_BASE_URL` | `https://api.telegram.org` | Override when using a custom Telegram API endpoint |
| `REQUEST_TIMEOUT` | `10s` | Timeout applied to the Telegram API request |

## Docker Deployment
1. Build the image:
   ```bash
   docker build -t uptimekuma-webhook-tgbot .
   ```
2. Run a container (example):
   ```bash
   docker run -d \
     --name uptimekuma-webhook \
     -p 8080:8080 \
     -e WEBHOOK_AUTH_TOKEN=replace-with-secure-token \
     -e TELEGRAM_BOT_TOKEN=123456:ABCDEF-your-telegram-token \
     -e TELEGRAM_CHAT_ID=123456789 \
     uptimekuma-webhook-tgbot
   ```

You can also mount the `.env` file directly:
```bash
docker run --env-file .env -p 8080:8080 uptimekuma-webhook-tgbot
```

## Configuring Uptime Kuma
- Webhook URL: `http://<host>:<port>/uptimekuma-webhook`
- Method: `POST`
- Custom header: `Authorization: Bearer <WEBHOOK_AUTH_TOKEN>`
- Payload: keep Uptime Kuma's default JSON. The service parses key fields and sends a summary plus the raw payload to Telegram.

## Local Smoke Test
```bash
curl -X POST "http://localhost:8080/uptimekuma-webhook" \
  -H "Authorization: Bearer $WEBHOOK_AUTH_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"monitor":{"name":"Demo"},"status":"up","msg":"All good"}'
```

A `202 Accepted` response and a message in the configured Telegram chat indicate a successful forward. Inspect the container or process logs for troubleshooting details.

