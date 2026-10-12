# TODO

> 生成于 2026-10-11（无 todo 文件；来源：docs/project-plan.md 全文 **0 个未完成 checkbox**，Phase 4 愿景 14 项全部 [x]，近期走 PR 流 #7-#20 已合）。

## 待用户审核（本仓疑似已清空，需人工确认后决定是否重新派活）
- [ ] 用户逐仓审核：确认 Phase 1 / Phase 2 是否存在 project-plan 之外的未落地项（本轮自动扫描未发现未完成 checkbox）
- [ ] 审核通过后由用户分配下一批任务，再重新加入监控

审核材料（2026-10-12，Phase 1/2 逐 FR 落地证据，供审核一眼核对；
每条均为「代码模块 + 测试 + 文档三面齐」状态）：

| FR | 阶段归属 | 证据 |
|----|---------|------|
| FR-1 用户与认证 | P1 | internal/user + internal/auth（register/login/me/logout，D8 会话），各有测试 |
| FR-2 Provider | P1；P2 目录公开读+种子 | internal/provider（2 个测试文件）；种子注册表随 migration 0006 |
| FR-3 Product | P1 | internal/product，有测试 |
| FR-4 Subscription | P1 | internal/subscription，有测试 |
| FR-5 Coterie 生命周期 | P1 | internal/coterie（状态机/名称校验），有测试 |
| FR-6 共享模式 | P1 基础；P2 Quota | internal/seat + internal/usage（D9 usage 分摊），D23 Resource 复用 Seat |
| FR-7 Member | P1 | internal/coterie（成员/退出/移除），同包测试 |
| FR-8 Seat | P1 | internal/seat，有测试 |
| FR-9 分摊结算 | P1 手动；P2 账本 | internal/billing + internal/payment（adapter+payments 账本）+ internal/automation（D13 滚期） |
| FR-10 Sharing Policy | P1 基础 | internal/coterie policy + internal/usage（D21 上限强制） |
| FR-11 Invitation | P1 | internal/coterie/http.go:28-30 三条邀请路由（create/list/accept） |
| FR-12 Notification | P1；P2 三通道 | internal/notification（4 个测试文件）+ internal/push（D22） |
| FR-13 Dashboard | P1 | apps/web SPA 全页面，vitest 18/18 |
| FR-14 Admin | P1 基础 | ADMIN_EMAILS 引导 + internal/report（D26 收件箱） |
| FR-15 Audit Log | P1 | internal/audit（订阅/圈/平台三级账本），有测试 |
| FR-16 Marketplace | P2 | internal/marketplace（目录+JoinRequest+D25 闸门），有测试 |

端到端面：scripts/smoke.sh 119 检查全绿覆盖十一步旅程；全仓 23 个
internal 包中 21 个有直接测试（apps/server 装配层与 testsupport 为合理例外）。

## 维护项（常规）
- [ ] 依赖与安全告警按需处理（dependabot；code scanning 见下条，暂不启用）
- [ ] 每批 PR 合并后 README/文档站同步

最近一次分诊（2026-10-11）：dependabot 开放告警 0；npm audit 两棵树 0 漏洞；
govulncheck 可达漏洞清零——工具链钉到 go1.27.2 修掉 10 个 stdlib 漏洞，
仅剩 x/crypto 的 openpgp 模块级公告（未导入该子包，Fixed: N/A，常驻噪音）；
已合 PR #21 无用户可见 API 变化，README 无需同步。

跳过项清偿（2026-10-12）：typescript 7.0.2 与 @types/node 26 两个 major
经验证合入（#23，tsc/vitest/lint 全绿零代码改动）；vapidkeygen 补上
密钥契约测试（#24，生成对须过服务端同款校验）；nightly 定时构建正常
（10-10 schedule 成功）。零覆盖仅剩 apps/server 装配层与 testsupport
（测试基建自身），属合理状态。

分诊续批（2026-10-12）：dependabot 0、npm audit 两树 0、govulncheck
可达 0（openpgp 模块级噪音同前）；新过期项仅 @tanstack/react-query
5.104.1→5.104.2 patch，随本轮合入。#23/#24 无 API 变化，README 无需同步。

一致性复查（2026-10-12 第二轮）：docs/api.md 82 条路由与代码 mux 注册
逐条一致（healthz 根路径亦一致）；修掉 README.md/README.zh.md 徽章与
前置声明仍写 Go 1.25+ 的漂移（实际底线 1.26）；code scanning 维持未
启用（按用户明令暂挂，登记在案）；vitepress 维持 alpha pin（latest
1.6.4 正是已修漏洞线）。

## 暂不开（等用户明令）
- [ ] CodeQL / code scanning：暂不开。无用户明令，且会引入持续告警分诊面，按默认从简口径挂起。

## 等外部密钥/凭据（暂挂）
- [ ] Web Push 公钥端点：推送通道与订阅端点已就绪，SPA 还需服务端 VAPID 公钥才能订阅；等外部密钥。
- [ ] Stripe 支付渠道（D24）：等凭据。

验收：以用户审核结论为准；新任务由用户分配后按测试绿才 push 执行。
