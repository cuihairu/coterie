package payment

import (
	"context"

	"github.com/google/uuid"
)

// Sandbox is an offline demo channel: it "processes" the charge
// instantly and deterministically, returning a synthetic external
// reference. It exists to exercise the plugin seam end to end (and for
// demos) without external networks or credentials — the template real
// channels (Stripe, PayPal, …) follow when they arrive.
type Sandbox struct{}

// Name identifies the adapter in payments.method.
func (Sandbox) Name() string { return "sandbox" }

// Charge always succeeds with a fresh synthetic reference.
func (Sandbox) Charge(_ context.Context, _ Charge) (Receipt, error) {
	return Receipt{ExternalRef: "sbx_" + uuid.NewString()}, nil
}
