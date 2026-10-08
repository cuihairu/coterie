package usage_test

import (
	"fmt"
	"net/http"
	"testing"

	"github.com/cuihairu/coterie/internal/testsupport"
)

const recordsPath = "/api/v1/subscriptions"

// quotaSeat patches the subscription's first seat into a quota seat and
// assigns it to the member, returning the seat id.
func quotaSeat(t *testing.T, client *http.Client, base, tok, subID, memberID, quota string) string {
	t.Helper()
	code, body := testsupport.DoAuthJSON(t, client, http.MethodGet,
		base+recordsPath+"/"+subID+"/seats", "", tok)
	if code != http.StatusOK {
		t.Fatalf("list seats: status = %d: %v", code, body)
	}
	items, _ := body["items"].([]any)
	if len(items) == 0 {
		t.Fatalf("no seats to patch: %v", body)
	}
	seat, _ := items[0].(map[string]any)
	seatID, _ := seat["id"].(string)

	code, body = testsupport.DoAuthJSON(t, client, http.MethodPatch,
		base+"/api/v1/seats/"+seatID,
		fmt.Sprintf(`{"metadata":{"quota":%q,"unit":"credits","used":0}}`, quota), tok)
	if code != http.StatusOK {
		t.Fatalf("patch quota seat: status = %d: %v", code, body)
	}
	code, body = testsupport.DoAuthJSON(t, client, http.MethodPost,
		base+"/api/v1/seats/"+seatID+"/assign",
		fmt.Sprintf(`{"member_id":%q}`, memberID), tok)
	if code != http.StatusOK {
		t.Fatalf("assign quota seat: status = %d: %v", code, body)
	}
	return seatID
}

func seatMetadata(t *testing.T, client *http.Client, base, tok, seatID string) map[string]any {
	t.Helper()
	code, body := testsupport.DoAuthJSON(t, client, http.MethodGet,
		base+"/api/v1/seats/"+seatID, "", tok)
	if code != http.StatusOK {
		t.Fatalf("get seat: status = %d: %v", code, body)
	}
	md, _ := body["metadata"].(map[string]any)
	return md
}

// record posts one usage record and returns the decoded response.
func record(t *testing.T, client *http.Client, base, tok, subID, payload string) (int, map[string]any) {
	t.Helper()
	return testsupport.DoAuthJSON(t, client, http.MethodPost,
		base+recordsPath+"/"+subID+"/usage-records", payload, tok)
}

func TestRecordUsageProjectsQuotaUsed(t *testing.T) {
	db := testsupport.NewDB(t)
	srv := testsupport.NewServer(t, db)
	client := srv.Client()
	tok, _, subID, _, joined := testsupport.SeedCircle(t, client, srv.URL, "use-ledger", "10.00", 2, 1, 1)
	m := joined[0]
	seatID := quotaSeat(t, client, srv.URL, tok, subID, m.MemberID, "100")

	code, body := record(t, client, srv.URL, tok, subID,
		fmt.Sprintf(`{"member_id":%q,"amount":"12.5","unit":"credits"}`, m.MemberID))
	if code != http.StatusCreated {
		t.Fatalf("record usage: status = %d: %v", code, body)
	}
	if body["seat_id"] != seatID {
		t.Fatalf("record not attributed to the quota seat: %v", body)
	}
	if md := seatMetadata(t, client, srv.URL, tok, seatID); md["used"] != 12.5 {
		t.Fatalf("used = %v, want 12.5", md)
	}

	// A second record accumulates via the ledger sum, not float math.
	code, body = record(t, client, srv.URL, tok, subID,
		fmt.Sprintf(`{"member_id":%q,"amount":"1.25","unit":"credits","metadata":{"source":"api"}}`, m.MemberID))
	if code != http.StatusCreated {
		t.Fatalf("record usage 2: status = %d: %v", code, body)
	}
	if md := seatMetadata(t, client, srv.URL, tok, seatID); md["used"] != 13.75 {
		t.Fatalf("used = %v, want 13.75", md)
	}

	// A negative record corrects the ledger and the projection.
	code, body = record(t, client, srv.URL, tok, subID,
		fmt.Sprintf(`{"member_id":%q,"amount":"-0.75","unit":"credits"}`, m.MemberID))
	if code != http.StatusCreated {
		t.Fatalf("correct usage: status = %d: %v", code, body)
	}
	if md := seatMetadata(t, client, srv.URL, tok, seatID); md["used"] != 13.0 {
		t.Fatalf("used = %v, want 13", md)
	}

	// Single record lookup.
	id, _ := body["id"].(string)
	code, body = testsupport.DoAuthJSON(t, client, http.MethodGet,
		srv.URL+"/api/v1/usage-records/"+id, "", tok)
	if code != http.StatusOK || body["amount"] != "-0.7500" {
		t.Fatalf("get record: status = %d body = %v", code, body)
	}

	// Anonymous reads are rejected like every protected route.
	code, _ = testsupport.DoJSON(t, client, http.MethodGet,
		srv.URL+recordsPath+"/"+subID+"/usage-records", "")
	if code != http.StatusUnauthorized {
		t.Fatalf("anonymous list status = %d, want 401", code)
	}
}

func TestUsageRecordValidation(t *testing.T) {
	db := testsupport.NewDB(t)
	srv := testsupport.NewServer(t, db)
	client := srv.Client()
	tok, _, subID, _, joined := testsupport.SeedCircle(t, client, srv.URL, "use-valid", "10.00", 2, 1, 1)
	m := joined[0]

	other, _ := testsupport.RegisterAndLogin(t, client, srv.URL, "use-valid-other")

	cases := []struct {
		name    string
		token   string
		sub     string
		payload string
		want    int
	}{
		{"unknown member", tok, subID, `{"member_id":"00000000-0000-0000-0000-000000000000","amount":"1","unit":"credits"}`, http.StatusUnprocessableEntity},
		{"owner user id is not a member", tok, subID, `{"member_id":"00000000-0000-0000-0000-000000000abc","amount":"1","unit":"credits"}`, http.StatusUnprocessableEntity},
		{"bad amount", tok, subID, fmt.Sprintf(`{"member_id":%q,"amount":"1.00001","unit":"credits"}`, m.MemberID), http.StatusUnprocessableEntity},
		{"non-numeric amount", tok, subID, fmt.Sprintf(`{"member_id":%q,"amount":"abc","unit":"credits"}`, m.MemberID), http.StatusUnprocessableEntity},
		{"empty unit", tok, subID, fmt.Sprintf(`{"member_id":%q,"amount":"1","unit":""}`, m.MemberID), http.StatusUnprocessableEntity},
		{"bad recorded_at", tok, subID, fmt.Sprintf(`{"member_id":%q,"amount":"1","unit":"credits","recorded_at":"yesterday"}`, m.MemberID), http.StatusUnprocessableEntity},
		{"metadata not an object", tok, subID, fmt.Sprintf(`{"member_id":%q,"amount":"1","unit":"credits","metadata":[1]}`, m.MemberID), http.StatusUnprocessableEntity},
		{"non-owner", other, subID, fmt.Sprintf(`{"member_id":%q,"amount":"1","unit":"credits"}`, m.MemberID), http.StatusForbidden},
		{"unknown subscription", tok, "00000000-0000-0000-0000-00000000e2e4", fmt.Sprintf(`{"member_id":%q,"amount":"1","unit":"credits"}`, m.MemberID), http.StatusNotFound},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			code, body := record(t, client, srv.URL, c.token, c.sub, c.payload)
			if code != c.want {
				t.Fatalf("status = %d, want %d: %v", code, c.want, body)
			}
		})
	}
}

func TestUsageSeatAttribution(t *testing.T) {
	db := testsupport.NewDB(t)
	srv := testsupport.NewServer(t, db)
	client := srv.Client()
	// capacity 2 → two seats; both become quota seats.
	tok, _, subID, _, joined := testsupport.SeedCircle(t, client, srv.URL, "use-seat", "10.00", 3, 2, 1)
	m := joined[0]

	code, seats := testsupport.DoAuthJSON(t, client, http.MethodGet,
		srv.URL+recordsPath+"/"+subID+"/seats", "", tok)
	items, _ := seats["items"].([]any)
	seatA, _ := items[0].(map[string]any)
	seatB, _ := items[1].(map[string]any)
	seatAID, _ := seatA["id"].(string)
	seatBID, _ := seatB["id"].(string)
	for _, sid := range []string{seatAID, seatBID} {
		code, body := testsupport.DoAuthJSON(t, client, http.MethodPatch,
			srv.URL+"/api/v1/seats/"+sid,
			`{"metadata":{"quota":50,"unit":"credits","used":0}}`, tok)
		if code != http.StatusOK {
			t.Fatalf("patch seat: status = %d: %v", code, body)
		}
	}
	code, _ = testsupport.DoAuthJSON(t, client, http.MethodPost,
		srv.URL+"/api/v1/seats/"+seatAID+"/assign",
		fmt.Sprintf(`{"member_id":%q}`, m.MemberID), tok)
	if code != http.StatusOK {
		t.Fatalf("assign seat A: status = %d", code)
	}
	code, _ = testsupport.DoAuthJSON(t, client, http.MethodPost,
		srv.URL+"/api/v1/seats/"+seatBID+"/assign",
		fmt.Sprintf(`{"member_id":%q}`, m.MemberID), tok)
	if code != http.StatusOK {
		t.Fatalf("assign seat B: status = %d", code)
	}

	// Two quota seats and no explicit seat_id → ambiguous → 422.
	code, body := record(t, client, srv.URL, tok, subID,
		fmt.Sprintf(`{"member_id":%q,"amount":"5","unit":"credits"}`, m.MemberID))
	if code != http.StatusUnprocessableEntity {
		t.Fatalf("ambiguous attribution status = %d, want 422: %v", code, body)
	}

	// Explicit seat lands and projects only that seat.
	code, body = record(t, client, srv.URL, tok, subID,
		fmt.Sprintf(`{"member_id":%q,"seat_id":%q,"amount":"5","unit":"credits"}`, m.MemberID, seatBID))
	if code != http.StatusCreated {
		t.Fatalf("explicit attribution status = %d: %v", code, body)
	}
	if md := seatMetadata(t, client, srv.URL, tok, seatBID); md["used"] != 5.0 {
		t.Fatalf("seat B used = %v, want 5", md)
	}
	if md := seatMetadata(t, client, srv.URL, tok, seatAID); md["used"] != 0.0 {
		t.Fatalf("seat A used = %v, want 0", md)
	}

	// A seat of the same subscription occupied by nobody → 422.
	code, _ = testsupport.DoAuthJSON(t, client, http.MethodPost,
		srv.URL+recordsPath+"/"+subID+"/seats", `{"count":1}`, tok)
	if code != http.StatusCreated {
		t.Fatalf("provision extra seat: status = %d", code)
	}
	code, seats = testsupport.DoAuthJSON(t, client, http.MethodGet,
		srv.URL+recordsPath+"/"+subID+"/seats", "", tok)
	var freeSeatID string
	for _, raw := range seats["items"].([]any) {
		st, _ := raw.(map[string]any)
		if st["status"] == "free" {
			freeSeatID, _ = st["id"].(string)
		}
	}
	code, body = record(t, client, srv.URL, tok, subID,
		fmt.Sprintf(`{"member_id":%q,"seat_id":%q,"amount":"1","unit":"credits"}`, m.MemberID, freeSeatID))
	if code != http.StatusUnprocessableEntity {
		t.Fatalf("foreign seat status = %d, want 422: %v", code, body)
	}
}

func TestUsageListFilters(t *testing.T) {
	db := testsupport.NewDB(t)
	srv := testsupport.NewServer(t, db)
	client := srv.Client()
	tok, _, subID, _, joined := testsupport.SeedCircle(t, client, srv.URL, "use-list", "10.00", 2, 1, 2)
	a, b := joined[0], joined[1]

	mk := func(member, unit, at string) {
		code, body := record(t, client, srv.URL, tok, subID,
			fmt.Sprintf(`{"member_id":%q,"amount":"1","unit":%q,"recorded_at":%q}`, member, unit, at))
		if code != http.StatusCreated {
			t.Fatalf("seed record: status = %d: %v", code, body)
		}
	}
	mk(a.MemberID, "credits", "2026-10-01T10:00:00Z")
	mk(a.MemberID, "credits", "2026-10-15T10:00:00Z")
	mk(b.MemberID, "GB", "2026-10-20T10:00:00Z")

	list := func(query string) map[string]any {
		t.Helper()
		code, body := testsupport.DoAuthJSON(t, client, http.MethodGet,
			srv.URL+recordsPath+"/"+subID+"/usage-records"+query, "", tok)
		if code != http.StatusOK {
			t.Fatalf("list: status = %d: %v", code, body)
		}
		return body
	}
	total := func(body map[string]any) float64 {
		meta, _ := body["meta"].(map[string]any)
		n, _ := meta["total"].(float64)
		return n
	}

	if n := total(list("?member_id=" + a.MemberID)); n != 2 {
		t.Fatalf("member filter total = %v, want 2", n)
	}
	if n := total(list("?unit=GB")); n != 1 {
		t.Fatalf("unit filter total = %v, want 1", n)
	}
	if n := total(list("?from=2026-10-10&to=2026-10-20")); n != 2 {
		t.Fatalf("window filter total = %v, want 2", n)
	}
	if n := total(list("")); n != 3 {
		t.Fatalf("unfiltered total = %v, want 3", n)
	}
	if n := total(list("?limit=1")); n != 3 {
		t.Fatalf("paginated total = %v, want 3", n)
	}

	// Bad window boundaries → 422 envelope.
	for _, q := range []string{"?from=nope", "?to=2026-13-40"} {
		code, body := testsupport.DoAuthJSON(t, client, http.MethodGet,
			srv.URL+recordsPath+"/"+subID+"/usage-records"+q, "", tok)
		if code != http.StatusUnprocessableEntity {
			t.Fatalf("bad filter %s status = %d, want 422: %v", q, code, body)
		}
	}
}

// TestUsageLimit covers D21: the sharing policy's usage_limit caps the
// per-member, per-period ledger sum for its unit; corrections shrink
// the judged total; uncovered windows and other units are free.
func TestUsageLimit(t *testing.T) {
	db := testsupport.NewDB(t)
	srv := testsupport.NewServer(t, db)
	client, base := srv.Client(), srv.URL
	tok, _, subID, coterieID, joined := testsupport.SeedCircle(t, client, base, "use-limit", "10.00", 2, 1, 1)
	m := joined[0]

	code, body := testsupport.DoAuthJSON(t, client, http.MethodPatch,
		base+recordsPath+"/"+subID,
		`{"sharing_policy":{"mode":"quota","usage_limit":{"unit":"credits","per_period":"100"}}}`, tok)
	if code != http.StatusOK {
		t.Fatalf("set policy: status = %d: %v", code, body)
	}
	// The October window covers the default recorded_at of new records.
	openPeriodForUsage(t, client, base, tok, subID, "2026-10-01", "2026-10-31")

	post := func(amount, unit, at string) (int, map[string]any) {
		payload := fmt.Sprintf(`{"member_id":%q,"amount":%q,"unit":%q`, m.MemberID, amount, unit)
		if at != "" {
			payload += `,"recorded_at":` + `"` + at + `"`
		}
		payload += "}"
		return record(t, client, base, tok, subID, payload)
	}

	if code, body := post("60", "credits", ""); code != http.StatusCreated {
		t.Fatalf("first record: status = %d: %v", code, body)
	}
	// Exactly at the cap is allowed; one more milli-credit is not.
	if code, body := post("40", "credits", ""); code != http.StatusCreated {
		t.Fatalf("record to the cap: status = %d: %v", code, body)
	}
	code, body = post("0.01", "credits", "")
	if code != http.StatusConflict {
		t.Fatalf("over-cap record: status = %d, want 409: %v", code, body)
	}

	// A correction shrinks the judged sum, so the budget reopens.
	if code, body := post("-10", "credits", ""); code != http.StatusCreated {
		t.Fatalf("correction: status = %d: %v", code, body)
	}
	if code, body := post("10", "credits", ""); code != http.StatusCreated {
		t.Fatalf("record after correction: status = %d: %v", code, body)
	}
	if code, _ := post("0.01", "credits", ""); code != http.StatusConflict {
		t.Fatal("cap must re-engage after the refill")
	}

	// Another unit is not capped.
	if code, body := post("5000", "GB", ""); code != http.StatusCreated {
		t.Fatalf("other unit: status = %d: %v", code, body)
	}
	// A record outside every period is not capped either.
	if code, body := post("5000", "credits", "2026-11-15T10:00:00Z"); code != http.StatusCreated {
		t.Fatalf("uncovered window: status = %d: %v", code, body)
	}

	// The cap is per member: the owner's ledger row has its own budget.
	code, mems := testsupport.DoAuthJSON(t, client, http.MethodGet,
		base+"/api/v1/coteries/"+coterieID+"/members", "", tok)
	if code != http.StatusOK {
		t.Fatalf("list members: status = %d: %v", code, mems)
	}
	var ownerMember string
	for _, it := range mems["items"].([]any) {
		mm := it.(map[string]any)
		if role, _ := mm["role"].(string); role == "owner" {
			ownerMember, _ = mm["id"].(string)
		}
	}
	if ownerMember == "" || ownerMember == m.MemberID {
		t.Fatalf("owner member row = %q", ownerMember)
	}
	payload := fmt.Sprintf(`{"member_id":%q,"amount":"100","unit":"credits"}`, ownerMember)
	if code, body := record(t, client, base, tok, subID, payload); code != http.StatusCreated {
		t.Fatalf("owner budget: status = %d: %v", code, body)
	}
}

// openPeriodForUsage opens a billing period (owner action).
func openPeriodForUsage(t *testing.T, client *http.Client, base, tok, subID, start, end string) string {
	t.Helper()
	code, body := testsupport.DoAuthJSON(t, client, http.MethodPost,
		base+recordsPath+"/"+subID+"/billing-periods",
		fmt.Sprintf(`{"start_date":%q,"end_date":%q}`, start, end), tok)
	if code != http.StatusCreated {
		t.Fatalf("open period: status = %d: %v", code, body)
	}
	id, _ := body["id"].(string)
	return id
}
