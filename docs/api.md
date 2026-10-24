# API reference (v1)

All endpoints are mounted under `/api/v1` and speak JSON. This is a flat list of the current implementation; semantics and field details are in [design §9](/design). The list tracks the implementation; once a server runs locally, `scripts/smoke.sh http://localhost:8080` smoke-tests the whole journey.

General conventions:

- **Auth**: every endpoint except those marked "public" requires `Authorization: Bearer <token>`. Tokens are issued on register/login, valid for 30 days, and invalidated on logout.
- **List pagination**: `?limit=` (default 20, max 100) and `?offset=`; responses are `{"items": [...], "meta": {"total", "limit", "offset"}}`.
- **List filtering**: filterable endpoints accept the following query parameters, combinable with `limit`/`offset`:
  - `GET /providers?category=` · `GET /products?provider_id=` · `GET /marketplace/coteries?product_id=`
  - `GET /subscriptions?owner_user_id=&product_id=` — results are always limited to subscriptions you own; `owner_user_id` only accepts yourself, passing someone else yields 403
  - `GET /coteries?subscription_id=` — only returns coteries you are a member of
  - `GET /notifications?unread=true`
  - `GET /subscriptions/{id}/disputes?status=` · `GET /coteries/{id}/join-requests?status=` · `GET /admin/reports?status=`
  - `GET /subscriptions/{id}/usage-records?member_id=&unit=&from=&to=` (`from`/`to` are `YYYY-MM-DD`)
  - `GET /subscriptions/{id}/audit-logs?action=` · `GET /coteries/{id}/audit-logs?action=` · `GET /admin/audit-logs?action=&actor_id=&coterie_id=&subscription_id=`
- **Error envelope**: non-2xx returns `{"error": {"code", "message", "details"}}`; missing/invalid auth 401, no permission 403, semantic error 422, state conflict 409.

Auth labels: **public** = readable anonymously; **Bearer** = any authenticated user; **Owner** = the Owner of the subscription the resource belongs to; **admin** = platform administrator (bootstrapped via `ADMIN_EMAILS`). Fine-grained rules (e.g. whether a member can read) are in the design doc.

## Health

| Method | Path | Auth | Description |
|---|---|---|---|
| GET | `/healthz` | public | Readiness probe |

## Auth

| Method | Path | Auth | Description |
|---|---|---|---|
| POST | `/auth/register` | public | Register and issue a token |
| POST | `/auth/login` | public | Log in with email + password |
| GET | `/auth/me` | Bearer | Current user |
| POST | `/auth/logout` | Bearer | Log out the current session |

## Users and reputation

| Method | Path | Auth | Description |
|---|---|---|---|
| GET | `/users` | Bearer | User list (display projection: id / username / avatar; email/role etc. are not exposed — see D29) |
| POST | `/users` | Bearer | Create a user (echoes the create payload) |
| GET | `/users/{id}` | Bearer | User detail (same projection; your own full fields are on `/auth/me`) |
| GET | `/users/{id}/reputation` | Bearer | Settlement reputation (read-only projection) |

## Catalog: Provider and Product

Reads are public; writes are limited to catalog maintainers (Owner or creator).

| Method | Path | Auth | Description |
|---|---|---|---|
| GET | `/providers` | public | Catalog list, supports filtering |
| POST | `/providers` | Bearer | Create |
| GET | `/providers/{id}` | public | Detail |
| PATCH | `/providers/{id}` | Bearer | Update |
| DELETE | `/providers/{id}` | Bearer | Delete |
| GET | `/products` | public | Catalog list |
| POST | `/products` | Bearer | Create |
| GET | `/products/{id}` | public | Detail |
| PATCH | `/products/{id}` | Bearer | Update |
| DELETE | `/products/{id}` | Bearer | Delete |

## Subscriptions

| Method | Path | Auth | Description |
|---|---|---|---|
| GET | `/subscriptions` | Bearer | Your own subscriptions |
| POST | `/subscriptions` | Bearer | Create (`owner_user_id` must be yourself) |
| GET | `/subscriptions/{id}` | Bearer | Detail |
| PATCH | `/subscriptions/{id}` | Owner | Price/policy/status changes, audit-logged |
| DELETE | `/subscriptions/{id}` | Owner | Delete |

## Seats

| Method | Path | Auth | Description |
|---|---|---|---|
| GET | `/subscriptions/{subID}/seats` | Bearer | Seat list |
| POST | `/subscriptions/{subID}/seats` | Owner | Add seats |
| GET | `/seats/{id}` | Bearer | Seat detail |
| PATCH | `/seats/{id}` | Owner | Rename/metadata/disable-enable (occupied must be released first) |
| POST | `/seats/{id}/assign` | Owner | Assign to a member |
| POST | `/seats/{id}/release` | Owner | Release |

## Billing periods and contributions

| Method | Path | Auth | Description |
|---|---|---|---|
| GET | `/subscriptions/{subID}/billing-periods` | Bearer | Billing period list |
| POST | `/subscriptions/{subID}/billing-periods` | Owner | Create a billing period |
| GET | `/billing-periods/{id}` | Bearer | Billing period detail |
| POST | `/billing-periods/{id}/close` | Owner | Close a billing period, audit-logged |
| GET | `/billing-periods/{id}/contributions` | Bearer | Contribution list |
| POST | `/billing-periods/{id}/contributions/generate` | Owner | Generate equal or per-usage contributions |
| PATCH | `/contributions/{id}` | Owner | Adjust a contribution amount, audit-logged |

## Payments

| Method | Path | Auth | Description |
|---|---|---|---|
| GET | `/payments/methods` | Bearer | Registered channels |
| POST | `/contributions/{id}/payments` | Bearer | Record a payment (manual lands immediately; async channels go to pending) |
| GET | `/contributions/{id}/payments` | Bearer | Payment history |
| GET | `/payments/{id}` | Bearer | Payment detail |
| POST | `/payments/webhooks/{method}` | public + signature | Channel confirmation callback (Stripe etc.), idempotent by external_ref |

## Disputes

| Method | Path | Auth | Description |
|---|---|---|---|
| POST | `/contributions/{id}/disputes` | Bearer | Dispute a contribution |
| GET | `/subscriptions/{id}/disputes` | Bearer | Dispute list |
| GET | `/disputes/{id}` | Bearer | Dispute detail |
| POST | `/disputes/{id}/decide` | Owner | Decide (one-way) |

## Coteries and members

| Method | Path | Auth | Description |
|---|---|---|---|
| POST | `/coteries` | Bearer | Create a coterie around a subscription; seats are placed per capacity |
| GET | `/coteries` | Bearer | Coteries you take part in |
| GET | `/coteries/{id}` | Bearer | Coterie detail (non-members only get a restricted view) |
| PATCH | `/coteries/{id}` | Owner | Name/status transitions/listing, audit-logged |
| GET | `/coteries/{id}/members` | Bearer | Member list |
| DELETE | `/members/{id}` | Owner | Remove a member and release the seat |
| POST | `/coteries/{id}/leave` | Bearer | A member leaves on their own (the Owner cannot) |
| POST | `/coteries/{id}/invitations` | Owner | Send an invitation (one-time token) |
| GET | `/coteries/{id}/invitations` | Owner | Invitation list |
| POST | `/invitations/accept` | Bearer | Accept an invitation with a token |

## Marketplace and join requests

| Method | Path | Auth | Description |
|---|---|---|---|
| GET | `/marketplace/coteries` | public | Public directory (with an Owner reputation badge) |
| POST | `/coteries/{coterieID}/join-requests` | Bearer | Request to join (403 if blocked) |
| GET | `/coteries/{coterieID}/join-requests` | Owner | Inbox |
| POST | `/join-requests/{id}/accept` | Owner | Accept and assign a seat |
| POST | `/join-requests/{id}/decline` | Owner | Decline |
| DELETE | `/join-requests/{id}` | Bearer | The requester withdraws |
| GET | `/me/join-requests` | Bearer | Your own request list |
| POST | `/join-requests/{id}/payments` | Bearer | Pay the admission fee (when the payment gate is on) |
| POST | `/marketplace/webhooks/{method}` | public + signature | Gate confirmation callback; once confirmed, the member joins |

## Abuse control

| Method | Path | Auth | Description |
|---|---|---|---|
| PUT | `/coteries/{id}/blocks/{userID}` | Owner | Block a user |
| DELETE | `/coteries/{id}/blocks/{userID}` | Owner | Unblock a user |
| GET | `/coteries/{id}/blocks` | Owner | Blocklist |
| POST | `/coteries/{id}/report` | Bearer | Report a publicly listed coterie |
| GET | `/admin/reports` | admin | Report inbox, filter with `?status=` |
| POST | `/admin/reports/{id}/resolve` | admin | Uphold as valid (note optional) |
| POST | `/admin/reports/{id}/dismiss` | admin | Dismiss (note optional) |

## Audit

| Method | Path | Auth | Description |
|---|---|---|---|
| GET | `/subscriptions/{id}/audit-logs` | Owner | Subscription ledger, filter with `?action=` |
| GET | `/coteries/{id}/audit-logs` | Owner | Coterie ledger, filter with `?action=` |
| GET | `/admin/audit-logs` | admin | Full ledger, filters `?action=` `?actor_id=` `?coterie_id=` `?subscription_id=` |

## Usage

| Method | Path | Auth | Description |
|---|---|---|---|
| GET | `/subscriptions/{subID}/usage-records` | Bearer | Usage list |
| POST | `/subscriptions/{subID}/usage-records` | Bearer | Record one usage entry (a plugin may re-verify) |
| GET | `/usage-records/{id}` | Bearer | Usage detail |

## Notifications

| Method | Path | Auth | Description |
|---|---|---|---|
| GET | `/notifications` | Bearer | Your own inbox |
| POST | `/notifications/{id}/read` | Bearer | Mark as read |
| GET | `/push/subscriptions` | Bearer | Registered push endpoints |
| POST | `/push/subscriptions` | Bearer | Register a Web Push endpoint |
| DELETE | `/push/subscriptions/{id}` | Bearer | Unregister a push endpoint |
| GET | `/push/vapid-public-key` | Bearer | Server VAPID public key (for browser push subscription; 404 when push is not configured) |
