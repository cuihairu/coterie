package payment

import (
	"context"
	"net/http"
)

// Charge is one payment request handed to an adapter. Amount and
// currency always come from the contribution being settled (invariant
// 5) so an adapter can never invent a different price.
type Charge struct {
	ContributionID string
	PayerUserID    string
	Amount         string
	Currency       string
	// Description overrides the channel's statement text (the
	// admission-charge path charges before a contribution exists,
	// design D25).
	Description string
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

// Pending is an async channel's answer to a charge start (D24): the
// channel transaction exists but is not confirmed yet; the client
// secret (when the channel uses one) goes to the payer's client only.
type Pending struct {
	Receipt
	ClientSecret string
}

// AsyncAdapter is the optional face of channels that confirm
// asynchronously (Stripe-style). The service records their charges as
// pending rows and lets the channel's webhook drive the outcome; the
// synchronous Manual/Sandbox adapters never implement it and keep the
// charge-inside-the-transaction semantics.
type AsyncAdapter interface {
	Adapter

	// ChargeAsync creates the channel-side transaction. The returned
	// external reference is what the webhook will name later.
	ChargeAsync(ctx context.Context, charge Charge) (Pending, error)

	// ParseWebhook verifies the channel's outbound call (signature or
	// equivalent) and normalizes it to a ledger event. Ignored event
	// kinds return ok=false with a nil error.
	ParseWebhook(header http.Header, payload []byte) (event WebhookEvent, ok bool, err error)
}

// WebhookEvent is a normalized channel confirmation: the external
// reference it speaks about and whether the charge went through.
type WebhookEvent struct {
	ExternalRef string
	Succeeded   bool
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
