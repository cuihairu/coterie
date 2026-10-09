<div align="center">
  <img src="assets/logo.svg" width="64" height="64" alt="Coterie logo" />
  <h1>Coterie</h1>
  <p><strong>Open-source platform for shared digital services.</strong></p>
  <p>
    <a href="LICENSE"><img alt="License" src="https://img.shields.io/badge/license-Apache--2.0-blue"></a>
    <img alt="Status" src="https://img.shields.io/badge/status-early%20development-orange">
    <img alt="Go" src="https://img.shields.io/badge/Go-1.25%2B-00ADD8?logo=go&logoColor=white">
    <img alt="Docker" src="https://img.shields.io/badge/docker-compose%20ready-2496ED?logo=docker&logoColor=white">
  </p>
</div>

[English](README.md) | [中文](README.zh.md)

---

**Organize subscriptions, seats, members, and shared costs in one place.**

Coterie is designed for any digital service that can be legitimately shared
among multiple people — from streaming and music to software, AI, cloud
services and more.

> 🚧 **Status: Phase 4 backend complete.** Provider plugin face with
> policy/admission/usage hooks plus a real `claude` plugin sample,
> sandbox payment channel, auto-billing rollover (monthly, yearly, and
> custom day-count cycles), prorated (per-day) split, contribution
> disputes with owner decisions, derived settlement reputation (own
> endpoint plus an owner badge in the marketplace directory),
> append-only audit log, rate limiting on the public auth endpoints,
> per-member per-period usage limits from the sharing policy, Web Push
> notifications, resource-pool sharing, the Stripe payment channel
> (async adapter with a signed public webhook), and the marketplace
> payment gate (admission charged before joining) — on top of Phase 2's
> usage tracking, quota, marketplace, provider catalog, payment
> adapter, and email/webhook notification channels. Live Stripe
> credentials are all that remains to take real money.
> The full REST API works
> end to end: auth → catalog → subscription → coterie → seats →
> invitations → billing → payments → notifications → usage records →
> marketplace. Web and mobile clients have not started yet — the API is
> the product for now ([design §9](docs/design.md#9-api-design)).

## Quick start

Requires Go 1.25+ and a PostgreSQL 15+ instance (migrations apply
automatically on startup).

```bash
# 1. Configure (see .env.example for all variables)
export DATABASE_URL=postgres://coterie:coterie@localhost:5432/coterie?sslmode=disable
export PORT=8080

# 2. Run
go run ./apps/server
```

Every route except `GET /healthz`, `POST /api/v1/auth/register`,
`POST /api/v1/auth/login`, and the public catalog reads (`GET /api/v1/providers`,
`GET /api/v1/products`, `GET /api/v1/marketplace/coteries`) needs
`Authorization: Bearer <token>` (tokens are issued on register/login and
live for 30 days).

A complete smoke test of the MVP journey (register → provider → product →
subscription → coterie → seats → invite → join → billing → settlement →
notifications) against a running server:

```bash
scripts/smoke.sh http://localhost:8080
```

## API overview (v1)

| Area | Endpoints |
|------|-----------|
| Auth | `POST /api/v1/auth/register` · `login` · `logout` · `GET me` |
| Catalog | `/api/v1/providers` · `/api/v1/products` (CRUD; reads public, seeded registry ships in migration 0006) |
| Subscription | `/api/v1/subscriptions` (CRUD) · `/api/v1/subscriptions/{id}/seats` |
| Seats | `/api/v1/seats/{id}` · `assign` · `release` |
| Coterie | `/api/v1/coteries` (create with capacity, lifecycle, members, leave) |
| Invitations | `POST /api/v1/coteries/{id}/invitations` · `POST /api/v1/invitations/accept` |
| Billing | `/api/v1/subscriptions/{id}/billing-periods` · `generate` (equal / per_seat / fixed / usage / prorated) · `/api/v1/contributions/{id}` |
| Payments | `POST/GET /api/v1/contributions/{id}/payments` (manual, plus configured channels) · `GET /api/v1/payments/{id}` · `GET /api/v1/payments/methods` · `POST /api/v1/payments/webhooks/{method}` (public, signature-only) |
| Disputes | `POST /api/v1/contributions/{id}/disputes` · `POST /api/v1/disputes/{id}/decide` (owner) · `GET /api/v1/subscriptions/{id}/disputes` |
| Reputation | `GET /api/v1/users/{id}/reputation` (derived settlement stats) |
| Usage | `POST/GET /api/v1/subscriptions/{id}/usage-records` · `GET /api/v1/usage-records/{id}` |
| Marketplace | `GET /api/v1/marketplace/coteries` (public; entries carry a derived owner reputation badge) · `POST/GET /api/v1/coteries/{id}/join-requests` · `POST /api/v1/join-requests/{id}/accept` · `decline` · `DELETE /api/v1/join-requests/{id}` · `POST /api/v1/join-requests/{id}/payments` (payment gate) · `POST /api/v1/marketplace/webhooks/{method}` (public) |
| Notifications | `GET /api/v1/notifications` · `POST /api/v1/notifications/{id}/read` · push endpoints `POST/GET /api/v1/push/subscriptions` · `DELETE /api/v1/push/subscriptions/{id}` |
| Audit | `GET /api/v1/subscriptions/{id}/audit-logs` · `GET /api/v1/coteries/{id}/audit-logs` (owner, read-only) |

Errors use a single envelope `{"error": {"code", "message", "details?}}`;
lists use `{"items": [...], "meta": {total, limit, offset}}`. Details in the
[design doc](docs/design.md).

## How it works

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

- **Provider → Product** — any service vendor and its plans, described as plain
  metadata. Generic-first: no hardcoded provider logic in the core.
- **Subscription** — an actual purchase: price, billing cycle, renewal date,
  sharing policy, seat capacity. The single source of cost and capacity.
- **Coterie** — the sharing circle organized around exactly one subscription:
  members, roles, invitations, and cost splitting.
- **Member / Seat** — people are platform users; seats are allocatable units of
  a subscription (quota sharing reuses the same concept).
- **Contribution** — what each member owes per billing period. Phase 1 records
  and settles manually; real payment is an adapter, not the core.

## Design principles

| | |
|---|---|
| Generic first | Netflix is just a provider, never a special case |
| Subscription ≠ Account | Subscriptions, seats, quotas, and credentials are separate concepts |
| Provider agnostic | No `if netflix` in the core — providers are optional adapters |
| Self-hosting first | Single Go binary + PostgreSQL, `docker compose up -d` |
| Payment is an adapter | Phase 1 ships manual settlement only |
| Secrets are optional | No third-party passwords stored by default |

## Documentation

| Document | Description |
|----------|-------------|
| [Project plan](docs/project-plan.md) | Positioning, vision, principles, roadmap |
| [Requirements](docs/requirements.md) | Functional / non-functional requirements, MVP scope and non-goals |
| [Design](docs/design.md) | Domain model, sharing models, billing, architecture, API, deployment |

> 📝 Documentation is currently written in Chinese.

## Roadmap

- **Phase 1 — MVP ✅**: users, providers, products, subscriptions, coteries,
  members, seats, contributions, invitations, notifications (manual settlement)
- **Phase 2 ✅**: marketplace, provider catalog, payment adapters, email,
  webhooks, usage tracking, quotas
- **Phase 3 ✅**: provider/payment/usage plugins, automation (auto-billing
  rollover), advanced billing (prorated split), disputes, reputation
- **Phase 4 ✅**: audit log, owner reputation badge in the marketplace,
  rate limiting, custom billing cycles, per-member usage limits,
  Web Push notifications, resource sharing mode, the Stripe payment
  channel (D24), and the marketplace payment gate (D25)

## Tech stack

Go (stdlib `net/http`, GORM) · PostgreSQL (hand-written SQL migrations,
applied by golang-migrate on startup) · React + TypeScript (Vite, Tailwind
CSS, shadcn/ui, TanStack Query / Router) — see the [design
doc](docs/design.md) for details.

## Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md). Please also read our
[Code of Conduct](CODE_OF_CONDUCT.md).

## Security

See [SECURITY.md](SECURITY.md).

## License

[Apache License 2.0](LICENSE)
