package dispute_test

import (
	"net/http"
	"testing"

	"github.com/cuihairu/coterie/internal/testsupport"
)

// seedDebt builds a circle with one open billing period and an equal
// split over the owner and the joined members.
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

func TestDisputeLifecycle(t *testing.T) {
	srv := testsupport.NewServer(t, testsupport.NewDB(t))
	client, base := srv.Client(), srv.URL
	tok, subID, _, joined, contributions := seedDebt(t, client, base, "disp-life", "12.00", 1)
	cid, _ := contributionOf(t, contributions, joined[0].MemberID)["id"].(string)

	// The member disputes their share; the owner is told.
	code, body := testsupport.DoAuthJSON(t, client, http.MethodPost,
		base+"/api/v1/contributions/"+cid+"/disputes",
		`{"reason":"I left before the period started","evidence":"screenshot attached"}`, joined[0].Token)
	if code != http.StatusCreated {
		t.Fatalf("raise: status = %d: %v", code, body)
	}
	if body["status"] != "open" || body["contribution_id"] != cid {
		t.Fatalf("dispute body = %v", body)
	}
	disputeID, _ := body["id"].(string)

	// A second open dispute on the same contribution 409s.
	code, body = testsupport.DoAuthJSON(t, client, http.MethodPost,
		base+"/api/v1/contributions/"+cid+"/disputes", `{"reason":"again"}`, joined[0].Token)
	if code != http.StatusConflict {
		t.Fatalf("double raise: status = %d, want 409: %v", code, body)
	}

	// The owner decides; the raiser is told.
	code, body = testsupport.DoAuthJSON(t, client, http.MethodPost,
		base+"/api/v1/disputes/"+disputeID+"/decide",
		`{"decision":"resolved","note":"credited next period"}`, tok)
	if code != http.StatusOK || body["status"] != "resolved" {
		t.Fatalf("decide: status = %d: %v", code, body)
	}
	if body["resolution_note"] != "credited next period" || body["decided_at"] == nil {
		t.Fatalf("decision body = %v", body)
	}

	// Deciding is one-way.
	code, body = testsupport.DoAuthJSON(t, client, http.MethodPost,
		base+"/api/v1/disputes/"+disputeID+"/decide", `{"decision":"rejected"}`, tok)
	if code != http.StatusConflict {
		t.Fatalf("re-decide: status = %d, want 409: %v", code, body)
	}

	// Both sides got their notifications.
	for _, tc := range []struct{ name, tok string }{{"owner", tok}, {"raiser", joined[0].Token}} {
		code, body = testsupport.DoAuthJSON(t, client, http.MethodGet,
			base+"/api/v1/notifications", "", tc.tok)
		if code != http.StatusOK {
			t.Fatalf("%s notifications: status = %d", tc.name, code)
		}
		found := map[string]bool{}
		items, _ := body["items"].([]any)
		for _, it := range items {
			typ, _ := it.(map[string]any)["type"].(string)
			found[typ] = true
		}
		if tc.name == "owner" && !found["dispute_opened"] {
			t.Fatalf("owner missing dispute_opened: %v", body)
		}
		if tc.name == "raiser" && !found["dispute_decided"] {
			t.Fatalf("raiser missing dispute_decided: %v", body)
		}
	}

	// The owner lists the subscription's disputes with filters.
	code, body = testsupport.DoAuthJSON(t, client, http.MethodGet,
		base+"/api/v1/subscriptions/"+subID+"/disputes?status=resolved", "", tok)
	if code != http.StatusOK {
		t.Fatalf("list: status = %d: %v", code, body)
	}
	if n, _ := body["meta"].(map[string]any)["total"].(float64); n != 1 {
		t.Fatalf("resolved total = %v", body["meta"])
	}
}

func TestDisputePermissions(t *testing.T) {
	srv := testsupport.NewServer(t, testsupport.NewDB(t))
	client, base := srv.Client(), srv.URL
	tok, subID, _, joined, contributions := seedDebt(t, client, base, "disp-perm", "10.00", 1)
	cid, _ := contributionOf(t, contributions, joined[0].MemberID)["id"].(string)

	// A stranger may neither raise nor see anything.
	strangerTok, _ := testsupport.RegisterAndLogin(t, client, base, "disp-perm-stranger")
	code, body := testsupport.DoAuthJSON(t, client, http.MethodPost,
		base+"/api/v1/contributions/"+cid+"/disputes", `{"reason":"not mine"}`, strangerTok)
	if code != http.StatusForbidden {
		t.Fatalf("stranger raise: status = %d, want 403: %v", code, body)
	}

	// The member raises; the stranger still cannot view it, the owner
	// and the paying member can.
	code, body = testsupport.DoAuthJSON(t, client, http.MethodPost,
		base+"/api/v1/contributions/"+cid+"/disputes", `{"reason":"wrong amount"}`, joined[0].Token)
	if code != http.StatusCreated {
		t.Fatalf("member raise: status = %d: %v", code, body)
	}
	disputeID, _ := body["id"].(string)

	code, _ = testsupport.DoAuthJSON(t, client, http.MethodGet,
		base+"/api/v1/disputes/"+disputeID, "", strangerTok)
	if code != http.StatusForbidden {
		t.Fatalf("stranger view: status = %d, want 403", code)
	}
	code, _ = testsupport.DoAuthJSON(t, client, http.MethodGet,
		base+"/api/v1/disputes/"+disputeID, "", joined[0].Token)
	if code != http.StatusOK {
		t.Fatalf("raiser view: status = %d, want 200", code)
	}

	// Only the owner decides — and only the owner lists.
	code, body = testsupport.DoAuthJSON(t, client, http.MethodPost,
		base+"/api/v1/disputes/"+disputeID+"/decide", `{"decision":"resolved"}`, joined[0].Token)
	if code != http.StatusForbidden {
		t.Fatalf("member decide: status = %d, want 403: %v", code, body)
	}
	code, _ = testsupport.DoAuthJSON(t, client, http.MethodGet,
		base+"/api/v1/subscriptions/"+subID+"/disputes", "", strangerTok)
	if code != http.StatusForbidden {
		t.Fatalf("stranger list: status = %d, want 403", code)
	}

	// Bad decisions and unknown ids.
	code, body = testsupport.DoAuthJSON(t, client, http.MethodPost,
		base+"/api/v1/disputes/"+disputeID+"/decide", `{"decision":"refund"}`, tok)
	if code != http.StatusUnprocessableEntity {
		t.Fatalf("bad decision: status = %d, want 422: %v", code, body)
	}
	code, _ = testsupport.DoAuthJSON(t, client, http.MethodGet,
		base+"/api/v1/disputes/00000000-0000-0000-0000-000000000000", "", tok)
	if code != http.StatusNotFound {
		t.Fatalf("unknown dispute: status = %d, want 404", code)
	}
	code, _ = testsupport.DoAuthJSON(t, client, http.MethodPost,
		base+"/api/v1/contributions/00000000-0000-0000-0000-000000000000/disputes",
		`{"reason":"ghost"}`, tok)
	if code != http.StatusNotFound {
		t.Fatalf("unknown contribution: status = %d, want 404", code)
	}
}
