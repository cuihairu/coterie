# Coterie 项目总体计划

> **Coterie — Open-source platform for shared digital services.**
>
> Coterie 是一个开源的数字服务共享与订阅拼车平台。

| | |
|---|---|
| 状态 | 规划中（Planning） |
| 文档版本 | v0.1 |
| 相关文档 | [需求文档](./requirements.md) · [设计计划](./design.md) |

---

## 1. 一句话定位

> Coterie 是一个开源的数字服务共享与订阅拼车平台。

英文：

> **Coterie — Open-source platform for shared digital services.**

Coterie 面向适合多人共同使用、共同承担费用的数字服务，例如流媒体、音乐、软件、AI 与云服务。

---

## 2. 核心理念

Coterie 不应该被设计成：

> Netflix 拼车网站

而应该设计成：

> **多人围绕一个数字服务组成共享圈，并共同承担服务成本的平台。**

例如：

```text
                    Coterie
                       │
          ┌────────────┼────────────┐
          │            │            │
       Streaming      Music         AI
          │            │            │
       Netflix       Spotify      AI Service
       Disney+       YouTube      Software
          │            │            │
          └────────────┼────────────┘
                       │
                    Coterie
                  /    |    \
               Alice  Bob   Charlie
```

---

## 3. 为什么重新设计

旧项目主要围绕影视账号，模型是：

```text
影视平台
   ↓
账号
   ↓
账号成员
   ↓
费用
```

这种模型存在明显限制：

1. Provider 被写死为影视平台。
2. 账号和订阅概念混在一起。
3. 很难扩展到 AI、软件、音乐、游戏等服务。
4. 很难表达不同服务的不同共享规则。
5. 不容易支持「席位」「额度」「设备数量」「区域限制」等复杂约束。
6. 后续很容易演化成大量 Provider 特殊代码。

因此新版本不再以「影视账号」为核心，而是从业务模型上彻底抽象，统一为：

```text
Provider → Product → Subscription → Coterie → Member / Seat
```

**这次重新开源最应该改变的地方：不是换一个名字，而是重新定义领域模型。**

---

## 4. 产品边界

Coterie 最终应该成为：

> **一个「数字服务共享基础设施」，而不是一个「影视账号交易网站」。**

它解决的是：

```text
我有一个数字服务
        ↓
它允许多人共享
        ↓
我需要组织这些人
        ↓
管理席位
        ↓
管理费用
        ↓
管理周期
        ↓
管理成员
        ↓
管理访问方式
```

这才是 Coterie 真正的核心价值。

### 4.1 明确不是什么

- 不是第三方账号的交易市场（Marketplace 是后续独立模块）。
- 不是密码管理器（默认不保存第三方账号密码，见 [需求文档 NFR-1](./requirements.md#nfr-1-安全模型)）。
- 不是支付系统（支付是 Adapter，第一阶段只做手动结算）。
- 不是「自动登录/自动注册第三方服务」的自动化工具。

### 4.2 最终愿景

```text
                    Coterie
                       │
        ┌──────────────┼──────────────┐
        │              │              │
     Streaming       Software         AI
        │              │              │
     Netflix         Adobe          Claude
     Disney+         JetBrains      ChatGPT
        │              │              │
        └──────────────┼──────────────┘
                       │
                Shared Services
                       │
                  ┌────┴────┐
                  │ Coterie │
                  └────┬────┘
                       │
            ┌──────────┼──────────┐
            │          │          │
          Member     Member     Member
            │          │          │
          Seat       Seat       Seat
            │          │          │
            └──────────┼──────────┘
                       │
                    Billing
                       │
                  Contribution
```

**Coterie 的最终目标不是帮助用户「租账号」，而是帮助用户组织和管理合法的共享数字服务。**

---

## 5. 产品设计原则

这八条原则是所有后续设计与实现决策的最高约束。

| # | 原则 | 含义 |
|---|------|------|
| 1 | **Generic First** | 不做 Netflix-first，做 Generic-first。Netflix 只是一个 Provider。 |
| 2 | **Subscription ≠ Account** | 订阅和账号必须分离。订阅之下才是 Account、Seats、Quota、Access。 |
| 3 | **Member ≠ Account** | 用户是平台成员，第三方账号是外部资源，两者必须分离。 |
| 4 | **Coterie 是核心** | Coterie 是产品核心概念，代表「一群人围绕一个共享资源形成的协作圈」。 |
| 5 | **Provider Agnostic** | 核心系统不依赖 Netflix、Disney+ 等任何具体平台。 |
| 6 | **Self-hosting First** | 开源项目应该：下载 → 配置 → Docker Compose → 运行。 |
| 7 | **Payment Is an Adapter** | 支付不是核心业务模型。 |
| 8 | **Secret Is Optional** | 默认不保存第三方账号密码。 |

补充推论（来自领域模型设计）：

- **Coterie 的核心不是 Account Sharing，而是 Resource / Subscription Sharing。**
- **Provider Adapter 必须是可选扩展**，核心平台不应依赖任何特定 Provider。
- **平台能力 ≠ Provider 允许**：系统不能默认宣称所有账号都可以共享。

---

## 6. 路线图

### Phase 1 — MVP

只实现这些模块：

```text
User
Provider
Product
Subscription
Coterie
Member
Seat
Contribution
Invitation
Notification
```

完整流程：

```text
注册
 ↓
创建 Provider
 ↓
创建 Product
 ↓
创建 Subscription
 ↓
创建 Coterie
 ↓
设置 Seats
 ↓
邀请成员
 ↓
成员加入
 ↓
记录费用
 ↓
周期结算
```

第一阶段明确**不做**的内容见 [需求文档 · MVP 非目标](./requirements.md#7-mvp-非目标)。

### Phase 2

```text
Marketplace
Provider Catalog
Payment Adapter
Email
Webhook
Usage Tracking
Quota
```

目标流程：

```text
Browse
 ↓
Find Coterie
 ↓
Request Join
 ↓
Payment
 ↓
Join
```

### Phase 3（已完成）

```text
Provider Plugins        ✅ PolicyValidator / AdmissionGuard / UsageValidator 插件面（D12）
Payment Plugins         ✅ 配置注册渠道 + Sandbox 演示适配器（D11）
Usage Plugins           ✅ D12 第三钩子（记账复验）
Automation              ✅ auto_billing 调度器滚期（D13）
Advanced Billing        ✅ prorated 按天比例分摊
Dispute                 ✅ 争议账本 + Owner 单向裁决（D14）
Reputation              ✅ 结算信誉派生只读投影（D15）
```

### Phase 4 — 愿景与工作清单

最终成为：

> **Open-source marketplace and infrastructure for shared digital services.**

```text
                    Coterie
                       │
       ┌───────────────┼────────────────┐
       │               │                │
    Sharing         Billing          Marketplace
       │               │                │
   Membership       Payment          Discovery
   Seat             Settlement       Matching
   Quota            Contribution     Reputation
       │               │                │
       └───────────────┼────────────────┘
                       │
                  Provider System
```

愿景落地为以下批次工作清单（按序连做，勾选即完成；每项先落设计条目再实现）：

1. [x] **Audit Log**（FR-15 / NFR-7 Phase 1 基础）— append-only `audit_logs` 账本 + 资金/成员变更同事务埋点 + Owner 只读审计端点（设计 D17、§6.3）
2. [x] **Marketplace 信誉展示位**（FR-18 后续）— 目录条目附 Owner 结算信誉徽标（§4.4 预留）
3. [x] **Rate Limit**（NFR-7 Phase 1 基础）— 注册/登录等公开端点限速
4. [x] **自定义计费周期自动化**（§13 遗留）— custom 周期表达方式决策 + 调度器滚期支持（D13 扩展）
5. [x] **Sharing Policy UsageLimit**（FR-10 后续扩展）— 核心侧每成员每期用量上限（D21）
6. [x] **真实支付渠道**（FR-9 后续）— Stripe 异步适配器 + 公开 Webhook（D24）；Marketplace 支付闸门（D25：payment_gate + awaiting_payment + 准入费独立账本 + Webhook 驱动入圈）。生产使用仍需真实凭据
7. [x] **Push 通知**（FR-12 后续）— Web Push 标准渠道（D22）
8. [x] **Resource 共享模式**（FR-6 后续，§2.5）— 复用 Seat+metadata（D23）
9. [x] **防滥用收口**（NFR-7 / FR-14）— 平台管理员角色（`ADMIN_EMAILS` 启动引导）、举报（公开圈可报 + 管理员收件箱与 resolve/dismiss）、圈级拉黑（全准入路径 403）（D26）

---

## 7. 对外表述（README 首页）

Logo 已入库：`assets/logo.svg`（圆形 + 直角切角的「C」字标，主色 `#FF0054`）。

重新开源时 README 首页应表达：

````markdown
<div align="center">
  <img src="assets/logo.svg" width="120" alt="Coterie logo" />
  <h1>Coterie</h1>
  <p><strong>Open-source platform for shared digital services.</strong></p>
  <p>
    <a href="LICENSE"><img alt="License" src="https://img.shields.io/badge/license-Apache--2.0-blue"></a>
    <img alt="Go" src="https://img.shields.io/badge/Go-1.22%2B-00ADD8?logo=go&logoColor=white">
    <img alt="Docker" src="https://img.shields.io/badge/docker-compose%20ready-2496ED?logo=docker&logoColor=white">
  </p>
</div>

Organize subscriptions, seats, members, and shared costs in one place.

Coterie is designed for any digital service that can be legitimately shared
among multiple people — from streaming and music to software, AI, cloud
services and more.
````

紧接着展示核心模型：

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

徽章（shields.io）保持最少必要：License、Go 版本、Docker 状态；CI 就绪后再加构建/覆盖率徽章。

> 注：当前 `README.md` 仅有标题，重新开源前需要按本节重写。

---

## 8. 文档结构

### 8.1 当前文档与资源

```text
docs/
├── project-plan.md   # 本文：定位、愿景、原则、路线图、文档索引
├── requirements.md   # 需求文档：功能需求、非功能需求、MVP 与不做清单
└── design.md         # 设计计划：领域模型、共享模型、计费/支付、
                      # 技术架构、模块划分、API、部署

assets/
└── logo.svg          # 项目 Logo（主色 #FF0054）
```

阅读顺序建议：**project-plan（为什么做）→ requirements（做成什么）→ design（怎么做）**。

每部分内容只在一个文档中维护，其余文档通过相对链接引用，避免重复导致的不一致。

### 8.2 规划中的文档

随着项目推进，`docs/` 可以扩展为：

```text
docs/
├── project-plan.md
├── requirements.md
├── design.md
├── architecture.md
├── domain-model.md
├── sharing-model.md
├── billing.md
├── provider.md
├── plugin.md
├── security.md
├── deployment.md
└── roadmap.md
```

### 8.3 仓库根目录文档

```text
README.md
LICENSE               # Apache 2.0
CONTRIBUTING.md
SECURITY.md
CODE_OF_CONDUCT.md    # Contributor Covenant v2.1
```

以上文件均已就位；README 按 [§7](#7-对外表述readme-首页) 的表述编写。
