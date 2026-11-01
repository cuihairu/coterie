# 代码深度审查（2026-11-01）

> 全仓静态扫描 + 隐藏缺陷猎捕 + 质量门。范围：`internal/**`、`pkg/**`、`apps/server`、`cmd/**`、`providers/claude`、`apps/web/src`、`migrations/`、`scripts/`。
> 方法：安全静态扫描（硬编码密钥 / shell 注入 / SQL 拼接 / eval / 反序列化 / 路径穿越 / SSRF）→ 隐藏缺陷猎捕（错误处理 / 竞态 / 越界 / 资源泄漏 / off-by-one / 并发）→ 质量门（lint / 类型检查 / 测试基线）。每条发现附 file:line 证据，按严重度排序。

## 结论

- **发现 3 条**：P2 × 1，P3 × 2。P0 × 0，P1 × 0。
- **当场修复 0 条**（无 P0/P1）；**P2/P3 记入 todo.md**（3 条）。
- 质量门全绿：`go test -race ./...` 26 包 0 失败 0 skip 0 数据竞争；`go vet` 通过；`staticcheck` 0 问题；`golangci-lint` 0 问题；`tsc -b` 通过；`oxlint` 0 问题；`vitest` 48/48。
- 未发现 P0/P1（无安全漏洞、无易触发数据损坏/崩溃）。代码库整体工程质量高：金额拆分为整数/大数精确计算、并发写用条件更新与 `FOR UPDATE`、令牌哈希存储、HMAC 常量时间比较、HTTP body 全部关闭。

## 发现（按严重度排序）

### P2

#### C1 重叠计费周期被允许，导致用量/限额归属歧义与用量拆分重复计数

- **证据**：
  - `migrations/0001_init.up.sql:108-116` — `billing_periods` 仅 `UNIQUE (subscription_id, start_date)` + `CHECK (end_date > start_date)`，**无周期不重叠约束**。
  - `internal/billing/service.go:46-90` — `CreatePeriod` 只校验 `end.After(start)` 与唯一 start_date；**不校验与既有周期重叠**。
  - `internal/usage/store.go:189-206` — `PeriodCovering` 对覆盖某日的多个周期取 `ORDER BY start_date ASC LIMIT 1`（最早开始者胜出），重叠时把用量记录静默归属到较早周期。
  - `internal/billing/service.go:414-419` — `splitUsage` 按 `[StartDate, EndDate)` 窗口读账本；两个重叠周期各自拆分时，共享窗口内的用量记录被**两份账单重复计入**。
  - 无重叠不变量的设计文档（`docs/design.md` / `docs/zh/design.md` 均无 overlap/重叠）与测试。
- **影响**：Owner 经 `POST /api/v1/subscriptions/{id}/billing-periods` 手工创建重叠周期（例如 `[1-01,1-31]` 与 `[1-15,2-15]`）后，(a) `PeriodCovering` 将重叠日期的用量归入最早周期，用量上限（D21）/配额上限（FR-10）按错误窗口判定；(b) 用量拆分（D9 usage mode）会把重叠窗口的用量记录计入两份周期账单，造成金额重复分摊。
- **触发条件**：Owner 主动创建重叠周期（边缘操作，非默认路径；自动滚期调度器 `internal/automation/scheduler.go:109-129` 按 `EndDate+1` 起算，不产生重叠）。
- **建议修复**：在 `CreatePeriod` 增加重叠校验（查询该订阅既有周期，拒绝 `new.start <= existing.end AND new.end >= existing.start`）；更稳做法是新增迁移，用 `EXCLUDE USING gist (subscription_id WITH =, daterange(start_date, end_date) WITH &&)` 在库层禁止重叠。**（P2，记 todo）**

### P3

#### C2 Web Push 端点存在低危 SSRF 面（Web Push 固有，已缓解）

- **证据**：`internal/push/service.go:38-42`（端点必须 `https` 且 host 非空）；`internal/notification/webpush.go:288`（服务器向 `sub.Endpoint` 发 POST）。
- **影响**：用户可注册任意 HTTPS URL 作为推送端点；其本人触发通知时服务器向其发 POST。响应被丢弃，payload 为 Web Push 密文；https 约束挡住 http 内网/云元数据。属 Web Push 标准模式，风险低。**（P3，记 todo，暂不修）**

#### C3 本地静态分析工具链低于项目 toolchain，本地 lint 门无法运行

- **证据**：`go.mod:3` `toolchain go1.27.3`；本机 `golangci-lint` 2.14.0 与 `staticcheck` 2026.2.1 均以 go1.26 构建，运行报「export data version ... greater than maximum supported」/「Go language version ... lower than targeted」。CI 不受影响（`.github/workflows/ci.yml:33-35`、`nightly.yml:21-23` 均 `go-version: "1.27"`）。
- **影响**：按项目约定「推送 Go 改动前本地跑 golangci-lint」，本地门当前失效；需 `go install ...@latest` 重装（本次以临时安装 go1.27 版验证：`golangci-lint` 0 问题、`staticcheck` 0 问题）。**（P3，环境项，记 todo）**

## 质量门

| 门 | 命令 | 结果 |
|---|---|---|
| Go 测试 + 竞态 | `go test -race -count=1 ./...` | 26 包 ok，0 失败，**0 `--- SKIP`**，0 数据竞争（testcontainers postgres:18） |
| Go vet | `go vet ./...` | 通过 |
| staticcheck | `staticcheck ./...`（go1.27 版） | 0 问题 |
| golangci-lint | `golangci-lint run ./...`（go1.27 版，项目 .golangci.yml） | **0 issues** |
| Web 类型检查 | `npx tsc -b` | 通过 |
| Web lint | `npm run lint`（oxlint） | 0 问题 |
| Web 测试 | `npm run test`（vitest 5.0.4） | 8 文件 48 用例全过 |

## 已验证无问题的重点面

- **金额/拆分**：`splitEqual/splitPerSeat/splitWeighted/splitProrated`（`internal/billing/service.go:361-530`）— 整数与大数（`big.Rat`）精确计算、最大余数法分币、总额守恒；`parseCents`/`scaled` 仅收非负（billing `amountPattern` 与 `pricePattern` 为 `^\d{...}$`，usage `scaled` 正确处理负修正）；`formatCents`/`splitFixed` 边界正确。
- **并发**：席位 `Occupy` 条件更新（`internal/seat/service.go:160-177`）、市场闸门 `ChargeByExternalRefForUpdate` + `FOR UPDATE`（`internal/marketplace/service.go:516-540`）、限流器 `sync.Mutex` 固定窗口 + 每窗清扫（`internal/ratelimit/limiter.go:64-99`）、通知 `dispatchAsync` goroutine 带 panic 恢复与超时（`internal/notification/channel.go:36-56`）。
- **认证/安全**：会话令牌仅存 SHA-256 哈希（`internal/auth/service.go:77-111`，TTL 30 天）、bcrypt 口令、Stripe webhook `hmac.Equal` 常量时间比较（`internal/payment/stripe.go:204`）、邮件头 `headerSafe` 去 CR/LF（`internal/notification/email.go:54-57,73-75`）、审计日志仅追加（`internal/audit` store 只暴露 List + `Record`→Create）。
- **资源/边界**：全部出站 HTTP body 关闭（`stripe.go:157`、`webhook.go:82`、`webpush.go:297`）、分页 limit 钳制 [1,100]/offset 仅正数（`pkg/api/pagination.go:28-47`）、`PeriodCovering`/`splitProrated` 日期无 off-by-one（纯 `YYYY-MM-DD`，`database.ParseDate`）、调度器滚期 `EndDate+1` 起算含端点（`internal/automation/scheduler.go:109-129`）。
- **安全扫描**：无硬编码密钥（仅注释 `stripe.go:29`）、无 shell/exec、无 SQL 拼接（全部 `?` 参数化，含 `user/service.go:127-130` 与 `dispute/store.go:127-130`）、无 eval/unsafe、无路径穿越（SPA 服务嵌入资产，路径取 ID 走库键）、web 端无 `dangerouslySetInnerHTML`。
