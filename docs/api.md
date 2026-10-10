# API 参考（v1）

所有接口挂在 `/api/v1` 下，JSON 收发。这是当前实现的平铺清单，语义与字段细节见[技术设计 §9](/design)。清单随实现更新；本地起服务后可用 `scripts/smoke.sh http://localhost:8080` 对全旅程做一次冒烟。

通用约定：

- **鉴权**：除标注「公开」的接口外都要 `Authorization: Bearer <token>`。令牌在注册/登录时签发，有效期 30 天，注销即失效。
- **列表分页**：`?limit=`（默认 20，上限 100）与 `?offset=`，响应为 `{"items": [...], "meta": {"total", "limit", "offset"}}`。
- **错误信封**：非 2xx 返回 `{"error": {"code", "message", "details"}}`；鉴权缺失/失效 401，无权 403，语义错误 422，状态冲突 409。

权限标注：**公开** = 匿名可读；**Bearer** = 任意认证用户；**Owner** = 资源所属订阅的 Owner；**管理员** = 平台管理员（`ADMIN_EMAILS` 引导）。细粒度规则（如成员能否读）见设计文档。

## 健康

| 方法 | 路径 | 鉴权 | 说明 |
|---|---|---|---|
| GET | `/healthz` | 公开 | 就绪探针 |

## 认证

| 方法 | 路径 | 鉴权 | 说明 |
|---|---|---|---|
| POST | `/auth/register` | 公开 | 注册并签发令牌 |
| POST | `/auth/login` | 公开 | 登录，邮箱 + 密码 |
| GET | `/auth/me` | Bearer | 当前用户 |
| POST | `/auth/logout` | Bearer | 注销当前会话 |

## 用户与信誉

| 方法 | 路径 | 鉴权 | 说明 |
|---|---|---|---|
| GET | `/users` | Bearer | 用户列表 |
| POST | `/users` | Bearer | 创建用户 |
| GET | `/users/{id}` | Bearer | 用户详情 |
| GET | `/users/{id}/reputation` | Bearer | 结算信誉（只读投影） |

## 目录：Provider 与 Product

读取公开；写操作限目录维护者（Owner 或创建者）。

| 方法 | 路径 | 鉴权 | 说明 |
|---|---|---|---|
| GET | `/providers` | 公开 | 目录列表，支持过滤 |
| POST | `/providers` | Bearer | 创建 |
| GET | `/providers/{id}` | 公开 | 详情 |
| PATCH | `/providers/{id}` | Bearer | 更新 |
| DELETE | `/providers/{id}` | Bearer | 删除 |
| GET | `/products` | 公开 | 目录列表 |
| POST | `/products` | Bearer | 创建 |
| GET | `/products/{id}` | 公开 | 详情 |
| PATCH | `/products/{id}` | Bearer | 更新 |
| DELETE | `/products/{id}` | Bearer | 删除 |

## 订阅

| 方法 | 路径 | 鉴权 | 说明 |
|---|---|---|---|
| GET | `/subscriptions` | Bearer | 自己的订阅 |
| POST | `/subscriptions` | Bearer | 创建（`owner_user_id` 必须是本人） |
| GET | `/subscriptions/{id}` | Bearer | 详情 |
| PATCH | `/subscriptions/{id}` | Owner | 价格/策略/状态变更，审计留痕 |
| DELETE | `/subscriptions/{id}` | Owner | 删除 |

## 席位

| 方法 | 路径 | 鉴权 | 说明 |
|---|---|---|---|
| GET | `/subscriptions/{subID}/seats` | Bearer | 席位列表 |
| POST | `/subscriptions/{subID}/seats` | Owner | 追加席位 |
| GET | `/seats/{id}` | Bearer | 席位详情 |
| PATCH | `/seats/{id}` | Owner | 改名/元数据/停用启用（occupied 需先 release） |
| POST | `/seats/{id}/assign` | Owner | 指派给成员 |
| POST | `/seats/{id}/release` | Owner | 释放 |

## 账期与分摊

| 方法 | 路径 | 鉴权 | 说明 |
|---|---|---|---|
| GET | `/subscriptions/{subID}/billing-periods` | Bearer | 账期列表 |
| POST | `/subscriptions/{subID}/billing-periods` | Owner | 建账期 |
| GET | `/billing-periods/{id}` | Bearer | 账期详情 |
| POST | `/billing-periods/{id}/close` | Owner | 关闭账期，审计留痕 |
| GET | `/billing-periods/{id}/contributions` | Bearer | 分摊列表 |
| POST | `/billing-periods/{id}/contributions/generate` | Owner | 生成等额或按用量分摊 |
| PATCH | `/contributions/{id}` | Owner | 调整分摊金额，审计留痕 |

## 支付

| 方法 | 路径 | 鉴权 | 说明 |
|---|---|---|---|
| GET | `/payments/methods` | Bearer | 已注册渠道 |
| POST | `/contributions/{id}/payments` | Bearer | 记一笔支付（manual 直接到账；异步渠道进 pending） |
| GET | `/contributions/{id}/payments` | Bearer | 支付流水 |
| GET | `/payments/{id}` | Bearer | 支付详情 |
| POST | `/payments/webhooks/{method}` | 公开 + 签名 | 渠道确认回调（Stripe 等），按 external_ref 幂等 |

## 争议

| 方法 | 路径 | 鉴权 | 说明 |
|---|---|---|---|
| POST | `/contributions/{id}/disputes` | Bearer | 对分摊提出争议 |
| GET | `/subscriptions/{id}/disputes` | Bearer | 争议列表 |
| GET | `/disputes/{id}` | Bearer | 争议详情 |
| POST | `/disputes/{id}/decide` | Owner | 裁决（单向） |

## 共享圈与成员

| 方法 | 路径 | 鉴权 | 说明 |
|---|---|---|---|
| POST | `/coteries` | Bearer | 围绕订阅建圈，席位按 capacity 落位 |
| GET | `/coteries` | Bearer | 自己参与的圈 |
| GET | `/coteries/{id}` | Bearer | 圈详情（非成员只回受限于视图） |
| PATCH | `/coteries/{id}` | Owner | 名称/状态流转/listing，审计留痕 |
| GET | `/coteries/{id}/members` | Bearer | 成员列表 |
| DELETE | `/members/{id}` | Owner | 移除成员并释放席位 |
| POST | `/coteries/{id}/leave` | Bearer | 成员自行退出（Owner 不可） |
| POST | `/coteries/{id}/invitations` | Owner | 发邀请（一次性 token） |
| GET | `/coteries/{id}/invitations` | Owner | 邀请列表 |
| POST | `/invitations/accept` | Bearer | 凭 token 接受邀请 |

## 市场与加入请求

| 方法 | 路径 | 鉴权 | 说明 |
|---|---|---|---|
| GET | `/marketplace/coteries` | 公开 | 公开目录（附 Owner 信誉徽标） |
| POST | `/coteries/{coterieID}/join-requests` | Bearer | 申请加入（被拉黑 403） |
| GET | `/coteries/{coterieID}/join-requests` | Owner | 收件箱 |
| POST | `/join-requests/{id}/accept` | Owner | 接受并分配席位 |
| POST | `/join-requests/{id}/decline` | Owner | 拒绝 |
| DELETE | `/join-requests/{id}` | Bearer | 申请人撤回 |
| GET | `/me/join-requests` | Bearer | 自己的申请列表 |
| POST | `/join-requests/{id}/payments` | Bearer | 付入圈费（支付闸门开启时） |
| POST | `/marketplace/webhooks/{method}` | 公开 + 签名 | 闸门确认回调，确认即入圈 |

## 防滥用

| 方法 | 路径 | 鉴权 | 说明 |
|---|---|---|---|
| PUT | `/coteries/{id}/blocks/{userID}` | Owner | 拉黑用户 |
| DELETE | `/coteries/{id}/blocks/{userID}` | Owner | 解除拉黑 |
| GET | `/coteries/{id}/blocks` | Owner | 拉黑名单 |
| POST | `/coteries/{id}/report` | Bearer | 举报公开列出的圈 |
| GET | `/admin/reports` | 管理员 | 举报收件箱，`?status=` 过滤 |
| POST | `/admin/reports/{id}/resolve` | 管理员 | 裁定属实（note 可选） |
| POST | `/admin/reports/{id}/dismiss` | 管理员 | 驳回（note 可选） |

## 审计

| 方法 | 路径 | 鉴权 | 说明 |
|---|---|---|---|
| GET | `/subscriptions/{id}/audit-logs` | Owner | 订阅账本，`?action=` 过滤 |
| GET | `/coteries/{id}/audit-logs` | Owner | 圈账本，`?action=` 过滤 |
| GET | `/admin/audit-logs` | 管理员 | 全量账本，`?action=` `?actor_id=` `?coterie_id=` `?subscription_id=` 过滤 |

## 用量

| 方法 | 路径 | 鉴权 | 说明 |
|---|---|---|---|
| GET | `/subscriptions/{subID}/usage-records` | Bearer | 用量列表 |
| POST | `/subscriptions/{subID}/usage-records` | Bearer | 记一笔用量（插件可复验） |
| GET | `/usage-records/{id}` | Bearer | 用量详情 |

## 通知

| 方法 | 路径 | 鉴权 | 说明 |
|---|---|---|---|
| GET | `/notifications` | Bearer | 自己的信箱 |
| POST | `/notifications/{id}/read` | Bearer | 标记已读 |
| GET | `/push/subscriptions` | Bearer | 已注册的推送端点 |
| POST | `/push/subscriptions` | Bearer | 注册 Web Push 端点 |
| DELETE | `/push/subscriptions/{id}` | Bearer | 注销推送端点 |
