# TODO

> 需求清单：一条一句。完成项保留一行证据（commit + 测试数）。

## 待用户操作
- [x] 逐仓审核：确认 Phase 1/2 无 project-plan 之外的未落地项 — 92007f0，docs build 通过；结论：15 条 Phase 1 FR + 5 项 Phase 2 子需求 + 十步旅程 + NFR 七条 Phase 1/2 部分全部落地，无未跟踪项
- [ ] 审核通过后分配下一批任务
  - 卡点 R1013：依赖上条；所需输入：用户给出下一批任务清单
- [x] CodeQL / code scanning — 2eea3e1，codeql run success
- [x] CodeQL 告警修复（seat 分配上限 + email CRLF 注入）— f7b0b14，go test 全绿
- [x] CodeQL email-injection 收口（to/from/title/body 全经 headerSafe 净化 + 首条误报 dismiss）— 342138c/2b220cd/31ea1a2，0 open alerts，CI 全绿
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
- [x] README 测试口径补 codeql paths-ignore（d3da44b 引入后的漂移）— b628d4d，npm --prefix docs run build 通过

## 待补测
- [x] apps/server 组合根补测：spaHandler 嵌入客户端回落、apiOnly 分流、channels/paymentAdapters/marketplaceAdapters/providerPlugins 配置装配（含 stripe 缺 key 跳过、marketplace webhook secret 回落）— e175ad0，go test ./apps/server/ 全绿 9 用例 27 子用例 0 skip
- [x] internal/database 纯函数层补测：Date（Parse/Value/Scan/Marshal/Unmarshal 的 null、零值、非法输入分支）、JSONB（Value/Scan/Marshal/Unmarshal 与缓冲拷贝）、IsUniqueViolation/IsFKViolation/IsCheckViolation 的 SQLSTATE 与约束名判定 — e1dff49，go test ./internal/database/ 全绿 12 用例 20 子用例 0 skip
- [x] internal/reputation fold 派生层补测：状态分桶、多币种合计与排序、支付率分母（只算 paid+pending）、无计费历史时 payment_ratio 为 null、fmtRatio 取整 — eb4e528，go test ./internal/reputation/ 全绿 9 用例 3 子用例 0 skip

## 维护项（常开）
- [x] 依赖与安全告警按需处理（dependabot / npm audit / govulncheck）— 已核：dependabot 5 条陈旧（vite 8.3.4 与 moby/go-archive v0.3.3 均最新）、npm audit 双树 0、govulncheck 0 可达
- [x] 每批合并后 README/文档站同步 — 已核：PR #36 后审计一致（a570606/841c5ca）；d3da44b 改 codeql paths-ignore 引入漂移，已修 b628d4d
- [x] 冒烟覆盖缺口：6 个端点无 smoke 覆盖（GET providers/products/billing-periods 详情、GET push/vapid-public-key、GET subscriptions 审计、DELETE members）— b59f6e9，smoke 全绿 61/63 路径（2 webhook 端点需异步渠道）
