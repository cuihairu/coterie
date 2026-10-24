# Coterie Project Master Plan

> **Coterie — Open-source platform for shared digital services.**
>
> Coterie is an open-source platform for sharing digital services and subscription carpooling.

| | |
|---|---|
| Status | Planning |
| Document version | v0.1 |
| Related docs | [Requirements](/requirements) · [Design Plan](/design) |

---

## 1. One-Sentence Positioning

> Coterie is an open-source platform for sharing digital services and subscription carpooling.

English:

> **Coterie — Open-source platform for shared digital services.**

Coterie targets digital services that are suitable for multiple people to use together and split the cost of — for example streaming, music, software, AI, and cloud services.

---

## 2. Core Philosophy

Coterie should not be designed as:

> A Netflix carpooling site

It should instead be designed as:

> **A platform where multiple people form a shared circle around a digital service and collectively bear the service cost.**

For example:

```text
                    Coterie
                       │
          ┌────────────┼────────────┐
          │            │            │
       Streaming      Music         AI
          │            │            │
       Netflix       Spotify      AI Service
       Disney+       YouTube      Software
          │            │            │
          └────────────┼────────────┘
                       │
                    Coterie
                  /    |    \
               Alice  Bob   Charlie
```

---

## 3. Why Redesign

The old project was centered mostly on film/TV accounts, with a model of:

```text
Film/TV platform
   ↓
Account
   ↓
Account members
   ↓
Cost
```

This model has obvious limitations:

1. Provider is hard-coded to be a film/TV platform.
2. The account and subscription concepts are mixed together.
3. It is hard to extend to services like AI, software, music, and gaming.
4. It is hard to express different sharing rules for different services.
5. It is not easy to support complex constraints such as "seats", "quota", "device count", and "regional restrictions".
6. It can easily evolve into a large amount of Provider-specific code.

Therefore the new version no longer uses "film/TV account" as the core; instead it abstracts thoroughly at the business-model level, unified as:

```text
Provider → Product → Subscription → Coterie → Member / Seat
```

**The thing that most should change in this re-open-sourcing: not changing a name, but redefining the domain model.**

---

## 4. Product Boundaries

Coterie should ultimately become:

> **A piece of "shared digital services infrastructure", not a "film/TV account trading site".**

It solves:

```text
I have a digital service
        ↓
It allows sharing by multiple people
        ↓
I need to organize these people
        ↓
Manage seats
        ↓
Manage costs
        ↓
Manage cycles
        ↓
Manage members
        ↓
Manage access methods
```

This is the true core value of Coterie.

### 4.1 What It Explicitly Is Not

- It is not a trading marketplace for third-party accounts (Marketplace is a later, independent module).
- It is not a password manager (does not store third-party account passwords by default; see [Requirements NFR-1](/requirements#nfr-1-security-model)).
- It is not a payment system (payment is an Adapter; Phase 1 only does manual settlement).
- It is not an automation tool for "auto-login / auto-register to third-party services".

### 4.2 End Vision

```text
                    Coterie
                       │
        ┌──────────────┼──────────────┐
        │              │              │
     Streaming       Software         AI
        │              │              │
     Netflix         Adobe          Claude
     Disney+         JetBrains      ChatGPT
        │              │              │
        └──────────────┼──────────────┘
                       │
                Shared Services
                       │
                  ┌────┴────┐
                  │ Coterie │
                  └────┬────┘
                       │
            ┌──────────┼──────────┐
            │          │          │
          Member     Member     Member
            │          │          │
          Seat       Seat       Seat
            │          │          │
            └──────────┼──────────┘
                       │
                    Billing
                       │
                  Contribution
```

**Coterie's end goal is not to help users "rent accounts", but to help users organize and manage legitimate shared digital services.**

---

## 5. Product Design Principles

These eight principles are the highest constraints for all subsequent design and implementation decisions.

| # | Principle | Meaning |
|---|------|------|
| 1 | **Generic First** | Don't do Netflix-first; do Generic-first. Netflix is just one Provider. |
| 2 | **Subscription ≠ Account** | Subscription and account must be separated. Under a subscription come Account, Seats, Quota, Access. |
| 3 | **Member ≠ Account** | A user is a platform member; a third-party account is an external resource; the two must be separated. |
| 4 | **Coterie is the Core** | Coterie is the product's core concept, representing "a collaboration circle formed by a group of people around a shared resource". |
| 5 | **Provider Agnostic** | The core system does not depend on any specific platform such as Netflix or Disney+. |
| 6 | **Self-hosting First** | An open-source project should be: download → configure → Docker Compose → run. |
| 7 | **Payment Is an Adapter** | Payment is not part of the core business model. |
| 8 | **Secret Is Optional** | Does not store third-party account passwords by default. |

Supplementary inferences (from the domain model design):

- **Coterie's core is not Account Sharing, but Resource / Subscription Sharing.**
- **Provider Adapter must be an optional extension**; the core platform should not depend on any specific Provider.
- **Platform capability ≠ Provider allowance**: the system cannot claim by default that all accounts can be shared.

---

## 6. Roadmap

### Phase 1 — MVP

Only implement these modules:

```text
User
Provider
Product
Subscription
Coterie
Member
Seat
Contribution
Invitation
Notification
```

Complete flow:

```text
Register
 ↓
Create Provider
 ↓
Create Product
 ↓
Create Subscription
 ↓
Create Coterie
 ↓
Set Seats
 ↓
Invite members
 ↓
Members join
 ↓
Record costs
 ↓
Cycle settlement
```

The content Phase 1 explicitly **does not do** is in [Requirements · MVP Non-Goals](/requirements#7-mvp-non-goals).

### Phase 2

```text
Marketplace
Provider Catalog
Payment Adapter
Email
Webhook
Usage Tracking
Quota
```

Target flow:

```text
Browse
 ↓
Find Coterie
 ↓
Request Join
 ↓
Payment
 ↓
Join
```

### Phase 3 (completed)

```text
Provider Plugins        ✅ PolicyValidator / AdmissionGuard / UsageValidator plugin surface (D12)
Payment Plugins         ✅ Configure registered channels + Sandbox demo adapter (D11)
Usage Plugins           ✅ D12 third hook (accounting re-verification)
Automation              ✅ auto_billing scheduler cycle roll (D13)
Advanced Billing        ✅ prorated per-day proportional allocation
Dispute                 ✅ dispute ledger + Owner one-way adjudication (D14)
Reputation              ✅ settlement-reputation derived read-only projection (D15)
```

### Phase 4 — Vision and Work List

Ultimately become:

> **Open-source marketplace and infrastructure for shared digital services.**

```text
                    Coterie
                       │
       ┌───────────────┼────────────────┐
       │               │                │
    Sharing         Billing          Marketplace
       │               │                │
   Membership       Payment          Discovery
   Seat             Settlement       Matching
   Quota            Contribution     Reputation
       │               │                │
       └───────────────┼────────────────┘
                       │
                  Provider System
```

The vision breaks down into the following batches of work (done in order; check the box when complete; each item first lands a design entry, then gets implemented):

1. [x] **Audit Log** (FR-15 / NFR-7 Phase 1 foundation) — append-only `audit_logs` ledger + same-transaction instrumentation for fund/member changes + Owner read-only audit endpoint (design D17, §6.3)
2. [x] **Marketplace reputation display slot** (FR-18 follow-up) — catalog entries carry an Owner settlement-reputation badge (§4.4 reserved)
3. [x] **Rate Limit** (NFR-7 Phase 1 foundation) — rate limiting on public endpoints such as register/login
4. [x] **Custom billing cycle automation** (§13 leftover) — decision on how to express custom cycles + scheduler cycle-roll support (D13 extension)
5. [x] **Sharing Policy UsageLimit** (FR-10 follow-up extension) — core-side per-member per-cycle usage cap (D21)
6. [x] **Real payment channel** (FR-9 follow-up) — Stripe async adapter + public Webhook (D24); Marketplace payment gate (D25: payment_gate + awaiting_payment + independent ledger for the admission fee + Webhook-driven join). Production use still requires real credentials
7. [x] **Push notifications** (FR-12 follow-up) — Web Push standard channel (D22)
8. [x] **Resource sharing mode** (FR-6 follow-up, §2.5) — reuse Seat+metadata (D23)
9. [x] **Anti-abuse closure** (NFR-7 / FR-14) — platform admin role (bootstrap via `ADMIN_EMAILS`), reporting (public circles reportable + admin inbox with resolve/dismiss), circle-level blacklist (403 across all admission paths) (D26)
10. [x] **Deployment closure** (§11) — one-command full-stack start via compose (postgres healthcheck + source-built image), CI gate per push (gofmt/build/vet/test), docs-site deployment page
11. [x] **GHCR image publishing** (§11.2) — push `ghcr.io/cuihairu/coterie` after nightly tests pass (nightly rolling tag + date tag, amd64/arm64); compose switches to the registry image; also closes out duplicate serial numbers for manual payment (409 instead of 500)
12. [x] **Admin audit query surface** (NFR-7 / D27) — `GET /api/v1/admin/audit-logs` (platform admins only), action/actor/coterie/subscription filters + pagination, sharing the same append-only table as the Owner read surface
13. [x] **API reference page** (docs site) — flat, full endpoint list; auth/pagination/error-envelope conventions
14. [x] **Web client start** (§7.5) — React+TS+Vite+Tailwind+TanStack (Query/Router); login/register, circle list, circle-detail read-only views (members/seats/cycle allocations); CI web job (typecheck+build+lint); also fixes the `GET /coteries` over-permission (not filtered by membership, so any logged-in user could see every circle)
15. [x] **CI compose full-stack smoke** — per push, build the image, bring up the compose stack, and run the full journey via `scripts/smoke.sh` (needs: test)
16. [x] **golangci-lint into the gate** — v2 standard set (errcheck/govet/ineffassign/staticcheck/unused) + CI step; all 29 existing findings converged (including migrating VAPID key parsing to crypto/ecdh and migrating moby client.New)
17. [x] **Single-port full stack and session lifecycle** (§7.5/§7.6, D8) — go:embed bakes the Vite build output into the binary (multi-stage Dockerfile with a node build stage); non-API paths fall back to the SPA; client logout goes through `POST /auth/logout` for server-side revocation, a 401 while a token is present auto-clears the session back to the login page, and entry is validated via `/auth/me`; server-side expired sessions are deleted on sight and swept per user at login; circle-creation wizard (provider→product→subscription→coterie), one-time display of invitation tokens, seat add/assign write operations
18. [x] **Dashboard write surface** (§7.5 Dashboard-first) — cycle write operations (create cycle, equal-split to generate allocations, close cycle, register manual settlement entry by entry); invitation-token join entry (paste on the circle list page to join); notification inbox (unread badge with 30s polling + per-item mark-read); shared plaza read-only directory (Owner reputation badge + equal-split estimate + request to join, FR-16/D18 display slot)
19. [x] **Request/usage/dispute operation surfaces** — Owner join-request inbox (accept/decline + D25 payment-gate toggle: on accept, set `awaiting_payment`; initiate charging the manual admission fee to join); usage-accounting UI (D9 ledger read-only + Owner accounting/negative correction + D21 per-member per-cycle cap setting, over-limit surfaces a 409); dispute UI (D14: a member raises a dispute on an allocation row with reason/evidence; Owner dispute ledger with status filter + one-way accept/reject adjudication; adjudication notification receipt)

---

## 7. External Messaging (README Front Page)

The logo is in the repo: `assets/logo.svg` (a circular mark with right-angle cut corners forming a "C" wordmark, primary color `#FF0054`).

When re-open-sourcing, the README front page should express:

````markdown
<div align="center">
  <img src="assets/logo.svg" width="120" alt="Coterie logo" />
  <h1>Coterie</h1>
  <p><strong>Open-source platform for shared digital services.</strong></p>
  <p>
    <a href="LICENSE"><img alt="License" src="https://img.shields.io/badge/license-Apache--2.0-blue"></a>
    <img alt="Go" src="https://img.shields.io/badge/Go-1.22%2B-00ADD8?logo=go&logoColor=white">
    <img alt="Docker" src="https://img.shields.io/badge/docker-compose%20ready-2496ED?logo=docker&logoColor=white">
  </p>
</div>

Organize subscriptions, seats, members, and shared costs in one place.

Coterie is designed for any digital service that can be legitimately shared
among multiple people — from streaming and music to software, AI, cloud
services and more.
````

Immediately after, show the core model:

```text
Provider
    ↓
Subscription
    ↓
Coterie
    ↓
Members
    ↓
Seats / Quota / Cost
```

Badges (shields.io) should be kept to the bare minimum necessary: License, Go version, Docker status; add build/coverage badges only after CI is ready.

> Note: `README.md` / `README.zh.md` have been rewritten per this section (bilingual, including CI badges, the core model diagram, and a quick start).

---

## 8. Document Structure

### 8.1 Current Docs and Resources

```text
docs/
├── project-plan.md   # This doc: positioning, vision, principles, roadmap, doc index
├── requirements.md   # Requirements doc: functional requirements, non-functional requirements, MVP and non-goals
└── design.md         # Design plan: domain model, sharing model, billing/payment,
                      # technical architecture, module breakdown, API, deployment

assets/
└── logo.svg          # Project logo (primary color #FF0054)
```

Suggested reading order: **project-plan (why we build it) → requirements (what to build) → design (how to build it)**.

Each piece of content is maintained in only one document; the others reference it via relative links, avoiding inconsistencies caused by duplication.

### 8.2 Planned Docs

As the project advances, `docs/` can be expanded to:

```text
docs/
├── project-plan.md
├── requirements.md
├── design.md
├── architecture.md
├── domain-model.md
├── sharing-model.md
├── billing.md
├── provider.md
├── plugin.md
├── security.md
├── deployment.md
└── roadmap.md
```

### 8.3 Repository Root Docs

```text
README.md
LICENSE               # Apache 2.0
CONTRIBUTING.md
SECURITY.md
CODE_OF_CONDUCT.md    # Contributor Covenant v2.1
```

All of the files above are in place; the README is written per the wording in [§7](#7-external-messaging-readme-front-page).
