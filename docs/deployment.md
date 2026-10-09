# 部署

Coterie 是一个 Go 二进制加一份 PostgreSQL（15+）。迁移在服务启动时自动应用，没有单独的迁移步骤，升级就是换一个二进制再启动。

## 从源码运行

需要 Go 1.25+。

```bash
# 配置（全部变量见 .env.example）
export DATABASE_URL=postgres://coterie:coterie@localhost:5432/coterie?sslmode=disable
export PORT=8080

# 启动
go run ./apps/server
```

`GET /healthz` 返回 200 即就绪。对运行中的服务做一次完整旅程冒烟：`scripts/smoke.sh http://localhost:8080`。

## Docker Compose

仓库自带 `deployments/docker-compose.yml`，只有两个服务：postgres 和 coterie。镜像从源码构建，还没有发布到公共镜像仓库。

```bash
docker compose -f deployments/docker-compose.yml up -d --build
```

postgres 带 healthcheck，coterie 等它就绪后再启动；服务器连不上数据库会直接退出，由 compose 的 restart 策略拉起。compose 里的 5432 和 8080 是宿主机端口，被占用时改映射即可，不要动容器内的端口。

## 配置项

全部通过环境变量读取，默认值在 `internal/config` 里，注释版清单见 `.env.example`。

核心项：

| 变量 | 默认 | 说明 |
|---|---|---|
| `PORT` | 8080 | 监听端口 |
| `DATABASE_URL` | `postgres://coterie:coterie@localhost:5432/coterie?sslmode=disable` | PostgreSQL 连接串 |
| `MIGRATE_ON_START` | true | 启动时应用迁移 |
| `MIGRATIONS_DIR` | `migrations` | 迁移文件目录（镜像内为 `/migrations`） |
| `LOG_LEVEL` | INFO | DEBUG / INFO / WARN / ERROR |
| `AUTO_BILLING_INTERVAL` | 1h | 账期滚动调度间隔，0 关闭 |
| `RATE_LIMIT_REGISTER_PER_MIN` | 5 | 注册限速，每地址每分钟 |
| `RATE_LIMIT_LOGIN_PER_MIN` | 10 | 登录限速，每地址每分钟 |
| `ADMIN_EMAILS` | 空 | 逗号分隔的邮箱，启动时提升为平台管理员 |

通知与支付渠道，不配置就不启用：

| 变量 | 启用条件 | 说明 |
|---|---|---|
| `SMTP_HOST` `SMTP_PORT` `SMTP_USERNAME` `SMTP_PASSWORD` `SMTP_FROM` | `SMTP_HOST` 与 `SMTP_FROM` 都设置 | 邮件渠道 |
| `WEBHOOK_URL` `WEBHOOK_SECRET` | `WEBHOOK_URL` 设置 | 出站 Webhook，正文带 HMAC-SHA256 签名 |
| `VAPID_PUBLIC_KEY` `VAPID_PRIVATE_KEY` `VAPID_SUBJECT` | 三项都设置 | Web Push，密钥用 `go run ./cmd/vapidkeygen` 生成 |
| `PAYMENT_METHODS` | 逗号分隔 | 在内置 manual 之上注册渠道，`sandbox` 是内置演示渠道 |
| `STRIPE_SECRET_KEY` | 设置后启用 stripe | 需同时加入 `PAYMENT_METHODS` |
| `STRIPE_WEBHOOK_SECRET` | Stripe 验签 | 公开端点 `POST /api/v1/payments/webhooks/stripe` |
| `STRIPE_MARKETPLACE_WEBHOOK_SECRET` | 可选 | 入圈闸门 Webhook 的独立密钥，缺省回落到 `STRIPE_WEBHOOK_SECRET` |
| `PROVIDER_PLUGINS` | 逗号分隔 | `claude` 是内置示例插件，加区域/套餐策略与用量上限 |

## 平台管理员

`ADMIN_EMAILS` 列出的邮箱在启动时被提升为 admin 角色，账号还不存在时注册后再启动也会被提升，重复启动不会重复处理。管理员只处置举报（收件箱与 resolve/dismiss），不管理任何人的圈。

## 升级

拉取新代码，重建镜像，重启服务。迁移只向前滚动；回滚镜像不会回滚 schema，每个迁移都带 down 文件，需要时手动执行。
