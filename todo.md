# TODO

> 需求清单：一条一句。完成项保留一行证据（commit + 测试数）。

## 待用户操作
- [ ] Stripe 支付渠道（D24）：等真实凭据（sk_test_*/whsec_*），适配器与分支测试已齐 — 按 docs/review/2026-10-30-decisions.md 暂缓待凭据（2026-10-30）
- [x] 重叠计费周期防护（代码深审 C1，P2）：CreatePeriod 拒绝重叠（半开窗口 [start,end)，边界日归较晚周期、背靠背合法；同 start 仍 409）+ 库层 EXCLUDE gist 约束（migration 0019）+ 23P01 映射 409 — 3b3c806，go test -race 26 包全绿 0 skip
- [ ] Web Push 端点 SSRF 面评估（代码深审 C2，P3）：https 约束已缓解，评估是否需域名/端口白名单（internal/push/service.go:38-42）— 依 docs/review/2026-11-01-code-review.md
- [ ] 本地静态分析工具链刷新（代码深审 C3，P3）：重装 go1.27 版 golangci-lint/staticcheck，恢复本地 lint 门（go.mod:3 toolchain go1.27.3）— 依 docs/review/2026-11-01-code-review.md

## 已交付
- [x] 逐仓审核：确认 Phase 1/2 无 project-plan 之外的未落地项 — 92007f0，docs build 通过
- [x] 席位配额强制（D4 收口）：写后 used 超 quota → 409 — 36e9457，go test 25 包全绿 0 skip
- [x] QuotaLimit 订阅总量上限（FR-10）：全成员每账期合计用量上限，写后超限 409 — 15776d9，go test 25 包全绿 0 skip；Web 端 UsageCapsEditor 双上限编辑 — 9e9e82e，vitest 48/48
- [x] CodeQL / code scanning — 2eea3e1，codeql run success
- [x] CodeQL 告警修复（seat 分配上限 + email CRLF 注入）— f7b0b14，go test 全绿
- [x] CodeQL email-injection 收口（to/from/title/body 全经 headerSafe 净化）— 342138c/2b220cd/31ea1a2，0 open alerts
- [x] Web Push 公钥端点与 SPA 订阅 UI（D22 两端）— 02f4588，vitest 41/41
- [x] 推送端点管理（多设备列表与注销）— c9a7242，vitest 41/41
- [x] 路由一致性守卫（api.md ↔ mux 双向 diff）— 0b715f3，go test 25 包 0 skip
- [x] compose 渠道变量透传（20 个运维变量）— 2ca86a5，docker compose config 验证
- [x] Stripe 适配器分支测试补齐 — 6d1dbde，go test 全绿
- [x] NFR-2 UI 合规提示（ComplianceNotice）— c42aac0，vitest 35/35
- [x] vitest 5.0.4 升级 — ac0045e
- [x] deployment.md 补 STRIPE_API_BASE 配置行 — 49c9a2c
- [x] 文档一致性审计（6 项漂移修复 + todo 整理 130→24 行）— a570606/841c5ca，PR #36 CI 全绿
- [x] README 测试口径补 codeql paths-ignore — b628d4d，npm --prefix docs run build 通过
- [x] 调度器生产入口补测：Run 覆盖 0→81.8%，短间隔 ticker 自转滚期、ctx 取消干净退出 — f143527；PATCH cycle_days=0 对 custom 的 D20 配对拒绝 — 82677c4；两包全绿 37 用例 0 skip
- [x] apps/server 组合根补测：spaHandler 嵌入客户端回落、apiOnly 分流、channels/adapters 装配（含 stripe 缺 key 跳过）— e175ad0，go test 全绿 9 用例 27 子用例 0 skip
- [x] internal/database 纯函数层补测：Date/JSONB/IsUniqueViolation 等分支 — e1dff49，go test 全绿 12 用例 20 子用例 0 skip
- [x] internal/reputation fold 派生层补测：状态分桶、多币种合计、payment_ratio 分母 — eb4e528，go test 全绿 9 用例 3 子用例 0 skip
- [x] 依赖与安全告警按需处理：dependabot 0 待处理、npm audit 双树 0、govulncheck 0 可达；陈旧 docs/pnpm-lock.yaml 已删 — d352bb2
- [x] 每批合并后 README/文档站同步：PR #36 后审计一致；d3da44b 引入漂移已修 b628d4d
- [x] 冒烟覆盖缺口：6 个端点无 smoke 覆盖 — b59f6e9，smoke 全绿 61/63 路径（2 webhook 端点需异步渠道）
- [x] README 徽章/链接审计修复（A1-A6：双斜杠、空锚点、状态徽章冲突、围栏游离块、zh 深链、zh 文档链接语言）— aa58dd3 + 本批，docs build 通过、go test 26 包绿、链接检查 0 死链
