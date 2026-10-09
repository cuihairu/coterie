---
layout: home

hero:
  name: "Coterie"
  text: "通用数字服务共享平台"
  tagline: "围绕一份订阅组织共享圈，把席位、账期和费用管起来"
  actions:
    - theme: brand
      text: 快速开始
      link: "#快速开始"
    - theme: alt
      text: 需求规格
      link: /requirements
    - theme: alt
      text: 技术设计
      link: /design
    - theme: alt
      text: GitHub
      link: https://github.com/cuihairu/coterie

features:
  - title: 订阅共享圈
    details: 围绕一份订阅建圈。邀请制或公开目录加入，Owner/Admin/Member 三种角色，成员进出留记录。
  - title: 席位与配额
    details: 席位属于订阅，成员属于圈。席位的分配与释放记录在审计日志里，用量上限由共享策略设定。
  - title: 费用分摊与账期
    details: 等额或按用量分摊，账期可自动滚动。分摊有争议时走争议账本，由 Owner 裁决。
  - title: 公开目录与支付闸门
    details: 圈主可把圈发布到公开目录。开启支付闸门后，新成员先付入圈费，Stripe 确认后自动入圈。
  - title: 通知渠道
    details: 站内信箱之外可接邮件、Webhook 和 Web Push。渠道没有配置就不启用。
  - title: 防滥用与审计
    details: 注册与登录限速；资金与成员变更有只追加的审计日志；圈主可拉黑恶意用户，管理员处置举报。
---

## 项目简介

Coterie 是一个开源的数字服务共享与订阅拼车平台：让用户持有一份数字服务订阅，确认其允许多人共享后，围绕它组织共享圈（Coterie），统一管理席位、费用、账期、成员与访问方式。平台要做的是把共享的组织、分摊、准入和裁决管清楚。服务本身只做基础设施，不代持任何第三方的账号密码。

当前处于早期开发阶段（MVP 已跑通核心旅程：注册 → 目录 → 订阅 → 共享圈 → 席位 → 邀请加入 → 分摊账单 → 结算 → 通知），接口形态与数据模型仍在按 [项目计划](/project-plan) 推进。

## 快速开始

要求 Go 1.25+ 与 PostgreSQL 15+（迁移在服务启动时自动应用）。

```bash
# 1. 配置（全部变量见 .env.example）
export DATABASE_URL=postgres://coterie:coterie@localhost:5432/coterie?sslmode=disable
export PORT=8080

# 2. 启动
go run ./apps/server
```

除 `GET /healthz`、`POST /api/v1/auth/register`、`POST /api/v1/auth/login` 与公开目录读取外，所有接口都需要 `Authorization: Bearer <token>`（注册或登录时签发，有效期 30 天）。

对运行中的服务做一次 MVP 全旅程冒烟（注册 → Provider → Product → Subscription → 共享圈 → 席位 → 邀请 → 加入 → 账单 → 结算 → 通知）：

```bash
scripts/smoke.sh http://localhost:8080
```

## 文档导航

| 文档 | 内容 |
| --- | --- |
| [需求规格](/requirements) | 目标用户、用户故事、功能需求（FR）、非功能需求（NFR）、MVP 边界与不做清单 |
| [技术设计](/design) | 领域模型、共享与计费模型、支付适配器、技术架构、模块划分、API 清单与设计决策记录（ADR） |
| [项目计划](/project-plan) | 定位、愿景、原则、阶段路线图与文档索引 |
