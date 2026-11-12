# TODO

> 需求清单：一条一句。完成项保留一行证据（commit + 测试数）。

## 待用户操作
- [ ] Stripe 支付渠道（D24）：等真实凭据（sk_test_*/whsec_*），适配器与分支测试已齐 — 按 docs/review/2026-10-30-decisions.md 暂缓待凭据（2026-10-30）
- [x] 重叠计费周期防护（代码深审 C1，P2）：CreatePeriod 拒绝重叠（半开窗口 [start,end)，边界日归较晚周期、背靠背合法；同 start 仍 409）+ 库层 EXCLUDE gist 约束（migration 0019）+ 23P01 映射 409 — 3b3c806，go test -race 26 包全绿 0 skip
- [x] Web Push 端点 SSRF 面评估（代码深审 C2，P3）：结论=端点本就是第三方推送服务（FCM/Mozilla/Apple，浏览器自选），域名/端口白名单会误伤合法端点，不采纳；落地的兼容加固=注册时拒绝私有/回环/链路本地/未指定/多播 IP 字面量端点（internal/push/service.go privateHost），https 约束与发送侧守卫保留；新增 11 个 422 用例 + 3 个公网放行用例，DNS 重绑定残余记为超范围（载荷为不透明密文、响应丢弃）— 4cbaf10，go test -race 全绿 0 skip
- [x] 本地静态分析工具链刷新（代码深审 C3，P3）：staticcheck 升至 2026.2.2（支持 go1.27 export data，全仓 0 报错）；golangci-lint 2.15.0 全仓扫描需 --timeout 5m，0 issues — 本地工具链恢复，无代码改动

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
- [x] 覆盖率基线推进（2026-10-20 巡检单）：coterie 77.13%→82.09%（marketplace accept 入圈路径 AdmitMember/AdmitUserInTx 0%落地、claude 插件 guard 咨询 pro/max 双臂、max_members 核心上限、404/403 读路径）— 1f0acbd；marketplace 73.41%→76.32%（未上架 404/成员 409/拉黑 403/重复申请 409/超长留言 422/未知请求 404/外人 decline 403/webhook 1MiB 上限 422/未知支付方式 422 带 Methods 清单）— 39d148a；总覆盖率 76.53%→77.53%（语句加权），go test 26 包全绿 0 skip、go vet 0 错、golangci-lint 0 issues；automation 包防御分支（custom 无 cycle_days/未知 cycle/owner 消失）经核被迁移 0012 CHECK 约束与 users FK 挡死，schema 层不可达不硬凑，standalone 稳定 75.0%（全量并行跑时其他包 fixture 恰好满足 Due 会偶发覆盖其 skip 分支，属环境波动）；死代码如实记录：coterie store CreateCoterie/CreateMember/UpdateCoterie 与 marketplace store CreateCharge/ChargeByID 无调用方（生产代码不动）；不可达上限：handler 层防御性 401（requireUser 中间件保证）、checkAdmissionGuard 非 api 错误包装分支（仓内插件只返回 api 错误）
- [x] internal/database 纯函数层补测：Date/JSONB/IsUniqueViolation 等分支 — e1dff49，go test 全绿 12 用例 20 子用例 0 skip
- [x] internal/reputation fold 派生层补测：状态分桶、多币种合计、payment_ratio 分母 — eb4e528，go test 全绿 9 用例 3 子用例 0 skip
- [x] 依赖与安全告警按需处理：dependabot 0 待处理、npm audit 双树 0、govulncheck 0 可达；陈旧 docs/pnpm-lock.yaml 已删 — d352bb2；go 直接+间接依赖刷新（pgx v5.12.0、x/crypto 伪版本、toolchain go1.27.3 等，误入的 docs/pnpm-lock.yaml 再删）— 本提交，go build ./... 绿 + go test ./... 全包绿
- [x] 每批合并后 README/文档站同步：PR #36 后审计一致；d3da44b 引入漂移已修 b628d4d
- [x] 冒烟覆盖缺口：6 个端点无 smoke 覆盖 — b59f6e9，smoke 全绿 61/63 路径（2 webhook 端点需异步渠道）
- [x] README 徽章/链接审计修复（A1-A6：双斜杠、空锚点、状态徽章冲突、围栏游离块、zh 深链、zh 文档链接语言）— aa58dd3 + 本批，docs build 通过、go test 26 包绿、链接检查 0 死链

## 漏洞扫描遗留（2026-11-03 全舰队扫描，源 scratch/vuln-scan-*.md）

- [ ] VULN-COTERIE-001 [高危·GHSA-mrhx-c7xc-m3w5] zod →≥4.6.6（coterie/apps/web，package-lock.json 仓，终扫 1）【BLOCKED·规范禁 npm 改 lock】npm-lock 仓只审计未改动；验收：迁 pnpm（pnpm import）后 pnpm audit --audit-level=high 该包不再命中（或编排者批准 npm 侧修复后 npm audit 清零）；该仓无 test 脚本，audit-only

## 源码分层分析 2026-11-07

### 抽象层清单

- L1 进程与环境：加载环境配置、连接/迁移 PostgreSQL、引导管理员并管理 HTTP、调度器与停机 —— 代表模块 `apps/server/main.go:189`。
- L2 交付契约：API mux、鉴权/限流/日志/恢复与标准错误信封约定 HTTP 请求入口 —— 代表模块 `internal/app/app.go:95`。
- L3 领域服务：各边界模块持有校验、权限、不变量、事务与跨聚合编排 —— 代表模块 `internal/coterie/service.go:63`。
- L4 持久化：GORM Store 访问 PostgreSQL，手写迁移是 schema 事实源 —— 代表模块 `internal/database/database.go:37`。
- L5 集成接缝：通知出站通道、支付同步/异步适配器与服务商插件面注入外部能力 —— 代表模块 `internal/payment/adapter.go:33`。
- L6 表现层：React 路由界面通过统一 API helper 调用后端并承载内联文案 —— 代表模块 `apps/web/src/lib/api.ts:52`。

### 超大文件候选

- [x] LAY-001 [P1] 拆分 `apps/web/src/routes/_app/coteries.$coterieId.tsx`（2149 行）—— 路由文件瘦身为 166 行（Route+页面骨架+查询），10 个界面块提取为 `apps/web/src/components/coterie/` 下独立组件（ReportBar 63、LifecycleBar 75、RequestsTab 240、UsageTab 166、SubscriptionTab 140、DisputesTab 144、AuditTab 103、MembersTab 248、SeatsTab 352、PeriodsTab 475 行）— 37fe282，tsc -b 绿 + vitest 48/48。

### 目录/分类不合理处

- `internal/marketplace/service.go` 既有公开目录与入圈申请，又在同一服务内实现 Stripe 风格入圈费收取、Webhook 确认和跨 `coterie.Service` 原子入圈；收费状态机可独立成边界（`internal/marketplace/service.go:349`、`internal/marketplace/service.go:496`）。
- `internal/audit` 只提供审计读接口，写入函数却在 `internal/audit/record.go` 中被所有领域模块直接调用；读门面与写入 helper 分属不同调用面（`internal/audit/record.go:32`、`internal/audit/service.go:13`）。
- `apps/web/src/lib/api.ts` 除 API 传输、会话存储与类型定义外，还保存中文审计标签 UI 字典，传输契约和文案混在一个文件（`apps/web/src/lib/api.ts:52`、`apps/web/src/lib/api.ts:318`）。
- `apps/web/src/routes/_app/coteries.$coterieId.tsx` 以单路由承载多业务面板，而其他路由按旅程拆分，页面内子域可下沉为组件或 hooks（`apps/web/src/routes/_app/coteries.$coterieId.tsx:57`）。

## 审核修复批（巡检单 2026-10-20，2026-11-12 执行）

- [x] P0#1 Stripe webhook secret 缺失时签名形同虚设：启动期强校验 + 适配器 fail-closed（无 whsec 拒签）— 1ba7a0f，apps/server main_test 双子用例 + internal/payment/stripe_test.go noSecret；internal/payment/stripe.go ParseWebhook 空密钥守卫、apps/server/main.go whsec 缺省跳过
- [x] P1#2 入圈费报价把已退出成员算入少收：GateQuote 补 left_at IS NULL — bb53430，TestGateQuoteExcludesDepartedMembers（12.00 过 2 人报 6.00，红 4.00→绿 6.00）；internal/marketplace/store.go:243
- [x] P1#3 入圈容量 TOCTOU 并发超卖：admitUserInTx 持 coterie 行锁（CoterieForUpdate）串行化检查与插入 — 83ccb2c，TestConcurrentAdmissionsRespectMemberCap（parkingGuard 编排：旧码 [200 200] 超卖→红，新码 200+409 恰一成一败→绿）；internal/coterie/store.go:249、internal/coterie/service.go admitUserInTx
- [x] P1#7 邀请接受两段事务不一致：Join 单事务收口（admit + MarkInvitationAccepted 同 tx，标记失败即回滚）— 83ccb2c（与 #3 同批，代码同重构）
- [x] P1#4 用量限额窗口与分摊窗口口径冲突：usage 对齐 billing 半开 [start_date, end_date)——PeriodCovering 改 end_date > day、Sum 改 < end（去掉 end+1d）— ae71527，TestUsageLimitWindowHalfOpen（末日记录旧码计入超限 409→红，新码不计入→绿）；internal/usage/store.go:189/227/247
- [x] P1#5 私有圈子存在性泄露：Get/ListMembers 补 requireVisible（非公开圈外人 404，与未上架申请同语义；admin 放行）— dd3c1d8，TestPrivateCoterieHiddenFromOutsiders（外人读私有圈旧码 200 泄露 listing/member_count→红，新码 404→绿）；internal/coterie/service.go requireVisible
- [x] P1#6 跨租户财务读基线过宽：billing/payments/usage/seats 读接口收 actor 验属主（payment 另放行 payer 本人）— b331549，TestBillingReadsOwnerScoped/TestPaymentReadsOwnerScoped/TestUsageReadsOwnerScoped/TestSeatReadsOwnerScoped（各旧码 200→红，新码 403→绿）+ 更新 TestPaymentPermissions/TestSeatOwnerOnlyMutations 旧基线（越权读 200→403）；internal/billing/service.go GetPeriod/ListPeriods/ListContributions、internal/payment/service.go List/Get、internal/usage/service.go Get/List、internal/seat/service.go Get/List
- [x] 批次收口：go test ./... 26 包全绿 0 skip、go vet 0 错、golangci-lint 0 issues；提交 1ba7a0f..b331549 全部推送 origin main（P2 简列 9 项不在本批指令范围，未推进）

## Codex 深审发现（2026-11-12，luna profile 近5 commit 审核，0C/1M/1m，待修）

- [ ] [M] 更新 Coteries API 契约描述（docs/api.md:120）——契约仍写 non-members restricted view，实现已改为非成员 404（internal/coterie/service.go:158-175），文档与行为漂移
- [ ] [m] 修正 todo.md:73 验证提交指向（9437787 声称验证 1ba7a0f..b331549 但其父提交即 b331549，范围自引）——台账范围失真
