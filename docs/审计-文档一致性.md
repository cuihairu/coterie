# 文档-代码一致性审计（2026-10-13）

> 按《文档与源码一致性审计规范》执行：三类差异（超前/滞后/不符），逐条 file:line 取证，以码为准修复。
> 审计面：README.md / README.zh.md / docs/index.md / docs/api.md / docs/deployment.md / docs/design.md / docs/requirements.md / docs/project-plan.md / .env.example / deployments/docker-compose.yml / 仓库 About 与 topics。

## 三桶计数

- 文档超前于码：**0**
- 码超前于文档：**1**（P2）
- 描述不符：**5**（P1×2，P2×3）

## 审计表

| ID | 页 | 主张 | 源码证据(file:line) | 类别 | 严重度 | 修法 |
|---|---|---|---|---|---|---|
| D1 | docs/deployment.md:7 | 需要 Go 1.25+ | go.mod:3 `go 1.26.0`（toolchain go1.27.2） | 不符 | P1 | 改为 1.26+ |
| D2 | docs/index.md:45 | 要求 Go 1.25+ | 同 D1 | 不符 | P1 | 改为 1.26+ |
| D3 | docs/index.md:28 | 等额或按用量分摊 | internal/billing/model.go:28-32 五种模式（equal/per_seat/fixed/usage/prorated） | 不符 | P2 | 改为五种分摊模式（D9） |
| D4 | docs/index.md:41 | 接口形态与数据模型仍在按项目计划推进 | docs/project-plan.md:257 Phase 3 已完成；269-309 Phase 4 全部 [x] | 不符 | P2 | 进度描述对齐计划 |
| D5 | docs/index.md:65 | 游离 ``` 围栏（渲染为字面量） | 无对应源码；markdown 结构错误 | 不符 | P2 | 删除 |
| D6 | （无文档） | 路由一致性守卫存在 | internal/app/routes_consistency_test.go:1（mux 注册 ↔ docs/api.md 双向 diff） | 码超前 | P2 | deployment.md 补 CI 门禁说明 |

## 已核验一致（抽样留证）

- 会话有效期 30 天：docs/index.md:56 ↔ internal/auth/service.go:19 `sessionTTL = 30 * 24 * time.Hour`
- 渠道启用条件：docs/deployment.md:54-62 ↔ internal/notification/email.go:38（Host 且 From 非空）、webhook.go:41（URL 非空）、webpush.go:229（store 且 keys 非空）、apps/server/main.go:128,155（StripeSecretKey 非空）
- 配置变量 25 个全对齐：.env.example ↔ internal/config/config.go（envOr/envList/envInt/envBool/envLevel/envDuration 六 helper 提取，零差集）
- API 面 84 条：docs/api.md 84 行 ↔ internal/** mux 注册（含 6 条 PATCH + 根 /healthz），由 internal/app/routes_consistency_test.go 门禁保证
- nightly 双 tag：docs/deployment.md:22 ↔ .github/workflows/nightly.yml:29,51（nightly + nightly-YYYYMMDD）
- 仓库 About/topics：gh repo view 描述与 14 个 topic 与实现面一致，无旧词汇
- 角色三档：docs/index.md:24 ↔ internal/user 模型（owner/admin/member）

## Phase 1/2 完整性审核（2026-10-14）

> 确认 Phase 1/2 无 project-plan 之外的未落地项。证据：FR 表（§4）+ NFR 七条（§5）+ §3 十步旅程。

### Phase 1（15 条 FR）

| FR | 需求 | 代码证据 | 状态 |
|---|---|---|---|
| FR-1 | 用户与认证 | internal/auth + internal/user | ✅ |
| FR-2 | Provider | internal/provider | ✅ |
| FR-3 | Product | internal/product | ✅ |
| FR-4 | Subscription | internal/subscription | ✅ |
| FR-5 | Coterie 与生命周期 | internal/coterie | ✅ |
| FR-6 | 共享模式（Account/Seat/Family） | internal/seat + subscription.sharing_policy | ✅ |
| FR-7 | Member | internal/coterie（members） | ✅ |
| FR-8 | Seat | internal/seat | ✅ |
| FR-9 | 费用分摊与结算（Manual） | internal/billing + internal/payment | ✅ |
| FR-10 | Sharing Policy（基础限制） | internal/subscription | ✅ |
| FR-11 | Invitation | internal/coterie（invitations） | ✅ |
| FR-12 | Notification（Web + Email） | internal/notification | ✅ |
| FR-13 | Dashboard | web client | ✅ |
| FR-14 | Admin（基础） | internal/user（admin）+ internal/report | ✅ |
| FR-15 | Audit Log | internal/audit | ✅ |

### Phase 2（5 项子需求）

| 子需求 | 代码证据 | 状态 |
|---|---|---|
| FR-2 目录公开读 + 种子注册表 | migration 0006 + public reads | ✅ |
| FR-6 Quota 用量记账 | internal/usage | ✅ |
| FR-9 Adapter 接口 + payments 账本 | internal/payment | ✅ |
| FR-12 Email SMTP + Webhook + Push | internal/notification + internal/push | ✅ |
| FR-16 Marketplace（只读目录 + JoinRequest） | internal/marketplace | ✅ |

### §3 十步旅程

全部 10 步均可通过 Web + API 完成，由 scripts/smoke.sh 端到端覆盖。

### NFR 七条（Phase 1/2 部分）

| NFR | Phase 1/2 要求 | 状态 |
|---|---|---|
| NFR-1 | 不存 Provider 密码 | ✅ |
| NFR-2 | UI 合规提示 | ✅（c42aac0，vitest 35/35） |
| NFR-3 | docker compose 一键起 | ✅ |
| NFR-4 | API-first | ✅ |
| NFR-5 | Provider Agnostic + Plugin 架构 | ✅ |
| NFR-6 | Single Tenant / Self-hosted First | ✅（约束，非功能） |
| NFR-7 | Rate Limit + Report + Block + Audit Log | ✅ |

### 结论

Phase 1/2 无 project-plan 之外的未落地项。所有 FR/NFR/旅程条目均有对应代码实现，且由 project-plan Phase 1/2 章节跟踪。
