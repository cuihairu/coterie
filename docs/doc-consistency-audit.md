# Documentation-code consistency audit (2026-10-13)

> Run per the *Documentation–Source Consistency Audit Spec*: three kinds of divergence (ahead / lagging / mismatched), each verified with file:line evidence, fixed with code as the source of truth.
> Scope: README.md / README.zh.md / docs/index.md / docs/api.md / docs/deployment.md / docs/design.md / docs/requirements.md / docs/project-plan.md / .env.example / deployments/docker-compose.yml / repo About and topics.

## Three-bucket counts

- Docs ahead of code: **0**
- Code ahead of docs: **1** (P2)
- Descriptions mismatched: **5** (P1×2, P2×3)

## Audit table

| ID | Page | Claim | Source evidence (file:line) | Category | Severity | Fix |
|---|---|---|---|---|---|---|
| D1 | docs/deployment.md:7 | Requires Go 1.25+ | go.mod:3 `go 1.26.0` (toolchain go1.27.2) | mismatched | P1 | Change to 1.26+ |
| D2 | docs/index.md:45 | Requires Go 1.25+ | Same as D1 | mismatched | P1 | Change to 1.26+ |
| D3 | docs/index.md:28 | Equal or per-usage splitting | internal/billing/model.go:28-32 five modes (equal/per_seat/fixed/usage/prorated) | mismatched | P2 | Change to five split modes (D9) |
| D4 | docs/index.md:41 | API shape and data model are still being advanced per the project plan | docs/project-plan.md:257 Phase 3 done; 269-309 Phase 4 all [x] | mismatched | P2 | Align the progress description with the plan |
| D5 | docs/index.md:65 | Stray ``` fence (renders as a literal) | No corresponding source; markdown structure error | mismatched | P2 | Delete |
| D6 | (no doc) | A route-consistency guard exists | internal/app/routes_consistency_test.go:1 (mux registration ↔ docs/api.md bidirectional diff) | code ahead | P2 | Add a CI gate note to deployment.md |

## Verified consistent (samples with evidence)

- 30-day session lifetime: docs/index.md:56 ↔ internal/auth/service.go:19 `sessionTTL = 30 * 24 * time.Hour`
- Channel enable conditions: docs/deployment.md:54-62 ↔ internal/notification/email.go:38 (Host and From non-empty), webhook.go:41 (URL non-empty), webpush.go:229 (store and keys non-empty), apps/server/main.go:128,155 (StripeSecretKey non-empty)
- All 25 config variables aligned: .env.example ↔ internal/config/config.go (extracted via six helpers envOr/envList/envInt/envBool/envLevel/envDuration, zero diff)
- 84 API surface entries: 84 rows in docs/api.md ↔ internal/** mux registrations (including 6 PATCH + root /healthz), enforced by the internal/app/routes_consistency_test.go gate
- nightly double tag: docs/deployment.md:22 ↔ .github/workflows/nightly.yml:29,51 (nightly + nightly-YYYYMMDD)
- Repo About/topics: the gh repo view description and 14 topics match the implementation surface, no stale wording
- Three role tiers: docs/index.md:24 ↔ internal/user model (owner/admin/member)

## Phase 1/2 completeness review (2026-10-14)

> Confirms Phase 1/2 have no unimplemented items outside the project plan. Evidence: FR table (§4) + the seven NFRs (§5) + the ten-step journey in §3.

### Phase 1 (15 FRs)

| FR | Requirement | Code evidence | Status |
|---|---|---|---|
| FR-1 | Users and auth | internal/auth + internal/user | ✅ |
| FR-2 | Provider | internal/provider | ✅ |
| FR-3 | Product | internal/product | ✅ |
| FR-4 | Subscription | internal/subscription | ✅ |
| FR-5 | Coterie and lifecycle | internal/coterie | ✅ |
| FR-6 | Sharing modes (Account/Seat/Family) | internal/seat + subscription.sharing_policy | ✅ |
| FR-7 | Member | internal/coterie (members) | ✅ |
| FR-8 | Seat | internal/seat | ✅ |
| FR-9 | Cost splitting and settlement (Manual) | internal/billing + internal/payment | ✅ |
| FR-10 | Sharing Policy (basic limits) | internal/subscription | ✅ |
| FR-11 | Invitation | internal/coterie (invitations) | ✅ |
| FR-12 | Notification (Web + Email) | internal/notification | ✅ |
| FR-13 | Dashboard | web client | ✅ |
| FR-14 | Admin (basic) | internal/user (admin) + internal/report | ✅ |
| FR-15 | Audit Log | internal/audit | ✅ |

### Phase 2 (5 sub-requirements)

| Sub-requirement | Code evidence | Status |
|---|---|---|
| FR-2 catalog public read + seed registry | migration 0006 + public reads | ✅ |
| FR-6 Quota usage accounting | internal/usage | ✅ |
| FR-9 Adapter interface + payments ledger | internal/payment | ✅ |
| FR-12 Email SMTP + Webhook + Push | internal/notification + internal/push | ✅ |
| FR-16 Marketplace (read-only directory + JoinRequest) | internal/marketplace | ✅ |

### §3 ten-step journey

All 10 steps are completable through Web + API, covered end to end by scripts/smoke.sh.

### Seven NFRs (Phase 1/2 portion)

| NFR | Phase 1/2 requirement | Status |
|---|---|---|
| NFR-1 | No Provider passwords stored | ✅ |
| NFR-2 | UI compliance notice | ✅ (c42aac0, vitest 35/35) |
| NFR-3 | docker compose one-click start | ✅ |
| NFR-4 | API-first | ✅ |
| NFR-5 | Provider Agnostic + Plugin architecture | ✅ |
| NFR-6 | Single Tenant / Self-hosted First | ✅ (constraint, non-functional) |
| NFR-7 | Rate Limit + Report + Block + Audit Log | ✅ |

### Conclusion

Phase 1/2 have no unimplemented items outside the project plan. Every FR/NFR/journey entry has a corresponding code implementation, and is tracked in the project-plan Phase 1/2 sections.
