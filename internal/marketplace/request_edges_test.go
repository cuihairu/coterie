package marketplace_test

import (
	"fmt"
	"net/http"
	"strings"
	"testing"

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
