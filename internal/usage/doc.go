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
// post-write ledger sum would pass the cap. Corrections reduce the
// sum, so they always pass when the resulting total fits.
package usage
