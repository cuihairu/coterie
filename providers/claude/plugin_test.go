package claude_test

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"testing"

	"github.com/cuihairu/coterie/internal/app"
	"github.com/cuihairu/coterie/internal/provider"
	"github.com/cuihairu/coterie/internal/testsupport"
	"github.com/cuihairu/coterie/pkg/api"
	"github.com/cuihairu/coterie/providers/claude"
)

// newServer boots the full mux with the claude plugin registered, the
// way apps/server does under PROVIDER_PLUGINS=claude.
func newServer(t *testing.T) (*http.Client, string) {
	t.Helper()
	srv := testsupport.NewServer(t, testsupport.NewDB(t), app.WithProviderPlugins(claude.Plugin{}))
	return srv.Client(), srv.URL
}

// findClaude resolves the seeded catalog entry (migration 0006) and its
// Max product through the public reads.
func findClaude(t *testing.T, client *http.Client, base string) (providerID, maxProductID string) {
	t.Helper()
	code, body := testsupport.DoJSON(t, client, http.MethodGet, base+"/api/v1/providers?category=ai", "")
	if code != http.StatusOK {
		t.Fatalf("list providers: status = %d: %v", code, body)
	}
	items, _ := body["items"].([]any)
	for _, it := range items {
		p := it.(map[string]any)
		if p["slug"] == claude.Slug {
			providerID, _ = p["id"].(string)
		}
	}
	if providerID == "" {
		t.Fatalf("seeded claude provider not found in catalog: %v", body)
	}
	code, body = testsupport.DoJSON(t, client, http.MethodGet,
		base+"/api/v1/products?provider_id="+providerID, "")
	if code != http.StatusOK {
		t.Fatalf("list products: status = %d: %v", code, body)
	}
	items, _ = body["items"].([]any)
	for _, it := range items {
		p := it.(map[string]any)
		if p["name"] == "Max" {
			maxProductID, _ = p["id"].(string)
		}
	}
	if maxProductID == "" {
		t.Fatalf("seeded claude Max product not found: %v", body)
	}
	return providerID, maxProductID
}

// selfID resolves the acting user's id via /auth/me.
func selfID(t *testing.T, client *http.Client, base, token string) string {
	t.Helper()
	code, body := testsupport.DoAuthJSON(t, client, http.MethodGet, base+"/api/v1/auth/me", "", token)
	if code != http.StatusOK {
		t.Fatalf("me: status = %d: %v", code, body)
	}
	id, _ := body["id"].(string)
	return id
}

// subscribe creates a subscription on a product with the given sharing
// policy, owned by the token's user.
func subscribe(t *testing.T, client *http.Client, base, tok, productID, policy string) (int, map[string]any) {
	t.Helper()
	payload := fmt.Sprintf(`{"product_id":%q,"owner_user_id":%q,"billing_cycle":"monthly","price":"20.00","currency":"USD","start_date":"2026-10-01","max_seats":8`, productID, selfID(t, client, base, tok))
	if policy != "" {
		payload += `,"sharing_policy":` + policy
	}
	payload += `}`
	return testsupport.DoAuthJSON(t, client, http.MethodPost, base+"/api/v1/subscriptions", payload, tok)
}

// openCircle creates a draft circle with 8 seats and publishes it open.
func openCircle(t *testing.T, client *http.Client, base, tok, subID string) string {
	t.Helper()
	code, body := testsupport.DoAuthJSON(t, client, http.MethodPost, base+"/api/v1/coteries",
		fmt.Sprintf(`{"subscription_id":%q,"name":"Claude Circle","capacity":8}`, subID), tok)
	if code != http.StatusCreated {
		t.Fatalf("create coterie: status = %d: %v", code, body)
	}
	coterieID, _ := body["id"].(string)
	code, _ = testsupport.DoAuthJSON(t, client, http.MethodPatch,
		base+"/api/v1/coteries/"+coterieID, `{"status":"open"}`, tok)
	if code != http.StatusOK {
		t.Fatalf("publish coterie: status = %d", code)
	}
	return coterieID
}

// joinViaInvitation mints a fresh invitation and accepts it as a new
// user, returning the accept response.
func joinViaInvitation(t *testing.T, client *http.Client, base, tok, coterieID, tag string) (int, map[string]any) {
	t.Helper()
	code, inv := testsupport.DoAuthJSON(t, client, http.MethodPost,
		base+"/api/v1/coteries/"+coterieID+"/invitations", `{"role":"member"}`, tok)
	if code != http.StatusCreated {
		t.Fatalf("invitation: status = %d: %v", code, inv)
	}
	inviteToken, _ := inv["token"].(string)
	joinerTok, _ := testsupport.RegisterAndLogin(t, client, base, tag)
	return testsupport.DoAuthJSON(t, client, http.MethodPost, base+"/api/v1/invitations/accept",
		fmt.Sprintf(`{"token":%q}`, inviteToken), joinerTok)
}

// policyOK is a valid max-plan policy reused across tests.
const policyOK = `{"mode":"seat","region":"eu","plan":"max"}`

func TestPolicyValidation(t *testing.T) {
	client, base := newServer(t)
	tok, _ := testsupport.RegisterAndLogin(t, client, base, "cl-pol")
	_, productID := findClaude(t, client, base)

	// No region → the plugin refuses the subscription outright.
	code, body := subscribe(t, client, base, tok, productID, `{"mode":"seat"}`)
	if code != http.StatusUnprocessableEntity {
		t.Fatalf("no region: status = %d, want 422: %v", code, body)
	}
	if msg, _ := body["error"].(map[string]any)["message"].(string); msg != "policy rejected by provider" {
		t.Fatalf("reject message = %q", msg)
	}

	// Region outside the residency enum → refused.
	code, _ = subscribe(t, client, base, tok, productID, `{"region":"apac"}`)
	if code != http.StatusUnprocessableEntity {
		t.Fatalf("bad region: status = %d, want 422", code)
	}

	// Unknown plan → refused.
	code, _ = subscribe(t, client, base, tok, productID, `{"region":"us","plan":"ultra"}`)
	if code != http.StatusUnprocessableEntity {
		t.Fatalf("bad plan: status = %d, want 422", code)
	}

	// A policy the plugin accepts goes through.
	code, body = subscribe(t, client, base, tok, productID, policyOK)
	if code != http.StatusCreated {
		t.Fatalf("valid policy: status = %d: %v", code, body)
	}
	subID, _ := body["id"].(string)

	// Updates re-validate: switching to a valid pro plan works, an
	// invalid region on update is refused.
	code, body = testsupport.DoAuthJSON(t, client, http.MethodPatch,
		base+"/api/v1/subscriptions/"+subID, `{"sharing_policy":{"region":"us","plan":"pro"}}`, tok)
	if code != http.StatusOK {
		t.Fatalf("update to pro: status = %d: %v", code, body)
	}
	code, _ = testsupport.DoAuthJSON(t, client, http.MethodPatch,
		base+"/api/v1/subscriptions/"+subID, `{"sharing_policy":{"region":"cn"}}`, tok)
	if code != http.StatusUnprocessableEntity {
		t.Fatalf("invalid update: status = %d, want 422", code)
	}
}

func TestAdmissionGuard(t *testing.T) {
	client, base := newServer(t)
	tok, _ := testsupport.RegisterAndLogin(t, client, base, "cl-adm")
	_, productID := findClaude(t, client, base)

	// Pro is personal: the circle opens but nobody may join.
	code, body := subscribe(t, client, base, tok, productID, `{"mode":"seat","region":"us","plan":"pro"}`)
	if code != http.StatusCreated {
		t.Fatalf("pro subscription: status = %d: %v", code, body)
	}
	subID, _ := body["id"].(string)
	proCircle := openCircle(t, client, base, tok, subID)
	code, body = joinViaInvitation(t, client, base, tok, proCircle, "cl-adm-joiner1")
	if code != http.StatusConflict {
		t.Fatalf("pro join: status = %d, want 409: %v", code, body)
	}
	if msg, _ := body["error"].(map[string]any)["message"].(string); msg != "claude pro is a personal plan; sharing requires the max plan" {
		t.Fatalf("pro guard message = %q", msg)
	}

	// Switching the plan to max unblocks admission.
	code, _ = testsupport.DoAuthJSON(t, client, http.MethodPatch,
		base+"/api/v1/subscriptions/"+subID, `{"sharing_policy":{"region":"us","plan":"max"}}`, tok)
	if code != http.StatusOK {
		t.Fatalf("switch to max: status = %d", code)
	}
	code, body = joinViaInvitation(t, client, base, tok, proCircle, "cl-adm-joiner2")
	if code != http.StatusCreated {
		t.Fatalf("max join: status = %d: %v", code, body)
	}
}

// The cap arithmetic is pure — exercise its boundary directly.
func TestAdmissionCapBoundary(t *testing.T) {
	p := claude.Plugin{}
	in := provider.GuardInput{UserID: "u", Role: "member", Policy: []byte(policyOK)}
	for count := 1; count < claude.MaxMembers; count++ {
		in.MemberCount = count
		if err := p.CheckAdmission(context.Background(), in); err != nil {
			t.Fatalf("count %d refused: %v", count, err)
		}
	}
	in.MemberCount = claude.MaxMembers
	err := p.CheckAdmission(context.Background(), in)
	var apiErr *api.APIError
	if !errors.As(err, &apiErr) || apiErr.Status != http.StatusConflict {
		t.Fatalf("count %d not refused with 409: %v", claude.MaxMembers, err)
	}
}

func TestUsageValidation(t *testing.T) {
	client, base := newServer(t)
	tok, _ := testsupport.RegisterAndLogin(t, client, base, "cl-usage")
	_, productID := findClaude(t, client, base)

	code, body := subscribe(t, client, base, tok, productID, policyOK)
	if code != http.StatusCreated {
		t.Fatalf("subscription: status = %d: %v", code, body)
	}
	claudeSub, _ := body["id"].(string)
	circleID := openCircle(t, client, base, tok, claudeSub)
	code, mems := testsupport.DoAuthJSON(t, client, http.MethodGet,
		base+"/api/v1/coteries/"+circleID+"/members", "", tok)
	if code != http.StatusOK {
		t.Fatalf("list members: status = %d: %v", code, mems)
	}
	memberItems, _ := mems["items"].([]any)
	ownerMemberID, _ := memberItems[0].(map[string]any)["id"].(string)

	record := func(amount, unit string) (int, map[string]any) {
		t.Helper()
		return testsupport.DoAuthJSON(t, client, http.MethodPost,
			fmt.Sprintf("%s/api/v1/subscriptions/%s/usage-records", base, claudeSub),
			fmt.Sprintf(`{"member_id":%q,"amount":%q,"unit":%q}`, ownerMemberID, amount, unit), tok)
	}

	// Tokens within the cap land; negative corrections stay allowed
	// (ledger semantics); requests sit under their tighter cap.
	code, body = record("1000", "tokens")
	if code != http.StatusCreated {
		t.Fatalf("tokens: status = %d: %v", code, body)
	}
	code, body = record("-250.5", "tokens")
	if code != http.StatusCreated {
		t.Fatalf("correction: status = %d: %v", code, body)
	}
	code, body = record("100", "requests")
	if code != http.StatusCreated {
		t.Fatalf("requests: status = %d: %v", code, body)
	}

	// Foreign units are refused, per-record caps hold for both units.
	code, body = record("1", "hours")
	if code != http.StatusUnprocessableEntity {
		t.Fatalf("hours: status = %d, want 422: %v", code, body)
	}
	code, _ = record("10000001", "tokens")
	if code != http.StatusUnprocessableEntity {
		t.Fatalf("over-cap tokens: status = %d, want 422", code)
	}
	code, _ = record("100001", "requests")
	if code != http.StatusUnprocessableEntity {
		t.Fatalf("over-cap requests: status = %d, want 422", code)
	}
}

func TestGenericProvidersUnaffected(t *testing.T) {
	client, base := newServer(t)

	// A generically seeded chain needs no region and no plan — the
	// plugin face is scoped to its slug.
	chainTok, _, chainSub := testsupport.SeedChain(t, client, base, "cl-generic", "10.00", 4)
	code, body := testsupport.DoAuthJSON(t, client, http.MethodPatch,
		base+"/api/v1/subscriptions/"+chainSub, `{"sharing_policy":{"mode":"family"}}`, chainTok)
	if code != http.StatusOK {
		t.Fatalf("generic policy: status = %d: %v", code, body)
	}
}
