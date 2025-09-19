# Uptime Kuma Webhook 转发 Telegram Bot

使用 Go 编写的轻量服务，接收 [Uptime Kuma](https://github.com/louislam/uptime-kuma) 的 Webhook 回调，并将事件内容转发到 Telegram 机器人。

## 快速开始
1. 安装 Go（建议 1.21 及以上）。
2. 复制 `.env.example` 为 `.env` 并填写必填项，或在环境变量中直接设置（程序启动时会自动读取 `.env`，若不存在则忽略）。
3. 启动服务：
   ```bash
   go run ./...
   ```

## 必填环境变量
| 变量名 | 说明 |
| --- | --- |
| `WEBHOOK_AUTH_TOKEN` | Webhook 请求头需携带的 Bearer Token 值 |
| `TELEGRAM_BOT_TOKEN` | Telegram 机器人 Token |
| `TELEGRAM_CHAT_ID` | 接收通知的聊天 ID（个人或群组） |

### 可选环境变量
| 变量名 | 默认值 | 说明 |
| --- | --- | --- |
| `LISTEN_ADDR` | `:8080` | HTTP 服务监听地址 |
| `TELEGRAM_API_BASE_URL` | `https://api.telegram.org` | 自定义 Telegram API 地址（如自建代理） |
| `REQUEST_TIMEOUT` | `10s` | 调用 Telegram API 的超时时间 |

## Docker 部署
1. 构建镜像：
   ```bash
   docker build -t uptimekuma-webhook-tgbot .
   ```
2. 创建容器（示例）：
   ```bash
   docker run -d \
     --name uptimekuma-webhook \
     -p 8080:8080 \
     -e WEBHOOK_AUTH_TOKEN=replace-with-secure-token \
     -e TELEGRAM_BOT_TOKEN=123456:ABCDEF-your-telegram-token \
     -e TELEGRAM_CHAT_ID=123456789 \
     uptimekuma-webhook-tgbot
   ```

也可通过挂载 `.env` 文件传入配置：
```bash
docker run --env-file .env -p 8080:8080 uptimekuma-webhook-tgbot
```

## 在 Uptime Kuma 中配置
- Webhook URL：`http://<服务器IP或域名>:<端口>/uptimekuma-webhook`
- 请求方法：`POST`
- 自定义请求头：`Authorization: Bearer <WEBHOOK_AUTH_TOKEN>`
- 请求体：保持 Uptime Kuma 默认 JSON，不需要额外修改。

## 本地调试
```bash
curl -X POST "http://localhost:8080/uptimekuma-webhook" \
  -H "Authorization: Bearer $WEBHOOK_AUTH_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"monitor":{"name":"Demo"},"status":"up","msg":"All good"}'
```

命令返回 `202 Accepted` 且 Telegram 收到消息即表示转发成功。服务日志会打印请求状态，便于排查问题。

