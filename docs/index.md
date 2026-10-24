---
layout: home

hero:
  name: "Coterie"
  text: "General-purpose platform for shared digital services"
  tagline: "Organize a sharing circle around one subscription — manage seats, billing periods, and costs"
  actions:
    - theme: brand
      text: Quick start
      link: "#quick-start"
    - theme: alt
      text: Requirements
      link: /requirements
    - theme: alt
      text: Technical design
      link: /design
    - theme: alt
      text: GitHub
      link: https://github.com/cuihairu/coterie

features:
  - title: Subscription circles
    details: Build a circle around one subscription. Join by invitation or through the public directory; three roles (Owner/Admin/Member); member joins and leaves are logged.
  - title: Seats and quotas
    details: Seats belong to a subscription; members belong to a circle. Seat assignment and release are recorded in the audit log; usage caps are set by the sharing policy.
  - title: Cost splitting and billing periods
    details: Five split modes (D9) — equal, per-seat, fixed, by usage, and prorated (per-day); billing periods can roll over automatically. Disputed contributions go to the disputes ledger, decided by the Owner.
  - title: Public directory and payment gate
    details: Circle owners can publish a circle to the public directory. With the payment gate enabled, new members pay an admission fee first; once Stripe confirms, they join automatically.
  - title: Notification channels
    details: Beyond the in-site inbox, email, Webhook, and Web Push can be attached. A channel is not enabled until it is configured.
  - title: Abuse control and audit
    details: Rate limiting on registration and login; an append-only audit log for money and membership changes; circle owners can block abusive users, and admins handle reports.
---

## Overview

Coterie is an open-source platform for shared digital services and subscription carpooling: a user holds a subscription to a digital service, confirms that it may be legitimately shared among multiple people, and then organizes a sharing circle (Coterie) around it, managing seats, costs, billing periods, members, and access in one place. What the platform does is keep the organization, splitting, admission, and adjudication of sharing clear. The service itself is infrastructure only and never holds any third party's account credentials.

The project is in early development; Phases 1-4 of the [project plan](/project-plan) have been delivered (the core journey is fully usable: register → catalog → subscription → coterie → seats → invitation → join → contribution bills → settlement → notifications; the Stripe payment channel awaits real credentials to go to production).

## Quick start

Requires Go 1.26+ and PostgreSQL 15+ (migrations apply automatically on startup).

```bash
# 1. Configure (all variables in .env.example)
export DATABASE_URL=postgres://coterie:coterie@localhost:5432/coterie?sslmode=disable
export PORT=8080

# 2. Start
go run ./apps/server
```

Every endpoint except `GET /healthz`, `POST /api/v1/auth/register`, `POST /api/v1/auth/login`, and the public catalog reads requires `Authorization: Bearer <token>` (issued on register or login, valid for 30 days).

Run one MVP full-journey smoke test against a running server (register → Provider → Product → Subscription → coterie → seats → invitation → join → bills → settlement → notifications):

```bash
scripts/smoke.sh http://localhost:8080
```

Deployment (Docker Compose, configuration, platform-admin bootstrap) is covered in [deployment](/deployment).

## Documentation guide

| Document | Contents |
| --- | --- |
| [Requirements](/requirements) | Target users, user stories, functional requirements (FR), non-functional requirements (NFR), MVP boundaries and the non-goals list |
| [Technical design](/design) | Domain model, sharing and billing models, payment adapter, technical architecture, module breakdown, API list, and design decision records (ADR) |
| [Project plan](/project-plan) | Positioning, vision, principles, phased roadmap, and document index |
