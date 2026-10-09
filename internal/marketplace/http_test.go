package marketplace_test

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/cuihairu/coterie/internal/app"
	"github.com/cuihairu/coterie/internal/payment"
	"github.com/cuihairu/coterie/internal/testsupport"
)

const dirPath = "/api/v1/marketplace/coteries"

// publish sets the coterie's listing (owner action) and fails the test
// on a non-200.
func publish(t *testing.T, client *http.Client, base, tok, coterieID, listing string) {
	t.Helper()
	code, body := testsupport.DoAuthJSON(t, client, http.MethodPatch,
		base+"/api/v1/coteries/"+coterieID, fmt.Sprintf(`{"listing":%q}`, listing), tok)
	if code != http.StatusOK {
		t.Fatalf("patch listing: status = %d: %v", code, body)
	}
}

// directory returns the public directory listing.
func directory(t *testing.T, client *http.Client, base, query string) []map[string]any {
	t.Helper()
	code, body := testsupport.DoJSON(t, client, http.MethodGet, base+dirPath+query, "")
	if code != http.StatusOK {
		t.Fatalf("directory: status = %d: %v", code, body)
	}
	items, _ := body["items"].([]any)
	out := make([]map[string]any, 0, len(items))
	for _, it := range items {
		out = append(out, it.(map[string]any))
	}
	return out
}

// directoryHas reports whether the entry with the coterie id exists.
func directoryHas(entries []map[string]any, coterieID string) bool {
	for _, e := range entries {
		if e["coterie_id"] == coterieID {
			return true
		}
	}
	return false
}

// entryByID returns the directory entry for the coterie id.
func entryByID(t *testing.T, entries []map[string]any, coterieID string) map[string]any {
	t.Helper()
	for _, e := range entries {
		if e["coterie_id"] == coterieID {
			return e
		}
	}
	t.Fatalf("coterie %s not in directory: %v", coterieID, entries)
	return nil
}

// createRequest posts a join request and returns (status, body).
func createRequest(t *testing.T, client *http.Client, base, tok, coterieID, payload string) (int, map[string]any) {
	t.Helper()
	return testsupport.DoAuthJSON(t, client, http.MethodPost,
		base+"/api/v1/coteries/"+coterieID+"/join-requests", payload, tok)
}

// inbox lists the coterie's join requests as owner/admin.
func inbox(t *testing.T, client *http.Client, base, tok, coterieID, status string) []map[string]any {
	t.Helper()
	q := ""
	if status != "" {
		q = "?status=" + status
	}
	code, body := testsupport.DoAuthJSON(t, client, http.MethodGet,
		base+"/api/v1/coteries/"+coterieID+"/join-requests"+q, "", tok)
	if code != http.StatusOK {
		t.Fatalf("list join requests: status = %d: %v", code, body)
	}
	items, _ := body["items"].([]any)
	out := make([]map[string]any, 0, len(items))
	for _, it := range items {
		out = append(out, it.(map[string]any))
	}
	return out
}

// decide posts accept/decline on a join request.
func decide(t *testing.T, client *http.Client, base, tok, action, requestID string) (int, map[string]any) {
	t.Helper()
	return testsupport.DoAuthJSON(t, client, http.MethodPost,
		base+"/api/v1/join-requests/"+requestID+"/"+action, "", tok)
}

// memberUserIDs returns the coterie's active member user ids.
func memberUserIDs(t *testing.T, client *http.Client, base, tok, coterieID string) map[string]bool {
	t.Helper()
	code, body := testsupport.DoAuthJSON(t, client, http.MethodGet,
		base+"/api/v1/coteries/"+coterieID+"/members", "", tok)
	if code != http.StatusOK {
		t.Fatalf("list members: status = %d: %v", code, body)
	}
	ids := map[string]bool{}
	items, _ := body["items"].([]any)
	for _, it := range items {
		m := it.(map[string]any)
		ids[m["user_id"].(string)] = true
	}
	return ids
}

// notificationTypes returns the actor's notification types.
func notificationTypes(t *testing.T, client *http.Client, base, tok string) []string {
	t.Helper()
	code, body := testsupport.DoAuthJSON(t, client, http.MethodGet,
		base+"/api/v1/notifications", "", tok)
	if code != http.StatusOK {
		t.Fatalf("list notifications: status = %d: %v", code, body)
	}
	var types []string
	items, _ := body["items"].([]any)
	for _, it := range items {
		n := it.(map[string]any)
		types = append(types, n["type"].(string))
	}
	return types
}

func hasType(types []string, want string) bool {
	for _, ty := range types {
		if ty == want {
			return true
		}
	}
	return false
}

// firstFreeSeat returns the subscription's first free seat id.
func firstFreeSeat(t *testing.T, client *http.Client, base, tok, subID string) string {
	t.Helper()
	code, body := testsupport.DoAuthJSON(t, client, http.MethodGet,
		base+"/api/v1/subscriptions/"+subID+"/seats", "", tok)
	if code != http.StatusOK {
		t.Fatalf("list seats: status = %d: %v", code, body)
	}
	items, _ := body["items"].([]any)
	for _, it := range items {
		s := it.(map[string]any)
		if s["status"] == "free" {
			return s["id"].(string)
		}
	}
	t.Fatalf("no free seat: %v", body)
	return ""
}

func TestDirectoryVisibility(t *testing.T) {
	srv := testsupport.NewServer(t, testsupport.NewDB(t))
	client, base := srv.Client(), srv.URL

	// Two independent circles; only A gets listed.
	tokA, _, _, coterieA, _ := testsupport.SeedCircle(t, client, base, "dir-a", "10.00", 4, 2, 0)
	tokB, _, _, coterieB, _ := testsupport.SeedCircle(t, client, base, "dir-b", "8.00", 4, 2, 0)

	entries := directory(t, client, base, "")
	if directoryHas(entries, coterieA) || directoryHas(entries, coterieB) {
		t.Fatalf("private coteries must not be listed: %v", entries)
	}

	publish(t, client, base, tokA, coterieA, "public")
	publish(t, client, base, tokB, coterieB, "public")

	entries = directory(t, client, base, "")
	if !directoryHas(entries, coterieA) || !directoryHas(entries, coterieB) {
		t.Fatalf("listed coteries missing from directory: %v", entries)
	}

	// Product filter narrows to one product's circle.
	eA := entryByID(t, entries, coterieA)
	productA, _ := eA["product_id"].(string)
	if productA == "" {
		t.Fatalf("directory entry lacks product_id: %v", eA)
	}
	entries = directory(t, client, base, "?product_id="+productA)
	if !directoryHas(entries, coterieA) || directoryHas(entries, coterieB) {
		t.Fatalf("product filter mismatch: %v", entries)
	}
}

func TestDirectoryEntryShape(t *testing.T) {
	srv := testsupport.NewServer(t, testsupport.NewDB(t))
	client, base := srv.Client(), srv.URL

	// Owner + 1 invited member over 3 seats: full=false, estimate = 10.00 / 2.
	tok, _, _, coterieID, _ := testsupport.SeedCircle(t, client, base, "dir-shape", "10.00", 4, 3, 1)
	publish(t, client, base, tok, coterieID, "public")

	e := entryByID(t, directory(t, client, base, ""), coterieID)
	if e["name"] != "Circle dir-shape" {
		t.Fatalf("name = %v", e["name"])
	}
	if e["price"] != "10.00" || e["currency"] != "USD" {
		t.Fatalf("price/currency = %v/%v", e["price"], e["currency"])
	}
	if e["product_name"] != "Seed Product dir-shape" || e["provider_name"] != "Seed Provider dir-shape" {
		t.Fatalf("product/provider name = %v/%v", e["product_name"], e["provider_name"])
	}
	if e["member_count"] != float64(2) {
		t.Fatalf("member_count = %v", e["member_count"])
	}
	if e["seats_total"] != float64(3) || e["seats_free"] != float64(3) {
		t.Fatalf("seats = %v/%v", e["seats_total"], e["seats_free"])
	}
	if e["full"] != false {
		t.Fatalf("full = %v", e["full"])
	}
	if e["share_estimate"] != "5.00" {
		t.Fatalf("share_estimate = %v, want 5.00", e["share_estimate"])
	}
}

func TestListingValidation(t *testing.T) {
	srv := testsupport.NewServer(t, testsupport.NewDB(t))
	client, base := srv.Client(), srv.URL
	tok, _, _, coterieID, joined := testsupport.SeedCircle(t, client, base, "dir-val", "10.00", 4, 2, 1)

	code, body := testsupport.DoAuthJSON(t, client, http.MethodPatch,
		base+"/api/v1/coteries/"+coterieID, `{"listing":"bogus"}`, tok)
	if code != http.StatusUnprocessableEntity {
		t.Fatalf("bogus listing: status = %d: %v", code, body)
	}

	// Non-owner (plain member) may not change exposure.
	code, body = testsupport.DoAuthJSON(t, client, http.MethodPatch,
		base+"/api/v1/coteries/"+coterieID, `{"listing":"public"}`, joined[0].Token)
	if code != http.StatusForbidden {
		t.Fatalf("member patch listing: status = %d: %v", code, body)
	}
}

func TestJoinRequestLifecycle(t *testing.T) {
	srv := testsupport.NewServer(t, testsupport.NewDB(t))
	client, base := srv.Client(), srv.URL
	tok, _, _, coterieID, joined := testsupport.SeedCircle(t, client, base, "jr-life", "12.00", 4, 3, 1)
	publish(t, client, base, tok, coterieID, "public")

	reqToken, reqUserID := testsupport.RegisterAndLogin(t, client, base, "jr-life-req")

	// Private circles 404 — existence stays hidden even to a fresh user.
	// (coterieID here is public; check the other circle from a second
	// seed instead.)
	_, _, _, hiddenID, _ := testsupport.SeedCircle(t, client, base, "jr-hidden", "9.00", 4, 2, 0)
	code, body := createRequest(t, client, base, reqToken, hiddenID, `{}`)
	if code != http.StatusNotFound {
		t.Fatalf("private join request: status = %d: %v", code, body)
	}

	code, body = createRequest(t, client, base, reqToken, coterieID, `{"message":"count me in"}`)
	if code != http.StatusCreated {
		t.Fatalf("create join request: status = %d: %v", code, body)
	}
	if body["status"] != "pending" || body["message"] != "count me in" {
		t.Fatalf("request body = %v", body)
	}
	requestID, _ := body["id"].(string)

	// Duplicate pending → 409.
	code, body = createRequest(t, client, base, reqToken, coterieID, `{}`)
	if code != http.StatusConflict {
		t.Fatalf("duplicate join request: status = %d: %v", code, body)
	}

	// Non-member and plain member cannot see the inbox.
	code, body = testsupport.DoAuthJSON(t, client, http.MethodGet,
		base+"/api/v1/coteries/"+coterieID+"/join-requests", "", reqToken)
	if code != http.StatusForbidden {
		t.Fatalf("requester inbox: status = %d: %v", code, body)
	}
	code, body = testsupport.DoAuthJSON(t, client, http.MethodGet,
		base+"/api/v1/coteries/"+coterieID+"/join-requests", "", joined[0].Token)
	if code != http.StatusForbidden {
		t.Fatalf("member inbox: status = %d: %v", code, body)
	}

	// Owner sees the pending request with the requester's username.
	views := inbox(t, client, base, tok, coterieID, "pending")
	if len(views) != 1 || views[0]["id"] != requestID {
		t.Fatalf("pending inbox = %v", views)
	}
	if views[0]["user_id"] != reqUserID || views[0]["username"] == "" {
		t.Fatalf("inbox view = %v", views[0])
	}

	// Owner notified about the request.
	if !hasType(notificationTypes(t, client, base, tok), "join_requested") {
		t.Fatalf("owner notifications missing join_requested")
	}

	// Accept admits the requester as a member.
	code, body = decide(t, client, base, tok, "accept", requestID)
	if code != http.StatusOK {
		t.Fatalf("accept: status = %d: %v", code, body)
	}
	if body["status"] != "accepted" || body["decided_at"] == nil {
		t.Fatalf("accepted body = %v", body)
	}
	if !memberUserIDs(t, client, base, tok, coterieID)[reqUserID] {
		t.Fatalf("requester %s not a member after accept", reqUserID)
	}
	if !hasType(notificationTypes(t, client, base, reqToken), "join_decided") {
		t.Fatalf("requester notifications missing join_decided")
	}

	// Accepted member may not re-request.
	code, body = createRequest(t, client, base, reqToken, coterieID, `{}`)
	if code != http.StatusConflict {
		t.Fatalf("member re-request: status = %d: %v", code, body)
	}

	// Pending inbox is now empty.
	if views := inbox(t, client, base, tok, coterieID, "pending"); len(views) != 0 {
		t.Fatalf("pending inbox after accept = %v", views)
	}
}

func TestJoinRequestDeclineAndReapply(t *testing.T) {
	srv := testsupport.NewServer(t, testsupport.NewDB(t))
	client, base := srv.Client(), srv.URL
	tok, _, _, coterieID, _ := testsupport.SeedCircle(t, client, base, "jr-dec", "12.00", 4, 2, 0)
	publish(t, client, base, tok, coterieID, "public")

	reqToken, _ := testsupport.RegisterAndLogin(t, client, base, "jr-dec-req")

	code, body := createRequest(t, client, base, reqToken, coterieID, `{}`)
	if code != http.StatusCreated {
		t.Fatalf("create: status = %d: %v", code, body)
	}
	requestID, _ := body["id"].(string)

	code, body = decide(t, client, base, tok, "decline", requestID)
	if code != http.StatusOK {
		t.Fatalf("decline: status = %d: %v", code, body)
	}
	if body["status"] != "declined" {
		t.Fatalf("declined body = %v", body)
	}
	if !hasType(notificationTypes(t, client, base, reqToken), "join_decided") {
		t.Fatalf("requester notifications missing join_decided")
	}

	// Declined is terminal — no pending row left, so re-applying works.
	code, body = createRequest(t, client, base, reqToken, coterieID, `{}`)
	if code != http.StatusCreated {
		t.Fatalf("re-apply: status = %d: %v", code, body)
	}
}

func TestJoinRequestCancel(t *testing.T) {
	srv := testsupport.NewServer(t, testsupport.NewDB(t))
	client, base := srv.Client(), srv.URL
	tok, _, _, coterieID, _ := testsupport.SeedCircle(t, client, base, "jr-cancel", "12.00", 4, 2, 0)
	publish(t, client, base, tok, coterieID, "public")

	reqToken, _ := testsupport.RegisterAndLogin(t, client, base, "jr-cancel-req")
	otherToken, _ := testsupport.RegisterAndLogin(t, client, base, "jr-cancel-other")

	code, body := createRequest(t, client, base, reqToken, coterieID, `{}`)
	if code != http.StatusCreated {
		t.Fatalf("create: status = %d: %v", code, body)
	}
	requestID, _ := body["id"].(string)

	// Only the requester may withdraw.
	code, body = testsupport.DoAuthJSON(t, client, http.MethodDelete,
		base+"/api/v1/join-requests/"+requestID, "", otherToken)
	if code != http.StatusForbidden {
		t.Fatalf("stranger cancel: status = %d: %v", code, body)
	}

	code, body = testsupport.DoAuthJSON(t, client, http.MethodDelete,
		base+"/api/v1/join-requests/"+requestID, "", reqToken)
	if code != http.StatusNoContent {
		t.Fatalf("cancel: status = %d: %v", code, body)
	}

	views := inbox(t, client, base, tok, coterieID, "")
	if len(views) != 1 || views[0]["status"] != "cancelled" {
		t.Fatalf("inbox after cancel = %v", views)
	}
	if views := inbox(t, client, base, tok, coterieID, "pending"); len(views) != 0 {
		t.Fatalf("pending inbox after cancel = %v", views)
	}

	// Cancelled is terminal too — re-applying works.
	code, body = createRequest(t, client, base, reqToken, coterieID, `{}`)
	if code != http.StatusCreated {
		t.Fatalf("re-apply after cancel: status = %d: %v", code, body)
	}

	// Unknown request id → 404.
	code, body = testsupport.DoAuthJSON(t, client, http.MethodDelete,
		base+"/api/v1/join-requests/00000000-0000-0000-0000-000000000000", "", reqToken)
	if code != http.StatusNotFound {
		t.Fatalf("unknown cancel: status = %d: %v", code, body)
	}
}

func TestJoinRequestFullConflict(t *testing.T) {
	srv := testsupport.NewServer(t, testsupport.NewDB(t))
	client, base := srv.Client(), srv.URL
	// Capacity 1: the one seat makes the circle full once assigned.
	tok, _, subID, coterieID, joined := testsupport.SeedCircle(t, client, base, "jr-full", "12.00", 4, 1, 1)
	publish(t, client, base, tok, coterieID, "public")

	reqToken, reqUserID := testsupport.RegisterAndLogin(t, client, base, "jr-full-req")

	// Not full yet (seat unassigned): request files fine.
	code, body := createRequest(t, client, base, reqToken, coterieID, `{}`)
	if code != http.StatusCreated {
		t.Fatalf("create: status = %d: %v", code, body)
	}
	requestID, _ := body["id"].(string)

	// Fill the only seat.
	seatID := firstFreeSeat(t, client, base, tok, subID)
	code, body = testsupport.DoAuthJSON(t, client, http.MethodPost,
		base+"/api/v1/seats/"+seatID+"/assign", fmt.Sprintf(`{"member_id":%q}`, joined[0].MemberID), tok)
	if code != http.StatusOK {
		t.Fatalf("assign seat: status = %d: %v", code, body)
	}

	// Full circle: creating a request → 409.
	code, body = createRequest(t, client, base, reqToken, coterieID, `{}`)
	if code != http.StatusConflict {
		t.Fatalf("full create: status = %d: %v", code, body)
	}

	// Accept re-checks capacity: 409, and the request stays pending.
	code, body = decide(t, client, base, tok, "accept", requestID)
	if code != http.StatusConflict {
		t.Fatalf("accept while full: status = %d: %v", code, body)
	}
	if views := inbox(t, client, base, tok, coterieID, "pending"); len(views) != 1 {
		t.Fatalf("request must stay pending after failed accept: %v", views)
	}

	// Removing the seat holder frees the seat; the retry succeeds.
	code, body = testsupport.DoAuthJSON(t, client, http.MethodDelete,
		base+"/api/v1/members/"+joined[0].MemberID, "", tok)
	if code != http.StatusNoContent {
		t.Fatalf("remove member: status = %d: %v", code, body)
	}
	code, body = decide(t, client, base, tok, "accept", requestID)
	if code != http.StatusOK {
		t.Fatalf("retry accept: status = %d: %v", code, body)
	}
	if !memberUserIDs(t, client, base, tok, coterieID)[reqUserID] {
		t.Fatalf("requester not a member after retry accept")
	}
}

func TestDirectoryOwnerBadge(t *testing.T) {
	srv := testsupport.NewServer(t, testsupport.NewDB(t))
	client, base := srv.Client(), srv.URL
	tok, ownerID, subID, coterieID, joined := testsupport.SeedCircle(t, client, base, "dir-badge", "12.00", 4, 2, 1)
	publish(t, client, base, tok, coterieID, "public")

	// No billing history yet: counts at zero, ratio null (never a
	// fake 100%), and the slot is visible to anonymous browsers.
	badge := entryByID(t, directory(t, client, base, ""), coterieID)["owner"].(map[string]any)
	if badge["user_id"] != ownerID {
		t.Fatalf("badge owner = %v, want %v", badge["user_id"], ownerID)
	}
	if u, _ := badge["username"].(string); u == "" {
		t.Fatalf("badge missing username: %v", badge)
	}
	counts := badge["contributions"].(map[string]any)
	for _, k := range []string{"paid", "pending", "waived", "cancelled"} {
		if counts[k] != float64(0) {
			t.Fatalf("empty-history %s = %v", k, counts[k])
		}
	}
	if badge["payment_ratio"] != nil {
		t.Fatalf("empty-history ratio = %v, want null", badge["payment_ratio"])
	}
	if _, leaked := badge["currencies"]; leaked {
		t.Fatalf("amounts leaked into the public badge: %v", badge)
	}

	// Generate the split: owner and member each owe one share. The
	// member pays; the owner still owes → owner ratio 0.00.
	code, period := testsupport.DoAuthJSON(t, client, http.MethodPost,
		base+"/api/v1/subscriptions/"+subID+"/billing-periods",
		`{"start_date":"2026-10-01","end_date":"2026-10-31"}`, tok)
	if code != http.StatusCreated {
		t.Fatalf("create period: status = %d: %v", code, period)
	}
	periodID, _ := period["id"].(string)
	code, gen := testsupport.DoAuthJSON(t, client, http.MethodPost,
		base+"/api/v1/billing-periods/"+periodID+"/contributions/generate",
		`{"mode":"equal"}`, tok)
	if code != http.StatusCreated {
		t.Fatalf("generate: status = %d: %v", code, gen)
	}
	pay := map[string]string{}
	for _, it := range gen["items"].([]any) {
		row := it.(map[string]any)
		pay[row["member_id"].(string)] = row["id"].(string)
	}
	code, body := testsupport.DoAuthJSON(t, client, http.MethodPost,
		base+"/api/v1/contributions/"+pay[joined[0].MemberID]+"/payments",
		`{}`, joined[0].Token)
	if code != http.StatusCreated {
		t.Fatalf("pay: status = %d: %v", code, body)
	}

	badge = entryByID(t, directory(t, client, base, ""), coterieID)["owner"].(map[string]any)
	counts = badge["contributions"].(map[string]any)
	if counts["paid"] != float64(0) || counts["pending"] != float64(1) {
		t.Fatalf("owner counts = %v", counts)
	}
	if badge["payment_ratio"] != "0.00" {
		t.Fatalf("owner ratio = %v, want 0.00", badge["payment_ratio"])
	}

	// The owner settles their own share → ratio 1.00.
	code, body = testsupport.DoAuthJSON(t, client, http.MethodGet,
		base+"/api/v1/billing-periods/"+periodID+"/contributions", "", tok)
	if code != http.StatusOK {
		t.Fatalf("list contributions: status = %d: %v", code, body)
	}
	ownerMemberID := memberUserIDs(t, client, base, tok, coterieID)
	var ownerContribution string
	for _, it := range body["items"].([]any) {
		row := it.(map[string]any)
		if ownerMemberID[ownerID] && row["member_id"] == ownerMemberIDOf(t, client, base, tok, coterieID, ownerID) {
			ownerContribution, _ = row["id"].(string)
		}
	}
	if ownerContribution == "" {
		t.Fatalf("owner contribution not found")
	}
	code, body = testsupport.DoAuthJSON(t, client, http.MethodPost,
		base+"/api/v1/contributions/"+ownerContribution+"/payments", `{}`, tok)
	if code != http.StatusCreated {
		t.Fatalf("owner pay: status = %d: %v", code, body)
	}

	badge = entryByID(t, directory(t, client, base, ""), coterieID)["owner"].(map[string]any)
	if badge["payment_ratio"] != "1.00" {
		t.Fatalf("owner ratio = %v, want 1.00", badge["payment_ratio"])
	}
	counts = badge["contributions"].(map[string]any)
	if counts["paid"] != float64(1) || counts["pending"] != float64(0) {
		t.Fatalf("owner counts after settle = %v", counts)
	}
}

// ownerMemberIDOf returns the member row id behind the owner's user
// account in the coterie.
func ownerMemberIDOf(t *testing.T, client *http.Client, base, tok, coterieID, ownerID string) string {
	t.Helper()
	code, body := testsupport.DoAuthJSON(t, client, http.MethodGet,
		base+"/api/v1/coteries/"+coterieID+"/members", "", tok)
	if code != http.StatusOK {
		t.Fatalf("list members: status = %d: %v", code, body)
	}
	for _, it := range body["items"].([]any) {
		m := it.(map[string]any)
		if m["user_id"] == ownerID {
			id, _ := m["id"].(string)
			return id
		}
	}
	t.Fatalf("owner member row not found in %v", body["items"])
	return ""
}

// signGate produces a Stripe-Signature header value for the payload,
// mirroring the payments test helper (the adapter is shared).
func signGate(t *testing.T, secret string, ts time.Time, payload []byte) string {
	t.Helper()
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(strconv.FormatInt(ts.Unix(), 10)))
	mac.Write([]byte("."))
	mac.Write(payload)
	return "t=" + strconv.FormatInt(ts.Unix(), 10) + ",v1=" + hex.EncodeToString(mac.Sum(nil))
}

// deliverGate posts a signed webhook to the gate's public endpoint.
func deliverGate(t *testing.T, client *http.Client, base, secret, payload string) (int, map[string]any) {
	t.Helper()
	req, err := http.NewRequest(http.MethodPost,
		base+"/api/v1/marketplace/webhooks/stripe", strings.NewReader(payload))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Stripe-Signature", signGate(t, secret, time.Now(), []byte(payload)))
	res, err := client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = res.Body.Close() }()
	var out map[string]any
	_ = json.NewDecoder(res.Body).Decode(&out)
	return res.StatusCode, out
}

// gateFixture lists the owner's circle and files a pending join request
// from a fresh user, ready for the gate flow.
func gateFixture(t *testing.T, client *http.Client, base, tag string) (ownerTok, reqTok, coterieID, requestID string) {
	t.Helper()
	ownerTok, _, _, coterieID, _ = testsupport.SeedCircle(t, client, base, tag, "12.00", 4, 2, 0)
	publish(t, client, base, ownerTok, coterieID, "public")

	// Flip the gate on — owner-only, audit-logged like the listing.
	code, body := testsupport.DoAuthJSON(t, client, http.MethodPatch,
		base+"/api/v1/coteries/"+coterieID, `{"payment_gate":true}`, ownerTok)
	if code != http.StatusOK {
		t.Fatalf("enable gate: status = %d: %v", code, body)
	}

	reqTok, _ = testsupport.RegisterAndLogin(t, client, base, tag+"-req")
	code, body = createRequest(t, client, base, reqTok, coterieID, `{}`)
	if code != http.StatusCreated {
		t.Fatalf("create join request: status = %d: %v", code, body)
	}
	requestID, _ = body["id"].(string)
	return ownerTok, reqTok, coterieID, requestID
}

// The gate holds admission at awaiting_payment: the owner's accept no
// longer admits, the requester's stripe charge lands pending, and the
// signed webhook admits — idempotently. (Design D25.)
func TestPaymentGateAsyncFlow(t *testing.T) {
	var intents int
	fake := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		intents++
		_, _ = fmt.Fprintf(w, `{"id":"pi_gate_%d","client_secret":"pi_gate_%d_secret"}`, intents, intents)
	}))
	t.Cleanup(fake.Close)
	gateStripe := payment.Stripe{SecretKey: "sk_test_x", WebhookSecret: "whsec_gate", APIBase: fake.URL}

	srv := testsupport.NewServer(t, testsupport.NewDB(t),
		app.WithMarketplaceAdapters(gateStripe))
	client, base := srv.Client(), srv.URL
	ownerTok, reqTok, coterieID, requestID := gateFixture(t, client, base, "gate-async")

	// Accept holds the request and tells the requester to pay.
	code, body := decide(t, client, base, ownerTok, "accept", requestID)
	if code != http.StatusOK || body["status"] != "awaiting_payment" {
		t.Fatalf("gated accept: status = %d: %v", code, body)
	}
	code, body = testsupport.DoAuthJSON(t, client, http.MethodGet,
		base+"/api/v1/coteries/"+coterieID, "", ownerTok)
	if code != http.StatusOK {
		t.Fatalf("coterie read: %d", code)
	}
	if n, _ := body["member_count"].(float64); n != 1 {
		t.Fatalf("member_count = %v, admission must be held", body["member_count"])
	}
	types := notificationTypes(t, client, base, reqTok)
	if !hasType(types, "admission_due") {
		t.Fatalf("requester notifications = %v, want admission_due", types)
	}

	// The requester starts the charge: 12.00 over owner+1 = 6.00.
	code, body = testsupport.DoAuthJSON(t, client, http.MethodPost,
		base+"/api/v1/join-requests/"+requestID+"/payments", `{"method":"stripe"}`, reqTok)
	if code != http.StatusCreated {
		t.Fatalf("start charge: status = %d: %v", code, body)
	}
	if body["status"] != "pending" || body["amount"] != "6.00" || body["currency"] != "USD" {
		t.Fatalf("charge = %v", body)
	}
	if body["client_secret"] != "pi_gate_1_secret" {
		t.Fatalf("client_secret = %v", body["client_secret"])
	}
	ref, _ := body["external_ref"].(string)

	// Still no member before the webhook.
	_, body = testsupport.DoAuthJSON(t, client, http.MethodGet,
		base+"/api/v1/coteries/"+coterieID, "", ownerTok)
	if n, _ := body["member_count"].(float64); n != 1 {
		t.Fatalf("member_count = %v before webhook", body["member_count"])
	}

	// Unsigned webhook rejected.
	code, _ = testsupport.DoJSON(t, client, http.MethodPost,
		base+"/api/v1/marketplace/webhooks/stripe", `{"type":"payment_intent.succeeded"}`)
	if code != http.StatusUnprocessableEntity {
		t.Fatalf("unsigned gate webhook: status = %d, want 422", code)
	}

	succeeded := fmt.Sprintf(`{"id":"evt_g1","type":"payment_intent.succeeded","data":{"object":{"id":%q,"object":"payment_intent"}}}`, ref)
	code, body = deliverGate(t, client, base, "whsec_gate", succeeded)
	if code != http.StatusOK || body["status"] != "succeeded" {
		t.Fatalf("gate webhook: status = %d: %v", code, body)
	}

	// Admitted: member_count up, request accepted, welcome notified.
	_, body = testsupport.DoAuthJSON(t, client, http.MethodGet,
		base+"/api/v1/coteries/"+coterieID, "", ownerTok)
	if n, _ := body["member_count"].(float64); n != 2 {
		t.Fatalf("member_count = %v after webhook, want 2", body["member_count"])
	}
	types = notificationTypes(t, client, base, reqTok)
	if !hasType(types, "join_decided") {
		t.Fatalf("requester notifications = %v, want join_decided", types)
	}

	// Replay is idempotent.
	code, body = deliverGate(t, client, base, "whsec_gate", succeeded)
	if code != http.StatusOK || body["status"] != "succeeded" {
		t.Fatalf("replay: status = %d: %v", code, body)
	}

	// A decided request takes no further charges.
	code, _ = testsupport.DoAuthJSON(t, client, http.MethodPost,
		base+"/api/v1/join-requests/"+requestID+"/payments", `{"method":"stripe"}`, reqTok)
	if code != http.StatusConflict {
		t.Fatalf("charge on decided request: status = %d, want 409", code)
	}
}

// Failure keeps the request at awaiting_payment and allows a fresh
// charge; decisions are refused while money is in flight.
func TestPaymentGateFailureAndRetry(t *testing.T) {
	var intents int
	fake := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		intents++
		_, _ = fmt.Fprintf(w, `{"id":"pi_gf_%d","client_secret":"pi_gf_%d_secret"}`, intents, intents)
	}))
	t.Cleanup(fake.Close)
	gateStripe := payment.Stripe{SecretKey: "sk_test_x", WebhookSecret: "whsec_gate", APIBase: fake.URL}

	srv := testsupport.NewServer(t, testsupport.NewDB(t),
		app.WithMarketplaceAdapters(gateStripe))
	client, base := srv.Client(), srv.URL
	ownerTok, reqTok, _, requestID := gateFixture(t, client, base, "gate-fail")

	if code, body := decide(t, client, base, ownerTok, "accept", requestID); code != http.StatusOK {
		t.Fatalf("accept: %d: %v", code, body)
	}

	// First charge fails by webhook.
	code, body := testsupport.DoAuthJSON(t, client, http.MethodPost,
		base+"/api/v1/join-requests/"+requestID+"/payments", `{"method":"stripe"}`, reqTok)
	if code != http.StatusCreated {
		t.Fatalf("start charge: status = %d: %v", code, body)
	}
	ref, _ := body["external_ref"].(string)
	failed := fmt.Sprintf(`{"id":"evt_gf","type":"payment_intent.payment_failed","data":{"object":{"id":%q,"object":"payment_intent"}}}`, ref)
	code, body = deliverGate(t, client, base, "whsec_gate", failed)
	if code != http.StatusOK || body["status"] != "failed" {
		t.Fatalf("failure webhook: status = %d: %v", code, body)
	}

	// The requester retries: a failed charge allows a fresh one.
	code, body = testsupport.DoAuthJSON(t, client, http.MethodPost,
		base+"/api/v1/join-requests/"+requestID+"/payments", `{"method":"stripe"}`, reqTok)
	if code != http.StatusCreated {
		t.Fatalf("retry charge: status = %d: %v", code, body)
	}
	ref2, _ := body["external_ref"].(string)

	// Decisions are refused while a charge is pending — cancel too.
	code, _ = testsupport.DoAuthJSON(t, client, http.MethodDelete,
		base+"/api/v1/join-requests/"+requestID, "", reqTok)
	if code != http.StatusConflict {
		t.Fatalf("cancel with pending charge: status = %d, want 409", code)
	}
	if ref2 == ref {
		t.Fatalf("retry reused external_ref %q", ref2)
	}
	succeeded := fmt.Sprintf(`{"id":"evt_gf2","type":"payment_intent.succeeded","data":{"object":{"id":%q,"object":"payment_intent"}}}`, ref2)
	code, body = deliverGate(t, client, base, "whsec_gate", succeeded)
	if code != http.StatusOK || body["status"] != "succeeded" {
		t.Fatalf("retry webhook: status = %d: %v", code, body)
	}
}

// Manual admission is owner-only: accept holds, the owner records the
// cash, membership lands in the same transaction.
func TestPaymentGateManual(t *testing.T) {
	srv := testsupport.NewServer(t, testsupport.NewDB(t))
	client, base := srv.Client(), srv.URL
	ownerTok, reqTok, coterieID, requestID := gateFixture(t, client, base, "gate-manual")

	if code, body := decide(t, client, base, ownerTok, "accept", requestID); code != http.StatusOK {
		t.Fatalf("accept: %d: %v", code, body)
	}

	// The requester may never self-certify a manual payment.
	code, _ := testsupport.DoAuthJSON(t, client, http.MethodPost,
		base+"/api/v1/join-requests/"+requestID+"/payments",
		`{"method":"manual","external_ref":"cash on Friday"}`, reqTok)
	if code != http.StatusForbidden {
		t.Fatalf("requester manual charge: status = %d, want 403", code)
	}

	// The owner records it; the member is admitted inline.
	code, body := testsupport.DoAuthJSON(t, client, http.MethodPost,
		base+"/api/v1/join-requests/"+requestID+"/payments",
		`{"method":"manual","external_ref":"cash on Friday"}`, ownerTok)
	if code != http.StatusCreated {
		t.Fatalf("owner manual charge: status = %d: %v", code, body)
	}
	if body["status"] != "succeeded" || body["method"] != "manual" {
		t.Fatalf("charge = %v", body)
	}
	_, body = testsupport.DoAuthJSON(t, client, http.MethodGet,
		base+"/api/v1/coteries/"+coterieID, "", ownerTok)
	if n, _ := body["member_count"].(float64); n != 2 {
		t.Fatalf("member_count = %v after manual admission, want 2", body["member_count"])
	}
}

// Without the gate flag, accept admits exactly as before — the gate is
// a per-circle opt-in.
func TestPaymentGateOffUnchanged(t *testing.T) {
	srv := testsupport.NewServer(t, testsupport.NewDB(t))
	client, base := srv.Client(), srv.URL
	ownerTok, _, _, coterieID, _ := testsupport.SeedCircle(t, client, base, "gate-off", "12.00", 4, 2, 0)
	publish(t, client, base, ownerTok, coterieID, "public")
	reqTok, _ := testsupport.RegisterAndLogin(t, client, base, "gate-off-req")
	code, body := createRequest(t, client, base, reqTok, coterieID, `{}`)
	if code != http.StatusCreated {
		t.Fatalf("create: %d: %v", code, body)
	}
	requestID, _ := body["id"].(string)
	code, body = decide(t, client, base, ownerTok, "accept", requestID)
	if code != http.StatusOK || body["status"] != "accepted" {
		t.Fatalf("plain accept: status = %d: %v", code, body)
	}
	_, body = testsupport.DoAuthJSON(t, client, http.MethodGet,
		base+"/api/v1/coteries/"+coterieID, "", ownerTok)
	if n, _ := body["member_count"].(float64); n != 2 {
		t.Fatalf("member_count = %v, want 2", body["member_count"])
	}
}
