# 术语表

本表记录 2026-11-07 源码分层分析使用的实现层概念，并与
[需求术语](requirements.md#13-术语)、
[设计概念定义](design.md#11-概念定义)互为补充。

| 术语 | 英文 | 定义 | 证据 |
|---|---|---|---|
| 运行时配置 | Runtime Configuration | 进程启动前一次性加载的环境变量服务配置。 | `internal/config/config.go:16` |
| 数据库生命周期 | Database Lifecycle | PostgreSQL 连接、启动迁移与手写 SQL schema 的所有权。 | `internal/database/database.go:37` |
| 处理器装配 | Handler Assembly | 单一 API mux 构造模块服务、注入通道/插件/适配器，并装配路由和中间件。 | `internal/app/app.go:95` |
| 前端客户端 | Web Client | React SPA，会话状态存于 `localStorage`，统一通过共享 JSON API helper 调用后端。 | `apps/web/src/lib/api.ts:52` |
| 用户 | User | 平台账号与资料，被会话、成员关系和所有权检查引用。 | `internal/user/model.go:7` |
| 会话 | Session | 以 SHA-256 摘要持久化的 Bearer Token，包含过期与撤销状态。 | `internal/auth/model.go:11` |
| 共享圈 | Coterie | 与订阅严格一对一的社交聚合，包含所有者/成员、邀请与拉黑记录。 | `internal/coterie/model.go:26` |
| 订阅 | Subscription | 已购买的产品实例，拥有价格、币种、账期模式、共享策略与席位容量。 | `internal/subscription/model.go:38` |
| 席位 | Seat | 订阅容量的可分配单元；分配、释放与配额元数据都落在席位上。 | `internal/seat/model.go:22` |
| 账期 | Billing Period | 订阅下可开启或关闭的日期窗口，用于聚合一组分摊。 | `internal/billing/model.go:38` |
| 分摊 | Contribution | 成员在一个账期内的应付金额与结算状态。 | `internal/billing/model.go:52` |
| 支付流水 | Payment | 分摊费用的渠道结果，携带方式、状态和外部引用。 | `internal/payment/model.go:9` |
| 入圈申请 | Join Request | 公开市场请求，具有待处理/待支付/已接受/已拒绝生命周期。 | `internal/marketplace/model.go:23` |
| 入圈费 | Admission Charge | 支付闸门产生的入圈费用；确认时可原子完成入圈。 | `internal/marketplace/model.go:104` |
| 分摊争议 | Dispute | 针对分摊的成员争议，由所有者单向裁定。 | `internal/dispute/model.go:26` |
| 用量流水 | Usage Record | 只增不改的计量记录，归属到成员并可选归属席位。 | `internal/usage/model.go:12` |
| 目录服务商 | Provider Catalog Entry | 描述服务商的元数据，不承载套餐和价格。 | `internal/provider/model.go:13` |
| 目录产品 | Product Catalog Entry | 服务商提供的可购买套餐元数据；实际价格属于订阅。 | `internal/product/model.go:12` |
| 站内通知 | In-App Notification | 领域服务写入的持久用户消息，可继续扇出到外部通道。 | `internal/notification/model.go:26` |
| 推送端点 | Push Endpoint | 注册的浏览器 Web Push 端点，包含加密密钥和生成标识。 | `internal/push/model.go:9` |
| 审计流水 | Audit Entry | 只增不改的变更记录，包含行动者、动作、实体、范围和前后快照。 | `internal/audit/model.go:52` |
| 信誉视图 | Reputation View | 从分摊记录派生的分币种统计与付款比例视图。 | `internal/reputation/model.go:9` |
| 举报 | Report | 用户对公开圈子的标记，由平台管理员按待处理/属实/驳回处置。 | `internal/report/model.go:25` |
| 鉴权门 | Authentication Gate | 在处理器执行前解析并校验 Bearer 会话的路由中间件。 | `internal/auth/middleware.go:31` |
| 平台管理员 | Platform Admin | 启动引导提升的用户角色，可读审计账本并处置举报。 | `internal/user/model.go:23` |
| 出站通道 | Outbound Channel | SMTP、Webhook 或 Web Push 的尽力通知投递实现。 | `internal/notification/channel.go:18` |
| 支付适配器 | Payment Adapter | 将应收费用转换为渠道回执的接缝，可选支持异步 Webhook。 | `internal/payment/adapter.go:33` |
| 异步支付适配器 | Async Payment Adapter | 可选支付面，创建待确认渠道费用并解析签名 Webhook 事件。 | `internal/payment/adapter.go:51` |
| 服务商插件面 | Provider Plugin Face | 服务商特有共享策略校验、准入守卫与用量校验的切面注册表。 | `internal/provider/plugin.go:57` |
| 内置 Claude 插件 | Built-in Claude Plugin | 示例插件，校验驻留/套餐策略、准入上限和用量单位。 | `providers/claude/plugin.go:46` |
| 计费调度器 | Billing Scheduler | 后台 ticker，为开启自动续费记录的订阅开启并均分下一个账期。 | `internal/automation/scheduler.go:28` |
| 进程生命周期 | Process Lifecycle | 配置、数据库、引导、HTTP、调度器和优雅停机的信号感知启动序列。 | `apps/server/main.go:189` |
| 内嵌资源 | Embedded Assets | 编译进二进制的 Web 客户端树，以静态资源加 SPA 回退方式服务。 | `apps/server/main.go:39` |
| 公共协议 | Public API Contract | 共享 JSON、错误信封、分页、恢复、日志和限流语义。 | `pkg/api/errors.go:16` |
| 速率限制器 | Rate Limiter | 按客户端地址计数公共注册/登录写入的内存固定窗口。 | `internal/ratelimit/limiter.go:28` |
