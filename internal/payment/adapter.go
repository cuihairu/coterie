package payment

import (
	"context"
)

// Charge is one payment request handed to an adapter. Amount and
// currency always come from the contribution being settled (invariant
// 5) so an adapter can never invent a different price.
type Charge struct {
	ContributionID string
	PayerUserID    string
	Amount         string
	Currency       string
}

// Receipt is the adapter's outcome. ExternalRef is whatever the channel
// uses to identify the transaction elsewhere; the manual adapter has
// none (the caller's free-text reference is stored instead).
type Receipt struct {
	ExternalRef string
}

// Adapter is a payment channel (design §4.2). Adapters only turn a
// receivable into a received — they never drive the domain model: the
// contribution status flip is the billing side's job, done here in the
// payment service's transaction.
type Adapter interface {
	Name() string
	Charge(ctx context.Context, charge Charge) (Receipt, error)
}

// Manual is the default adapter: the payer settled out of band (cash,
// transfer, whatever the group agreed on) and someone records that it
// happened. It always succeeds — recording is the whole operation.
type Manual struct{}

// Name identifies the adapter in payments.method.
func (Manual) Name() string { return "manual" }

// Charge succeeds immediately, echoing no external reference.
func (Manual) Charge(_ context.Context, _ Charge) (Receipt, error) {
	return Receipt{}, nil
}
