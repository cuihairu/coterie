# Glossary

This table records the implementation-facing terms used by the source
layer analysis on 2026-11-07. It complements the domain definitions in
[Requirements](requirements.md#13-glossary) and
[Design](design.md#11-concept-definitions).

| Term | English | Definition | Evidence |
|---|---|---|---|
| 运行时配置 | Runtime Configuration | Environment-derived server settings loaded once before process startup. | `internal/config/config.go:16` |
| 数据库生命周期 | Database Lifecycle | PostgreSQL connection, startup migration, and schema ownership backed by hand-written SQL. | `internal/database/database.go:37` |
| 处理器装配 | Handler Assembly | Single API mux that constructs module services, wires routes and middleware, and injects channel/plugin/adapters. | `internal/app/app.go:95` |
| 前端客户端 | Web Client | React SPA that stores session state in `localStorage` and calls one shared JSON API helper. | `apps/web/src/lib/api.ts:52` |
| 用户 | User | Platform account and profile referenced by sessions, membership, and ownership checks. | `internal/user/model.go:7` |
| 会话 | Session | Bearer-token credential persisted as a SHA-256 digest with expiry and revocation state. | `internal/auth/model.go:11` |
| 共享圈 | Coterie | Social aggregate with a strict one-to-one subscription relation, owner/member records, invitations, and blocks. | `internal/coterie/model.go:26` |
| 订阅 | Subscription | Purchased plan instance that owns price, currency, billing cycle, sharing policy, and seat capacity. | `internal/subscription/model.go:38` |
| 席位 | Seat | Allocatable unit of subscription capacity; assignment, release, and quota metadata live on it. | `internal/seat/model.go:22` |
| 账期 | Billing Period | Open or closed date interval under a subscription used to aggregate contributions. | `internal/billing/model.go:38` |
| 分摊 | Contribution | A member's owed amount and settlement state inside one billing period. | `internal/billing/model.go:52` |
| 支付流水 | Payment | Appendable channel result for a contribution charge, carrying method, status, and external reference. | `internal/payment/model.go:9` |
| 入圈申请 | Join Request | Public-marketplace request with pending/awaiting-payment/accepted/declined lifecycle. | `internal/marketplace/model.go:23` |
| 入圈费 | Admission Charge | Payment-gate charge for a join request; confirmation can atomically admit the user. | `internal/marketplace/model.go:104` |
| 分摊争议 | Dispute | One-way owner decision over a member's challenged contribution. | `internal/dispute/model.go:26` |
| 用量流水 | Usage Record | Append-only metering entry attributed to a member and optionally a seat. | `internal/usage/model.go:12` |
| 目录服务商 | Provider Catalog Entry | Metadata describing a vendor; it does not own plans or prices. | `internal/provider/model.go:13` |
| 目录产品 | Product Catalog Entry | Purchasable plan metadata supplied by a provider; actual price belongs to a subscription. | `internal/product/model.go:12` |
| 站内通知 | In-App Notification | Persistent user message written by domain services and optionally fanned out outbound. | `internal/notification/model.go:26` |
| 推送端点 | Push Endpoint | Registered browser Web Push endpoint with encryption keys and generated identifier. | `internal/push/model.go:9` |
| 审计流水 | Audit Entry | Append-only mutation record with actor, action, entity, scope, and before/after snapshots. | `internal/audit/model.go:52` |
| 信誉视图 | Reputation View | Derived per-currency contribution statistics and payment ratio read from billing rows. | `internal/reputation/model.go:9` |
| 举报 | Report | User flag for a listed coterie triaged by the platform admin through open/resolved/dismissed states. | `internal/report/model.go:25` |
| 鉴权门 | Authentication Gate | Route middleware that resolves and validates a bearer session before handlers execute. | `internal/auth/middleware.go:31` |
| 平台管理员 | Platform Admin | Bootstrap-promoted user role allowed to read audit ledgers and decide abuse reports. | `internal/user/model.go:23` |
| 出站通道 | Outbound Channel | Best-effort notification delivery implementation for SMTP, webhook, or Web Push. | `internal/notification/channel.go:18` |
| 支付适配器 | Payment Adapter | Channel seam that converts a receivable into a receipt, optionally with async webhook confirmation. | `internal/payment/adapter.go:33` |
| 异步支付适配器 | Async Payment Adapter | Optional payment face that creates pending channel charges and parses signed webhook events. | `internal/payment/adapter.go:51` |
| 服务商插件面 | Provider Plugin Face | Facet registry for provider-specific sharing-policy validation, admission guards, and usage validation. | `internal/provider/plugin.go:57` |
| 内置 Claude 插件 | Built-in Claude Plugin | Example provider plugin that validates residency/plan policy, admission caps, and usage units. | `providers/claude/plugin.go:46` |
| 计费调度器 | Billing Scheduler | Background ticker that opens and equally splits next periods for subscriptions enabled for auto-billing. | `internal/automation/scheduler.go:28` |
| 进程生命周期 | Process Lifecycle | Signal-aware startup sequence for configuration, database, bootstrap, HTTP, scheduler, and graceful shutdown. | `apps/server/main.go:189` |
| 内嵌资源 | Embedded Assets | Compiled-in web client tree served as static assets with SPA route fallback. | `apps/server/main.go:39` |
| 公共协议 | Public API Contract | Shared JSON, error envelope, pagination, recovery, logging, and rate-limit semantics. | `pkg/api/errors.go:16` |
| 速率限制器 | Rate Limiter | In-memory fixed-window counter for public registration/login writes, keyed by client address. | `internal/ratelimit/limiter.go:28` |

