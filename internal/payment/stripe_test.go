package payment_test

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/cuihairu/coterie/internal/payment"
	"github.com/cuihairu/coterie/pkg/api"
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

// The remaining adapter branches: minor-unit edge cases, the default
// description, the synchronous-charge guard, and API responses the
// happy-path tests never see.
func TestStripeChargeAsyncAmountEdges(t *testing.T) {
	srv, last := newFakeStripe(t, http.StatusOK,
		`{"id":"pi_e","client_secret":"s"}`)
	adapter := payment.Stripe{SecretKey: "sk", WebhookSecret: testWebhookSecret, APIBase: srv.URL}
	charge := func(amount, currency string) (string, error) {
		pending, err := adapter.ChargeAsync(context.Background(), payment.Charge{
			ContributionID: "c-e", Amount: amount, Currency: currency,
		})
		return pending.ExternalRef, err
	}

	for _, tc := range []struct {
		name, amount, currency, want string
	}{
		{"no fraction", "12", "USD", "1200"},
		{"one frac digit pads", "12.5", "USD", "1250"},
		{"negative", "-12.99", "USD", "-1299"},
		// Zero-decimal currencies drop the fraction (scale 1, no digits).
		{"zero decimal drops fraction", "12.99", "JPY", "12"},
	} {
		if _, err := charge(tc.amount, tc.currency); err != nil {
			t.Fatalf("%s: %v", tc.name, err)
		}
		if got := lastCall(t, last, len(*last)-1).Form.Get("amount"); got != tc.want {
			t.Fatalf("%s: amount = %q, want %q", tc.name, got, tc.want)
		}
	}

	// A value that parses as int64 but overflows once scaled is
	// rejected before reaching the channel; the detail says why.
	before := len(*last)
	_, err := charge("92233720368547758.08", "USD")
	if detailMessage(t, err) == "" || !strings.Contains(detailMessage(t, err), "overflows") {
		t.Fatalf("overflow: err = %v", err)
	}
	// And one too large even for int64 is a plain invalid amount.
	_, err = charge("99999999999999999999", "USD")
	if detailMessage(t, err) == "" || !strings.Contains(detailMessage(t, err), "invalid amount") {
		t.Fatalf("unparsable: err = %v", err)
	}
	if len(*last) != before {
		t.Fatal("rejected amounts must not reach the channel")
	}
}

func TestStripeChargeAsyncDefaults(t *testing.T) {
	srv, last := newFakeStripe(t, http.StatusOK, `{"id":"pi_d","client_secret":"s"}`)
	adapter := payment.Stripe{SecretKey: "sk", WebhookSecret: testWebhookSecret, APIBase: srv.URL}

	if _, err := adapter.ChargeAsync(context.Background(), payment.Charge{
		ContributionID: "c-d", Amount: "5.00", Currency: "usd",
	}); err != nil {
		t.Fatalf("ChargeAsync: %v", err)
	}
	form := lastCall(t, last, 0).Form
	if got := form.Get("description"); got != "Coterie contribution c-d" {
		t.Fatalf("description = %q, want the default", got)
	}
	if _, ok := form["metadata[contribution_id]"]; !ok {
		t.Fatal("contribution metadata missing")
	}

	// No ids, no metadata keys; the description still names the intent.
	if _, err := adapter.ChargeAsync(context.Background(), payment.Charge{
		Amount: "5.00", Currency: "usd", Description: "top-up",
	}); err != nil {
		t.Fatalf("ChargeAsync bare: %v", err)
	}
	form = lastCall(t, last, 1).Form
	if _, ok := form["metadata[contribution_id]"]; ok {
		t.Fatal("empty contribution id must not carry metadata")
	}
	if got := form.Get("description"); got != "top-up" {
		t.Fatalf("description = %q, want the given one", got)
	}
}

func TestStripeChargeSyncRejected(t *testing.T) {
	// The sync method exists to catch a future caller that forgets the
	// adapter is async-only.
	_, err := (payment.Stripe{}).Charge(context.Background(), payment.Charge{})
	if err == nil || !strings.Contains(detailMessage(t, err), "asynchronously") {
		t.Fatalf("sync Charge: err = %v", err)
	}
}

func TestStripeAPIResponses(t *testing.T) {
	ctx := context.Background()

	// A non-JSON error body surfaces as the opaque internal error —
	// the channel message path needs a parseable envelope.
	srv, _ := newFakeStripe(t, http.StatusInternalServerError, "oops")
	adapter := payment.Stripe{SecretKey: "sk", WebhookSecret: testWebhookSecret, APIBase: srv.URL}
	_, err := adapter.ChargeAsync(ctx, payment.Charge{Amount: "1.00", Currency: "usd"})
	if err == nil || strings.Contains(err.Error(), "oops") {
		t.Fatalf("non-JSON error body: err = %v", err)
	}

	// A 200 whose body is not an intent fails at decode.
	srv2, _ := newFakeStripe(t, http.StatusOK, "not json")
	adapter.APIBase = srv2.URL
	_, err = adapter.ChargeAsync(ctx, payment.Charge{Amount: "1.00", Currency: "usd"})
	if err == nil || !strings.Contains(err.Error(), "decode response") {
		t.Fatalf("garbage 200: err = %v", err)
	}
}

func TestStripeWebhookFutureTimestamp(t *testing.T) {
	adapter := payment.Stripe{WebhookSecret: testWebhookSecret}
	payload := []byte(`{"id":"evt_f","type":"payment_intent.succeeded","data":{"object":{"id":"pi_f"}}}`)
	// A timestamp from the future is as suspicious as a stale one.
	future := time.Now().Add(6 * time.Minute)
	if _, _, err := adapter.ParseWebhook(http.Header{"Stripe-Signature": {signStripe(t, testWebhookSecret, future, payload)}}, payload); err == nil {
		t.Fatal("future signature accepted")
	}
}

// detailMessage returns the first validation detail's message; an empty
// string means the error was not an *api.APIError with details.
func detailMessage(t *testing.T, err error) string {
	t.Helper()
	if err == nil {
		return ""
	}
	var apiErr *api.APIError
	if !errors.As(err, &apiErr) || len(apiErr.Details) == 0 {
		return ""
	}
	return apiErr.Details[0].Message
}
