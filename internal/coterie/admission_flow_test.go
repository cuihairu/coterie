package coterie_test

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/cuihairu/coterie/internal/app"
	"github.com/cuihairu/coterie/internal/payment"
	"github.com/cuihairu/coterie/internal/provider"
	"github.com/cuihairu/coterie/internal/testsupport"
	"github.com/cuihairu/coterie/providers/claude"
)

// publishListing flips the coterie's listing so the marketplace sees it.
func publishListing(t *testing.T, client *http.Client, base, tok, coterieID string) {
	t.Helper()
	code, body := testsupport.DoAuthJSON(t, client, http.MethodPatch,
		base+"/api/v1/coteries/"+coterieID, `{"listing":"public"}`, tok)
	if code != http.StatusOK {
		t.Fatalf("publish listing: status = %d: %v", code, body)
	}
}

// fileJoinRequest files a marketplace join request and returns (status, body).
func fileJoinRequest(t *testing.T, client *http.Client, base, tok, coterieID string) (int, map[string]any) {
	t.Helper()
	return testsupport.DoAuthJSON(t, client, http.MethodPost,
		base+"/api/v1/coteries/"+coterieID+"/join-requests", `{}`, tok)
}

// acceptJoinRequest has the owner accept the request via the marketplace.
func acceptJoinRequest(t *testing.T, client *http.Client, base, ownerTok, requestID string) (int, map[string]any) {
	t.Helper()
	return testsupport.DoAuthJSON(t, client, http.MethodPost,
		base+"/api/v1/join-requests/"+requestID+"/accept", ``, ownerTok)
}

// memberUserIDs returns the coterie's active member user ids.
func memberUserIDs(t *testing.T, client *http.Client, base, tok, coterieID string) map[string]bool {
	t.Helper()
	code, body := testsupport.DoAuthJSON(t, client, http.MethodGet,
		base+"/api/v1/coteries/"+coterieID+"/members", ``, tok)
	if code != http.StatusOK {
		t.Fatalf("list members: status = %d: %v", code, body)
	}
	ids := map[string]bool{}
	for _, it := range body["items"].([]any) {
		ids[it.(map[string]any)["user_id"].(string)] = true
	}
	return ids
}

// errorMessage digs the message out of the JSON error envelope.
func errorMessage(body map[string]any) string {
	e, _ := body["error"].(map[string]any)
	msg, _ := e["message"].(string)
	return msg
}

// inviteAs mints an invitation from the given owner token and returns the
// raw token.
func inviteAs(t *testing.T, client *http.Client, base, ownerTok, coterieID string) string {
	t.Helper()
	code, body := testsupport.DoAuthJSON(t, client, http.MethodPost,
		base+"/api/v1/coteries/"+coterieID+"/invitations", `{"role":"member"}`, ownerTok)
	if code != http.StatusCreated {
		t.Fatalf("invite on %s: status = %d: %v", coterieID, code, body)
	}
	tok, _ := body["token"].(string)
	return tok
}

// acceptInvite joins via an invitation token.
func acceptInvite(t *testing.T, client *http.Client, base, tok, inviteToken string) (int, map[string]any) {
	t.Helper()
	return testsupport.DoAuthJSON(t, client, http.MethodPost,
		base+"/api/v1/invitations/accept", fmt.Sprintf(`{"token":%q}`, inviteToken), tok)
}

// TestMarketplaceAcceptAdmitsMember pins the non-gated accept path: the
// owner's accept on a pending request admits the requester as a member
// (the coterie-package AdmitMember entry).
func TestMarketplaceAcceptAdmitsMember(t *testing.T) {
	srv := testsupport.NewServer(t, testsupport.NewDB(t))
	client, base := srv.Client(), srv.URL

	ownerTok, _, _, coterieID, _ := testsupport.SeedCircle(t, client, base, "cot-mkt", "19.99", 4, 2, 0)
	publishListing(t, client, base, ownerTok, coterieID)

	reqTok, reqID := testsupport.RegisterAndLogin(t, client, base, "cot-mkt-req")
	code, body := fileJoinRequest(t, client, base, reqTok, coterieID)
	if code != http.StatusCreated {
		t.Fatalf("file join request: status = %d: %v", code, body)
	}
	requestID, _ := body["id"].(string)
	if requestID == "" {
		t.Fatalf("join request missing id: %v", body)
	}

	code, body = acceptJoinRequest(t, client, base, ownerTok, requestID)
	if code != http.StatusOK || body["status"] != "accepted" {
		t.Fatalf("accept: status = %d: %v", code, body)
	}
	if !memberUserIDs(t, client, base, ownerTok, coterieID)[reqID] {
		t.Fatalf("requester %s not admitted after accept", reqID)
	}
}

// parkingGuard implements provider.AdmissionGuard: the FIRST consult
// parks until released — parking that admission deterministically
// between the capacity checks and the member insert, so the second
// admission's outcome pins whether check and insert are serialized.
type parkingGuard struct {
	consults atomic.Int32
	started  chan struct{} // closed when the first consult parks
	release  chan struct{} // the parked consult returns when closed
}

func (p *parkingGuard) Slug() string { return "ts-cot-toctou" }

func (p *parkingGuard) CheckAdmission(ctx context.Context, _ provider.GuardInput) error {
	if p.consults.Add(1) == 1 {
		close(p.started)
		select {
		case <-p.release:
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	return nil
}

// TestConcurrentAdmissionsRespectMemberCap pins the admission TOCTOU fix
// (audit P1): while the first admission sits between its capacity check
// and its member insert, a second admission for the last slot must read
// the fresh count — under the coterie row lock it queues behind the
// parked transaction and 409s; without the lock it sails through and
// both admit.
func TestConcurrentAdmissionsRespectMemberCap(t *testing.T) {
	guard := &parkingGuard{started: make(chan struct{}), release: make(chan struct{})}
	srv := testsupport.NewServer(t, testsupport.NewDB(t), app.WithProviderPlugins(guard))
	client, base := srv.Client(), srv.URL

	// Custom provider → product → subscription → coterie, then cap
	// max_members at 2 (owner + exactly one more slot).
	ownerTok, ownerID := testsupport.RegisterAndLogin(t, client, base, "cot-toctou")
	code, body := testsupport.DoAuthJSON(t, client, http.MethodPost, base+"/api/v1/providers",
		`{"slug":"ts-cot-toctou","name":"Toctou Provider","category":"video"}`, ownerTok)
	if code != http.StatusCreated {
		t.Fatalf("seed provider: status = %d: %v", code, body)
	}
	providerID, _ := body["id"].(string)
	code, body = testsupport.DoAuthJSON(t, client, http.MethodPost, base+"/api/v1/products",
		fmt.Sprintf(`{"provider_id":%q,"name":"Toctou Product"}`, providerID), ownerTok)
	if code != http.StatusCreated {
		t.Fatalf("seed product: status = %d: %v", code, body)
	}
	productID, _ := body["id"].(string)
	code, body = testsupport.DoAuthJSON(t, client, http.MethodPost, base+"/api/v1/subscriptions",
		fmt.Sprintf(`{"product_id":%q,"owner_user_id":%q,"billing_cycle":"monthly","price":"19.99","currency":"USD","start_date":"2026-10-01","max_seats":4}`, productID, ownerID), ownerTok)
	if code != http.StatusCreated {
		t.Fatalf("seed subscription: status = %d: %v", code, body)
	}
	subID, _ := body["id"].(string)
	code, body = testsupport.DoAuthJSON(t, client, http.MethodPost, base+"/api/v1/coteries",
		fmt.Sprintf(`{"subscription_id":%q,"name":"Toctou Circle","capacity":4}`, subID), ownerTok)
	if code != http.StatusCreated {
		t.Fatalf("create coterie: status = %d: %v", code, body)
	}
	coterieID, _ := body["id"].(string)
	code, body = testsupport.DoAuthJSON(t, client, http.MethodPatch,
		base+"/api/v1/coteries/"+coterieID, `{"status":"open","listing":"public"}`, ownerTok)
	if code != http.StatusOK {
		t.Fatalf("open coterie: status = %d: %v", code, body)
	}
	code, body = testsupport.DoAuthJSON(t, client, http.MethodPatch,
		base+"/api/v1/subscriptions/"+subID, `{"max_members":2}`, ownerTok)
	if code != http.StatusOK {
		t.Fatalf("cap members: status = %d: %v", code, body)
	}

	// Two pending join requests, both eligible while a seat shows free.
	requestIDs := make([]string, 2)
	for i := range requestIDs {
		reqTok, _ := testsupport.RegisterAndLogin(t, client, base, fmt.Sprintf("cot-toctou-r%d", i))
		code, body := fileJoinRequest(t, client, base, reqTok, coterieID)
		if code != http.StatusCreated {
			t.Fatalf("file join request %d: status = %d: %v", i, code, body)
		}
		requestIDs[i], _ = body["id"].(string)
	}

	// Admission #1 parks at the guard, capacity checks already passed.
	// Admission #2 starts while it sits there: under the fix #2 queues on
	// the coterie row lock and cannot settle until the parked one
	// commits; without the lock it reads the stale count and settles
	// immediately. The wait below turns that difference into a verdict.
	results := make(chan int, 2)
	go func() {
		code, _ := acceptJoinRequest(t, client, base, ownerTok, requestIDs[0])
		results <- code
	}()
	<-guard.started
	go func() {
		code, _ := acceptJoinRequest(t, client, base, ownerTok, requestIDs[1])
		results <- code
	}()
	var early int
	select {
	case early = <-results:
	case <-time.After(3 * time.Second):
	}
	close(guard.release)
	codes := []int{}
	if early != 0 {
		codes = append(codes, early)
	}
	for len(codes) < 2 {
		codes = append(codes, <-results)
	}

	admitted := 0
	for _, c := range codes {
		if c == http.StatusOK {
			admitted++
		} else if c != http.StatusConflict {
			t.Errorf("accept: status = %d, want 200 or 409: %v", c, codes)
		}
	}
	if admitted != 1 {
		t.Fatalf("admissions straddling the parked check admitted %d, want exactly 1 (codes %v)", admitted, codes)
	}
	if ids := memberUserIDs(t, client, base, ownerTok, coterieID); len(ids) != 2 {
		t.Fatalf("active members = %d, want exactly 2 (owner + winner): %v", len(ids), ids)
	}
}

// TestAdmissionGuardConsultsPlugin wires the claude plugin in and pins
// the consult outcomes through the invitation path: a shareable (max)
// policy admits, a personal (pro) policy is refused with 409, and the
// core max_members cap still bounds admission on top of the plugin.
func TestAdmissionGuardConsultsPlugin(t *testing.T) {
	db := testsupport.NewDB(t)
	srv := testsupport.NewServer(t, db, app.WithProviderPlugins(claude.Plugin{}))
	client, base := srv.Client(), srv.URL

	// The catalog (migration 0006) already seeds the Claude provider and
	// its products; the plugin only engages through the seeded slug.
	maxTok, maxOwner := testsupport.RegisterAndLogin(t, client, base, "cot-guard")
	code, body := testsupport.DoAuthJSON(t, client, http.MethodGet,
		base+"/api/v1/providers", ``, maxTok)
	if code != http.StatusOK {
		t.Fatalf("list providers: status = %d: %v", code, body)
	}
	providerID := ""
	for _, it := range body["items"].([]any) {
		p := it.(map[string]any)
		if p["slug"] == "claude" {
			providerID, _ = p["id"].(string)
		}
	}
	if providerID == "" {
		t.Fatalf("seeded claude provider not found: %v", body["items"])
	}
	code, body = testsupport.DoAuthJSON(t, client, http.MethodGet,
		base+"/api/v1/products?provider_id="+providerID, ``, maxTok)
	if code != http.StatusOK {
		t.Fatalf("list claude products: status = %d: %v", code, body)
	}
	productID := ""
	for _, it := range body["items"].([]any) {
		p := it.(map[string]any)
		if p["name"] == "Max" {
			productID, _ = p["id"].(string)
		}
	}
	if productID == "" {
		t.Fatalf("seeded Claude Max product not found: %v", body["items"])
	}

	subscription := func(t *testing.T, ownerTok, ownerID, policy string) string {
		t.Helper()
		code, body := testsupport.DoAuthJSON(t, client, http.MethodPost, base+"/api/v1/subscriptions",
			fmt.Sprintf(`{"product_id":%q,"owner_user_id":%q,"billing_cycle":"monthly","price":"60.00","currency":"USD","start_date":"2026-10-01","max_seats":4,"sharing_policy":%s}`,
				productID, ownerID, policy), ownerTok)
		if code != http.StatusCreated {
			t.Fatalf("seed subscription: status = %d: %v", code, body)
		}
		id, _ := body["id"].(string)
		return id
	}
	coterieFor := func(t *testing.T, tok, subID, name string) string {
		t.Helper()
		code, body := testsupport.DoAuthJSON(t, client, http.MethodPost, base+"/api/v1/coteries",
			fmt.Sprintf(`{"subscription_id":%q,"name":%q,"capacity":4}`, subID, name), tok)
		if code != http.StatusCreated {
			t.Fatalf("create coterie: status = %d: %v", code, body)
		}
		id, _ := body["id"].(string)
		// Invitation acceptance needs a recruiting state: draft refuses.
		code, body = testsupport.DoAuthJSON(t, client, http.MethodPatch,
			base+"/api/v1/coteries/"+id, `{"status":"open"}`, tok)
		if code != http.StatusOK {
			t.Fatalf("open coterie: status = %d: %v", code, body)
		}
		return id
	}

	maxSub := subscription(t, maxTok, maxOwner, `{"region":"us"}`)
	maxCoterie := coterieFor(t, maxTok, maxSub, "Guarded Max")

	proTok, proOwner := testsupport.RegisterAndLogin(t, client, base, "cot-guard-pro")
	proSub := subscription(t, proTok, proOwner, `{"region":"eu","plan":"pro"}`)
	proCoterie := coterieFor(t, proTok, proSub, "Guarded Pro")

	t.Run("max plan admits", func(t *testing.T) {
		utok, uid := testsupport.RegisterAndLogin(t, client, base, "cot-guard-max-user")
		code, body := acceptInvite(t, client, base, utok, inviteAs(t, client, base, maxTok, maxCoterie))
		if code != http.StatusCreated {
			t.Fatalf("max accept: status = %d: %v", code, body)
		}
		if !memberUserIDs(t, client, base, maxTok, maxCoterie)[uid] {
			t.Fatalf("user %s not admitted on the max plan", uid)
		}
	})

	t.Run("pro plan refuses admission", func(t *testing.T) {
		utok, _ := testsupport.RegisterAndLogin(t, client, base, "cot-guard-pro-user")
		code, body := acceptInvite(t, client, base, utok, inviteAs(t, client, base, proTok, proCoterie))
		if code != http.StatusConflict {
			t.Fatalf("pro accept: status = %d, want 409: %v", code, body)
		}
		if msg := errorMessage(body); !strings.Contains(msg, "personal plan") {
			t.Fatalf("pro refusal message = %q, want the personal-plan wording", msg)
		}
	})

	t.Run("core max_members cap still applies", func(t *testing.T) {
		capTok, capUID := testsupport.RegisterAndLogin(t, client, base, "cot-guard-cap-user")
		code, body := testsupport.DoAuthJSON(t, client, http.MethodPatch,
			base+"/api/v1/subscriptions/"+maxSub, `{"max_members":2}`, maxTok)
		if code != http.StatusOK {
			t.Fatalf("set max_members: status = %d: %v", code, body)
		}
		code, body = acceptInvite(t, client, base, capTok, inviteAs(t, client, base, maxTok, maxCoterie))
		if code != http.StatusConflict {
			t.Fatalf("over-cap accept: status = %d, want 409: %v", code, body)
		}
		if msg := errorMessage(body); !strings.Contains(msg, "membership limit reached") {
			t.Fatalf("cap message = %q, want the membership-limit wording", msg)
		}
		if memberUserIDs(t, client, base, maxTok, maxCoterie)[capUID] {
			t.Fatalf("user %s admitted past max_members", capUID)
		}
	})
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

// TestPaymentGateWebhookAdmitsInTx pins the D25 transactional path: the
// requester's charge confirms through the gate's webhook and the member
// lands in the same transaction as the charge flip.
func TestPaymentGateWebhookAdmitsInTx(t *testing.T) {
	var intents int
	fake := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		intents++
		_, _ = fmt.Fprintf(w, `{"id":"pi_cot_%d","client_secret":"pi_cot_%d_secret"}`, intents, intents)
	}))
	t.Cleanup(fake.Close)
	gateStripe := payment.Stripe{SecretKey: "sk_test_x", WebhookSecret: "whsec_cot", APIBase: fake.URL}

	srv := testsupport.NewServer(t, testsupport.NewDB(t),
		app.WithMarketplaceAdapters(gateStripe))
	client, base := srv.Client(), srv.URL

	ownerTok, _, _, coterieID, _ := testsupport.SeedCircle(t, client, base, "cot-gate", "20.00", 4, 2, 0)
	publishListing(t, client, base, ownerTok, coterieID)
	code, body := testsupport.DoAuthJSON(t, client, http.MethodPatch,
		base+"/api/v1/coteries/"+coterieID, `{"payment_gate":true}`, ownerTok)
	if code != http.StatusOK {
		t.Fatalf("enable gate: status = %d: %v", code, body)
	}

	reqTok, _ := testsupport.RegisterAndLogin(t, client, base, "cot-gate-req")
	code, body = fileJoinRequest(t, client, base, reqTok, coterieID)
	if code != http.StatusCreated {
		t.Fatalf("file join request: status = %d: %v", code, body)
	}
	requestID, _ := body["id"].(string)

	code, body = acceptJoinRequest(t, client, base, ownerTok, requestID)
	if code != http.StatusOK || body["status"] != "awaiting_payment" {
		t.Fatalf("gated accept: status = %d: %v", code, body)
	}

	code, body = testsupport.DoAuthJSON(t, client, http.MethodPost,
		base+"/api/v1/join-requests/"+requestID+"/payments", `{"method":"stripe"}`, reqTok)
	if code != http.StatusCreated || body["status"] != "pending" {
		t.Fatalf("start charge: status = %d: %v", code, body)
	}
	ref, _ := body["external_ref"].(string)

	succeeded := fmt.Sprintf(`{"id":"evt_cot","type":"payment_intent.succeeded","data":{"object":{"id":%q,"object":"payment_intent"}}}`, ref)
	code, body = deliverGate(t, client, base, "whsec_cot", succeeded)
	if code != http.StatusOK || body["status"] != "succeeded" {
		t.Fatalf("gate webhook: status = %d: %v", code, body)
	}

	// The decisive assert is the member row: the webhook admitted in tx.
	if ids := memberUserIDs(t, client, base, ownerTok, coterieID); len(ids) != 2 {
		t.Fatalf("members = %v, want 2 after webhook admission", ids)
	}
}

// TestCoterieReadsNotFound pins the read-side error paths: unknown
// coteries 404 everywhere, and the invitation inbox stays owner/admin-only.
func TestCoterieReadsNotFound(t *testing.T) {
	srv := testsupport.NewServer(t, testsupport.NewDB(t))
	client, base := srv.Client(), srv.URL

	ownerTok, _, _, coterieID, joined := testsupport.SeedCircle(t, client, base, "cot-404", "9.99", 4, 2, 1)
	missing := "00000000-0000-0000-0000-000000000000"

	code, body := testsupport.DoAuthJSON(t, client, http.MethodGet,
		base+"/api/v1/coteries/"+missing, ``, ownerTok)
	if code != http.StatusNotFound {
		t.Fatalf("get unknown coterie: status = %d, want 404: %v", code, body)
	}

	code, body = testsupport.DoAuthJSON(t, client, http.MethodGet,
		base+"/api/v1/coteries/"+missing+"/members", ``, ownerTok)
	if code != http.StatusNotFound {
		t.Fatalf("members of unknown coterie: status = %d, want 404: %v", code, body)
	}

	code, body = testsupport.DoAuthJSON(t, client, http.MethodGet,
		base+"/api/v1/coteries/"+missing+"/invitations", ``, ownerTok)
	if code != http.StatusNotFound {
		t.Fatalf("invitations of unknown coterie: status = %d, want 404: %v", code, body)
	}

	// A plain member of the coterie still may not read the inbox.
	code, body = testsupport.DoAuthJSON(t, client, http.MethodGet,
		base+"/api/v1/coteries/"+coterieID+"/invitations", ``, joined[0].Token)
	if code != http.StatusForbidden {
		t.Fatalf("invitations as plain member: status = %d, want 403: %v", code, body)
	}
}

// TestPrivateCoterieHiddenFromOutsiders pins the read-side visibility
// rule (audit P1, design D10): an unlisted circle reads as 404 to
// anyone who is not an active member — the same existence-hiding the
// marketplace applies to join requests — while members read freely and
// listing public reopens the read to outsiders.
func TestPrivateCoterieHiddenFromOutsiders(t *testing.T) {
	srv := testsupport.NewServer(t, testsupport.NewDB(t))
	client, base := srv.Client(), srv.URL

	// SeedCircle leaves the circle unlisted (D10 default).
	ownerTok, _, _, coterieID, joined := testsupport.SeedCircle(t, client, base, "cot-hidden", "9.99", 4, 2, 1)

	outsider, _ := testsupport.RegisterAndLogin(t, client, base, "cot-hidden-out")
	code, body := testsupport.DoAuthJSON(t, client, http.MethodGet,
		base+"/api/v1/coteries/"+coterieID, ``, outsider)
	if code != http.StatusNotFound {
		t.Fatalf("outsider reads private coterie: status = %d, want 404: %v", code, body)
	}
	code, body = testsupport.DoAuthJSON(t, client, http.MethodGet,
		base+"/api/v1/coteries/"+coterieID+"/members", ``, outsider)
	if code != http.StatusNotFound {
		t.Fatalf("outsider lists private members: status = %d, want 404: %v", code, body)
	}

	// Members (owner included) keep reading their own circle.
	if code, body := testsupport.DoAuthJSON(t, client, http.MethodGet,
		base+"/api/v1/coteries/"+coterieID, ``, joined[0].Token); code != http.StatusOK {
		t.Fatalf("member reads coterie: status = %d: %v", code, body)
	}
	if code, body := testsupport.DoAuthJSON(t, client, http.MethodGet,
		base+"/api/v1/coteries/"+coterieID, ``, ownerTok); code != http.StatusOK {
		t.Fatalf("owner reads coterie: status = %d: %v", code, body)
	}

	// Listing public reopens the read (the directory exposes the entry).
	publishListing(t, client, base, ownerTok, coterieID)
	if code, body := testsupport.DoAuthJSON(t, client, http.MethodGet,
		base+"/api/v1/coteries/"+coterieID, ``, outsider); code != http.StatusOK {
		t.Fatalf("outsider reads public coterie: status = %d: %v", code, body)
	}
}
