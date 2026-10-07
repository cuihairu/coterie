<div align="center">
  <img src="assets/logo.svg" width="120" alt="Coterie logo" />
  <h1>Coterie</h1>
  <p><strong>Open-source platform for shared digital services.</strong></p>
  <p>
    <a href="LICENSE"><img alt="License" src="https://img.shields.io/badge/license-Apache--2.0-blue"></a>
    <img alt="Status" src="https://img.shields.io/badge/status-early%20development-orange">
    <img alt="Go" src="https://img.shields.io/badge/Go-1.25%2B-00ADD8?logo=go&logoColor=white">
    <img alt="Docker" src="https://img.shields.io/badge/docker-compose%20ready-2496ED?logo=docker&logoColor=white">
  </p>
</div>

---

**Organize subscriptions, seats, members, and shared costs in one place.**

Coterie is designed for any digital service that can be legitimately shared
among multiple people — from streaming and music to software, AI, cloud
services and more.

> ⚠️ **Status: early development.** Coterie is being rebuilt around a generic
> domain model (it started as a streaming-account sharing app). The Phase 1 MVP
> is not usable yet — see the [roadmap](#roadmap) and the planning docs below
> to see where things are heading.

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

- **Phase 1 — MVP**: users, providers, products, subscriptions, coteries,
  members, seats, contributions, invitations, notifications (manual settlement)
- **Phase 2**: marketplace, provider catalog, payment adapters, email,
  webhooks, usage tracking, quotas
- **Phase 3**: provider/payment/usage plugins, automation, advanced billing,
  disputes, reputation
- **Phase 4**: open-source marketplace and infrastructure for shared digital
  services

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
