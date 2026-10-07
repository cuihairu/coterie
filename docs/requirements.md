# Coterie 需求文档

| | |
|---|---|
| 状态 | 草稿（Draft） |
| 文档版本 | v0.1 |
| 相关文档 | [总体计划](./project-plan.md) · [设计计划](./design.md) |

---

## 1. 概述

### 1.1 目标

Coterie 提供一套通用的数字服务共享基础设施，让用户能够：

- 持有一个数字服务（订阅）；
- 确认该服务允许多人共享；
- 组织围绕该服务的共享圈（Coterie）；
- 管理席位、费用、周期、成员与访问方式。

本需求文档定义 **Phase 1（MVP）** 需要交付的能力，以及后续阶段的需求归属。

### 1.2 适用范围

Coterie 是 **Generic-first** 的：任何可被合法共享的数字服务都可以进入模型，包括但不限于流媒体、音乐、软件、AI、云服务、VPN、游戏服务。

核心系统不依赖任何特定 Provider（Netflix、Spotify 等只是 Provider 实例）。

### 1.3 术语

本文使用以下核心术语，完整模型定义见 [设计计划 · 领域模型](./design.md#1-领域模型)。

| 术语 | 含义 |
|------|------|
| **Provider** | 数字服务提供商（如 Netflix、Spotify、Claude），只描述提供方，不描述套餐。 |
| **Product** | Provider 提供的具体产品或套餐（如 Netflix Premium）。 |
| **Subscription** | 实际购买的订阅实例，含价格、周期、续费日、共享策略等。 |
| **Coterie** | 一群人围绕某项数字服务组成的共享圈，是系统最核心的概念。 |
| **Member** | Coterie 中的参与者，平台用户身份。 |
| **Seat** | 订阅容量的切分单元（通用分配单元，Quota 模式复用），与 Member 解耦。 |
| **Contribution** | 成员对共享费用的承担记录。 |
| **Sharing Policy** | 该订阅的共享规则（人数、设备、区域、额度等限制）。 |
| **Sharing Model** | 共享模式：Account / Seat / Family / Quota / Resource。 |

---

## 2. 用户与角色

### 2.1 平台级角色

| 角色 | 说明 |
|------|------|
| **User** | 普通用户，可创建/加入 Coterie。 |
| **Platform Admin** | 平台管理员，负责平台级管理，不参与每个 Coterie 的日常管理。 |

### 2.2 Coterie 内角色

```text
Owner
Admin
Member
```

未来可增加：

```text
BillingManager
Moderator
Viewer
```

角色与权限的细化在设计阶段确定；MVP 至少区分 Owner（创建者/管理者）与 Member（参与者）。

---

## 3. 核心用户旅程（MVP）

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

这是 MVP 的端到端验收流程：以上每一步都必须可以通过 Web 界面与 API 完成。

---

## 4. 功能需求

需求编号规则：`FR-x` 为功能需求，`NFR-x` 为非功能需求。编号一旦发布保持稳定，供其他文档与实现引用。

### FR-1 用户与认证

**必须**提供基础用户体系。

用户字段：

```text
User
├── ID
├── Username
├── Email
├── Avatar
├── Locale
├── Timezone
├── CreatedAt
└── Status
```

登录方式支持：

- Email
- OAuth
- Passkey
- OIDC

具体登录方式通过认证模块扩展，不写死在核心业务中。

### FR-2 Provider

Provider 表示数字服务提供商。

```yaml
provider:
  id: netflix
  name: Netflix
  category: streaming
```

要求：

- 用户可以创建、浏览 Provider；
- Provider 只描述服务提供方，**不包含具体套餐信息**；
- 分类（category）用于组织，例如 `streaming`、`music`、`ai`、`software`；
- **Generic Provider 必须开箱可用**，允许用户手动创建任意服务（见 FR-3）。

示例类别下的 Provider：Netflix、Disney+、Spotify、YouTube、Claude、ChatGPT、Adobe、Microsoft、JetBrains、VPN Provider、Cloud Provider、Game Service。

### FR-3 Product

Product 表示 Provider 提供的具体产品或套餐。

```text
Netflix
├── Standard
├── Premium
└── Other

Adobe
├── Creative Cloud
├── Acrobat
└── Photoshop
```

要求：

- Product 归属某个 Provider；
- 套餐信息**不写死在共享组（Coterie）里**，必须通过 Product 建模；
- 即使没有 Provider Adapter，用户也能通过 Generic Provider 手动创建任意 Provider / Product / Subscription：

```text
Provider:    My Software
Product:     Premium Plan
Subscription: 5 Seats
Coterie:     Development Team
Members:     5
```

### FR-4 Subscription

Subscription 表示实际购买的订阅。

```text
Netflix Premium
        │
        ▼
Subscription #12345
        │
        ▼
$22.99 / month
```

必须包含：

- Product
- Owner
- Billing Cycle（计费周期）
- Price（价格）
- Currency（货币）
- Start Date（开始日期）
- Renewal Date（续费日期）
- Status（状态）
- Sharing Policy（共享策略）
- Maximum Members（最大成员数）
- Maximum Seats（最大席位数）

### FR-5 Coterie

**Coterie 是整个系统最核心的概念。**

一个 Coterie 表示：

> 一群人围绕某项数字服务组成的共享圈。

```text
Coterie #1001

Netflix Premium
$22.99 / month

Owner
  ↓
Alice

Members
  ├── Alice
  ├── Bob
  ├── Charlie
  └── David
```

要求：

- 一个 Coterie **严格对应**一个 Subscription（1:1，决策 D1，见 [设计计划 §1.8](./design.md#18-设计决策记录adr)）；
- Coterie **不假设所有服务都是「账号共享」**，必须支持不同共享模式（见 FR-6）；
- 必须支持以下生命周期：

```text
Draft ──▶ Open ──▶ Active ⇄ Paused
   │        │        │
   └────────┴────────┴────▶ Closed（终态）
```

生命周期语义：

| 状态 | 含义 |
|------|------|
| **Draft** | 创建共享组，尚未发布。 |
| **Open** | 已发布，等待成员加入。 |
| **Active** | 正式运行中。 |
| **Paused** | 暂停（保留成员与席位，暂停结算，可恢复）。 |
| **Closed** | 结束共享（终态）。 |

**Full 不是持久化状态**：「席位已满」是由空闲席位数推导的展示标志（空闲席位为 0 时成立、释放后自动解除），用于展示与加入校验。状态机与流转规则见 [设计计划 §1.5](./design.md#15-coterie-生命周期与状态机)。

### FR-6 共享模式

系统**必须**支持多种共享模式，而不是只支持账号共享。这是 Coterie 最重要的扩展点之一。

#### 6.1 Account Sharing（账号共享）

多个用户共享一个账号。

```text
Netflix Account
      │
 ┌────┼────┐
 ▼    ▼    ▼
User User User
```

#### 6.2 Seat Sharing（席位共享）

每个人拥有独立席位。

```text
Software Subscription
        │
 ├── Seat 1 → Alice
 ├── Seat 2 → Bob
 └── Seat 3 → Charlie
```

适用：软件、SaaS、AI、团队服务。

#### 6.3 Family Sharing（家庭共享）

服务本身提供家庭组。

```text
Family Plan
    │
 ├── Owner
 ├── Member
 ├── Member
 └── Member
```

#### 6.4 Quota Sharing（额度共享）

共享额度而非账号。

```text
1000 credits
      │
 ├── Alice → 300
 ├── Bob   → 300
 └── Carol → 400
```

适用：AI、API、云、存储、VPN 流量。

> 建模方式：复用 Seat，每个 Seat 携带配额 metadata（见 FR-8 与 [设计计划 §1.6](./design.md#16-seat通用分配单元)）。

#### 6.5 Resource Sharing（资源共享）

共享某种资源。

```text
Cloud Storage
     │
     ├── 1 TB
     │
     ├── Alice
     ├── Bob
     └── Charlie
```

> **结论：Coterie 的核心不是 Account Sharing，而是 Resource / Subscription Sharing。**

### FR-7 Member

Member 表示 Coterie 中的参与者。

```text
Member
├── User
├── Role
├── Status
├── JoinedAt
└── LeftAt
```

要求：

- Member 关联平台 User，**Member ≠ 第三方账号**；
- 支持加入、移除、角色分配；
- 角色见 [§2.2](#22-coterie-内角色)。

### FR-8 Seat

Seat 是非常重要的抽象。**不要假设「一个 Member = 一个账号」。**

```text
1 Subscription
    │
    ├── Seat A
    ├── Seat B
    ├── Seat C
    └── Seat D
```

设计约束：

- Seat 归属 **Subscription**（容量源头），Coterie 只负责把席位分配给成员（决策 D2）；
- 任一时刻每个 Seat 至多一名占用者，允许空闲；
- Quota 共享同样复用 Seat，每个 Seat 携带配额 metadata（决策 D4）。

Member 与 Seat 的关系必须允许：

- 一个成员一个 Seat；
- 一个成员多个 Seat；
- Seat 暂时空闲；
- Seat 转移；
- Seat 回收。

### FR-9 费用分摊与结算（Contribution / Billing）

Contribution 表示成员对共享费用的承担。

```text
Subscription
$30/month

Alice      $10
Bob        $10
Charlie    $10
```

每条 Contribution 必须记录：

```text
amount
currency
billing_cycle
payer
status
period
```

计费（Billing）流程：

```text
Subscription
      │
      ▼
Billing Period
      │
      ▼
Member Contribution
      │
      ▼
Settlement
```

必须支持的计费方式：

- 月付 / 年付 / 自定义周期
- 按席位
- 按比例
- 固定金额
- 按使用量

**重要：Coterie 不应把「付款」直接绑定支付平台。** 第一阶段只实现 **Manual Settlement**——系统负责记录谁应该支付多少钱。真实支付通过 Payment Adapter 在后续阶段接入（见 [设计计划 · 计费与支付](./design.md#4-计费与支付)）。

### FR-10 Sharing Policy（共享限制）

不同服务具有不同共享规则，系统必须能通过 Sharing Policy 表达：

```yaml
sharing:
  mode: seat
  max_members: 5
  max_devices: 10
  region: US
  transfer: true
```

```yaml
sharing:
  mode: quota
  quota: 1000
  unit: credits
```

必须支持（当前或未来扩展）的限制类型：

```text
DeviceLimit
RegionLimit
ConcurrentLimit
MemberLimit
QuotaLimit
UsageLimit
TimeLimit
```

### FR-11 Invitation

```text
Owner
  │
  ▼
Invitation
  │
  ├── Token
  ├── ExpireAt
  ├── Role
  └── Coterie
```

必须支持：

- 邀请链接
- 邀请码
- Email Invitation
- Private Invitation

### FR-12 Notification

统一通知系统，必须覆盖以下类型：

```text
Notification
├── Invitation
├── Payment Due
├── Subscription Renewal
├── Seat Assigned
├── Subscription Expired
├── Coterie Closed
└── System
```

通知渠道：

```text
Web
Email
Push
Webhook
```

第一阶段只需实现 **Web Notification + Email Adapter**。

### FR-13 Dashboard

#### 13.1 用户首页

```text
My Coteries

Netflix
● Active
$5 / month

Spotify
● Active
$3 / month

AI Service
● Active
$10 / month
```

同时展示：

- 当前费用
- 下次结算时间
- Seat
- Subscription 状态
- 邀请
- 待付款
- 通知

#### 13.2 Owner Dashboard

```text
Subscription
├── Price
├── Billing
├── Members
├── Seats
├── Contributions
└── Settlement
```

例如：

```text
5 Seats

Occupied: 4
Available: 1

Monthly Cost: $30

Collected: $24
Pending: $6
```

### FR-14 Admin

平台管理员负责：

```text
Users
Providers
Products
Coteries
Subscriptions
Reports
System
```

**Admin 不参与每个 Coterie 的日常管理。**

### FR-15 Audit Log

由于涉及共享资源和费用，Audit Log **必须**记录：

```text
Who
What
When
Where
Before
After
```

示例：

```text
Alice
changed subscription price
$20 → $22
2026-10-07
```

重点记录的操作：

- 创建 Coterie
- 删除成员
- 转移 Seat
- 修改费用
- 修改订阅
- 结算
- 关闭 Coterie

### FR-16 Marketplace（Phase 2）

第一阶段不做重，作为后续独立模块。

```text
Browse
  ↓
Provider
  ↓
Product
  ↓
Available Coteries
  ↓
Join
```

例如：

```text
Netflix Premium

Available Coteries

Coterie #1024
3 / 5 seats
$5 / month

Coterie #1025
4 / 5 seats
$4.8 / month
```

**约束：Coterie 本身不应默认成为「账号交易市场」。**

### FR-17 Dispute（Phase 3）

第一阶段不实现。字段设计预留：

```text
Dispute
├── Buyer
├── Owner
├── Coterie
├── Transaction
├── Reason
├── Evidence
├── Status
└── Resolution
```

---

## 5. 非功能需求

### NFR-1 安全模型

**不要把账号密码作为核心模型**——这是最重要的设计原则之一。

系统应尽量存储：

```text
Credential Metadata
```

而不是：

```text
Provider Password
```

例如：

```text
Access Method:
    External

Credential:
    Not stored
```

如果未来确实需要保存 Account / Password / Recovery Code / API Key / License Key，必须进入独立的 **Secret 模块**：

```text
Coterie
   │
   ▼
Secret Reference
   │
   ▼
Secret Store
```

数据库只保存 `secret_id`，而不是明文 Secret。

> 对应设计原则 #8：**Secret Is Optional**。

### NFR-2 合规与风险

Coterie 是通用共享平台，**必须**区分：

```text
Platform Capability
        ≠
Provider Permission
```

某些服务允许家庭共享、团队成员、多人席位，但不允许账号共享，或存在地区限制。因此：

- 系统**不能默认宣称**「所有账号都可以共享」；
- 必须提供 Sharing Policy、Provider Terms、Region、Eligibility 等信息载体；
- **UI 必须明确提示**：用户需自行确认相关 Provider 的服务条款、地区限制和共享权限；
- 平台应尽可能支持**合法授权的家庭、团队、席位和订阅共享**。

### NFR-3 Self-hosting 与部署

开源项目必须做到：

```bash
docker compose up -d
```

即可运行。

最小部署：

```yaml
services:
  coterie:
    image: ...

  postgres:
    image: postgres
```

支持目标：Docker、Docker Compose、Kubernetes、Bare Metal；**官方优先 Docker Compose**，因为目标用户不一定是专业运维人员。

运行时目标形态：

```text
Single Binary
     +
PostgreSQL
```

### NFR-4 API-first

从第一天就设计 API-first，Web 界面只是 API 的一个客户端。

核心资源：

```text
/api/v1/users
/api/v1/providers
/api/v1/products
/api/v1/subscriptions
/api/v1/coteries
/api/v1/members
/api/v1/seats
/api/v1/contributions
/api/v1/invitations
/api/v1/notifications
```

### NFR-5 可扩展性

- **Provider Agnostic**：核心业务中不允许出现 `if netflix` / `if disney` / `if spotify` / `if chatgpt` 之类的分支；
- Provider Adapter 是**可选扩展**，核心平台不依赖任何特定 Provider；
- 通过 Plugin 架构（providers / payment / notification / authentication / storage）承载未来扩展，核心系统保持稳定；
- Provider Catalog / Registry 只是 **Metadata**，不与具体账号绑定。

### NFR-6 多租户

第一阶段明确：

> **Single Tenant / Self-hosted First**

不要过早加入复杂 SaaS Multi-tenancy。未来如需支持 SaaS，再引入 Instance → Organization → Users / Coteries 的层级。

### NFR-7 防滥用

第一阶段只实现基础能力：

```text
Rate Limit
Report
Block
Audit Log
```

后续开放公共 Marketplace 时再增加：

```text
Anti-spam
Fraud Detection
Account Reputation
Dispute
```

---

## 6. MVP 范围

**Phase 1 必须交付的模块：**

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

**验收标准**：[§3 核心用户旅程](#3-核心用户旅程mvp) 的十一步流程全部可走通（Web + API），且满足 NFR-1 至 NFR-7 中标注为第一阶段的部分。

---

## 7. MVP 非目标

第一版明确**不做**，避免项目失控：

- 复杂 Marketplace
- 自动购买
- 自动注册 Provider
- 自动登录第三方服务
- 自动抓取第三方账号
- 自动支付所有 Provider
- 钱包
- 积分
- 加密货币
- 复杂推荐系统
- AI Agent
- 大规模风控

---

## 8. 需求阶段归属

| 编号 | 需求 | 阶段 |
|------|------|------|
| FR-1 | 用户与认证 | Phase 1 |
| FR-2 | Provider（含 Generic Provider） | Phase 1；Phase 2（目录公开读 + 种子注册表，见设计 §5.4/§6.1）；Phase 3（插件面 PolicyValidator/AdmissionGuard，见 D12） |
| FR-3 | Product | Phase 1 |
| FR-4 | Subscription | Phase 1 |
| FR-5 | Coterie 与生命周期 | Phase 1 |
| FR-6 | 共享模式 | Phase 1（Account / Seat / Family 基础支持）；Phase 2（Quota 用量记账与 usage 分摊，见设计 D9；Resource 共享后续扩展） |
| FR-7 | Member | Phase 1 |
| FR-8 | Seat | Phase 1 |
| FR-9 | 费用分摊与结算（Manual Settlement） | Phase 1；Phase 2（Adapter 接口 + `payments` 账本 + Manual 渠道）；Phase 3（Sandbox 演示渠道经配置注册，`GET /payments/methods` 列渠道；真实渠道 → 后续） |
| FR-10 | Sharing Policy | Phase 1（基础限制）；Phase 3（MemberLimit `max_members` 准入强制 + Provider 插件策略复验/准入守卫已落地，见设计 D12；其余 Limit 后续扩展） |
| FR-11 | Invitation | Phase 1 |
| FR-12 | Notification（Web + Email） | Phase 1；Phase 2（Email SMTP 适配器 + 出站 Webhook 已落地；Push 后续） |
| FR-13 | Dashboard | Phase 1 |
| FR-14 | Admin | Phase 1（基础） |
| FR-15 | Audit Log | Phase 1 |
| FR-16 | Marketplace | Phase 2（只读目录 + JoinRequest 收件箱已落地，见设计 D10；支付门槛随 Payment Adapter） |
| FR-17 | Dispute | Phase 3 |
| NFR-1 ~ NFR-7 | 非功能需求 | 见各条目内的阶段标注 |
