package billing_test

import (
	"fmt"
	"net/http"
	"testing"

	"github.com/cuihairu/coterie/internal/testsupport"
)

const periodsPath = "/api/v1/subscriptions"

// openPeriod creates a billing period for the subscription.
func openPeriod(t *testing.T, client *http.Client, base, tok, subID, start, end string) map[string]any {
	t.Helper()
	code, body := testsupport.DoAuthJSON(t, client, http.MethodPost,
		base+periodsPath+"/"+subID+"/billing-periods",
		fmt.Sprintf(`{"start_date":%q,"end_date":%q}`, start, end), tok)
	if code != http.StatusCreated {
		t.Fatalf("create period: status = %d: %v", code, body)
	}
	return body
}

// generate runs a split and returns the created contributions.
func generate(t *testing.T, client *http.Client, base, tok, periodID, mode string) map[string]any {
	t.Helper()
	body := `{}`
	if mode != "" {
		body = fmt.Sprintf(`{"mode":%q}`, mode)
	}
	code, resp := testsupport.DoAuthJSON(t, client, http.MethodPost,
		base+"/api/v1/billing-periods/"+periodID+"/contributions/generate", body, tok)
	if code != http.StatusCreated {
		t.Fatalf("generate %q: status = %d: %v", mode, code, resp)
	}
	return resp
}

// amountsOf extracts the sorted amount strings from a generate/list
// response.
func amountsOf(t *testing.T, resp map[string]any) []string {
	t.Helper()
	items, _ := resp["items"].([]any)
	out := make([]string, 0, len(items))
	for _, raw := range items {
		item, _ := raw.(map[string]any)
		amount, _ := item["amount"].(string)
		out = append(out, amount)
	}
	for i := 1; i < len(out); i++ {
		for j := i; j > 0 && out[j] < out[j-1]; j-- {
			out[j], out[j-1] = out[j-1], out[j]
		}
	}
	return out
}

func TestBillingPeriodLifecycle(t *testing.T) {
	db := testsupport.NewDB(t)
	srv := testsupport.NewServer(t, db)
	client := srv.Client()
	tok, _, subID := testsupport.SeedChain(t, client, srv.URL, "bill-per", "30.00", 4)
	other, _ := testsupport.RegisterAndLogin(t, client, srv.URL, "bill-per-other")

	// Non-owner → 403.
	code, body := testsupport.DoAuthJSON(t, client, http.MethodPost,
		srv.URL+periodsPath+"/"+subID+"/billing-periods",
		`{"start_date":"2026-10-01","end_date":"2026-11-01"}`, other)
	if code != http.StatusForbidden {
		t.Fatalf("non-owner create status = %d, want 403: %v", code, body)
	}

	// Validation: end before start, bad dates.
	code, body = testsupport.DoAuthJSON(t, client, http.MethodPost,
		srv.URL+periodsPath+"/"+subID+"/billing-periods",
		`{"start_date":"2026-11-01","end_date":"2026-10-01"}`, tok)
	if code != http.StatusUnprocessableEntity {
		t.Fatalf("end<=start status = %d, want 422: %v", code, body)
	}
	code, body = testsupport.DoAuthJSON(t, client, http.MethodPost,
		srv.URL+periodsPath+"/"+subID+"/billing-periods",
		`{"start_date":"2026-13-99","end_date":"2026-12-01"}`, tok)
	if code != http.StatusUnprocessableEntity {
		t.Fatalf("bad date status = %d, want 422: %v", code, body)
	}

	p := openPeriod(t, client, srv.URL, tok, subID, "2026-10-01", "2026-11-01")
	if p["status"] != "open" {
		t.Fatalf("new period not open: %v", p)
	}
	id, _ := p["id"].(string)

	// Same start date → 409.
	code, body = testsupport.DoAuthJSON(t, client, http.MethodPost,
		srv.URL+periodsPath+"/"+subID+"/billing-periods",
		`{"start_date":"2026-10-01","end_date":"2026-12-01"}`, tok)
	if code != http.StatusConflict {
		t.Fatalf("duplicate start status = %d, want 409: %v", code, body)
	}

	// List.
	code, body = testsupport.DoAuthJSON(t, client, http.MethodGet,
		srv.URL+periodsPath+"/"+subID+"/billing-periods", "", tok)
	if code != http.StatusOK {
		t.Fatalf("list status = %d: %v", code, body)
	}
	if total, _ := body["meta"].(map[string]any)["total"].(float64); total != 1 {
		t.Fatalf("list total = %v, want 1", body["meta"])
	}

	// Close is one-way; generation on a closed period is rejected.
	code, body = testsupport.DoAuthJSON(t, client, http.MethodPost,
		srv.URL+"/api/v1/billing-periods/"+id+"/close", "", tok)
	if code != http.StatusOK || body["status"] != "closed" {
		t.Fatalf("close status = %d body = %v", code, body)
	}
	code, body = testsupport.DoAuthJSON(t, client, http.MethodPost,
		srv.URL+"/api/v1/billing-periods/"+id+"/close", "", tok)
	if code != http.StatusConflict {
		t.Fatalf("double close status = %d, want 409: %v", code, body)
	}
	code, body = testsupport.DoAuthJSON(t, client, http.MethodPost,
		srv.URL+"/api/v1/billing-periods/"+id+"/contributions/generate", `{}`, tok)
	if code != http.StatusConflict {
		t.Fatalf("generate on closed status = %d, want 409: %v", code, body)
	}
}

func TestEqualSplit(t *testing.T) {
	db := testsupport.NewDB(t)
	srv := testsupport.NewServer(t, db)
	client := srv.Client()
	tok, _, subID, coterieID, joined := testsupport.SeedCircle(t, client, srv.URL, "bill-eq", "12.50", 4, 0, 2)
	_ = coterieID

	period := openPeriod(t, client, srv.URL, tok, subID, "2026-10-01", "2026-11-01")
	periodID, _ := period["id"].(string)

	// Usage mode is Phase 2; unknown modes are rejected.
	code, body := testsupport.DoAuthJSON(t, client, http.MethodPost,
		srv.URL+"/api/v1/billing-periods/"+periodID+"/contributions/generate",
		`{"mode":"usage"}`, tok)
	if code != http.StatusUnprocessableEntity {
		t.Fatalf("usage mode status = %d, want 422: %v", code, body)
	}

	resp := generate(t, client, srv.URL, tok, periodID, "")
	amounts := amountsOf(t, resp)
	// 1250 cents over 3 members → 4.17, 4.17, 4.16 in some order.
	if len(amounts) != 3 || amounts[0] != "4.16" || amounts[1] != "4.17" || amounts[2] != "4.17" {
		t.Fatalf("equal split amounts = %v, want [4.16 4.17 4.17]", amounts)
	}
	items, _ := resp["items"].([]any)
	first, _ := items[0].(map[string]any)
	if first["currency"] != "USD" || first["status"] != "pending" {
		t.Fatalf("contribution fields wrong: %v", first)
	}

	// Regeneration is refused; adjust individually instead.
	code, body = testsupport.DoAuthJSON(t, client, http.MethodPost,
		srv.URL+"/api/v1/billing-periods/"+periodID+"/contributions/generate", `{}`, tok)
	if code != http.StatusConflict {
		t.Fatalf("regenerate status = %d, want 409: %v", code, body)
	}

	// List matches.
	code, body = testsupport.DoAuthJSON(t, client, http.MethodGet,
		srv.URL+"/api/v1/billing-periods/"+periodID+"/contributions", "", tok)
	if code != http.StatusOK {
		t.Fatalf("list contributions status = %d: %v", code, body)
	}
	if total, _ := body["meta"].(map[string]any)["total"].(float64); total != 3 {
		t.Fatalf("contributions total = %v, want 3", body["meta"])
	}

	// One of the joined members settles their share.
	memberID := joined[0].MemberID
	code, list := testsupport.DoAuthJSON(t, client, http.MethodGet,
		srv.URL+"/api/v1/billing-periods/"+periodID+"/contributions", "", tok)
	items, _ = list["items"].([]any)
	var contributionID string
	for _, raw := range items {
		item, _ := raw.(map[string]any)
		if item["member_id"] == memberID {
			contributionID, _ = item["id"].(string)
		}
	}
	if contributionID == "" {
		t.Fatal("no contribution found for member")
	}
	code, body = testsupport.DoAuthJSON(t, client, http.MethodPatch,
		srv.URL+"/api/v1/contributions/"+contributionID, `{"status":"paid"}`, tok)
	if code != http.StatusOK || body["status"] != "paid" {
		t.Fatalf("mark paid status = %d body = %v", code, body)
	}
	if _, ok := body["paid_at"]; !ok {
		t.Fatalf("paid_at missing after settlement: %v", body)
	}

	// Amount edits are frozen once settled.
	code, body = testsupport.DoAuthJSON(t, client, http.MethodPatch,
		srv.URL+"/api/v1/contributions/"+contributionID, `{"amount":"1.00"}`, tok)
	if code != http.StatusConflict {
		t.Fatalf("edit settled amount status = %d, want 409: %v", code, body)
	}

	// Waiving and cancelling keep paid_at clear.
	code, body = testsupport.DoAuthJSON(t, client, http.MethodPatch,
		srv.URL+"/api/v1/contributions/"+contributionID, `{"status":"waived"}`, tok)
	if code != http.StatusOK || body["status"] != "waived" {
		t.Fatalf("waive status = %d body = %v", code, body)
	}
	if _, ok := body["paid_at"]; ok {
		t.Fatalf("paid_at must clear when leaving paid: %v", body)
	}

	// Non-owner cannot touch settlement.
	code, body = testsupport.DoAuthJSON(t, client, http.MethodPatch,
		srv.URL+"/api/v1/contributions/"+contributionID, `{"status":"paid"}`, joined[1].Token)
	if code != http.StatusForbidden {
		t.Fatalf("non-owner settlement status = %d, want 403: %v", code, body)
	}
}

func TestPerSeatSplit(t *testing.T) {
	db := testsupport.NewDB(t)
	srv := testsupport.NewServer(t, db)
	client := srv.Client()
	tok, _, subID, _, joined := testsupport.SeedCircle(t, client, srv.URL, "bill-seat", "10.00", 2, 2, 2)

	// Occupy both seats: one per joined member.
	code, seats := testsupport.DoAuthJSON(t, client, http.MethodPost,
		srv.URL+"/api/v1/subscriptions/"+subID+"/seats", `{"count":2}`, tok)
	if code != http.StatusCreated {
		t.Fatalf("provision seats: status = %d: %v", code, seats)
	}
	seatItems, _ := seats["items"].([]any)
	for i, raw := range seatItems {
		seat, _ := raw.(map[string]any)
		seatID, _ := seat["id"].(string)
		code, body := testsupport.DoAuthJSON(t, client, http.MethodPost,
			srv.URL+"/api/v1/seats/"+seatID+"/assign",
			fmt.Sprintf(`{"member_id":%q}`, joined[i].MemberID), tok)
		if code != http.StatusOK {
			t.Fatalf("assign seat %d: status = %d: %v", i, code, body)
		}
	}

	period := openPeriod(t, client, srv.URL, tok, subID, "2026-10-01", "2026-11-01")
	periodID, _ := period["id"].(string)

	// 1000 cents over 2 occupied seats → 5.00 each; the owner holds no
	// seat and gets no contribution.
	resp := generate(t, client, srv.URL, tok, periodID, "per_seat")
	amounts := amountsOf(t, resp)
	if len(amounts) != 2 || amounts[0] != "5.00" || amounts[1] != "5.00" {
		t.Fatalf("per-seat amounts = %v, want [5.00 5.00]", amounts)
	}

	// Release one seat: the remaining occupant carries the whole price.
	seat0, _ := seatItems[0].(map[string]any)
	seat0ID, _ := seat0["id"].(string)
	code, body := testsupport.DoAuthJSON(t, client, http.MethodPost,
		srv.URL+"/api/v1/seats/"+seat0ID+"/release", "", tok)
	if code != http.StatusOK {
		t.Fatalf("release seat: status = %d: %v", code, body)
	}
	period2 := openPeriod(t, client, srv.URL, tok, subID, "2026-11-01", "2026-12-01")
	code, body = testsupport.DoAuthJSON(t, client, http.MethodPost,
		srv.URL+"/api/v1/billing-periods/"+period2["id"].(string)+"/contributions/generate",
		`{"mode":"per_seat"}`, tok)
	if code != http.StatusCreated {
		t.Fatalf("one-seat split status = %d, want 201: %v", code, body)
	}
	if amounts := amountsOf(t, body); len(amounts) != 1 || amounts[0] != "10.00" {
		t.Fatalf("one-seat amounts = %v, want [10.00]", amounts)
	}
}

func TestFixedSplit(t *testing.T) {
	db := testsupport.NewDB(t)
	srv := testsupport.NewServer(t, db)
	client := srv.Client()
	tok, _, subID, _, joined := testsupport.SeedCircle(t, client, srv.URL, "bill-fix", "30.00", 4, 0, 2)

	period := openPeriod(t, client, srv.URL, tok, subID, "2026-10-01", "2026-11-01")
	periodID, _ := period["id"].(string)

	// Invalid items → 422 (bad amount, non-member, duplicate).
	code, body := testsupport.DoAuthJSON(t, client, http.MethodPost,
		srv.URL+"/api/v1/billing-periods/"+periodID+"/contributions/generate",
		fmt.Sprintf(`{"mode":"fixed","items":[{"member_id":%q,"amount":"-1.00"}]}`, joined[0].MemberID), tok)
	if code != http.StatusUnprocessableEntity {
		t.Fatalf("negative fixed amount status = %d, want 422: %v", code, body)
	}
	code, body = testsupport.DoAuthJSON(t, client, http.MethodPost,
		srv.URL+"/api/v1/billing-periods/"+periodID+"/contributions/generate",
		`{"mode":"fixed","items":[{"member_id":"00000000-0000-0000-0000-000000000000","amount":"3.00"}]}`, tok)
	if code != http.StatusUnprocessableEntity {
		t.Fatalf("non-member fixed amount status = %d, want 422: %v", code, body)
	}

	code, body = testsupport.DoAuthJSON(t, client, http.MethodPost,
		srv.URL+"/api/v1/billing-periods/"+periodID+"/contributions/generate",
		fmt.Sprintf(`{"mode":"fixed","items":[{"member_id":%q,"amount":"20.00"},{"member_id":%q,"amount":"10.00"}]}`,
			joined[0].MemberID, joined[1].MemberID), tok)
	if code != http.StatusCreated {
		t.Fatalf("fixed generate status = %d, want 201: %v", code, body)
	}
	amounts := amountsOf(t, body)
	if len(amounts) != 2 || amounts[0] != "10.00" || amounts[1] != "20.00" {
		t.Fatalf("fixed amounts = %v, want [10.00 20.00]", amounts)
	}
}
