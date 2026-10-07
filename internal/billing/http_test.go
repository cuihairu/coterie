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

	// Usage split with an empty ledger has nothing to split.
	code, body := testsupport.DoAuthJSON(t, client, http.MethodPost,
		srv.URL+"/api/v1/billing-periods/"+periodID+"/contributions/generate",
		`{"mode":"usage"}`, tok)
	if code != http.StatusUnprocessableEntity {
		t.Fatalf("empty usage split status = %d, want 422: %v", code, body)
	}

	// Unknown modes are rejected.
	code, body = testsupport.DoAuthJSON(t, client, http.MethodPost,
		srv.URL+"/api/v1/billing-periods/"+periodID+"/contributions/generate",
		`{"mode":"bogus"}`, tok)
	if code != http.StatusUnprocessableEntity {
		t.Fatalf("bogus mode status = %d, want 422: %v", code, body)
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
	tok, _, subID, _, joined := testsupport.SeedCircle(t, client, srv.URL, "bill-seat", "10.00", 4, 2, 2)

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

func TestUsageSplit(t *testing.T) {
	db := testsupport.NewDB(t)
	srv := testsupport.NewServer(t, db)
	client := srv.Client()
	tok, _, subID, cotID, joined := testsupport.SeedCircle(t, client, srv.URL, "bill-usage", "10.00", 2, 0, 2)
	owner := testsupport.Member{MemberID: testsupport.OwnerMemberID(t, client, srv.URL, cotID, tok)}
	m1, m2 := joined[0], joined[1]

	// Seed usage through the API: owner 1 credit, m1 3 credits.
	for _, tc := range []struct{ member, amount string }{
		{owner.MemberID, "1"}, {m1.MemberID, "3"},
	} {
		code, body := testsupport.DoAuthJSON(t, client, http.MethodPost,
			srv.URL+"/api/v1/subscriptions/"+subID+"/usage-records",
			fmt.Sprintf(`{"member_id":%q,"amount":%q,"unit":"credits"}`, tc.member, tc.amount), tok)
		if code != http.StatusCreated {
			t.Fatalf("seed usage %s: status = %d: %v", tc.member, code, body)
		}
	}

	period := openPeriod(t, client, srv.URL, tok, subID, "2026-10-01", "2026-11-01")
	periodID, _ := period["id"].(string)
	resp := generate(t, client, srv.URL, tok, periodID, "usage")
	amounts := amountsOf(t, resp)
	// 10.00 split 1:3 → 2.50 / 7.50, exact.
	if len(amounts) != 2 || amounts[0] != "2.50" || amounts[1] != "7.50" {
		t.Fatalf("usage amounts = %v, want [2.50 7.50]", amounts)
	}

	// Contributions point at the right members: m2 has no usage, so
	// only two rows exist.
	code, body := testsupport.DoAuthJSON(t, client, http.MethodGet,
		srv.URL+"/api/v1/billing-periods/"+periodID+"/contributions", "", tok)
	if code != http.StatusOK {
		t.Fatalf("list contributions: status = %d", code)
	}
	byMember := map[string]string{}
	for _, raw := range body["items"].([]any) {
		c, _ := raw.(map[string]any)
		mid, _ := c["member_id"].(string)
		amt, _ := c["amount"].(string)
		byMember[mid] = amt
	}
	if byMember[owner.MemberID] != "2.50" || byMember[m1.MemberID] != "7.50" {
		t.Fatalf("per-member shares = %v, want owner 2.50 / m1 7.50", byMember)
	}
	if _, ok := byMember[m2.MemberID]; ok {
		t.Fatalf("usage-less member got a contribution: %v", byMember)
	}
}

func TestUsageSplitLargestRemainder(t *testing.T) {
	db := testsupport.NewDB(t)
	srv := testsupport.NewServer(t, db)
	client := srv.Client()
	tok, _, subID, cotID, joined := testsupport.SeedCircle(t, client, srv.URL, "bill-urem", "1.00", 2, 0, 2)
	owner := testsupport.Member{MemberID: testsupport.OwnerMemberID(t, client, srv.URL, cotID, tok)}
	m1, m2 := joined[0], joined[1]

	// 1.00 over three equal usages → 0.34 (earliest joined) + 0.33 + 0.33.
	for _, member := range []string{owner.MemberID, m1.MemberID, m2.MemberID} {
		code, body := testsupport.DoAuthJSON(t, client, http.MethodPost,
			srv.URL+"/api/v1/subscriptions/"+subID+"/usage-records",
			fmt.Sprintf(`{"member_id":%q,"amount":"7","unit":"credits"}`, member), tok)
		if code != http.StatusCreated {
			t.Fatalf("seed usage: status = %d: %v", code, body)
		}
	}

	period := openPeriod(t, client, srv.URL, tok, subID, "2026-10-01", "2026-11-01")
	periodID, _ := period["id"].(string)
	resp := generate(t, client, srv.URL, tok, periodID, "usage")
	amounts := amountsOf(t, resp)
	if len(amounts) != 3 || amounts[0] != "0.33" || amounts[1] != "0.33" || amounts[2] != "0.34" {
		t.Fatalf("remainder amounts = %v, want [0.33 0.33 0.34]", amounts)
	}
	// The extra cent belongs to the earliest-joined member (the owner).
	for _, raw := range resp["items"].([]any) {
		c, _ := raw.(map[string]any)
		mid, _ := c["member_id"].(string)
		amt, _ := c["amount"].(string)
		if mid == owner.MemberID && amt != "0.34" {
			t.Fatalf("owner share = %s, want 0.34 (tie goes to earliest joined)", amt)
		}
	}
}

func TestUsageSplitValidation(t *testing.T) {
	db := testsupport.NewDB(t)
	srv := testsupport.NewServer(t, db)
	client := srv.Client()

	// Mixed units in one window → 422.
	tok, _, subID, _, joined := testsupport.SeedCircle(t, client, srv.URL, "bill-uval", "10.00", 2, 0, 1)
	seed := func(tok, sub, member, amount, unit, at string) {
		code, body := testsupport.DoAuthJSON(t, client, http.MethodPost,
			srv.URL+"/api/v1/subscriptions/"+sub+"/usage-records",
			fmt.Sprintf(`{"member_id":%q,"amount":%q,"unit":%q,"recorded_at":%q}`, member, amount, unit, at), tok)
		if code != http.StatusCreated {
			t.Fatalf("seed usage: status = %d: %v", code, body)
		}
	}
	seed(tok, subID, joined[0].MemberID, "1", "credits", "2026-10-05T10:00:00Z")
	seed(tok, subID, joined[0].MemberID, "1", "GB", "2026-10-06T10:00:00Z")
	period := openPeriod(t, client, srv.URL, tok, subID, "2026-10-01", "2026-11-01")
	periodID, _ := period["id"].(string)
	code, body := testsupport.DoAuthJSON(t, client, http.MethodPost,
		srv.URL+"/api/v1/billing-periods/"+periodID+"/contributions/generate", `{"mode":"usage"}`, tok)
	if code != http.StatusUnprocessableEntity {
		t.Fatalf("mixed units status = %d, want 422: %v", code, body)
	}

	// Records outside the window are invisible → nothing to split.
	tok2, _, sub2, _, joined2 := testsupport.SeedCircle(t, client, srv.URL, "bill-uval2", "10.00", 2, 0, 1)
	seed(tok2, sub2, joined2[0].MemberID, "5", "credits", "2026-09-30T23:59:59Z")
	period = openPeriod(t, client, srv.URL, tok2, sub2, "2026-10-01", "2026-11-01")
	periodID, _ = period["id"].(string)
	code, body = testsupport.DoAuthJSON(t, client, http.MethodPost,
		srv.URL+"/api/v1/billing-periods/"+periodID+"/contributions/generate", `{"mode":"usage"}`, tok2)
	if code != http.StatusUnprocessableEntity {
		t.Fatalf("out-of-window usage status = %d, want 422: %v", code, body)
	}

	// A net-negative member blocks the split even with a positive total.
	tok3, _, sub3, cot3, joined3 := testsupport.SeedCircle(t, client, srv.URL, "bill-uval3", "10.00", 3, 0, 1)
	seed(tok3, sub3, joined3[0].MemberID, "-3", "credits", "2026-10-02T10:00:00Z") // net negative
	code, inv := testsupport.DoAuthJSON(t, client, http.MethodPost,
		srv.URL+"/api/v1/coteries/"+cot3+"/invitations", `{"role":"member"}`, tok3)
	if code != http.StatusCreated {
		t.Fatalf("invite: status = %d: %v", code, inv)
	}
	inviteToken, _ := inv["token"].(string)
	u3, _ := testsupport.RegisterAndLogin(t, client, srv.URL, "bill-uval3-m2")
	code, mem := testsupport.DoAuthJSON(t, client, http.MethodPost,
		srv.URL+"/api/v1/invitations/accept", fmt.Sprintf(`{"token":%q}`, inviteToken), u3)
	if code != http.StatusCreated {
		t.Fatalf("accept: status = %d: %v", code, mem)
	}
	m3, _ := mem["id"].(string)
	seed(tok3, sub3, m3, "2", "credits", "2026-10-04T10:00:00Z") // total +2 > 0, but m1 nets -3
	period = openPeriod(t, client, srv.URL, tok3, sub3, "2026-11-01", "2026-12-01")
	periodID, _ = period["id"].(string)
	code, body = testsupport.DoAuthJSON(t, client, http.MethodPost,
		srv.URL+"/api/v1/billing-periods/"+periodID+"/contributions/generate", `{"mode":"usage"}`, tok3)
	if code != http.StatusUnprocessableEntity {
		t.Fatalf("negative member net status = %d, want 422: %v", code, body)
	}
}

// The prorated split (advanced billing) weights each active member by
// the days they were in the coterie during the period: a member who
// joined mid-period owes only their slice of the window.
func TestProratedSplit(t *testing.T) {
	db := testsupport.NewDB(t)
	srv := testsupport.NewServer(t, db)
	client, base := srv.Client(), srv.URL
	tok, _, subID, _, joined := testsupport.SeedCircle(t, client, base, "bill-prorate", "30.00", 4, 0, 1)

	// Backdate the join instants to stand in for time travel: the owner
	// spans the whole September period (30 days), the member joined
	// 2026-09-25 (6 days). joined_at is write-once over the API, so the
	// raw handle does the backdating.
	if err := db.Exec(`UPDATE members SET joined_at = '2026-08-01 10:00:00+00'`).Error; err != nil {
		t.Fatalf("backdate owner join: %v", err)
	}
	if err := db.Exec(`UPDATE members SET joined_at = '2026-09-25 10:00:00+00' WHERE id = ?`, joined[0].MemberID).Error; err != nil {
		t.Fatalf("backdate member join: %v", err)
	}

	p := openPeriod(t, client, base, tok, subID, "2026-09-01", "2026-09-30")
	pid, _ := p["id"].(string)
	resp := generate(t, client, base, tok, pid, "prorated")

	// 30.00 splits 30:6 → 25.00 + 5.00 exactly (amountsOf sorts as
	// strings, so 25.00 precedes 5.00).
	if got := amountsOf(t, resp); len(got) != 2 || got[0] != "25.00" || got[1] != "5.00" {
		t.Fatalf("prorated amounts = %v, want [25.00 5.00]", got)
	}

	// A period entirely before anyone joined has no chargeable days.
	p = openPeriod(t, client, base, tok, subID, "2026-01-01", "2026-01-31")
	code, body := testsupport.DoAuthJSON(t, client, http.MethodPost,
		base+"/api/v1/billing-periods/"+p["id"].(string)+"/contributions/generate",
		`{"mode":"prorated"}`, tok)
	if code != http.StatusUnprocessableEntity {
		t.Fatalf("empty window: status = %d, want 422: %v", code, body)
	}

	// A period starting after every join gives everyone full weight:
	// an October period splits equally.
	p = openPeriod(t, client, base, tok, subID, "2026-10-01", "2026-10-31")
	resp = generate(t, client, base, tok, p["id"].(string), "prorated")
	if got := amountsOf(t, resp); len(got) != 2 || got[0] != "15.00" || got[1] != "15.00" {
		t.Fatalf("full-window amounts = %v, want [15.00 15.00]", got)
	}
}
