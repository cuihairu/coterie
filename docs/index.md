---
layout: home

hero:
  name: "Coterie"
  text: "通用数字服务共享平台"
  tagline: "订阅共享 · 席位分摊 · 圈层治理 —— 把一份订阅，共享成一群人的低成本"
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
  - icon: 🤝
    title: 订阅共享圈
    details: 围绕一份订阅组织共享圈：邀请制或公开目录加入，Owner/Admin/Member 角色清晰，成员进出全程留痕。
  - icon: 🪑
    title: 席位与配额
    details: 席位绑定订阅、成员绑定圈层；设备的 assign/release/transfer 全程可审计，用量上限按策略强制。
  - icon: 💰
    title: 费用分摊与账期
    details: 等额分摊账本、账期滚动、争议处理与信誉展示——分摊公开透明，欠费与纠吵都有章可循。
  - icon: 🏪
    title: 公开目录与支付闸门
    details: 圈主可公开列出圈层进入集市；可选支付闸门按入圈报价收费，Stripe 异步确认自动准入。
  - icon: 🔔
    title: 多渠道通知
    details: 站内信箱与邮件、Webhook、Web Push 渠道可插拔，邀请、账单、准入、争议事件一一触达。
  - icon: 🛡️
    title: 防滥用与审计
    details: 注册与登录限速、全量审计日志、圈级拉黑与管理员举报处置，平台治理有据可查。
---

## 项目简介

Coterie 是一个开源的数字服务共享与订阅拼车平台：让用户持有一份数字服务订阅，确认其允许多人共享后，围绕它组织共享圈（Coterie），统一管理席位、费用、账期、成员与访问方式。平台关心四件事——谁在共享、怎么分摊、如何准入、出了问题怎么裁决；服务本身只做基础设施，不代持任何第三方的账号密码。

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
