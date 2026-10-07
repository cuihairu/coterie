// Package billing generates billing periods and orchestrates settlement.
// Phase 1 is manual settlement; payment providers plug in as adapters
// (design principle: payment is an adapter, not the core).
package billing
