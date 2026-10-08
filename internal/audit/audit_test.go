package audit_test

import (
	"fmt"
	"net/http"
	"testing"

	"github.com/cuihairu/coterie/internal/testsupport"
)

// entries fetches an audit list and returns the decoded items.
func entries(t *testing.T, client *http.Client, base, url, tok string) []map[string]any {
	t.Helper()
	code, body := testsupport.DoAuthJSON(t, client, http.MethodGet, base+url, "", tok)
	if code != http.StatusOK {
		t.Fatalf("GET %s: status = %d: %v", url, code, body)
	}
	items, _ := body["items"].([]any)
	out := make([]map[string]any, 0, len(items))
	for _, it := range items {
		out = append(out, it.(map[string]any))
	}
	return out
}

// snapshotOf extracts a before/after snapshot map.
func snapshotOf(t *testing.T, e map[string]any, field string) map[string]any {
	t.Helper()
	raw, ok := e[field]
	if !ok || raw == nil {
		return nil
	}
	m, isMap := raw.(map[string]any)
	if !isMap {
		t.Fatalf("%s is not an object: %v", field, raw)
	}
	return m
}

// hasAction reports whether the list contains the action.
func hasAction(items []map[string]any, action string) bool {
	for _, e := range items {
		if a, _ := e["action"].(string); a == action {
			return true
		}
	}
	return false
}

func TestSubscriptionAuditTrail(t *testing.T) {
	db := testsupport.NewDB(t)
	srv := testsupport.NewServer(t, db)
	client, base := srv.Client(), srv.URL
	tok, _, subID := testsupport.SeedChain(t, client, base, "aud-sub", "10.00", 4)

	// Only the fields that changed land in the snapshots.
	code, _ := testsupport.DoAuthJSON(t, client, http.MethodPatch,
		base+"/api/v1/subscriptions/"+subID, `{"price":"22.00"}`, tok)
	if code != http.StatusOK {
		t.Fatalf("patch price: status = %d", code)
	}

	items := entries(t, client, base, "/api/v1/subscriptions/"+subID+"/audit-logs", tok)
	if len(items) != 1 {
		t.Fatalf("entry count = %d, want 1: %v", len(items), items)
	}
	e := items[0]
	if e["action"] != "subscription_updated" || e["entity_type"] != "subscription" {
		t.Fatalf("entry = %v", e)
	}
	if e["entity_id"] != subID || e["subscription_id"] != subID {
		t.Fatalf("entry pivots = %v / %v", e["entity_id"], e["subscription_id"])
	}
	before := snapshotOf(t, e, "before")
	after := snapshotOf(t, e, "after")
	if before["price"] != "10.00" || after["price"] != "22.00" {
		t.Fatalf("price snapshot = %v -> %v", before["price"], after["price"])
	}
	if _, touched := after["status"]; touched {
		t.Fatalf("unchanged field leaked into snapshot: %v", after)
	}
	if e["actor_id"] == nil {
		t.Fatalf("actor not recorded: %v", e)
	}

	// The action filter narrows the list; unknown actions are 422s.
	items = entries(t, client, base,
		"/api/v1/subscriptions/"+subID+"/audit-logs?action=subscription_updated", tok)
	if len(items) != 1 {
		t.Fatalf("filtered entry count = %d, want 1", len(items))
	}
	code, _ = testsupport.DoAuthJSON(t, client, http.MethodGet,
		base+"/api/v1/subscriptions/"+subID+"/audit-logs?action=bogus", "", tok)
	if code != http.StatusUnprocessableEntity {
		t.Fatalf("unknown action filter: status = %d, want 422", code)
	}
}

func TestCoterieAuditTrail(t *testing.T) {
	db := testsupport.NewDB(t)
	srv := testsupport.NewServer(t, db)
	client, base := srv.Client(), srv.URL
	tok, _, subID, coterieID, joined := testsupport.SeedCircle(t, client, base, "aud-cot", "10.00", 4, 4, 1)

	// The owner reassigns the member's seat, then removes the member,
	// then closes the circle.
	code, mems := testsupport.DoAuthJSON(t, client, http.MethodGet,
		base+"/api/v1/coteries/"+coterieID+"/members", "", tok)
	if code != http.StatusOK {
		t.Fatalf("list members: status = %d: %v", code, mems)
	}
	memberItems, _ := mems["items"].([]any)
	var memberID string
	for _, it := range memberItems {
		m := it.(map[string]any)
		if m["user_id"] == joined[0].UserID {
			memberID, _ = m["id"].(string)
		}
	}
	if memberID == "" {
		t.Fatalf("seeded member not found: %v", memberItems)
	}

	code, seats := testsupport.DoAuthJSON(t, client, http.MethodGet,
		base+"/api/v1/subscriptions/"+subID+"/seats", "", tok)
	if code != http.StatusOK {
		t.Fatalf("list seats: status = %d: %v", code, seats)
	}
	seatItems, _ := seats["items"].([]any)
	seatID, _ := seatItems[0].(map[string]any)["id"].(string)

	code, _ = testsupport.DoAuthJSON(t, client, http.MethodPost,
		fmt.Sprintf("%s/api/v1/seats/%s/assign", base, seatID),
		fmt.Sprintf(`{"member_id":%q}`, memberID), tok)
	if code != http.StatusOK {
		t.Fatalf("assign seat: status = %d", code)
	}
	code, _ = testsupport.DoAuthJSON(t, client, http.MethodPost,
		base+"/api/v1/seats/"+seatID+"/release", "", tok)
	if code != http.StatusOK {
		t.Fatalf("release seat: status = %d", code)
	}

	code, _ = testsupport.DoAuthJSON(t, client, http.MethodDelete,
		base+"/api/v1/members/"+memberID, "", tok)
	if code != http.StatusNoContent {
		t.Fatalf("remove member: status = %d", code)
	}

	code, _ = testsupport.DoAuthJSON(t, client, http.MethodPatch,
		base+"/api/v1/coteries/"+coterieID, `{"status":"closed"}`, tok)
	if code != http.StatusOK {
		t.Fatalf("close coterie: status = %d", code)
	}

	items := entries(t, client, base, "/api/v1/coteries/"+coterieID+"/audit-logs", tok)
	for _, want := range []string{"coterie_created", "seat_assigned", "seat_released", "member_removed", "coterie_updated"} {
		if !hasAction(items, want) {
			t.Fatalf("missing %s in %v", want, actionNames(items))
		}
	}

	// The seat trail keeps both halves of a reassignment.
	seatItems2 := entries(t, client, base,
		"/api/v1/coteries/"+coterieID+"/audit-logs?action=seat_released", tok)
	released := snapshotOf(t, seatItems2[0], "before")
	if released["member_id"] != memberID {
		t.Fatalf("seat_released before.member_id = %v, want %v", released["member_id"], memberID)
	}

	// Filtering by action excludes the rest.
	for _, e := range entries(t, client, base,
		"/api/v1/coteries/"+coterieID+"/audit-logs?action=member_removed", tok) {
		if e["action"] != "member_removed" {
			t.Fatalf("filter leaked %v", e)
		}
	}
}

// actionNames lists action names for failure messages.
func actionNames(items []map[string]any) []string {
	out := make([]string, 0, len(items))
	for _, e := range items {
		a, _ := e["action"].(string)
		out = append(out, a)
	}
	return out
}

func TestSettlementAuditTrail(t *testing.T) {
	db := testsupport.NewDB(t)
	srv := testsupport.NewServer(t, db)
	client, base := srv.Client(), srv.URL
	tok, _, subID, _, joined := testsupport.SeedCircle(t, client, base, "aud-pay", "20.00", 4, 4, 1)

	code, period := testsupport.DoAuthJSON(t, client, http.MethodPost,
		base+"/api/v1/subscriptions/"+subID+"/billing-periods",
		`{"start_date":"2026-10-01","end_date":"2026-10-31"}`, tok)
	if code != http.StatusCreated {
		t.Fatalf("period: status = %d: %v", code, period)
	}
	periodID, _ := period["id"].(string)
	code, gen := testsupport.DoAuthJSON(t, client, http.MethodPost,
		base+"/api/v1/billing-periods/"+periodID+"/contributions/generate",
		`{"mode":"equal"}`, tok)
	if code != http.StatusCreated {
		t.Fatalf("generate: status = %d: %v", code, gen)
	}
	contribs, _ := gen["items"].([]any)
	var contributionID string
	for _, it := range contribs {
		c := it.(map[string]any)
		if c["member_id"] == joined[0].MemberID {
			contributionID, _ = c["id"].(string)
		}
	}
	if contributionID == "" {
		t.Fatalf("member contribution not generated: %v", contribs)
	}

	// Amount change, payment, and period close each leave a trail.
	code, _ = testsupport.DoAuthJSON(t, client, http.MethodPatch,
		base+"/api/v1/contributions/"+contributionID, `{"amount":"11.00"}`, tok)
	if code != http.StatusOK {
		t.Fatalf("patch contribution: status = %d", code)
	}
	code, pay := testsupport.DoAuthJSON(t, client, http.MethodPost,
		base+"/api/v1/contributions/"+contributionID+"/payments",
		`{"method":"manual"}`, joined[0].Token)
	if code != http.StatusCreated {
		t.Fatalf("record payment: status = %d: %v", code, pay)
	}
	code, _ = testsupport.DoAuthJSON(t, client, http.MethodPost,
		base+"/api/v1/billing-periods/"+periodID+"/close", "", tok)
	if code != http.StatusOK {
		t.Fatalf("close period: status = %d", code)
	}

	items := entries(t, client, base, "/api/v1/subscriptions/"+subID+"/audit-logs", tok)
	for _, want := range []string{"contribution_updated", "payment_recorded", "period_closed"} {
		if !hasAction(items, want) {
			t.Fatalf("missing %s in %v", want, actionNames(items))
		}
	}
	for _, e := range items {
		if e["action"] == "contribution_updated" {
			after := snapshotOf(t, e, "after")
			if after["amount"] != "11.00" {
				t.Fatalf("contribution after = %v", after)
			}
		}
		if e["action"] == "period_closed" {
			after := snapshotOf(t, e, "after")
			if after["status"] != "closed" {
				t.Fatalf("period after = %v", after)
			}
		}
	}
}

func TestAuditReadAuthorization(t *testing.T) {
	db := testsupport.NewDB(t)
	srv := testsupport.NewServer(t, db)
	client, base := srv.Client(), srv.URL
	tok, _, subID := testsupport.SeedChain(t, client, base, "aud-auth", "10.00", 4)
	stranger, _ := testsupport.RegisterAndLogin(t, client, base, "aud-auth-x")

	// Anonymous reads are 401; non-owners get 403; unknown ids 404.
	code, _ := testsupport.DoJSON(t, client, http.MethodGet,
		base+"/api/v1/subscriptions/"+subID+"/audit-logs", "")
	if code != http.StatusUnauthorized {
		t.Fatalf("anonymous read: status = %d, want 401", code)
	}
	code, _ = testsupport.DoAuthJSON(t, client, http.MethodGet,
		base+"/api/v1/subscriptions/"+subID+"/audit-logs", "", stranger)
	if code != http.StatusForbidden {
		t.Fatalf("stranger read: status = %d, want 403", code)
	}
	code, _ = testsupport.DoAuthJSON(t, client, http.MethodGet,
		base+"/api/v1/subscriptions/00000000-0000-0000-0000-000000000000/audit-logs", "", tok)
	if code != http.StatusNotFound {
		t.Fatalf("unknown subscription: status = %d, want 404", code)
	}

	// The coterie scope authorizes through the subscription owner too.
	code, body := testsupport.DoAuthJSON(t, client, http.MethodPost,
		base+"/api/v1/coteries",
		fmt.Sprintf(`{"subscription_id":%q,"name":"Auth Circle","capacity":2}`, subID), tok)
	if code != http.StatusCreated {
		t.Fatalf("coterie: status = %d: %v", code, body)
	}
	coterieID, _ := body["id"].(string)
	code, _ = testsupport.DoAuthJSON(t, client, http.MethodGet,
		base+"/api/v1/coteries/"+coterieID+"/audit-logs", "", stranger)
	if code != http.StatusForbidden {
		t.Fatalf("stranger coterie read: status = %d, want 403", code)
	}
	items := entries(t, client, base, "/api/v1/coteries/"+coterieID+"/audit-logs", tok)
	if len(items) != 1 || items[0]["action"] != "coterie_created" {
		t.Fatalf("coterie trail = %v", actionNames(items))
	}
}
