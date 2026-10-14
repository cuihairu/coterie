// Package usage implements Phase 2 usage tracking: an append-only
// ledger of metered consumption per member of a subscription's coterie.
//
// Concepts (design D9):
//
//   - UsageRecord is the source of truth. Rows are immutable;
//     corrections are negative records.
//   - A record is attributed to a member (required) and optionally to
//     one of the member's occupied seats. Seat attribution decides
//     which quota bucket the usage hits.
//   - seat.metadata.used is a projection: when the attributed seat
//     carries a "quota" key, writing a record recomputes used as the
//     SUM of all records attributed to that seat under a row lock —
//     never an incremental float accumulation.
//
// The billing module consumes the same ledger for mode=usage splits
// over the period window.
//
// The sharing policy may carry a usage_limit (design D21): per member,
// per billing period, per unit, writes are rejected with 409 once the
// post-write ledger sum would pass the cap. quota_limit (design D30,
// FR-10) applies the same judgment subscription-wide — every member's
// rows pool into one per-period budget. Corrections reduce the sums,
// so they always pass when the resulting totals fit.
//
// The seat's own allowance is enforced too (design D4): when the
// attributed seat's metadata carries a numeric quota, the seat's
// post-write ledger sum must stay within it (409), with a metadata
// unit narrowing both the cap and the sum to that unit. A malformed
// quota value stays informational — it never blocks a write.
package usage
