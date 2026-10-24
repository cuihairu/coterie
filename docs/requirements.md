# Coterie Requirements Specification

| | |
|---|---|
| Status | Draft |
| Document version | v0.1 |
| Related documents | [Project Plan](./project-plan.md) · [Design Plan](./design.md) |

---

## 1. Overview

### 1.1 Goals

Coterie provides a general-purpose digital service sharing infrastructure that lets users:

- own a digital service (subscription);
- verify that the service permits sharing among multiple people;
- organize a sharing circle (Coterie) around that service;
- manage seats, costs, cycles, members and access methods.

This requirements document defines the capabilities that must be delivered in **Phase 1 (MVP)**, and assigns requirements to later phases.

### 1.2 Scope

Coterie is **Generic-first**: any digital service that can be legally shared can enter the model, including but not limited to streaming, music, software, AI, cloud services, VPN and game services.

The core system does not depend on any specific Provider (Netflix, Spotify and the like are only Provider instances).

### 1.3 Glossary

The core terms used here are defined below; the full model definition is in [Design Plan · Domain Model](./design.md#1-domain-model).

| Term | Meaning |
|------|---------|
| **Provider** | A digital service provider (e.g. Netflix, Spotify, Claude). Describes only the supplying side, not the plan. |
| **Product** | A specific product or plan offered by a Provider (e.g. Netflix Premium). |
| **Subscription** | The actually purchased subscription instance, including price, cycle, renewal date, sharing policy, etc. |
| **Coterie** | A sharing circle of people around a digital service — the single most central concept in the system. |
| **Member** | A participant in a Coterie; a platform user identity. |
| **Seat** | A unit of subscription capacity (a generic allocation unit reused by the Quota model), decoupled from Member. |
| **Contribution** | A record of a member's share of the shared cost. |
| **Sharing Policy** | The sharing rules for the subscription (limits on headcount, devices, region, quota, etc.). |
| **Sharing Model** | The sharing model: Account / Seat / Family / Quota / Resource. |

---

## 2. Users and Roles

### 2.1 Platform-level Roles

| Role | Description |
|------|-------------|
| **User** | A regular user who can create and join Coteries. |
| **Platform Admin** | A platform administrator responsible for platform-level management; does not take part in the day-to-day management of each Coterie. |

### 2.2 Roles Within a Coterie

```text
Owner
Admin
Member
```

Can be added in the future:

```text
BillingManager
Moderator
Viewer
```

The refinement of roles and permissions is decided at the design stage; MVP distinguishes at minimum Owner (creator/manager) and Member (participant).

---

## 3. Core User Journey (MVP)

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
Set up Seats
 ↓
Invite members
 ↓
Members join
 ↓
Record costs
 ↓
Cycle settlement
```

This is the end-to-end acceptance flow for the MVP: every step above must be completable through both the Web interface and the API.

---

## 4. Functional Requirements

Requirement numbering convention: `FR-x` for functional requirements, `NFR-x` for non-functional requirements. Numbers stay stable once published, for reference by other documents and the implementation.

### FR-1 Users and Authentication

**Must** provide a basic user system.

User fields:

```text
User
├── ID
├── Username
├── Email
├── Avatar
├── Locale
├── Timezone
├── CreatedAt
└── Status
```

Supported login methods:

- Email
- OAuth
- Passkey
- OIDC

Specific login methods are extended through the authentication module and are not hard-coded into core business logic.

### FR-2 Provider

Provider represents a digital service provider.

```yaml
provider:
  id: netflix
  name: Netflix
  category: streaming
```

Requirements:

- Users can create and browse Providers;
- A Provider describes only the service supplier and **does not contain specific plan information**;
- Category is used for organization, e.g. `streaming`, `music`, `ai`, `software`;
- **A Generic Provider must work out of the box**, allowing users to manually create any service (see FR-3).

Example Providers under sample categories: Netflix, Disney+, Spotify, YouTube, Claude, ChatGPT, Adobe, Microsoft, JetBrains, VPN Provider, Cloud Provider, Game Service.

### FR-3 Product

Product represents a specific product or plan offered by a Provider.

```text
Netflix
├── Standard
├── Premium
└── Other

Adobe
├── Creative Cloud
├── Acrobat
└── Photoshop
```

Requirements:

- A Product belongs to a Provider;
- Plan information is **not hard-coded into the sharing group (Coterie)** and must be modelled through Product;
- Even without a Provider Adapter, users can manually create any Provider / Product / Subscription through a Generic Provider:

```text
Provider:    My Software
Product:     Premium Plan
Subscription: 5 Seats
Coterie:     Development Team
Members:     5
```

### FR-4 Subscription

Subscription represents the subscription that was actually purchased.

```text
Netflix Premium
        │
        ▼
Subscription #12345
        │
        ▼
$22.99 / month
```

Must include:

- Product
- Owner
- Billing Cycle
- Price
- Currency
- Start Date
- Renewal Date
- Status
- Sharing Policy
- Maximum Members
- Maximum Seats

### FR-5 Coterie

**Coterie is the single most central concept in the entire system.**

A Coterie represents:

> A sharing circle of people around a digital service.

```text
Coterie #1001

Netflix Premium
$22.99 / month

Owner
  ↓
Alice

Members
  ├── Alice
  ├── Bob
  ├── Charlie
  └── David
```

Requirements:

- A Coterie **maps strictly** to one Subscription (1:1, decision D1, see [Design Plan §1.8](./design.md#18-design-decision-records-adr));
- A Coterie **does not assume that all services are "account sharing"**; it must support different sharing models (see FR-6);
- The following lifecycle must be supported:

```text
Draft ──▶ Open ──▶ Active ⇄ Paused
   │        │        │
   └────────┴────────┴────▶ Closed (terminal)
```

Lifecycle semantics:

| State | Meaning |
|------|---------|
| **Draft** | Sharing group created but not yet published. |
| **Open** | Published, waiting for members to join. |
| **Active** | Running normally. |
| **Paused** | Paused (members and seats are retained, settlement is suspended, and it can be resumed). |
| **Closed** | Sharing ended (terminal state). |

**Full is not a persisted state**: "seats are full" is a display flag derived from the number of free seats (true when free seats reach 0, lifted automatically once a seat is released), used for display and join validation. See [Design Plan §1.5](./design.md#15-coterie-lifecycle-and-state-machine) for the state machine and transition rules.

### FR-6 Sharing Models

The system **must** support multiple sharing models, not account sharing only. This is one of the most important extension points of Coterie.

#### 6.1 Account Sharing

Multiple users share a single account.

```text
Netflix Account
      │
 ┌────┼────┐
 ▼    ▼    ▼
User User User
```

#### 6.2 Seat Sharing

Each person owns an independent seat.

```text
Software Subscription
        │
 ├── Seat 1 → Alice
 ├── Seat 2 → Bob
 └── Seat 3 → Charlie
```

Applies to: software, SaaS, AI, team services.

#### 6.3 Family Sharing

The service itself provides a family group.

```text
Family Plan
    │
 ├── Owner
 ├── Member
 ├── Member
 └── Member
```

#### 6.4 Quota Sharing

A quota is shared rather than an account.

```text
1000 credits
      │
 ├── Alice → 300
 ├── Bob   → 300
 └── Carol → 400
```

Applies to: AI, API, cloud, storage, VPN traffic.

> Modelling approach: reuse Seat; each Seat carries quota metadata (see FR-8 and [Design Plan §1.6](./design.md#16-seat-the-universal-allocation-unit)).

#### 6.5 Resource Sharing

A kind of resource is shared.

```text
Cloud Storage
     │
     ├── 1 TB
     │
     ├── Alice
     ├── Bob
     └── Charlie
```

> Modelling approach: same mechanism as Quota (D23): the Seat's metadata carries `{"resource": <capacity>,
> "unit": "..."}`, and `used` is projected per seat from the usage ledger; billing splits are selected as usual.

> **Conclusion: the core of Coterie is not Account Sharing, but Resource / Subscription Sharing.**

### FR-7 Member

Member represents a participant in a Coterie.

```text
Member
├── User
├── Role
├── Status
├── JoinedAt
└── LeftAt
```

Requirements:

- A Member is linked to a platform User; **Member ≠ a third-party account**;
- Supports joining, removal, and role assignment;
- See [§2.2](#22-roles-within-a-coterie) for roles.

### FR-8 Seat

Seat is a very important abstraction. **Do not assume "one Member = one account".**

```text
1 Subscription
    │
    ├── Seat A
    ├── Seat B
    ├── Seat C
    └── Seat D
```

Design constraints:

- A Seat belongs to the **Subscription** (the source of capacity); the Coterie is only responsible for allocating seats to members (decision D2);
- At any point in time each Seat has at most one occupant; empty seats are allowed;
- Quota sharing likewise reuses Seat, with each Seat carrying quota metadata (decision D4).

The relationship between Member and Seat must allow:

- one member, one Seat;
- one member, multiple Seats;
- a Seat temporarily free;
- Seat transfer;
- Seat reclamation.

### FR-9 Cost Splitting and Settlement (Contribution / Billing)

Contribution represents a member's share of the shared cost.

```text
Subscription
$30/month

Alice      $10
Bob        $10
Charlie    $10
```

Each Contribution must record:

```text
amount
currency
billing_cycle
payer
status
period
```

Billing flow:

```text
Subscription
      │
      ▼
Billing Period
      │
      ▼
Member Contribution
      │
      ▼
Settlement
```

Billing methods that must be supported:

- Monthly / yearly / custom cycle (custom cycle = a `cycle_days` offset of 1–365 days; see design D20; automated rollover see D13)
- Per seat
- Proportional
- Fixed amount
- Per usage

**Important: Coterie must not bind "payment" directly to a payment platform.** Phase 1 implements only **Manual Settlement**: the system records who owes how much. Real payment is integrated through a Payment Adapter in later phases (see [Design Plan · Billing and Payment](./design.md#4-billing-and-payments)).

### FR-10 Sharing Policy (Sharing Limits)

Different services have different sharing rules, and the system must be able to express them through a Sharing Policy:

```yaml
sharing:
  mode: seat
  max_members: 5
  max_devices: 10
  region: US
  transfer: true
```

```yaml
sharing:
  mode: quota
  quota: 1000
  unit: credits
```

Limit types that must be supported (now or as future extensions):

```text
DeviceLimit
RegionLimit
ConcurrentLimit
MemberLimit
QuotaLimit
UsageLimit
TimeLimit
```

### FR-11 Invitation

```text
Owner
  │
  ▼
Invitation
  │
  ├── Token
  ├── ExpireAt
  ├── Role
  └── Coterie
```

Must support:

- invitation links
- invitation codes
- Email Invitation
- Private Invitation

### FR-12 Notification

A unified notification system that must cover the following types:

```text
Notification
├── Invitation
├── Payment Due
├── Subscription Renewal
├── Seat Assigned
├── Subscription Expired
├── Coterie Closed
└── System
```

Notification channels:

```text
Web
Email
Push
Webhook
```

The Push channel is implemented on the Web Push standard (D22): users register a browser push endpoint, and
notifications are encrypted per RFC 8291 and signed with RFC 8292 VAPID before being sent out; Web / Email (SMTP) /
Webhook (HMAC-signed POST) were already implemented previously.

Phase 1 needs only **Web Notification + Email Adapter**.

### FR-13 Dashboard

#### 13.1 User Home

```text
My Coteries

Netflix
● Active
$5 / month

Spotify
● Active
$3 / month

AI Service
● Active
$10 / month
```

Also displayed:

- current costs
- next settlement time
- Seats
- Subscription status
- invitations
- outstanding payments
- notifications

#### 13.2 Owner Dashboard

```text
Subscription
├── Price
├── Billing
├── Members
├── Seats
├── Contributions
└── Settlement
```

For example:

```text
5 Seats

Occupied: 4
Available: 1

Monthly Cost: $30

Collected: $24
Pending: $6
```

### FR-14 Admin

A platform administrator is responsible for:

```text
Users
Providers
Products
Coteries
Subscriptions
Reports
System
```

**An Admin does not take part in the day-to-day management of each Coterie.**

### FR-15 Audit Log

**Implemented (Phase 4, design D17, see design §6.3)**: `audit_logs` is an append-only ledger (who/what/when/where + before/after snapshot); money and member changes are instrumented inside the same business transaction (an audit failure rolls back the change); read-only subscription Owner endpoint plus `?action=` filtering; members get no audit read access and no edit surface.

Original requirement: because shared resources and money are involved, the Audit Log **must** record:

```text
Who
What
When
Where
Before
After
```

Example:

```text
Alice
changed subscription price
$20 → $22
2026-10-07
```

Key operations to record:

- create Coterie
- remove member
- transfer Seat
- modify costs
- modify subscription
- settlement
- close Coterie

### FR-16 Marketplace (Phase 2)

Not built heavily in Phase 1; kept as an independent later module.

```text
Browse
  ↓
Provider
  ↓
Product
  ↓
Available Coteries
  ↓
Join
```

For example:

```text
Netflix Premium

Available Coteries

Coterie #1024
3 / 5 seats
$5 / month

Coterie #1025
4 / 5 seats
$4.8 / month
```

**Constraint: Coterie itself must not default to becoming an "account trading market".**

### FR-17 Dispute (Phase 3)

Not implemented in Phase 1. Fields reserved in the design:

```text
Dispute
├── Buyer
├── Owner
├── Coterie
├── Transaction
├── Reason
├── Evidence
├── Status
└── Resolution
```

---

## 5. Non-functional Requirements

### NFR-1 Security Model

**Do not use account passwords as the core model**. This is one of the most important design principles.

The system should store as much as possible:

```text
Credential Metadata
```

rather than:

```text
Provider Password
```

For example:

```text
Access Method:
    External

Credential:
    Not stored
```

If Account / Password / Recovery Code / API Key / License Key really do need to be stored in the future, they must go into a separate **Secret module**:

```text
Coterie
   │
   ▼
Secret Reference
   │
   ▼
Secret Store
```

The database stores only `secret_id`, not the plaintext Secret.

> Corresponds to design principle #8: **Secret Is Optional**.

### NFR-2 Compliance and Risk

Coterie is a general sharing platform and **must** distinguish:

```text
Platform Capability
        ≠
Provider Permission
```

Some services allow family sharing, team members and multiple seats but do not allow account sharing, or carry regional restrictions. Therefore:

- The system **must not claim by default** that "all accounts can be shared";
- It must provide carriers for Sharing Policy, Provider Terms, Region, Eligibility and similar information;
- **The UI must state this clearly**: users need to verify the relevant Provider's terms of service, regional restrictions and sharing permissions themselves;
- The platform should support **lawfully authorized family, team, seat and subscription sharing** wherever possible.

### NFR-3 Self-hosting and Deployment

An open-source project must make it possible to:

```bash
docker compose up -d
```

and have it run.

Minimal deployment:

```yaml
services:
  coterie:
    image: ...

  postgres:
    image: postgres
```

Support targets: Docker, Docker Compose, Kubernetes, Bare Metal; **Docker Compose is the officially preferred path**, because target users are not necessarily professional operators.

Target runtime shape:

```text
Single Binary
     +
PostgreSQL
```

### NFR-4 API-first

Design API-first from day one; the Web interface is only one client of the API.

Core resources:

```text
/api/v1/users
/api/v1/providers
/api/v1/products
/api/v1/subscriptions
/api/v1/coteries
/api/v1/members
/api/v1/seats
/api/v1/contributions
/api/v1/invitations
/api/v1/notifications
```

### NFR-5 Extensibility

- **Provider Agnostic**: branches such as `if netflix` / `if disney` / `if spotify` / `if chatgpt` must not appear in core business logic;
- A Provider Adapter is an **optional extension**; the core platform does not depend on any specific Provider;
- Future extensions are carried by a Plugin architecture (providers / payment / notification / authentication / storage), while the core system stays stable;
- The Provider Catalog / Registry is **metadata only** and is not bound to specific accounts.

### NFR-6 Multi-tenancy

Stated clearly for Phase 1:

> **Single Tenant / Self-hosted First**

Do not introduce complex SaaS multi-tenancy too early. If SaaS support becomes necessary later, introduce the Instance → Organization → Users / Coteries hierarchy then.

### NFR-7 Abuse Prevention

Phase 1 implements only the basic capabilities:

```text
Rate Limit
Report
Block
Audit Log
```

Status: Audit Log implemented (D17); Rate Limit implemented (D19, fixed-window limits on public register/login endpoints); Report / Block implemented (D26: platform admin + coterie-level blocking + report inbox).

To be added when the public Marketplace opens:

```text
Anti-spam
Fraud Detection
Account Reputation
Dispute
```

Phase 3 progress: Dispute implemented (FR-17, design D14); Account Reputation implemented as a **pure derivation** (FR-18, design D15: payment statistics aggregated from the contributions ledger, no manual scoring surface); Anti-spam / Fraud Detection to be extended later.

---

## 6. MVP Scope

**Modules that Phase 1 must deliver:**

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

**Acceptance criteria**: all eleven steps of the flow in [§3 Core User Journey](#3-core-user-journey-mvp) work end to end (Web + API), and the parts marked as Phase 1 in NFR-1 through NFR-7 are met.

---

## 7. MVP Non-goals

The first version explicitly does **not** do the following, to avoid the project losing control:

- complex Marketplace
- automated purchasing
- automated Provider registration
- automated login to third-party services
- automated scraping of third-party accounts
- automated payment for all Providers
- wallet
- points
- cryptocurrency
- complex recommendation system
- AI Agent
- large-scale risk control

---

## 8. Requirement Phase Assignment

| ID | Requirement | Phase |
|------|------|------|
| FR-1 | Users and Authentication | Phase 1 |
| FR-2 | Provider (including Generic Provider) | Phase 1; Phase 2 (public read of the catalog + seeded registry, see design §5.4/§6.1); Phase 3 (plugin surface PolicyValidator/AdmissionGuard, see D12; the real `claude` plugin binds a seeded catalog slug and is registered via `PROVIDER_PLUGINS`, see D16) |
| FR-3 | Product | Phase 1 |
| FR-4 | Subscription | Phase 1 |
| FR-5 | Coterie and lifecycle | Phase 1 |
| FR-6 | Sharing models | Phase 1 (basic Account / Seat / Family support); Phase 2 (Quota usage metering and usage-based splitting, see design D9); Phase 3 (Provider plugin UsageValidator metering re-check surface, see D12); Phase 4 (Resource sharing implemented: reuses Seat + metadata, see design D23) |
| FR-7 | Member | Phase 1 |
| FR-8 | Seat | Phase 1 |
| FR-9 | Cost splitting and settlement (Manual Settlement) | Phase 1; Phase 2 (Adapter interface + `payments` ledger + Manual channel); Phase 3 (Sandbox demo channel registered via configuration, `GET /payments/methods` lists channels; the `auto_billing` scheduler rolls cycles, see design D13; `prorated` per-day proportional splitting); Phase 4 (Stripe real channel implemented: async confirmation + public Webhook, see design D24) |
| FR-10 | Sharing Policy | Phase 1 (basic limits); Phase 3 (MemberLimit `max_members` admission enforcement + Provider plugin policy re-check/admission guard implemented, see design D12); Phase 4 (UsageLimit implemented: per-member per-period usage ceiling, see design D21; QuotaLimit implemented: `quota_limit` subscription-wide aggregate ceiling + post-write seat quota enforcement, see design D30; remaining Limits to be extended later) |
| FR-11 | Invitation | Phase 1 |
| FR-12 | Notification (Web + Email) | Phase 1; Phase 2 (Email SMTP adapter + outbound Webhook + Push implemented, on the Web Push standard, D22) |
| FR-13 | Dashboard | Phase 1 |
| FR-14 | Admin | Phase 1 (basic); Phase 4 adds the platform admin role and report disposition surface (`ADMIN_EMAILS` bootstrap + `reports` inbox resolve/dismiss, see design D26; Admin does not take part in day-to-day coterie management) |
| FR-15 | Audit Log | Phase 1 (implemented, D17) |
| FR-16 | Marketplace | Phase 2 (read-only catalog + JoinRequest inbox implemented, see design D10); Phase 4 adds the payment gate (D25: coterie-level `payment_gate` switch + awaiting_payment + separate admission-fee ledger + Webhook-driven joining, channel reusing D24 Stripe) |
| FR-17 | Dispute | Phase 3 (implemented: `disputes` separate ledger + one-way Owner adjudication, adjudication decoupled from settlement, see design D14; Buyer/Owner/Reason/Evidence/Status/Resolution field mapping see §4 FR-17) |
| FR-18 | Account Reputation | Phase 3 (implemented: settlement-derived read-only reputation endpoint `GET /api/v1/users/{id}/reputation`, see design D15; Phase 4 completes the Marketplace display slot: catalog entries carry an Owner-derived badge, see design D18) |
| NFR-1 ~ NFR-7 | Non-functional requirements | See the phase annotations inside each item |
