package payment_test

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/cuihairu/coterie/internal/payment"
)

const testWebhookSecret = "whsec_test"

// newFakeStripe starts an httptest server speaking just enough of the
// Stripe API for the adapter: POST /v1/payment_intents echoes an intent
// and records each call's auth header and decoded form for assertions.
func newFakeStripe(t *testing.T, status int, body string) (*httptest.Server, *[]capturedCall) {
	t.Helper()
	var calls []capturedCall
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		form, err := url.ParseQuery(string(raw))
		calls = append(calls, capturedCall{Auth: r.Header.Get("Authorization"), Form: form, FormErr: err})
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_, _ = io.WriteString(w, body)
	}))
	t.Cleanup(srv.Close)
	return srv, &calls
}

// capturedCall is one API call the fake saw, captured while the handler
// was still alive.
type capturedCall struct {
	Auth    string
	Form    url.Values
	FormErr error
}

// lastCall returns the n-th (0-based) captured call.
func lastCall(t *testing.T, calls *[]capturedCall, n int) capturedCall {
	t.Helper()
	if n >= len(*calls) {
		t.Fatalf("api calls = %d, want more than %d", len(*calls), n)
	}
	c := (*calls)[n]
	if c.FormErr != nil {
		t.Fatalf("decode form: %v", c.FormErr)
	}
	return c
}

func TestStripeChargeAsync(t *testing.T) {
	srv, last := newFakeStripe(t, http.StatusOK,
		`{"id":"pi_123","object":"payment_intent","client_secret":"pi_123_secret"}`)
	adapter := payment.Stripe{SecretKey: "sk_test_x", WebhookSecret: testWebhookSecret, APIBase: srv.URL}

	pending, err := adapter.ChargeAsync(context.Background(), payment.Charge{
		ContributionID: "c-1", PayerUserID: "u-1", Amount: "12.99", Currency: "USD",
	})
	if err != nil {
		t.Fatalf("ChargeAsync: %v", err)
	}
	if pending.ExternalRef != "pi_123" || pending.ClientSecret != "pi_123_secret" {
		t.Fatalf("pending = %+v", pending)
	}
	call := lastCall(t, last, 0)
	if call.Auth != "Bearer sk_test_x" {
		t.Fatalf("Authorization = %q", call.Auth)
	}
	form := call.Form
	if form.Get("amount") != "1299" || form.Get("currency") != "usd" {
		t.Fatalf("amount/currency = %v", form)
	}
	if form.Get("metadata[contribution_id]") != "c-1" || form.Get("metadata[payer_user_id]") != "u-1" {
		t.Fatalf("metadata = %v", form)
	}

	// Zero-decimal currencies charge their integer part untouched.
	if _, err := adapter.ChargeAsync(context.Background(), payment.Charge{
		ContributionID: "c-2", Amount: "1200", Currency: "JPY",
	}); err != nil {
		t.Fatalf("ChargeAsync JPY: %v", err)
	}
	if form := lastCall(t, last, 1).Form; form.Get("amount") != "1200" || form.Get("currency") != "jpy" {
		t.Fatalf("JPY amount = %v", form)
	}
}

func TestStripeChargeAsyncErrors(t *testing.T) {
	srv, last := newFakeStripe(t, http.StatusPaymentRequired,
		`{"error":{"message":"Card was declined."}}`)
	adapter := payment.Stripe{SecretKey: "sk_test_x", WebhookSecret: testWebhookSecret, APIBase: srv.URL}

	// The channel's error message is surfaced verbatim.
	_, err := adapter.ChargeAsync(context.Background(), payment.Charge{
		ContributionID: "c-1", Amount: "12.99", Currency: "USD",
	})
	if err == nil || !strings.Contains(err.Error(), "Card was declined.") {
		t.Fatalf("err = %v, want the stripe message", err)
	}

	// A malformed amount is rejected before any call is made.
	_, err = adapter.ChargeAsync(context.Background(), payment.Charge{
		ContributionID: "c-1", Amount: "twelve", Currency: "USD",
	})
	if err == nil {
		t.Fatal("malformed amount accepted")
	}
	if len(*last) != 1 {
		t.Fatalf("api calls = %d, malformed amount must not reach the channel", len(*last))
	}

}

// signStripe produces a Stripe-Signature header value for the payload.
func signStripe(t *testing.T, secret string, ts time.Time, payload []byte) string {
	t.Helper()
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(strconv.FormatInt(ts.Unix(), 10)))
	mac.Write([]byte("."))
	mac.Write(payload)
	return "t=" + strconv.FormatInt(ts.Unix(), 10) + ",v1=" + hex.EncodeToString(mac.Sum(nil))
}

func TestStripeParseWebhook(t *testing.T) {
	adapter := payment.Stripe{SecretKey: "sk_test_x", WebhookSecret: testWebhookSecret}
	succeeded := []byte(`{"id":"evt_1","type":"payment_intent.succeeded","data":{"object":{"id":"pi_123","object":"payment_intent"}}}`)

	event, ok, err := adapter.ParseWebhook(http.Header{"Stripe-Signature": {signStripe(t, testWebhookSecret, time.Now(), succeeded)}}, succeeded)
	if err != nil || !ok {
		t.Fatalf("succeeded: ok = %v err = %v", ok, err)
	}
	if event.ExternalRef != "pi_123" || !event.Succeeded {
		t.Fatalf("event = %+v", event)
	}

	failed := []byte(`{"id":"evt_2","type":"payment_intent.payment_failed","data":{"object":{"id":"pi_9","object":"payment_intent"}}}`)
	event, ok, err = adapter.ParseWebhook(http.Header{"Stripe-Signature": {signStripe(t, testWebhookSecret, time.Now(), failed)}}, failed)
	if err != nil || !ok {
		t.Fatalf("failed: ok = %v err = %v", ok, err)
	}
	if event.ExternalRef != "pi_9" || event.Succeeded {
		t.Fatalf("event = %+v", event)
	}

	// Uninteresting event kinds are ignored, not errors.
	other := []byte(`{"id":"evt_3","type":"charge.refunded","data":{"object":{"id":"ch_1"}}}`)
	if _, ok, err := adapter.ParseWebhook(http.Header{"Stripe-Signature": {signStripe(t, testWebhookSecret, time.Now(), other)}}, other); ok || err != nil {
		t.Fatalf("other: ok = %v err = %v, want ignore", ok, err)
	}

	// A tampered payload fails the signature check.
	if _, _, err := adapter.ParseWebhook(http.Header{"Stripe-Signature": {signStripe(t, testWebhookSecret, time.Now(), succeeded)}}, []byte(`{"id":"evt_1","type":"payment_intent.succeeded","data":{"object":{"id":"pi_hack"}}}`)); err == nil {
		t.Fatal("tampered payload accepted")
	}

	// A wrong secret fails too.
	if _, _, err := adapter.ParseWebhook(http.Header{"Stripe-Signature": {signStripe(t, "whsec_other", time.Now(), succeeded)}}, succeeded); err == nil {
		t.Fatal("wrong-secret signature accepted")
	}

	// A stale timestamp is outside the tolerance window.
	stale := time.Now().Add(-6 * time.Minute)
	if _, _, err := adapter.ParseWebhook(http.Header{"Stripe-Signature": {signStripe(t, testWebhookSecret, stale, succeeded)}}, succeeded); err == nil {
		t.Fatal("stale signature accepted")
	}

	// Missing or malformed headers are rejected.
	for _, sig := range []string{"", "v1=deadbeef", "t=notanumber,v1=00"} {
		if _, _, err := adapter.ParseWebhook(http.Header{"Stripe-Signature": {sig}}, succeeded); err == nil {
			t.Fatalf("signature %q accepted", sig)
		}
	}

	// A signature over non-JSON bytes fails at the payload stage.
	if _, _, err := adapter.ParseWebhook(http.Header{"Stripe-Signature": {signStripe(t, testWebhookSecret, time.Now(), []byte("nope"))}}, []byte("nope")); err == nil {
		t.Fatal("non-JSON payload accepted")
	}
}

// The envelope decode only needs the fields we read; a full Stripe
// event parses fine.
func TestStripeWebhookEnvelopeShape(t *testing.T) {
	var payload map[string]any
	raw, _ := json.Marshal(map[string]any{
		"id":   "evt_full",
		"type": "payment_intent.succeeded",
		"data": map[string]any{"object": map[string]any{
			"id": "pi_full", "object": "payment_intent",
			"amount": 1299, "currency": "usd", "status": "succeeded",
		}},
	})
	if err := json.Unmarshal(raw, &payload); err != nil {
		t.Fatal(err)
	}
	adapter := payment.Stripe{WebhookSecret: testWebhookSecret}
	event, ok, err := adapter.ParseWebhook(
		http.Header{"Stripe-Signature": {signStripe(t, testWebhookSecret, time.Now(), raw)}}, raw)
	if err != nil || !ok || event.ExternalRef != "pi_full" || !event.Succeeded {
		t.Fatalf("full event: event = %+v ok = %v err = %v", event, ok, err)
	}
}
