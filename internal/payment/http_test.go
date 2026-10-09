package payment_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/cuihairu/coterie/internal/app"
	"github.com/cuihairu/coterie/internal/payment"
	"github.com/cuihairu/coterie/internal/testsupport"
)

// seedDebt builds a circle with one open billing period and an equal
// split over the owner and the joined members, returning the seeded
// members (tokens) and the created contributions.
func seedDebt(t *testing.T, client *http.Client, base, tag, price string, members int) (tok, subID, periodID string, joined []testsupport.Member, contributions []map[string]any) {
	t.Helper()
	tok, _, subID, _, joined = testsupport.SeedCircle(t, client, base, tag, price, 4, 0, members)

	code, period := testsupport.DoAuthJSON(t, client, http.MethodPost,
		base+"/api/v1/subscriptions/"+subID+"/billing-periods",
		`{"start_date":"2026-10-01","end_date":"2026-10-31"}`, tok)
	if code != http.StatusCreated {
		t.Fatalf("create period: status = %d: %v", code, period)
	}
	periodID, _ = period["id"].(string)

	code, resp := testsupport.DoAuthJSON(t, client, http.MethodPost,
		base+"/api/v1/billing-periods/"+periodID+"/contributions/generate",
		`{"mode":"equal"}`, tok)
	if code != http.StatusCreated {
		t.Fatalf("generate: status = %d: %v", code, resp)
	}
	items, _ := resp["items"].([]any)
	for _, it := range items {
		contributions = append(contributions, it.(map[string]any))
	}
	return tok, subID, periodID, joined, contributions
}

// contributionOf finds the contribution row for a member id.
func contributionOf(t *testing.T, contributions []map[string]any, memberID string) map[string]any {
	t.Helper()
	for _, c := range contributions {
		if c["member_id"] == memberID {
			return c
		}
	}
	t.Fatalf("no contribution for member %s in %v", memberID, contributions)
	return nil
}

// pay posts a payment for a contribution.
func pay(t *testing.T, client *http.Client, base, tok, contributionID, payload string) (int, map[string]any) {
	t.Helper()
	return testsupport.DoAuthJSON(t, client, http.MethodPost,
		base+"/api/v1/contributions/"+contributionID+"/payments", payload, tok)
}

func TestPaymentLifecycle(t *testing.T) {
	srv := testsupport.NewServer(t, testsupport.NewDB(t))
	client, base := srv.Client(), srv.URL
	tok, _, periodID, joined, contributions := seedDebt(t, client, base, "pay-life", "12.00", 1)

	c := contributionOf(t, contributions, joined[0].MemberID)
	if c["status"] != "pending" {
		t.Fatalf("seed contribution not pending: %v", c)
	}
	cid, _ := c["id"].(string)

	// The member pays their own share via the manual adapter.
	code, body := pay(t, client, base, joined[0].Token, cid,
		`{"method":"manual","external_ref":"cash on Friday"}`)
	if code != http.StatusCreated {
		t.Fatalf("record payment: status = %d: %v", code, body)
	}
	if body["status"] != "succeeded" || body["method"] != "manual" {
		t.Fatalf("payment body = %v", body)
	}
	if body["amount"] != c["amount"] || body["currency"] != "USD" {
		t.Fatalf("payment must echo the contribution amount: %v vs %v", body, c)
	}
	if body["payer_user_id"] != joined[0].UserID {
		t.Fatalf("payer = %v, want member %s", body["payer_user_id"], joined[0].UserID)
	}
	if body["external_ref"] != "cash on Friday" {
		t.Fatalf("external_ref = %v", body["external_ref"])
	}
	paymentID, _ := body["id"].(string)

	// The contribution is settled with paid_at maintained.
	code, body = testsupport.DoAuthJSON(t, client, http.MethodGet,
		base+"/api/v1/billing-periods/"+periodID+"/contributions", "", tok)
	if code != http.StatusOK {
		t.Fatalf("list contributions: status = %d: %v", code, body)
	}
	settled := false
	items, _ := body["items"].([]any)
	for _, it := range items {
		row := it.(map[string]any)
		if row["id"] == cid {
			settled = row["status"] == "paid" && row["paid_at"] != nil
		}
	}
	if !settled {
		t.Fatalf("contribution %s not settled after payment: %v", cid, body)
	}

	// The ledger keeps the payment; single fetch works.
	code, body = testsupport.DoAuthJSON(t, client, http.MethodGet,
		base+"/api/v1/payments/"+paymentID, "", tok)
	if code != http.StatusOK || body["id"] != paymentID {
		t.Fatalf("get payment: status = %d: %v", code, body)
	}

	// A second charge is rejected — the receivable is already settled.
	code, body = pay(t, client, base, joined[0].Token, cid, `{}`)
	if code != http.StatusConflict {
		t.Fatalf("double payment: status = %d, want 409: %v", code, body)
	}

	// The owner got the payment_received notification.
	code, body = testsupport.DoAuthJSON(t, client, http.MethodGet,
		base+"/api/v1/notifications", "", tok)
	if code != http.StatusOK {
		t.Fatalf("owner notifications: status = %d: %v", code, body)
	}
	found := false
	items, _ = body["items"].([]any)
	for _, it := range items {
		if it.(map[string]any)["type"] == "payment_received" {
			found = true
		}
	}
	if !found {
		t.Fatalf("owner notifications missing payment_received: %v", body)
	}
}

func TestPaymentPermissions(t *testing.T) {
	srv := testsupport.NewServer(t, testsupport.NewDB(t))
	client, base := srv.Client(), srv.URL
	tok, _, _, joined, contributions := seedDebt(t, client, base, "pay-perm", "10.00", 1)
	cid, _ := contributionOf(t, contributions, joined[0].MemberID)["id"].(string)

	// A stranger may neither pay for nor on behalf of the member.
	strangerTok, _ := testsupport.RegisterAndLogin(t, client, base, "pay-perm-stranger")
	code, body := pay(t, client, base, strangerTok, cid, `{}`)
	if code != http.StatusForbidden {
		t.Fatalf("stranger payment: status = %d, want 403: %v", code, body)
	}

	// The owner may record the member's payment; the payer stays the
	// member, not the actor.
	code, body = pay(t, client, base, tok, cid, `{"external_ref":"collected in person"}`)
	if code != http.StatusCreated {
		t.Fatalf("owner payment: status = %d: %v", code, body)
	}
	if body["payer_user_id"] != joined[0].UserID {
		t.Fatalf("payer = %v, want member %s", body["payer_user_id"], joined[0].UserID)
	}

	// Reads stay on the billing baseline: any authenticated user.
	code, body = testsupport.DoAuthJSON(t, client, http.MethodGet,
		base+"/api/v1/contributions/"+cid+"/payments", "", strangerTok)
	if code != http.StatusOK {
		t.Fatalf("stranger payments read: status = %d: %v", code, body)
	}
	if n, _ := body["meta"].(map[string]any)["total"].(float64); n != 1 {
		t.Fatalf("payments total = %v", body["meta"])
	}
}

func TestPaymentDuplicateExternalRef(t *testing.T) {
	srv := testsupport.NewServer(t, testsupport.NewDB(t))
	client, base := srv.Client(), srv.URL
	_, _, _, joined, contributions := seedDebt(t, client, base, "pay-dupref", "10.00", 2)
	if len(joined) < 2 {
		t.Fatalf("seed joined = %d, want 2", len(joined))
	}
	cid1 := contributionOf(t, contributions, joined[0].MemberID)["id"].(string)
	cid2 := contributionOf(t, contributions, joined[1].MemberID)["id"].(string)

	// Two contributions paid with the same free-text ref: the manual
	// ledger's unique constraint must surface as a 409, not a 500.
	code, body := pay(t, client, base, joined[0].Token, cid1, `{"external_ref":"receipt 42"}`)
	if code != http.StatusCreated {
		t.Fatalf("first payment: status = %d: %v", code, body)
	}
	code, body = pay(t, client, base, joined[1].Token, cid2, `{"external_ref":"receipt 42"}`)
	if code != http.StatusConflict {
		t.Fatalf("duplicate ref: status = %d, want 409: %v", code, body)
	}
}

func TestPaymentValidation(t *testing.T) {
	srv := testsupport.NewServer(t, testsupport.NewDB(t))
	client, base := srv.Client(), srv.URL
	tok, _, _, joined, contributions := seedDebt(t, client, base, "pay-val", "10.00", 1)
	c := contributionOf(t, contributions, joined[0].MemberID)
	cid, _ := c["id"].(string)

	// Unknown adapter names 422.
	code, body := pay(t, client, base, tok, cid, `{"method":"stripe"}`)
	if code != http.StatusUnprocessableEntity {
		t.Fatalf("unknown method: status = %d, want 422: %v", code, body)
	}

	// Over-long external_ref 422.
	code, body = pay(t, client, base, tok, cid,
		fmt.Sprintf(`{"external_ref":%q}`, strings.Repeat("x", 201)))
	if code != http.StatusUnprocessableEntity {
		t.Fatalf("long external_ref: status = %d, want 422: %v", code, body)
	}

	// Waived contributions are not payable.
	code, body = testsupport.DoAuthJSON(t, client, http.MethodPatch,
		base+"/api/v1/contributions/"+cid, `{"status":"waived"}`, tok)
	if code != http.StatusOK {
		t.Fatalf("waive: status = %d: %v", code, body)
	}
	code, body = pay(t, client, base, tok, cid, `{}`)
	if code != http.StatusConflict {
		t.Fatalf("waived payment: status = %d, want 409: %v", code, body)
	}

	// Unknown contribution and payment ids 404.
	code, _ = pay(t, client, base, tok, "00000000-0000-0000-0000-000000000000", `{}`)
	if code != http.StatusNotFound {
		t.Fatalf("unknown contribution: status = %d, want 404", code)
	}
	code, _ = testsupport.DoAuthJSON(t, client, http.MethodGet,
		base+"/api/v1/payments/00000000-0000-0000-0000-000000000000", "", tok)
	if code != http.StatusNotFound {
		t.Fatalf("unknown payment: status = %d, want 404", code)
	}
}

// The sandbox adapter is the plugin-seam demo channel (design §4.2):
// registered by configuration, settles instantly with a synthetic
// reference, and shows up in the methods listing.
func TestSandboxChannel(t *testing.T) {
	srv := testsupport.NewServer(t, testsupport.NewDB(t),
		app.WithPaymentAdapters(payment.Sandbox{}))
	client, base := srv.Client(), srv.URL
	tok, _, periodID, joined, contributions := seedDebt(t, client, base, "pay-sbx", "8.00", 1)
	c := contributionOf(t, contributions, joined[0].MemberID)
	cid, _ := c["id"].(string)

	// The member pays through the sandbox channel.
	code, body := pay(t, client, base, joined[0].Token, cid, `{"method":"sandbox"}`)
	if code != http.StatusCreated {
		t.Fatalf("sandbox payment: status = %d: %v", code, body)
	}
	if body["method"] != "sandbox" || body["status"] != "succeeded" {
		t.Fatalf("payment body = %v", body)
	}
	ref, _ := body["external_ref"].(string)
	if !strings.HasPrefix(ref, "sbx_") {
		t.Fatalf("external_ref = %q, want sbx_ prefix from the adapter receipt", ref)
	}
	if body["amount"] != c["amount"] {
		t.Fatalf("payment must echo the contribution amount: %v vs %v", body, c)
	}

	// The contribution is settled exactly as with manual.
	code, body = testsupport.DoAuthJSON(t, client, http.MethodGet,
		base+"/api/v1/billing-periods/"+periodID+"/contributions", "", tok)
	if code != http.StatusOK {
		t.Fatalf("list contributions: status = %d: %v", code, body)
	}
	settled := false
	items, _ := body["items"].([]any)
	for _, it := range items {
		row := it.(map[string]any)
		if row["id"] == cid {
			settled = row["status"] == "paid" && row["paid_at"] != nil
		}
	}
	if !settled {
		t.Fatalf("contribution %s not settled after sandbox payment: %v", cid, body)
	}
}

// The methods listing reports the registered adapters; manual is always
// present, sandbox only when configured.
func TestPaymentMethodsListing(t *testing.T) {
	plain := testsupport.NewServer(t, testsupport.NewDB(t))
	client, base := plain.Client(), plain.URL
	tok, _ := testsupport.RegisterAndLogin(t, client, base, "pay-methods-plain")

	code, body := testsupport.DoAuthJSON(t, client, http.MethodGet,
		base+"/api/v1/payments/methods", "", tok)
	if code != http.StatusOK {
		t.Fatalf("methods: status = %d: %v", code, body)
	}
	items, _ := body["items"].([]any)
	if len(items) != 1 || items[0] != "manual" {
		t.Fatalf("default methods = %v, want [manual]", items)
	}

	// Anonymous reads stay behind the auth baseline.
	code, body = testsupport.DoJSON(t, client, http.MethodGet, base+"/api/v1/payments/methods", "")
	if code != http.StatusUnauthorized {
		t.Fatalf("anonymous methods: status = %d, want 401: %v", code, body)
	}

	withSandbox := testsupport.NewServer(t, testsupport.NewDB(t),
		app.WithPaymentAdapters(payment.Sandbox{}))
	client2, base2 := withSandbox.Client(), withSandbox.URL
	tok2, _ := testsupport.RegisterAndLogin(t, client2, base2, "pay-methods-sbx")
	code, body = testsupport.DoAuthJSON(t, client2, http.MethodGet,
		base2+"/api/v1/payments/methods", "", tok2)
	if code != http.StatusOK {
		t.Fatalf("methods with sandbox: status = %d: %v", code, body)
	}
	items, _ = body["items"].([]any)
	if len(items) != 2 || items[0] != "manual" || items[1] != "sandbox" {
		t.Fatalf("methods = %v, want [manual sandbox]", items)
	}
}

// The Stripe channel settles asynchronously (design D24): the charge
// starts an intent and lands a pending row, the signed webhook flips it
// and settles the contribution, replays stay idempotent, and failures
// leave the contribution untouched.
func TestStripeAsyncChannel(t *testing.T) {
	// The fake channel resolves intents by counter and hands the
	// signature test a secret of our choosing.
	secret := "whsec_async"
	var intents int
	fake := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		intents++
		_, _ = fmt.Fprintf(w, `{"id":"pi_async_%d","client_secret":"pi_async_%d_secret"}`, intents, intents)
	}))
	t.Cleanup(fake.Close)

	srv := testsupport.NewServer(t, testsupport.NewDB(t),
		app.WithPaymentAdapters(payment.Stripe{
			SecretKey: "sk_test_x", WebhookSecret: secret, APIBase: fake.URL,
		}))
	client, base := srv.Client(), srv.URL
	tok, _, periodID, joined, contributions := seedDebt(t, client, base, "pay-async", "30.00", 1)
	memberTok := joined[0].Token
	c := contributionOf(t, contributions, joined[0].MemberID)
	cid, _ := c["id"].(string)

	// The member starts a stripe charge: 201, pending, client_secret,
	// contribution untouched.
	code, body := pay(t, client, base, memberTok, cid, `{"method":"stripe"}`)
	if code != http.StatusCreated {
		t.Fatalf("stripe charge: status = %d: %v", code, body)
	}
	if body["status"] != "pending" || body["method"] != "stripe" {
		t.Fatalf("payment body = %v", body)
	}
	if body["client_secret"] != "pi_async_1_secret" {
		t.Fatalf("client_secret = %v", body["client_secret"])
	}
	ref, _ := body["external_ref"].(string)
	if ref != "pi_async_1" {
		t.Fatalf("external_ref = %q", ref)
	}
	paid := func() bool {
		code, body := testsupport.DoAuthJSON(t, client, http.MethodGet,
			base+"/api/v1/billing-periods/"+periodID+"/contributions", "", tok)
		if code != http.StatusOK {
			t.Fatalf("list contributions: status = %d: %v", code, body)
		}
		for _, it := range body["items"].([]any) {
			if row := it.(map[string]any); row["id"] == cid {
				return row["status"] == "paid"
			}
		}
		return false
	}
	if paid() {
		t.Fatal("contribution settled before the webhook")
	}

	// A bad signature is rejected and changes nothing.
	code, body = testsupport.DoJSON(t, client, http.MethodPost,
		base+"/api/v1/payments/webhooks/stripe", `{"type":"payment_intent.succeeded"}`)
	if code != http.StatusUnprocessableEntity {
		t.Fatalf("unsigned webhook: status = %d, want 422: %v", code, body)
	}

	// The signed success webhook settles the row and the contribution.
	succeeded := fmt.Sprintf(`{"id":"evt_1","type":"payment_intent.succeeded","data":{"object":{"id":%q,"object":"payment_intent"}}}`, ref)
	code, body = postWebhook(t, client, base, "stripe", secret, succeeded)
	if code != http.StatusOK {
		t.Fatalf("success webhook: status = %d: %v", code, body)
	}
	if body["status"] != "succeeded" || body["paid_at"] == nil {
		t.Fatalf("payment after webhook = %v", body)
	}
	if !paid() {
		t.Fatal("contribution not settled by the success webhook")
	}

	// The owner is notified exactly the way a manual payment would.
	code, body = testsupport.DoAuthJSON(t, client, http.MethodGet, base+"/api/v1/notifications", "", tok)
	if code != http.StatusOK {
		t.Fatalf("owner notifications: status = %d: %v", code, body)
	}
	notified := false
	for _, it := range body["items"].([]any) {
		if it.(map[string]any)["type"] == "payment_received" {
			notified = true
		}
	}
	if !notified {
		t.Fatalf("owner notifications missing payment_received: %v", body)
	}

	// Replaying the same webhook is idempotent: 200 with the stored row.
	code, body = postWebhook(t, client, base, "stripe", secret, succeeded)
	if code != http.StatusOK || body["id"] != body["id"] || body["status"] != "succeeded" {
		t.Fatalf("replay: status = %d: %v", code, body)
	}

	// An unknown intent is a 404 — we never lose money silently.
	ghost := `{"id":"evt_9","type":"payment_intent.succeeded","data":{"object":{"id":"pi_ghost","object":"payment_intent"}}}`
	code, body = postWebhook(t, client, base, "stripe", secret, ghost)
	if code != http.StatusNotFound {
		t.Fatalf("unknown intent: status = %d, want 404: %v", code, body)
	}

	// Uninteresting events still 200 so the channel stops retrying.
	other := `{"id":"evt_10","type":"charge.refunded","data":{"object":{"id":"ch_1"}}}`
	code, body = postWebhook(t, client, base, "stripe", secret, other)
	if code != http.StatusOK {
		t.Fatalf("unrelated event: status = %d, want 200: %v", code, body)
	}
}

// TestStripeAsyncFailure covers payment_intent.payment_failed: the row
// flips to failed and the contribution stays pending (retryable).
func TestStripeAsyncFailure(t *testing.T) {
	var attempts int
	fake := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts++
		_, _ = fmt.Fprintf(w, `{"id":"pi_fail_%d","client_secret":"pi_fail_%d_secret"}`, attempts, attempts)
	}))
	t.Cleanup(fake.Close)

	srv := testsupport.NewServer(t, testsupport.NewDB(t),
		app.WithPaymentAdapters(payment.Stripe{
			SecretKey: "sk_test_x", WebhookSecret: "whsec_fail", APIBase: fake.URL,
		}))
	client, base := srv.Client(), srv.URL
	tok, _, periodID, joined, contributions := seedDebt(t, client, base, "pay-async-fail", "5.00", 1)
	cid, _ := contributionOf(t, contributions, joined[0].MemberID)["id"].(string)

	code, body := pay(t, client, base, joined[0].Token, cid, `{"method":"stripe"}`)
	if code != http.StatusCreated {
		t.Fatalf("stripe charge: status = %d: %v", code, body)
	}
	ref, _ := body["external_ref"].(string)

	failed := fmt.Sprintf(`{"id":"evt_f","type":"payment_intent.payment_failed","data":{"object":{"id":%q,"object":"payment_intent"}}}`, ref)
	code, body = postWebhook(t, client, base, "stripe", "whsec_fail", failed)
	if code != http.StatusOK || body["status"] != "failed" {
		t.Fatalf("failure webhook: status = %d: %v", code, body)
	}

	// The contribution is still pending — the member may retry.
	code, body = testsupport.DoAuthJSON(t, client, http.MethodGet,
		base+"/api/v1/billing-periods/"+periodID+"/contributions", "", tok)
	if code != http.StatusOK {
		t.Fatalf("list contributions: status = %d: %v", code, body)
	}
	for _, it := range body["items"].([]any) {
		if row := it.(map[string]any); row["id"] == cid && row["status"] != "pending" {
			t.Fatalf("contribution status = %v, want pending after failure", row["status"])
		}
	}

	// A retry is allowed: the failed row is not a receivable anymore,
	// so a fresh charge must succeed. Reuse the same receipt-free path
	// by charging again — the contribution was never settled.
	code, body = pay(t, client, base, joined[0].Token, cid, `{"method":"stripe"}`)
	if code != http.StatusCreated {
		t.Fatalf("retry charge: status = %d: %v", code, body)
	}
}

// postWebhook delivers a signed channel webhook to the public endpoint.
func postWebhook(t *testing.T, client *http.Client, base, method, secret, payload string) (int, map[string]any) {
	t.Helper()
	req, err := http.NewRequest(http.MethodPost,
		base+"/api/v1/payments/webhooks/"+method, strings.NewReader(payload))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Stripe-Signature", signStripe(t, secret, time.Now(), []byte(payload)))
	res, err := client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = res.Body.Close() }()
	var out map[string]any
	_ = json.NewDecoder(res.Body).Decode(&out)
	return res.StatusCode, out
}
