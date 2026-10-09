<div align="center">
  <img src="assets/logo.svg" width="64" height="64" alt="Coterie logo" />
  <h1>Coterie</h1>
  <p><strong>开源的数字服务共享平台。</strong></p>
  <p>
    <a href="LICENSE"><img alt="License" src="https://img.shields.io/badge/license-Apache--2.0-blue"></a>
    <img alt="Status" src="https://img.shields.io/badge/status-early%20development-orange">
    <img alt="Go" src="https://img.shields.io/badge/Go-1.25%2B-00ADD8?logo=go&logoColor=white">
    <img alt="Docker" src="https://img.shields.io/badge/docker-compose%20ready-2496ED?logo=docker&logoColor=white">
  </p>
</div>

[English](README.md) | [中文](README.zh.md)

文档站：https://cuihairu.github.io/coterie/

---

**把订阅、席位、成员和共享开销集中在一处管理。**

Coterie 面向一切可以在多人之间合法共享的数字服务——从流媒体、音乐到软件、AI、云服务等。

> 🚧 **状态:Phase 4 后端已完成。**provider 插件接口(含
> policy/admission/usage 钩子)与通过 `PROVIDER_PLUGINS` 注册的真实
> `claude` 插件示例、经配置注册的沙箱支付渠道、自动出账滚动调度器
> (支持 monthly / yearly / 自定义天数周期)、按天折算(prorated)的分摊、
> 带有所有者裁决的出资争议、结算声誉(独立端点 + 市场目录中的
> Owner 信誉徽标)、只追加的审计日志、公开认证端点限速、
> 共享策略驱动的每成员每账期用量上限、Web Push 通知、
> 资源池共享、Stripe 支付渠道(异步适配器 + 签名公开 Webhook)
> 与市场支付闸门(入圈先付费)——在 Phase 2 的用量跟踪、配额、市场、
> provider 目录、支付适配器与邮件/网页通知渠道之上。
> 距离真实收款只差可用的 Stripe 生产凭据。
> 完整的 REST API 已端到端可用:auth → catalog → subscription → coterie → seats →
> invitations → billing → payments → notifications → usage records →
> marketplace。Web 与移动端尚未启动——现阶段 API 即产品
> ([设计文档 §9](docs/design.md#9-api-design))。

## 快速开始

需要 Go 1.25+ 与一个 PostgreSQL 15+ 实例(迁移会在启动时
自动执行)。

```bash
# 1. 配置(全部变量见 .env.example)
export DATABASE_URL=postgres://coterie:***@localhost:5432/coterie?sslmode=disable
export PORT=8080

# 2. 运行
go run ./apps/server
```

除 `GET /healthz`、`POST /api/v1/auth/register`、
`POST /api/v1/auth/login` 以及目录类的公开读接口(`GET /api/v1/providers`、
`GET /api/v1/products`、`GET /api/v1/marketplace/coteries`)外,所有路由都需要
`Authorization: Bearer *** (token 在注册/登录时签发,
有效期为 30 天)。

针对运行中的服务器,对 MVP 全流程(注册 → provider → 产品 →
订阅 → coterie → 席位 → 邀请 → 加入 → 出账 → 结算 →
通知)做一次完整的冒烟测试:

```bash
scripts/smoke.sh http://localhost:8080
```

## API 概览(v1)

| 领域 | 端点 |
|------|-----------|
| Auth | `POST /api/v1/auth/register` · `login` · `logout` · `GET me` |
| Catalog | `/api/v1/providers` · `/api/v1/products`(CRUD;读接口公开,种子注册表随 migration 0006 提供) |
| Subscription | `/api/v1/subscriptions`(CRUD)· `/api/v1/subscriptions/{id}/seats` |
| Seats | `/api/v1/seats/{id}` · `assign` · `release` |
| Coterie | `/api/v1/coteries`(创建时指定容量、生命周期、成员、退出) |
| Invitations | `POST /api/v1/coteries/{id}/invitations` · `POST /api/v1/invitations/accept` |
| Billing | `/api/v1/subscriptions/{id}/billing-periods` · `generate`(equal / per_seat / fixed / usage / prorated)· `/api/v1/contributions/{id}` |
| Payments | `POST/GET /api/v1/contributions/{id}/payments`(手动,外加已配置的渠道)· `GET /api/v1/payments/{id}` · `GET /api/v1/payments/methods` · `POST /api/v1/payments/webhooks/{method}`(公开,仅签名鉴权) |
| Disputes | `POST /api/v1/contributions/{id}/disputes` · `POST /api/v1/disputes/{id}/decide`(所有者)· `GET /api/v1/subscriptions/{id}/disputes` |
| Reputation | `GET /api/v1/users/{id}/reputation`(推导的结算统计) |
| Usage | `POST/GET /api/v1/subscriptions/{id}/usage-records` · `GET /api/v1/usage-records/{id}` |
| Marketplace | `GET /api/v1/marketplace/coteries`(公开;条目附派生的 Owner 信誉徽标)· `POST/GET /api/v1/coteries/{id}/join-requests` · `POST /api/v1/join-requests/{id}/accept` · `decline` · `DELETE /api/v1/join-requests/{id}` · `POST /api/v1/join-requests/{id}/payments`(支付闸门)· `POST /api/v1/marketplace/webhooks/{method}`(公开) |
| Notifications | `GET /api/v1/notifications` · `POST /api/v1/notifications/{id}/read` · 推送端点 `POST/GET /api/v1/push/subscriptions` · `DELETE /api/v1/push/subscriptions/{id}` |
| Audit | `GET /api/v1/subscriptions/{id}/audit-logs` · `GET /api/v1/coteries/{id}/audit-logs`(Owner 只读) |

错误统一使用 `{"error": {"code", "message", "details?}}` 信封;
列表使用 `{"items": [...], "meta": {total, limit, offset}}`。详见
[设计文档](docs/design.md)。

## 工作原理

```text
Provider
    ↓
Subscription
    ↓
Coterie
    ↓
Members
    ↓
Seats / Quota / Cost
```

- **Provider → Product**——任意服务厂商及其套餐,以普通元数据描述。
  通用优先:核心代码中不存在硬编码的 provider 逻辑。
- **Subscription**——一次真实购买:价格、账单周期、续订日期、
  共享策略、席位容量。成本与容量的唯一事实来源。
- **Coterie**——围绕且仅围绕一个订阅组织的共享圈子:
  成员、角色、邀请与费用分摊。
- **Member / Seat**——人是平台用户;席位是订阅中可分配的单元
  (配额共享复用同一概念)。
- **Contribution**——每个成员在每个账单周期应付的金额。Phase 1
  通过手动方式记录与结算;真实支付是一个适配器,而非核心。

## 设计原则

| | |
|---|---|
| 通用优先 | Netflix 只是一个 provider,永远不是特例 |
| 订阅 ≠ 账号 | 订阅、席位、配额与凭证是彼此独立的概念 |
| Provider 无关 | 核心中没有 `if netflix`——provider 是可选的适配器 |
| 自托管优先 | 单个 Go 二进制 + PostgreSQL,`docker compose up -d` |
| 支付是适配器 | Phase 1 仅提供手动结算 |
| 密钥可选 | 默认不存储任何第三方密码 |

## 文档

| 文档 | 说明 |
|----------|-------------|
| [项目计划](docs/project-plan.md) | 定位、愿景、原则、路线图 |
| [需求](docs/requirements.md) | 功能/非功能需求、MVP 范围与非目标 |
| [设计](docs/design.md) | 领域模型、共享模型、出账、架构、API、部署 |

> 📝 文档目前以中文撰写。

## 路线图

- **Phase 1 — MVP ✅**:用户、provider、产品、订阅、coterie、
  成员、席位、出资、邀请、通知(手动结算)
- **Phase 2 ✅**:市场、provider 目录、支付适配器、邮件、
  webhook、用量跟踪、配额
- **Phase 3 ✅**:provider/支付/用量插件、自动化(自动出账滚动)、
  高级出账(按天折算分摊)、争议、声誉
- **Phase 4 ✅**:审计日志、市场 Owner 信誉徽标、限速、自定义计费周期、
  每成员用量上限、Web Push 通知、资源共享模式、Stripe 支付渠道(D24)、
  市场支付闸门(D25)

## 技术栈

Go(标准库 `net/http`、GORM)· PostgreSQL(手写 SQL 迁移,
启动时由 golang-migrate 执行)· React + TypeScript(Vite、Tailwind
CSS、shadcn/ui、TanStack Query / Router)——详见[设计
文档](docs/design.md)。

## 参与贡献

见 [CONTRIBUTING.md](CONTRIBUTING.md)。也请阅读我们的
[行为准则](CODE_OF_CONDUCT.md)。

## 安全

见 [SECURITY.md](SECURITY.md)。

## 许可证

[Apache License 2.0](LICENSE)
