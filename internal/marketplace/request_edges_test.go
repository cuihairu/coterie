package marketplace_test

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/cuihairu/coterie/internal/app"
	"github.com/cuihairu/coterie/internal/payment"
	"github.com/cuihairu/coterie/internal/testsupport"
)

// fileJoinRequest posts a join request and returns (status, body).
func fileJoinRequest(t *testing.T, client *http.Client, base, tok, coterieID, payload string) (int, map[string]any) {
	t.Helper()
	return testsupport.DoAuthJSON(t, client, http.MethodPost,
		base+"/api/v1/coteries/"+coterieID+"/join-requests", payload, tok)
}

// errorMessage digs the message out of the JSON error envelope.
func errorMessage(body map[string]any) string {
	e, _ := body["error"].(map[string]any)
	msg, _ := e["message"].(string)
	return msg
}

// detailMessage digs the first detail's message out of the envelope.
func detailMessage(body map[string]any) string {
	e, _ := body["error"].(map[string]any)
	details, _ := e["details"].([]any)
	if len(details) == 0 {
		return ""
	}
	d, _ := details[0].(map[string]any)
	msg, _ := d["message"].(string)
	return msg
}

// TestJoinRequestFilingGuards pins CreateJoinRequest's rejection paths:
// unlisted circles 404 (existence hidden), members 409, blocked users
// 403, one live request per user and coterie, and the message cap.
func TestJoinRequestFilingGuards(t *testing.T) {
	srv := testsupport.NewServer(t, testsupport.NewDB(t))
	client, base := srv.Client(), srv.URL

	// Unlisted circle: the request 404s exactly like an unknown id.
	hiddenOwner, _, _, hiddenID, _ := testsupport.SeedCircle(t, client, base, "mkt-edges-hidden", "10.00", 4, 2, 0)
	_ = hiddenOwner // kept un-published on purpose
	outsider, _ := testsupport.RegisterAndLogin(t, client, base, "mkt-edges-out")
	code, body := fileJoinRequest(t, client, base, outsider, hiddenID, `{}`)
	if code != http.StatusNotFound {
		t.Fatalf("request on unlisted circle: status = %d, want 404: %v", code, body)
	}

	// A listed circle with one joined member carries the rest.
	ownerTok, _, _, coterieID, joined := testsupport.SeedCircle(t, client, base, "mkt-edges-pub", "10.00", 4, 2, 1)
	publish(t, client, base, ownerTok, coterieID, "public")

	code, body = fileJoinRequest(t, client, base, joined[0].Token, coterieID, `{}`)
	if code != http.StatusConflict {
		t.Fatalf("request as member: status = %d, want 409: %v", code, body)
	}
	if msg := errorMessage(body); !strings.Contains(msg, "already a member") {
		t.Fatalf("member message = %q, want the already-member wording", msg)
	}

	blockedTok, blockedID := testsupport.RegisterAndLogin(t, client, base, "mkt-edges-blocked")
	code, body = testsupport.DoAuthJSON(t, client, http.MethodPut,
		base+"/api/v1/coteries/"+coterieID+"/blocks/"+blockedID, `{}`, ownerTok)
	if code != http.StatusCreated {
		t.Fatalf("block requester: status = %d: %v", code, body)
	}
	code, body = fileJoinRequest(t, client, base, blockedTok, coterieID, `{}`)
	if code != http.StatusForbidden {
		t.Fatalf("request while blocked: status = %d, want 403: %v", code, body)
	}
	if msg := errorMessage(body); !strings.Contains(msg, "blocked") {
		t.Fatalf("blocked message = %q, want the blocked wording", msg)
	}

	dupTok, _ := testsupport.RegisterAndLogin(t, client, base, "mkt-edges-dup")
	code, body = fileJoinRequest(t, client, base, dupTok, coterieID, `{}`)
	if code != http.StatusCreated {
		t.Fatalf("first request: status = %d: %v", code, body)
	}
	code, body = fileJoinRequest(t, client, base, dupTok, coterieID, `{}`)
	if code != http.StatusConflict {
		t.Fatalf("second request: status = %d, want 409: %v", code, body)
	}
	if msg := errorMessage(body); !strings.Contains(msg, "already pending") {
		t.Fatalf("duplicate message = %q, want the already-pending wording", msg)
	}

	longTok, _ := testsupport.RegisterAndLogin(t, client, base, "mkt-edges-long")
	code, body = fileJoinRequest(t, client, base, longTok, coterieID,
		fmt.Sprintf(`{"message":%q}`, strings.Repeat("a", 501)))
	if code != http.StatusUnprocessableEntity {
		t.Fatalf("overlong message: status = %d, want 422: %v", code, body)
	}
	if msg := detailMessage(body); !strings.Contains(msg, "at most 500 characters") {
		t.Fatalf("overlong message wording = %q, want the 500-character cap", msg)
	}
}

// TestGateQuoteExcludesDepartedMembers pins the gate quote's membership
// base (audit P1): leaving is a soft end (left_at stamped, status kept),
// so a departed member must not spread the share — 12.00 over owner only
// quotes 6.00, not 4.00.
func TestGateQuoteExcludesDepartedMembers(t *testing.T) {
	var intents int
	fake := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		intents++
		_, _ = fmt.Fprintf(w, `{"id":"pi_depart_%d","client_secret":"pi_depart_%d_secret"}`, intents, intents)
	}))
	t.Cleanup(fake.Close)
	gateStripe := payment.Stripe{SecretKey: "sk_test_x", WebhookSecret: "whsec_gate", APIBase: fake.URL}

	srv := testsupport.NewServer(t, testsupport.NewDB(t),
		app.WithMarketplaceAdapters(gateStripe))
	client, base := srv.Client(), srv.URL

	ownerTok, _, _, coterieID, joined := testsupport.SeedCircle(t, client, base, "mkt-quote", "12.00", 4, 4, 1)

	// The joined member leaves; the member row stays with left_at set.
	code, body := testsupport.DoAuthJSON(t, client, http.MethodPost,
		base+"/api/v1/coteries/"+coterieID+"/leave", `{}`, joined[0].Token)
	if code != http.StatusNoContent {
		t.Fatalf("member leave: status = %d: %v", code, body)
	}
	code, body = testsupport.DoAuthJSON(t, client, http.MethodGet,
		base+"/api/v1/coteries/"+coterieID, "", ownerTok)
	if code != http.StatusOK {
		t.Fatalf("coterie read: %d: %v", code, body)
	}
	if n, _ := body["member_count"].(float64); n != 1 {
		t.Fatalf("member_count = %v after leave, want 1", body["member_count"])
	}

	publish(t, client, base, ownerTok, coterieID, "public")
	code, body = testsupport.DoAuthJSON(t, client, http.MethodPatch,
		base+"/api/v1/coteries/"+coterieID, `{"payment_gate":true}`, ownerTok)
	if code != http.StatusOK {
		t.Fatalf("enable gate: status = %d: %v", code, body)
	}
	reqTok, _ := testsupport.RegisterAndLogin(t, client, base, "mkt-quote-req")
	code, body = createRequest(t, client, base, reqTok, coterieID, `{}`)
	if code != http.StatusCreated {
		t.Fatalf("create join request: status = %d: %v", code, body)
	}
	requestID, _ := body["id"].(string)
	code, body = decide(t, client, base, ownerTok, "accept", requestID)
	if code != http.StatusOK || body["status"] != "awaiting_payment" {
		t.Fatalf("gated accept: status = %d: %v", code, body)
	}

	// The quote splits over the members after admission: owner + the
	// requester = 2 → 6.00. The departed member must not count.
	code, body = testsupport.DoAuthJSON(t, client, http.MethodPost,
		base+"/api/v1/join-requests/"+requestID+"/payments", `{"method":"stripe"}`, reqTok)
	if code != http.StatusCreated {
		t.Fatalf("start charge: status = %d: %v", code, body)
	}
	if body["amount"] != "6.00" {
		t.Fatalf("charge amount = %v, want 6.00 (12.00 over 2, departed member excluded)", body["amount"])
	}
}

// TestDeclineGuards pins the decline path's permission and lookup errors:
// unknown requests 404 and outsiders 403, while the owner still decides.
func TestDeclineGuards(t *testing.T) {
	srv := testsupport.NewServer(t, testsupport.NewDB(t))
	client, base := srv.Client(), srv.URL

	ownerTok, _, _, coterieID, _ := testsupport.SeedCircle(t, client, base, "mkt-dec", "10.00", 4, 2, 0)
	publish(t, client, base, ownerTok, coterieID, "public")
	reqTok, _ := testsupport.RegisterAndLogin(t, client, base, "mkt-dec-req")
	code, body := fileJoinRequest(t, client, base, reqTok, coterieID, `{}`)
	if code != http.StatusCreated {
		t.Fatalf("file request: status = %d: %v", code, body)
	}
	requestID, _ := body["id"].(string)

	code, body = decide(t, client, base, ownerTok, "decline", "00000000-0000-0000-0000-000000000000")
	if code != http.StatusNotFound {
		t.Fatalf("decline unknown request: status = %d, want 404: %v", code, body)
	}

	stranger, _ := testsupport.RegisterAndLogin(t, client, base, "mkt-dec-stranger")
	code, body = decide(t, client, base, stranger, "decline", requestID)
	if code != http.StatusForbidden {
		t.Fatalf("decline by stranger: status = %d, want 403: %v", code, body)
	}

	code, body = decide(t, client, base, ownerTok, "decline", requestID)
	if code != http.StatusOK || body["status"] != "declined" {
		t.Fatalf("owner decline: status = %d: %v", code, body)
	}
}

// TestWebhookRejectsOversizedBody pins the gate webhook's read bound:
// payloads past maxWebhookBody never reach the adapter.
func TestWebhookRejectsOversizedBody(t *testing.T) {
	srv := testsupport.NewServer(t, testsupport.NewDB(t))
	client, base := srv.Client(), srv.URL

	code, body := testsupport.DoJSON(t, client, http.MethodPost,
		base+"/api/v1/marketplace/webhooks/stripe", strings.Repeat("x", 1<<20+1))
	if code != http.StatusUnprocessableEntity {
		t.Fatalf("oversized webhook: status = %d, want 422: %v", code, body)
	}
	if msg := detailMessage(body); !strings.Contains(msg, "too large") {
		t.Fatalf("oversized message = %q, want the too-large wording", msg)
	}
}

// TestPayAdmissionRejectsUnknownMethod pins the gate's method
// validation: unregistered channels 422 with the registered set named.
func TestPayAdmissionRejectsUnknownMethod(t *testing.T) {
	srv := testsupport.NewServer(t, testsupport.NewDB(t))
	client, base := srv.Client(), srv.URL

	ownerTok, reqTok, _, requestID := gateFixture(t, client, base, "mkt-edges-pay")
	code, body := decide(t, client, base, ownerTok, "accept", requestID)
	if code != http.StatusOK || body["status"] != "awaiting_payment" {
		t.Fatalf("gated accept: status = %d: %v", code, body)
	}

	code, body = testsupport.DoAuthJSON(t, client, http.MethodPost,
		base+"/api/v1/join-requests/"+requestID+"/payments", `{"method":"cash"}`, reqTok)
	if code != http.StatusUnprocessableEntity {
		t.Fatalf("unknown method: status = %d, want 422: %v", code, body)
	}
	if msg := detailMessage(body); !strings.Contains(msg, "registered payment methods") {
		t.Fatalf("unknown method message = %q, want the registered-methods wording", msg)
	}
}
