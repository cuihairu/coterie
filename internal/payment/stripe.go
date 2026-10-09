package payment

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/cuihairu/coterie/pkg/api"
)

// Stripe talks to the Stripe REST API over plain HTTP form posts (no
// stripe-go dependency). It implements AsyncAdapter: ChargeAsync
// creates a PaymentIntent and returns its client_secret for the
// client-side confirm; ParseWebhook verifies Stripe-Signature (HMAC-
// SHA256 over "t.payload", ±5min tolerance, D24) and reduces relevant
// payment_intent events to a WebhookEvent.
type Stripe struct {
	SecretKey     string // sk_test_…/sk_live_…, sent as the basic-auth password
	WebhookSecret string // whsec_…, key for the Stripe-Signature HMAC
	APIBase       string // default https://api.stripe.com; tests point at a fake
	Client        *http.Client
}

// Name implements Adapter.
func (Stripe) Name() string { return "stripe" }

// Charge satisfies Adapter but is never reachable through Record —
// the service takes the AsyncAdapter branch first. It errors in case
// a future caller forgets.
func (Stripe) Charge(context.Context, Charge) (Receipt, error) {
	return Receipt{}, api.Validation("invalid payment",
		api.Detail{Field: "method", Message: "stripe settles asynchronously and cannot be recorded synchronously"})
}

// zeroDecimalCurrencies are Stripe currencies whose smallest unit is
// the currency itself (no cents).
var zeroDecimalCurrencies = map[string]bool{
	"bif": true, "clp": true, "djf": true, "gnf": true, "jpy": true,
	"kmf": true, "krw": true, "mga": true, "pyg": true, "rwf": true,
	"ugx": true, "vnd": true, "vuv": true, "xaf": true, "xof": true, "xpf": true,
}

// stripeMinorUnits converts an amount string to Stripe minor units.
// Amounts are decimal strings in the ledger; zero-decimal currencies
// take the integer part as-is, others scale by 100.
func stripeMinorUnits(amount, currency string) (int64, error) {
	neg := strings.HasPrefix(amount, "-")
	if neg {
		amount = amount[1:]
	}
	intPart, fracPart, _ := strings.Cut(amount, ".")
	units, err := strconv.ParseInt(intPart, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid amount %q: %w", amount, err)
	}
	scale := int64(100)
	fracDigits := 2
	if zeroDecimalCurrencies[strings.ToLower(currency)] {
		scale, fracDigits = 1, 0
	}
	// The first fracDigits fraction digits become the minor part;
	// anything beyond them is sub-cent precision the ledger never
	// carries, and missing digits pad with zeros.
	frac := 0
	for i := 0; i < fracDigits; i++ {
		frac *= 10
		if i < len(fracPart) {
			frac += int(fracPart[i] - '0')
		}
	}
	if units > (1<<62)/scale {
		return 0, fmt.Errorf("amount %q overflows Stripe minor units", amount)
	}
	units = units*scale + int64(frac)
	if neg {
		units = -units
	}
	return units, nil
}

// ChargeAsync creates a PaymentIntent for the receivable and returns
// the intent id (external reference) and client_secret the payer's
// client confirms with.
func (s Stripe) ChargeAsync(ctx context.Context, charge Charge) (Pending, error) {
	amount, err := stripeMinorUnits(charge.Amount, charge.Currency)
	if err != nil {
		return Pending{}, api.Validation("invalid payment",
			api.Detail{Field: "amount", Message: err.Error()})
	}
	form := url.Values{}
	form.Set("amount", strconv.FormatInt(amount, 10))
	form.Set("currency", strings.ToLower(charge.Currency))
	form.Set("metadata[contribution_id]", charge.ContributionID)
	form.Set("metadata[payer_user_id]", charge.PayerUserID)
	form.Set("description", fmt.Sprintf("Coterie contribution %s", charge.ContributionID))

	intent, err := s.apiCall(ctx, http.MethodPost, "/v1/payment_intents", form)
	if err != nil {
		return Pending{}, err
	}
	return Pending{
		Receipt:      Receipt{ExternalRef: intent.ID},
		ClientSecret: intent.ClientSecret,
	}, nil
}

// stripeIntent is the slice of the PaymentIntent object the adapter
// consumes.
type stripeIntent struct {
	ID           string `json:"id"`
	ClientSecret string `json:"client_secret"`
}

// apiCall posts the form to the Stripe API (or its stand-in) with the
// secret key as the basic-auth password, decoding the object envelope.
func (s Stripe) apiCall(ctx context.Context, method, path string, form url.Values) (stripeIntent, error) {
	base := s.APIBase
	if base == "" {
		base = "https://api.stripe.com"
	}
	client := s.Client
	if client == nil {
		client = http.DefaultClient
	}
	var body io.Reader
	if form != nil {
		body = strings.NewReader(form.Encode())
	}
	req, err := http.NewRequestWithContext(ctx, method, base+path, body)
	if err != nil {
		return stripeIntent{}, err
	}
	req.Header.Set("Authorization", "Bearer "+s.SecretKey)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Stripe-Version", "2024-06-20")
	res, err := client.Do(req)
	if err != nil {
		return stripeIntent{}, err
	}
	defer res.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(res.Body, 1<<20))
	if err != nil {
		return stripeIntent{}, err
	}
	if res.StatusCode != http.StatusOK {
		var e struct {
			Error struct {
				Message string `json:"message"`
			} `json:"error"`
		}
		if json.Unmarshal(raw, &e) == nil && e.Error.Message != "" {
			return stripeIntent{}, fmt.Errorf("stripe: %s", e.Error.Message)
		}
		return stripeIntent{}, api.Internal()
	}
	var intent stripeIntent
	if err := json.Unmarshal(raw, &intent); err != nil {
		return stripeIntent{}, fmt.Errorf("stripe: decode response: %w", err)
	}
	return intent, nil
}

// webhookTolerance is how far a Stripe-Signature timestamp may drift —
// Stripe's own recommended default.
const webhookTolerance = 5 * time.Minute

// maxWebhookBody bounds the payload read in the webhook handler.
const maxWebhookBody = 1 << 20

// ParseWebhook verifies Stripe-Signature ("t=…,v1=…" scheme, HMAC-SHA256
// of "t.payload" with the webhook secret, constant-time compared) and
// maps payment_intent.succeeded / payment_intent.payment_failed to a
// WebhookEvent. Any other event kind is ok=false with a nil error — the
// caller answers 200 and ignores it.
func (s Stripe) ParseWebhook(header http.Header, payload []byte) (WebhookEvent, bool, error) {
	sig := header.Get("Stripe-Signature")
	t, digest, err := parseStripeSignature(sig)
	if err != nil {
		return WebhookEvent{}, false, api.Validation("invalid webhook",
			api.Detail{Field: "signature", Message: err.Error()})
	}
	if time.Since(t) > webhookTolerance || t.After(time.Now().Add(webhookTolerance)) {
		return WebhookEvent{}, false, api.Validation("invalid webhook",
			api.Detail{Field: "signature", Message: "timestamp outside the tolerance window"})
	}
	if !hmac.Equal(digest, signedPayload([]byte(s.WebhookSecret), t, payload)) {
		return WebhookEvent{}, false, api.Validation("invalid webhook",
			api.Detail{Field: "signature", Message: "signature mismatch"})
	}

	var env struct {
		Type string `json:"type"`
		Data struct {
			Object struct {
				ID     string `json:"id"`
				Object string `json:"object"`
			} `json:"object"`
		} `json:"data"`
	}
	if err := json.Unmarshal(payload, &env); err != nil {
		return WebhookEvent{}, false, api.Validation("invalid webhook",
			api.Detail{Field: "payload", Message: "not valid JSON"})
	}
	switch env.Type {
	case "payment_intent.succeeded":
		return WebhookEvent{ExternalRef: env.Data.Object.ID, Succeeded: true}, true, nil
	case "payment_intent.payment_failed":
		return WebhookEvent{ExternalRef: env.Data.Object.ID}, true, nil
	}
	return WebhookEvent{}, false, nil
}

// signedPayload computes the expected HMAC-SHA256 of "t.payload".
func signedPayload(secret []byte, t time.Time, payload []byte) []byte {
	mac := hmac.New(sha256.New, secret)
	mac.Write([]byte(strconv.FormatInt(t.Unix(), 10)))
	mac.Write([]byte("."))
	mac.Write(payload)
	return mac.Sum(nil)
}

// parseStripeSignature splits the "t=…,v1=…" header into its timestamp
// and the leading v1 hex digest.
func parseStripeSignature(sig string) (time.Time, []byte, error) {
	var (
		tStr   string
		digest []byte
	)
	for _, part := range strings.Split(sig, ",") {
		k, v, _ := strings.Cut(strings.TrimSpace(part), "=")
		switch k {
		case "t":
			tStr = v
		case "v1":
			if digest == nil {
				d, err := hex.DecodeString(v)
				if err != nil {
					return time.Time{}, nil, errors.New("malformed v1 signature")
				}
				digest = d
			}
		}
	}
	if tStr == "" || digest == nil {
		return time.Time{}, nil, errors.New("signature must carry t and v1")
	}
	sec, err := strconv.ParseInt(tStr, 10, 64)
	if err != nil {
		return time.Time{}, nil, errors.New("timestamp must be an integer")
	}
	return time.Unix(sec, 0), digest, nil
}
