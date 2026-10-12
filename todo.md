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

NFR 层复核（2026-10-13，requirements §5 对代码）：NFR-1 无凭据表
（迁移中无 license_key/api_key/recovery_code 类列）；NFR-2 的信息载体
已有一级 Sharing Policy 字段与 provider/product metadata JSONB，但
「UI 必须明确提示」此前缺失——本轮落地 ComplianceNotice 组件（建圈页
与共享广场渲染，vitest 钉住条款/地区/共享权限三关键词），是否再把
terms/region/eligibility 升为命名字段留审核拍板；NFR-3 compose/Dockerfile/
单二进制齐（K8s/bare metal 是支持目标非交付物）；NFR-4 77 条 /api/v1
路由即 API 面；NFR-5 代码内零硬编码 provider 分支；NFR-6 单租户无组织层；
NFR-7 的 D17/D19/D26/FR-17/FR-18 均在代码中验证（audit/ratelimit/report/
dispute/reputation 五包及测试在位）。

挂起项触发复核（2026-10-13）：CodeQL 触发条件=用户明令，本轮巡检单
明确「不许代拍板」，维持挂起；Stripe D24 触发条件=凭据到位，仓库/
env/.env.example 内确认无 sk_/whsec_ 真实凭据，维持等凭据；下一批任务
分配与逐仓审核签字均在用户手中，审核材料已备至 FR+NFR+plan 三层。

验收线复核（2026-10-13，requirements §3 MVP 旅程对两端）：十步
（注册→Provider→Product→Subscription→Coterie→Seats→邀请→加入→
记录费用→周期结算）逐步双面命中——API 面由 smoke 119 检查走通，
Web 面逐页核实（login.tsx 注册；coteries.new 向导内联建
Provider/Product/Subscription/Coterie；圈详情页 seats/billing-periods/
generate/usage-records/invitations；圈列表页接受邀请），§3 验收线
「每步都可通过 Web 与 API 完成」成立。至此审核证据四层齐：
FR 表 + plan bullets + NFR 七条 + §3 旅程。

续做令逐条过单（2026-10-13）：①用户逐仓审核——自主侧四层证据已尽，
卡用户签字，跳过；②审核后分配下一批——依赖①，跳过；③CodeQL——
等用户明令，跳过；④Stripe D24——等凭据（仓库内复核仍无 sk_/whsec_），
跳过。四条全部停在用户门槛，无自主余量。

SPA 推送订阅 UI（2026-10-13，PR #32）：FIRE5X 续做令下按最近 git log
定位的下一批——#27 时登记的「SPA 推送订阅 UI，SPA 侧零实现」。落地
public/sw.js（展示解密后的 title/body，解不开也出可见通知）、
lib/push.ts（启用流：特性检测→权限→注册 SW→取公钥(404=服务端未开)→
subscribe→POST 登记；失败回滚本地订阅；关闭流按 endpoint 匹配 DELETE）、
通知页 PushSetup 开关（不支持的浏览器整体隐藏）。vitest 35/35
（新增 16：lib 12 + 组件 4）、tsc+vite 构建、oxlint 全绿；design.md
§6.5 补 SPA 行，双语 README Web-client 条目同步。

逐 bullet 复核（2026-10-12，project-plan §6 对代码）：Phase 1 十模块
（user/provider/product/subscription/coterie/member/seat/contribution/
invitation/notification）与 Phase 2 七项（marketplace/provider 目录/
payment 适配器/email/webhook/usage/quota）全部落地——12 个模块目录
各有测试（notification 4 份最多）；两条目标流程的 17 条关键路由
（注册→…→结算、browse→…→入圈）逐一在 mux 命中，无缺项。

## 维护项（常规）
- [x] 依赖与安全告警按需处理（dependabot；code scanning 见下条，暂不启用）——2026-10-12 三轮执行：告警 0、audit 0/0、govulncheck 可达 0，过期项仅 vitepress 有意 pin（已登记）
- [x] 每批 PR 合并后 README/文档站同步——2026-10-12 核对：api.md 82 路由与代码一致；修掉 README Go 版本徽章 1.25+→1.26+ 漂移（#26）

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

维护项勾选（2026-10-12 第三轮）：dependabot 0、code scanning 未启用（按明令暂挂）、
npm audit 两树 0、go 直接依赖无更新、govulncheck 可达 0（openpgp 模块级噪音同前）、
npm 过期仅 vitepress alpha pin（有意保留，已登记）；#25/#26 均为文档/依赖补丁
无 API 变化，README/文档站已同步（82 路由上轮核验后无代码变更）。两项常开周期项
本轮执行完毕，勾选。

## 暂不开（等用户明令）
- [ ] CodeQL / code scanning：暂不开。无用户明令，且会引入持续告警分诊面，按默认从简口径挂起。

## 等外部密钥/凭据（暂挂）
- [x] Web Push 公钥端点：已落地 `GET /api/v1/push/vapid-public-key`（Bearer；未配置推送时 404）。VAPID 密钥本就可由 `cmd/vapidkeygen` 自签生成，无需外部密钥。SPA 侧订阅 UI（sw.js + 通知页开关）已于 2026-10-13 落地（PR #32），D22 两端齐。
- [ ] Stripe 支付渠道（D24）：等凭据。2026-10-12 补齐适配器分支测试（金额边界/溢出、默认描述、同步拒绝、坏响应、未来时间戳），除真实凭据验证外无遗留。

验收：以用户审核结论为准；新任务由用户分配后按测试绿才 push 执行。
