package subscription_test

import (
	"fmt"
	"net/http"
	"testing"

	"github.com/cuihairu/coterie/internal/testsupport"
)

const subscriptionsPath = "/api/v1/subscriptions"

// seedChain creates provider → product through the API and returns
// the product id plus the authenticated user's id, ready for
// subscription creation as that user.
func seedChain(t *testing.T, client *http.Client, base, tag, tok string) (productID, userID string) {
	t.Helper()
	code, body := testsupport.DoAuthJSON(t, client, http.MethodPost, base+"/api/v1/providers",
		fmt.Sprintf(`{"slug":"prov-%s","name":"Provider %s","category":"video"}`, tag, tag), tok)
	if code != http.StatusCreated {
		t.Fatalf("seed provider: status = %d: %v", code, body)
	}
	providerID, _ := body["id"].(string)

	code, body = testsupport.DoAuthJSON(t, client, http.MethodPost, base+"/api/v1/products",
		fmt.Sprintf(`{"provider_id":%q,"name":"Premium %s"}`, providerID, tag), tok)
	if code != http.StatusCreated {
		t.Fatalf("seed product: status = %d: %v", code, body)
	}
	productID, _ = body["id"].(string)

	// The authenticated user is the only owner a subscription may
	// have now that ownership is auth-enforced.
	code, body = testsupport.DoAuthJSON(t, client, http.MethodGet, base+"/api/v1/auth/me", "", tok)
	if code != http.StatusOK {
		t.Fatalf("me: status = %d: %v", code, body)
	}
	userID, _ = body["id"].(string)

	return productID, userID
}

func TestSubscriptionLifecycle(t *testing.T) {
	db := testsupport.NewDB(t)
	srv := testsupport.NewServer(t, db)
	client := srv.Client()
	tok, _ := testsupport.RegisterAndLogin(t, client, srv.URL, "sub-life")
	productID, userID := seedChain(t, client, srv.URL, "life", tok)

	createBody := fmt.Sprintf(`{
		"product_id":%q,
		"owner_user_id":%q,
		"billing_cycle":"monthly",
		"price":"12.50",
		"currency":"USD",
		"start_date":"2026-10-01",
		"renewal_date":"2026-11-01",
		"max_seats":4,
		"max_members":5,
		"sharing_policy":{"mode":"seat","max_members":5}
	}`, productID, userID)

	code, body := testsupport.DoAuthJSON(t, client, http.MethodPost, srv.URL+subscriptionsPath, createBody, tok)
	if code != http.StatusCreated {
		t.Fatalf("create status = %d, want 201: %v", code, body)
	}
	id, _ := body["id"].(string)
	if id == "" {
		t.Fatal("expected non-empty id")
	}
	if body["price"] != "12.50" {
		t.Fatalf("price = %v (%T), want string \"12.50\"", body["price"], body["price"])
	}
	if body["currency"] != "USD" || body["billing_cycle"] != "monthly" || body["status"] != "active" {
		t.Fatalf("unexpected fields: %v", body)
	}
	if body["start_date"] != "2026-10-01" || body["renewal_date"] != "2026-11-01" {
		t.Fatalf("dates not rendered as YYYY-MM-DD: %v", body)
	}
	if seats, _ := body["max_seats"].(float64); seats != 4 {
		t.Fatalf("max_seats = %v, want 4", body["max_seats"])
	}
	if policy, _ := body["sharing_policy"].(map[string]any); policy == nil || policy["mode"] != "seat" {
		t.Fatalf("sharing_policy not echoed: %v", body)
	}

	code, body = testsupport.DoAuthJSON(t, client, http.MethodGet, srv.URL+subscriptionsPath+"/"+id, "", tok)
	if code != http.StatusOK || body["price"] != "12.50" {
		t.Fatalf("get status = %d body = %v", code, body)
	}

	// PATCH price and status.
	code, body = testsupport.DoAuthJSON(t, client, http.MethodPatch, srv.URL+subscriptionsPath+"/"+id,
		`{"price":"15.00","status":"paused"}`, tok)
	if code != http.StatusOK || body["price"] != "15.00" || body["status"] != "paused" {
		t.Fatalf("patch status = %d body = %v", code, body)
	}

	// PATCH renewal_date "" clears the field.
	code, body = testsupport.DoAuthJSON(t, client, http.MethodPatch, srv.URL+subscriptionsPath+"/"+id,
		`{"renewal_date":""}`, tok)
	if code != http.StatusOK {
		t.Fatalf("clear renewal status = %d: %v", code, body)
	}
	if _, ok := body["renewal_date"]; ok {
		t.Fatalf("renewal_date should be cleared: %v", body)
	}

	// List filtered by owner.
	code, body = testsupport.DoAuthJSON(t, client, http.MethodGet,
		srv.URL+subscriptionsPath+"?owner_user_id="+userID, "", tok)
	if code != http.StatusOK {
		t.Fatalf("list status = %d: %v", code, body)
	}
	if total, _ := body["meta"].(map[string]any)["total"].(float64); total != 1 {
		t.Fatalf("owner filter total = %v, want 1", body["meta"])
	}

	code, _ = testsupport.DoAuthJSON(t, client, http.MethodDelete, srv.URL+subscriptionsPath+"/"+id, "", tok)
	if code != http.StatusNoContent {
		t.Fatalf("delete status = %d, want 204", code)
	}
}

func TestSubscriptionValidation(t *testing.T) {
	db := testsupport.NewDB(t)
	srv := testsupport.NewServer(t, db)
	client := srv.Client()
	tok, _ := testsupport.RegisterAndLogin(t, client, srv.URL, "sub-valid")
	productID, userID := seedChain(t, client, srv.URL, "valid", tok)

	cases := []struct {
		name string
		body string
	}{
		{"unknown product", fmt.Sprintf(`{"product_id":"00000000-0000-0000-0000-000000000000","owner_user_id":%q,"billing_cycle":"monthly","price":"1.00","currency":"USD","start_date":"2026-10-01","max_seats":2}`, userID)},
		{"bad currency", fmt.Sprintf(`{"product_id":%q,"owner_user_id":%q,"billing_cycle":"monthly","price":"1.00","currency":"usd","start_date":"2026-10-01","max_seats":2}`, productID, userID)},
		{"bad cycle", fmt.Sprintf(`{"product_id":%q,"owner_user_id":%q,"billing_cycle":"weekly","price":"1.00","currency":"USD","start_date":"2026-10-01","max_seats":2}`, productID, userID)},
		{"zero seats", fmt.Sprintf(`{"product_id":%q,"owner_user_id":%q,"billing_cycle":"monthly","price":"1.00","currency":"USD","start_date":"2026-10-01","max_seats":0}`, productID, userID)},
		{"negative member", fmt.Sprintf(`{"product_id":%q,"owner_user_id":%q,"billing_cycle":"monthly","price":"1.00","currency":"USD","start_date":"2026-10-01","max_seats":2,"max_members":0}`, productID, userID)},
		{"non-numeric price", fmt.Sprintf(`{"product_id":%q,"owner_user_id":%q,"billing_cycle":"monthly","price":"abc","currency":"USD","start_date":"2026-10-01","max_seats":2}`, productID, userID)},
		{"negative price", fmt.Sprintf(`{"product_id":%q,"owner_user_id":%q,"billing_cycle":"monthly","price":"-5.00","currency":"USD","start_date":"2026-10-01","max_seats":2}`, productID, userID)},
		{"bad start date", fmt.Sprintf(`{"product_id":%q,"owner_user_id":%q,"billing_cycle":"monthly","price":"1.00","currency":"USD","start_date":"2026-13-40","max_seats":2}`, productID, userID)},
		{"missing start date", fmt.Sprintf(`{"product_id":%q,"owner_user_id":%q,"billing_cycle":"monthly","price":"1.00","currency":"USD","max_seats":2}`, productID, userID)},
		{"renewal before start", fmt.Sprintf(`{"product_id":%q,"owner_user_id":%q,"billing_cycle":"monthly","price":"1.00","currency":"USD","start_date":"2026-10-01","renewal_date":"2026-09-01","max_seats":2}`, productID, userID)},
		{"unknown sharing mode", fmt.Sprintf(`{"product_id":%q,"owner_user_id":%q,"billing_cycle":"monthly","price":"1.00","currency":"USD","start_date":"2026-10-01","max_seats":2,"sharing_policy":{"mode":"fancy"}}`, productID, userID)},
		{"usage limit without unit", fmt.Sprintf(`{"product_id":%q,"owner_user_id":%q,"billing_cycle":"monthly","price":"1.00","currency":"USD","start_date":"2026-10-01","max_seats":2,"sharing_policy":{"usage_limit":{"per_period":"100"}}}`, productID, userID)},
		{"usage limit per_period not a number", fmt.Sprintf(`{"product_id":%q,"owner_user_id":%q,"billing_cycle":"monthly","price":"1.00","currency":"USD","start_date":"2026-10-01","max_seats":2,"sharing_policy":{"usage_limit":{"unit":"credits","per_period":"abc"}}}`, productID, userID)},
		{"usage limit per_period negative", fmt.Sprintf(`{"product_id":%q,"owner_user_id":%q,"billing_cycle":"monthly","price":"1.00","currency":"USD","start_date":"2026-10-01","max_seats":2,"sharing_policy":{"usage_limit":{"unit":"credits","per_period":"-1"}}}`, productID, userID)},
		{"usage limit per_period too fine", fmt.Sprintf(`{"product_id":%q,"owner_user_id":%q,"billing_cycle":"monthly","price":"1.00","currency":"USD","start_date":"2026-10-01","max_seats":2,"sharing_policy":{"usage_limit":{"unit":"credits","per_period":"1.12345"}}}`, productID, userID)},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			code, body := testsupport.DoAuthJSON(t, client, http.MethodPost, srv.URL+subscriptionsPath, tc.body, tok)
			if code != http.StatusUnprocessableEntity {
				t.Fatalf("status = %d, want 422: %v", code, body)
			}
		})
	}
}

// TestSubscriptionUsageLimitPolicy covers D21 policy-side validation: a
// well-formed usage_limit is accepted and echoed; the cap pins one unit.
func TestSubscriptionUsageLimitPolicy(t *testing.T) {
	db := testsupport.NewDB(t)
	srv := testsupport.NewServer(t, db)
	client := srv.Client()
	tok, userID := testsupport.RegisterAndLogin(t, client, srv.URL, "sub-ulimit")
	productID, _ := seedChain(t, client, srv.URL, "ulimit", tok)

	code, body := testsupport.DoAuthJSON(t, client, http.MethodPost, srv.URL+subscriptionsPath,
		fmt.Sprintf(`{"product_id":%q,"owner_user_id":%q,"billing_cycle":"monthly","price":"1.00","currency":"USD","start_date":"2026-10-01","max_seats":2,"sharing_policy":{"mode":"quota","usage_limit":{"unit":"credits","per_period":"100.5"}}}`, productID, userID), tok)
	if code != http.StatusCreated {
		t.Fatalf("status = %d, want 201: %v", code, body)
	}
	policy, _ := body["sharing_policy"].(map[string]any)
	lim, _ := policy["usage_limit"].(map[string]any)
	if lim == nil || lim["unit"] != "credits" || lim["per_period"] != "100.5" {
		t.Fatalf("usage_limit not echoed: %v", policy)
	}
}

func TestSubscriptionAggregateProtection(t *testing.T) {
	db := testsupport.NewDB(t)
	srv := testsupport.NewServer(t, db)
	client := srv.Client()
	tok, _ := testsupport.RegisterAndLogin(t, client, srv.URL, "sub-prot")
	productID, userID := seedChain(t, client, srv.URL, "prot", tok)

	code, body := testsupport.DoAuthJSON(t, client, http.MethodPost, srv.URL+subscriptionsPath, fmt.Sprintf(`{
		"product_id":%q,
		"owner_user_id":%q,
		"billing_cycle":"yearly",
		"price":"99.99",
		"currency":"USD",
		"start_date":"2026-10-01",
		"max_seats":6,
		"sharing_policy":{"mode":"quota","quota":300,"unit":"credits"}
	}`, productID, userID), tok)
	if code != http.StatusCreated {
		t.Fatalf("create status = %d, want 201: %v", code, body)
	}
	subID, _ := body["id"].(string)

	// A coterie bound to the subscription (inserted directly — the
	// coterie module itself is M2b scope).
	if err := db.Exec(
		"INSERT INTO coteries (subscription_id, name) VALUES (?, ?)",
		subID, "Protection Test Circle",
	).Error; err != nil {
		t.Fatal(err)
	}

	// Deleting the subscription with a bound coterie → 409.
	code, body = testsupport.DoAuthJSON(t, client, http.MethodDelete, srv.URL+subscriptionsPath+"/"+subID, "", tok)
	if code != http.StatusConflict {
		t.Fatalf("delete with coterie status = %d, want 409: %v", code, body)
	}

	// Deleting the product backing the subscription → 409.
	code, body = testsupport.DoAuthJSON(t, client, http.MethodDelete,
		srv.URL+"/api/v1/products/"+productID, "", tok)
	if code != http.StatusConflict {
		t.Fatalf("delete product with subscription status = %d, want 409: %v", code, body)
	}

	// Once the coterie is gone, deletion succeeds.
	if err := db.Exec("DELETE FROM coteries WHERE subscription_id = ?", subID).Error; err != nil {
		t.Fatal(err)
	}
	code, _ = testsupport.DoAuthJSON(t, client, http.MethodDelete, srv.URL+subscriptionsPath+"/"+subID, "", tok)
	if code != http.StatusNoContent {
		t.Fatalf("delete after coterie removal status = %d, want 204", code)
	}
}

// Ownership is enforced on every surface: only the authenticated owner
// may create (owner_user_id may only confirm it), view, list, modify,
// or delete a subscription.
func TestSubscriptionOwnerEnforcement(t *testing.T) {
	db := testsupport.NewDB(t)
	srv := testsupport.NewServer(t, db)
	client := srv.Client()
	tok, _ := testsupport.RegisterAndLogin(t, client, srv.URL, "sub-own")
	productID, userID := seedChain(t, client, srv.URL, "own", tok)
	stranger, _ := testsupport.RegisterAndLogin(t, client, srv.URL, "sub-own-x")

	// Creating for someone else is refused; the payload owner may only
	// confirm the authenticated user.
	code, body := testsupport.DoAuthJSON(t, client, http.MethodPost, srv.URL+subscriptionsPath,
		fmt.Sprintf(`{"product_id":%q,"owner_user_id":%q,"billing_cycle":"monthly","price":"1.00","currency":"USD","start_date":"2026-10-01","max_seats":2}`, productID, userID), stranger)
	if code != http.StatusForbidden {
		t.Fatalf("create for another user: status = %d, want 403: %v", code, body)
	}
	code, body = testsupport.DoAuthJSON(t, client, http.MethodPost, srv.URL+subscriptionsPath,
		fmt.Sprintf(`{"product_id":%q,"billing_cycle":"monthly","price":"1.00","currency":"USD","start_date":"2026-10-01","max_seats":2}`, productID), tok)
	if code != http.StatusCreated {
		t.Fatalf("create without owner_user_id: status = %d, want 201: %v", code, body)
	}
	subID, _ := body["id"].(string)

	// Stranger reads, writes, and deletes are 403.
	code, _ = testsupport.DoAuthJSON(t, client, http.MethodGet, srv.URL+subscriptionsPath+"/"+subID, "", stranger)
	if code != http.StatusForbidden {
		t.Fatalf("stranger get: status = %d, want 403", code)
	}
	code, _ = testsupport.DoAuthJSON(t, client, http.MethodPatch, srv.URL+subscriptionsPath+"/"+subID, `{"price":"9.99"}`, stranger)
	if code != http.StatusForbidden {
		t.Fatalf("stranger patch: status = %d, want 403", code)
	}
	code, _ = testsupport.DoAuthJSON(t, client, http.MethodDelete, srv.URL+subscriptionsPath+"/"+subID, "", stranger)
	if code != http.StatusForbidden {
		t.Fatalf("stranger delete: status = %d, want 403", code)
	}
	code, _ = testsupport.DoAuthJSON(t, client, http.MethodGet,
		srv.URL+subscriptionsPath+"?owner_user_id="+userID, "", stranger)
	if code != http.StatusForbidden {
		t.Fatalf("stranger list filter: status = %d, want 403", code)
	}

	// The owner's own list defaults to their subscriptions.
	code, body = testsupport.DoAuthJSON(t, client, http.MethodGet, srv.URL+subscriptionsPath, "", tok)
	if code != http.StatusOK {
		t.Fatalf("owner list: status = %d", code)
	}
	if total, _ := body["meta"].(map[string]any)["total"].(float64); total != 1 {
		t.Fatalf("owner list total = %v, want 1", body["meta"])
	}
}

// TestCustomCycleDays covers D20: cycle_days pairs with billing_cycle
// custom by a database CHECK, and the API rejects bad pairings with
// 422 before the constraint can trip.
func TestCustomCycleDays(t *testing.T) {
	db := testsupport.NewDB(t)
	srv := testsupport.NewServer(t, db)
	client, base := srv.Client(), srv.URL
	tok, _ := testsupport.RegisterAndLogin(t, client, base, "sub-cycles")
	productID, _ := seedChain(t, client, base, "cycles", tok)

	create := func(body string) (int, map[string]any) {
		return testsupport.DoAuthJSON(t, client, http.MethodPost, base+subscriptionsPath, body, tok)
	}
	sub := func(cycle, days string) string {
		body := fmt.Sprintf(`{"product_id":%q,"billing_cycle":%q,"price":"5.00","currency":"USD","start_date":"2026-10-01","max_seats":2`, productID, cycle)
		if days != "" {
			body += `,"cycle_days":` + days
		}
		return body + "}"
	}

	// Bad pairings fail validation with a cycle_days detail.
	for _, tc := range []struct{ name, body string }{
		{"custom without days", sub("custom", "")},
		{"custom zero days", sub("custom", "0")},
		{"custom 366 days", sub("custom", "366")},
		{"monthly with days", sub("monthly", "14")},
	} {
		t.Run(tc.name, func(t *testing.T) {
			code, body := create(tc.body)
			if code != http.StatusUnprocessableEntity {
				t.Fatalf("status = %d, want 422: %v", code, body)
			}
			errObj, _ := body["error"].(map[string]any)
			details, _ := errObj["details"].([]any)
			found := false
			for _, d := range details {
				if m, _ := d.(map[string]any)["field"].(string); m == "cycle_days" {
					found = true
				}
			}
			if !found {
				t.Fatalf("no cycle_days detail: %v", body)
			}
		})
	}

	// A good pairing persists and echoes the length.
	code, body := create(sub("custom", "14"))
	if code != http.StatusCreated {
		t.Fatalf("create custom: status = %d: %v", code, body)
	}
	subID, _ := body["id"].(string)
	if body["cycle_days"] != float64(14) {
		t.Fatalf("cycle_days = %v, want 14", body["cycle_days"])
	}

	// PATCH the length; the audit snapshot carries the change.
	code, body = testsupport.DoAuthJSON(t, client, http.MethodPatch,
		base+subscriptionsPath+"/"+subID, `{"cycle_days":30}`, tok)
	if code != http.StatusOK {
		t.Fatalf("patch days: status = %d: %v", code, body)
	}
	if body["cycle_days"] != float64(30) {
		t.Fatalf("patched cycle_days = %v, want 30", body["cycle_days"])
	}

	// Switching away from custom without clearing days is a 422 —
	// never a broken database invariant.
	code, body = testsupport.DoAuthJSON(t, client, http.MethodPatch,
		base+subscriptionsPath+"/"+subID, `{"billing_cycle":"monthly"}`, tok)
	if code != http.StatusUnprocessableEntity {
		t.Fatalf("switch without clear: status = %d, want 422: %v", code, body)
	}

	// Clearing (0) alongside the switch goes through; cycle_days
	// disappears from the payload.
	code, body = testsupport.DoAuthJSON(t, client, http.MethodPatch,
		base+subscriptionsPath+"/"+subID, `{"billing_cycle":"monthly","cycle_days":0}`, tok)
	if code != http.StatusOK {
		t.Fatalf("switch with clear: status = %d: %v", code, body)
	}
	if _, present := body["cycle_days"]; present {
		t.Fatalf("cycle_days should be absent after clear: %v", body["cycle_days"])
	}

	// The audit trail captured the length change (30) and the clear.
	code, body = testsupport.DoAuthJSON(t, client, http.MethodGet,
		base+subscriptionsPath+"/"+subID+"/audit-logs?action=subscription_updated", "", tok)
	if code != http.StatusOK {
		t.Fatalf("audit list: status = %d: %v", code, body)
	}
	sawThirty := false
	for _, it := range body["items"].([]any) {
		e := it.(map[string]any)
		after, _ := e["after"].(map[string]any)
		if v, ok := after["cycle_days"].(float64); ok && v == 30 {
			sawThirty = true
		}
	}
	if !sawThirty {
		t.Fatalf("no audit entry with after.cycle_days = 30: %v", body["items"])
	}
}
