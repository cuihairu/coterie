# Deployment

Coterie is a Go binary plus a PostgreSQL (15+) instance. Migrations are applied automatically when the service starts; there is no separate migration step, so an upgrade means swapping in a new binary and restarting.

## Running from Source

Requires Go 1.26+.

```bash
# Configuration (all variables in .env.example)
export DATABASE_URL=postgres://coterie:coterie@localhost:5432/coterie?sslmode=disable
export PORT=8080

# Start
go run ./apps/server
```

`GET /healthz` returning 200 means it is ready. To run a full-journey smoke test against a running service: `scripts/smoke.sh http://localhost:8080`.

## Docker Compose

The repo ships with `deployments/docker-compose.yml`, containing only two services: postgres and coterie. The image is built by the nightly job and published to `ghcr.io/cuihairu/coterie:nightly` (plus a same-day date tag).

```bash
docker compose -f deployments/docker-compose.yml up -d
```

To build the image from source: `docker build -t ghcr.io/cuihairu/coterie:nightly .` — compose will then use the local image of the same name directly.

postgres has a healthcheck, and coterie waits for it to become ready before starting; if the server cannot reach the database it exits immediately, and compose's restart policy brings it back up. The 5432 and 8080 in compose are host ports — if they are occupied, change the mapping; do not touch the ports inside the containers.

## Configuration

Everything is read from environment variables; defaults live in `internal/config`, and a commented listing is in `.env.example`. When deploying with compose, the variables below are passed through into the container with an empty default (`${VAR:-}`): drop a `.env` in the same directory as the compose file, or export the variables directly, to enable them; an empty value is the same as unset, and each default stays in force. `DATABASE_URL` and `PORT` are fixed and managed by compose.

CI also has a route-consistency guard (`internal/app/routes_consistency_test.go`): it takes a bidirectional diff between the endpoint table in `docs/api.md` and the mux registrations under `internal/**` (including the root `/healthz` and PATCH); the test goes red if either side adds or removes a route out of sync.

Core items:

| Variable | Default | Description |
|---|---|---|
| `PORT` | 8080 | Listen port |
| `DATABASE_URL` | `postgres://coterie:coterie@localhost:5432/coterie?sslmode=disable` | PostgreSQL connection string |
| `MIGRATE_ON_START` | true | Apply migrations at startup |
| `MIGRATIONS_DIR` | `migrations` | Migrations directory (inside the image it is `/migrations`) |
| `LOG_LEVEL` | INFO | DEBUG / INFO / WARN / ERROR |
| `AUTO_BILLING_INTERVAL` | 1h | Billing-cycle roll scheduling interval; 0 disables it |
| `RATE_LIMIT_REGISTER_PER_MIN` | 5 | Registration rate limit, per address per minute |
| `RATE_LIMIT_LOGIN_PER_MIN` | 10 | Login rate limit, per address per minute |
| `ADMIN_EMAILS` | empty | Comma-separated emails, promoted to platform admin at startup |

Notification and payment channels; if you don't configure them, they are not enabled:

| Variable | Enabled when | Description |
|---|---|---|
| `SMTP_HOST` `SMTP_PORT` `SMTP_USERNAME` `SMTP_PASSWORD` `SMTP_FROM` | Both `SMTP_HOST` and `SMTP_FROM` are set | Email channel |
| `WEBHOOK_URL` `WEBHOOK_SECRET` | `WEBHOOK_URL` is set | Outbound Webhook, body signed with HMAC-SHA256 |
| `VAPID_PUBLIC_KEY` `VAPID_PRIVATE_KEY` `VAPID_SUBJECT` | All three are set | Web Push; generate keys with `go run ./cmd/vapidkeygen` |
| `PAYMENT_METHODS` | Comma-separated | Registers channels on top of the built-in manual; `sandbox` is a built-in demo channel |
| `STRIPE_SECRET_KEY` | Enables stripe when set | Must also be added to `PAYMENT_METHODS` |
| `STRIPE_WEBHOOK_SECRET` | Stripe signature verification | Public endpoint `POST /api/v1/payments/webhooks/stripe` |
| `STRIPE_MARKETPLACE_WEBHOOK_SECRET` | Optional | Separate secret for the join-gate Webhook; falls back to `STRIPE_WEBHOOK_SECRET` when unset |
| `STRIPE_API_BASE` | Optional | Overrides the Stripe API endpoint; defaults to `https://api.stripe.com` (for testing/proxies) |
| `PROVIDER_PLUGINS` | Comma-separated | `claude` is a built-in sample plugin adding region/plan policy and usage limits |

## Platform Admins

Emails listed in `ADMIN_EMAILS` are promoted to the admin role at startup; if the account does not exist yet, registering and then starting again also promotes it, and repeated startups do not process it twice. Admins only handle reports (the inbox and resolve/dismiss); they do not manage anyone's circles.

## Upgrading

Pull the new code, rebuild the image, and restart the service. Migrations only roll forward; rolling back the image does not roll back the schema. Every migration ships with a down file, to be run manually when needed.
