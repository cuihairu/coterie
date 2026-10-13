# TODO

> 需求清单：一条一句。完成项保留一行证据（commit + 测试数）。

## 待用户操作
- [ ] 逐仓审核：确认 Phase 1/2 无 project-plan 之外的未落地项（证据：FR 表 + NFR 七条 + §3 十步旅程，见 docs/审计-文档一致性.md）
  - 卡点 R1013：等用户签字确认；所需输入：审核通过/驳回结论
- [ ] 审核通过后分配下一批任务
  - 卡点 R1013：依赖上条；所需输入：用户给出下一批任务清单
- [x] CodeQL / code scanning — 2eea3e1，codeql run success
- [ ] Stripe 支付渠道（D24）：等 sk_/whsec_ 真实凭据（适配器与分支测试已齐）
  - 卡点 R1013：无真实密钥（仓内仅测试夹具与 .env.example 占位）；所需输入：sk_test_*/whsec_* 或生产密钥

## 已交付
- [x] Web Push 公钥端点与 SPA 订阅 UI（D22 两端）— 02f4588，vitest 41/41
- [x] 推送端点管理（多设备列表与注销）— c9a7242，vitest 41/41
- [x] 路由一致性守卫（api.md ↔ mux 双向 diff）— 0b715f3，go test 25 包 0 skip
- [x] compose 渠道变量透传（20 个运维变量）— 2ca86a5，docker compose config 验证
- [x] Stripe 适配器分支测试补齐 — 6d1dbde，go test 全绿
- [x] NFR-2 UI 合规提示（ComplianceNotice）— c42aac0，vitest 35/35
- [x] vitest 5.0.4 升级 — ac0045e
- [x] deployment.md 补 STRIPE_API_BASE 配置行 — 49c9a2c
- [x] 文档一致性审计（6 项漂移修复 + todo 整理 130→24 行）— a570606/841c5ca，PR #36 CI 全绿

## 维护项（常开）
- [x] 依赖与安全告警按需处理（dependabot / npm audit / govulncheck）— 已核：dependabot 5 条陈旧（vite 8.3.4 与 moby/go-archive v0.3.3 均最新）、npm audit 双树 0、govulncheck 0 可达
- [x] 每批 PR 合并后 README/文档站同步 — 已核：PR #36 合并后审计核一致（a570606/841c5ca），其后无 PR 合入
- [x] 冒烟覆盖缺口：6 个端点无 smoke 覆盖（GET providers/products/billing-periods 详情、GET push/vapid-public-key、GET subscriptions 审计、DELETE members）— b59f6e9，smoke 全绿 61/63 路径（2 webhook 端点需异步渠道）
