package reputation_test

import (
	"net/http"
	"testing"

	"github.com/cuihairu/coterie/internal/testsupport"
)

// TestReputationDerivesFromLedger walks a member through paying one
// share, leaving another pending, and checks the derived report.
func TestReputationDerivesFromLedger(t *testing.T) {
	srv := testsupport.NewServer(t, testsupport.NewDB(t))
	client, base := srv.Client(), srv.URL
	tok, ownerID, subID, _, joined := testsupport.SeedCircle(t, client, base, "rep-basic", "12.00", 4, 0, 1)

	code, period := testsupport.DoAuthJSON(t, client, http.MethodPost,
		base+"/api/v1/subscriptions/"+subID+"/billing-periods",
		`{"start_date":"2026-10-01","end_date":"2026-10-31"}`, tok)
	if code != http.StatusCreated {
		t.Fatalf("create period: status = %d: %v", code, period)
	}
	periodID, _ := period["id"].(string)
	code, resp := testsupport.DoAuthJSON(t, client, http.MethodPost,
		base+"/api/v1/billing-periods/"+periodID+"/contributions/generate",
		`{"mode":"equal"}`, tok)
	if code != http.StatusCreated {
		t.Fatalf("generate: status = %d: %v", code, resp)
	}

	// The member pays; the owner still owes. That's 1 paid + 1 pending
	// for both users → ratio 0.50 each.
	memberPay := map[string]string{}
	for _, it := range resp["items"].([]any) {
		row := it.(map[string]any)
		memberPay[row["member_id"].(string)] = row["id"].(string)
	}
	code, body := testsupport.DoAuthJSON(t, client, http.MethodPost,
		base+"/api/v1/contributions/"+memberPay[joined[0].MemberID]+"/payments",
		`{}`, joined[0].Token)
	if code != http.StatusCreated {
		t.Fatalf("pay: status = %d: %v", code, body)
	}

	for _, tc := range []struct {
		name, uid, ratio string
		paid, pending    int
	}{
		{"member", joined[0].UserID, "1.00", 1, 0},
		{"owner", ownerID, "0.00", 0, 1},
	} {
		code, body = testsupport.DoAuthJSON(t, client, http.MethodGet,
			base+"/api/v1/users/"+tc.uid+"/reputation", "", tok)
		if code != http.StatusOK {
			t.Fatalf("%s reputation: status = %d: %v", tc.name, code, body)
		}
		if body["user_id"] != tc.uid {
			t.Fatalf("%s report user = %v", tc.name, body["user_id"])
		}
		counts := body["contributions"].(map[string]any)
		if counts["paid"] != float64(tc.paid) || counts["pending"] != float64(tc.pending) {
			t.Fatalf("%s counts = %v, want paid %d pending %d", tc.name, counts, tc.paid, tc.pending)
		}
		if body["payment_ratio"] != tc.ratio {
			t.Fatalf("%s ratio = %v, want %s", tc.name, body["payment_ratio"], tc.ratio)
		}
		curs := body["currencies"].([]any)
		if len(curs) != 1 {
			t.Fatalf("%s currencies = %v", tc.name, curs)
		}
		cur := curs[0].(map[string]any)
		if cur["currency"] != "USD" {
			t.Fatalf("%s currency = %v", tc.name, cur)
		}
	}

	// Unknown users 404.
	code, _ = testsupport.DoAuthJSON(t, client, http.MethodGet,
		base+"/api/v1/users/00000000-0000-0000-0000-000000000000/reputation", "", tok)
	if code != http.StatusNotFound {
		t.Fatalf("unknown user: status = %d, want 404", code)
	}

	// Reads sit behind the auth baseline.
	code, body = testsupport.DoJSON(t, client, http.MethodGet,
		base+"/api/v1/users/"+ownerID+"/reputation", "")
	if code != http.StatusUnauthorized {
		t.Fatalf("anonymous read: status = %d, want 401: %v", code, body)
	}
}

// TestReputationAllWaived covers the no-chargeable-history null ratio.
func TestReputationAllWaived(t *testing.T) {
	srv := testsupport.NewServer(t, testsupport.NewDB(t))
	client, base := srv.Client(), srv.URL
	tok, _, subID, _, joined := testsupport.SeedCircle(t, client, base, "rep-waive", "10.00", 4, 0, 1)

	code, period := testsupport.DoAuthJSON(t, client, http.MethodPost,
		base+"/api/v1/subscriptions/"+subID+"/billing-periods",
		`{"start_date":"2026-10-01","end_date":"2026-10-31"}`, tok)
	if code != http.StatusCreated {
		t.Fatalf("create period: status = %d: %v", code, period)
	}
	periodID, _ := period["id"].(string)
	code, resp := testsupport.DoAuthJSON(t, client, http.MethodPost,
		base+"/api/v1/billing-periods/"+periodID+"/contributions/generate",
		`{"mode":"equal"}`, tok)
	if code != http.StatusCreated {
		t.Fatalf("generate: status = %d: %v", code, resp)
	}
	for _, it := range resp["items"].([]any) {
		row := it.(map[string]any)
		cid, _ := row["id"].(string)
		code, body := testsupport.DoAuthJSON(t, client, http.MethodPatch,
			base+"/api/v1/contributions/"+cid, `{"status":"waived"}`, tok)
		if code != http.StatusOK {
			t.Fatalf("waive: status = %d: %v", code, body)
		}
	}

	code, body := testsupport.DoAuthJSON(t, client, http.MethodGet,
		base+"/api/v1/users/"+joined[0].UserID+"/reputation", "", tok)
	if code != http.StatusOK {
		t.Fatalf("reputation: status = %d: %v", code, body)
	}
	if body["payment_ratio"] != nil {
		t.Fatalf("all-waived ratio = %v, want null", body["payment_ratio"])
	}
	if counts := body["contributions"].(map[string]any); counts["waived"] != float64(1) {
		t.Fatalf("waived count = %v", counts)
	}
}
