// Package reputation derives account reputation (Phase 3) from the
// contributions ledger: how a user settles the shares they owe as a
// circle member. It is read-only and entirely derived — no manual
// scores, nothing to game; every number here is a projection of the
// append-only billing history.
package reputation

// CurrencyStats is one currency's settled/unsettled totals.
type CurrencyStats struct {
	Currency string `json:"currency"`
	Paid     string `json:"paid"`
	Pending  string `json:"pending"`
}

// Report is a user's settlement reputation.
type Report struct {
	UserID        string             `json:"user_id"`
	Contributions ContributionCounts `json:"contributions"`
	Currencies    []CurrencyStats    `json:"currencies"`
	PaymentRatio  *string            `json:"payment_ratio"`
}

// ContributionCounts buckets the user's contribution rows by status.
type ContributionCounts struct {
	Paid      int64 `json:"paid"`
	Pending   int64 `json:"pending"`
	Waived    int64 `json:"waived"`
	Cancelled int64 `json:"cancelled"`
}
