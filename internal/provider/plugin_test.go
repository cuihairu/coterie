package provider_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"testing"

	"github.com/cuihairu/coterie/internal/app"
	"github.com/cuihairu/coterie/internal/provider"
	"github.com/cuihairu/coterie/internal/testsupport"
	"github.com/cuihairu/coterie/pkg/api"
)

// regionPlugin is a PolicyValidator demanding a region on every policy
// of its provider (Validation facet).
type regionPlugin struct{}

func (regionPlugin) Slug() string { return "plugin-region" }

func (regionPlugin) ValidatePolicy(_ context.Context, policy json.RawMessage) error {
	var p struct {
		Region string `json:"region"`
	}
	_ = json.Unmarshal(policy, &p)
	if p.Region == "" {
		return api.Validation("policy rejected by provider",
			api.Detail{Field: "sharing_policy", Message: "region is required"})
	}
	return nil
}

// guardPlugin is an AdmissionGuard capping circles at two active
// members including the owner (Sharing facet).
type guardPlugin struct{}

func (guardPlugin) Slug() string { return "plugin-guard" }

func (guardPlugin) CheckAdmission(_ context.Context, in provider.GuardInput) error {
	if in.MemberCount >= 2 {
		return api.Conflict("this provider caps circles at 2 members")
	}
	return nil
}

// seedCatalog creates provider(slug) and one product, returning the
// product id.
func seedCatalog(t *testing.T, client *http.Client, base, slug, tok string) string {
	t.Helper()
	code, body := testsupport.DoAuthJSON(t, client, http.MethodPost, base+"/api/v1/providers",
		fmt.Sprintf(`{"slug":%q,"name":"Plugin Provider %s","category":"video"}`, slug, slug), tok)
	if code != http.StatusCreated {
		t.Fatalf("seed provider: status = %d: %v", code, body)
	}
	providerID, _ := body["id"].(string)
	code, body = testsupport.DoAuthJSON(t, client, http.MethodPost, base+"/api/v1/products",
		fmt.Sprintf(`{"provider_id":%q,"name":"Plugin Plan %s"}`, providerID, slug), tok)
	if code != http.StatusCreated {
		t.Fatalf("seed product: status = %d: %v", code, body)
	}
	productID, _ := body["id"].(string)
	return productID
}

// createSubscriptionAs posts a subscription owned by the token's user.
func createSubscriptionAs(t *testing.T, client *http.Client, base, tok, productID, policy string, maxMembers int) (int, map[string]any) {
	t.Helper()
	payload := fmt.Sprintf(`{"product_id":%q,"owner_user_id":%q,"billing_cycle":"monthly","price":"10.00","currency":"USD","start_date":"2026-10-01","max_seats":4`, productID, selfID(t, client, base, tok))
	if maxMembers > 0 {
		payload += fmt.Sprintf(`,"max_members":%d`, maxMembers)
	}
	if policy != "" {
		payload += `,"sharing_policy":` + policy
	}
	payload += `}`
	return testsupport.DoAuthJSON(t, client, http.MethodPost, base+"/api/v1/subscriptions", payload, tok)
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

// openCircle creates a draft circle with 3 seats and publishes it open.
func openCircle(t *testing.T, client *http.Client, base, tok, subID, name string) string {
	t.Helper()
	code, body := testsupport.DoAuthJSON(t, client, http.MethodPost, base+"/api/v1/coteries",
		fmt.Sprintf(`{"subscription_id":%q,"name":%q,"capacity":3}`, subID, name), tok)
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

// joinViaInvitation mints an invitation and accepts it as a fresh user.
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

func TestPluginPolicyValidation(t *testing.T) {
	srv := testsupport.NewServer(t, testsupport.NewDB(t), app.WithProviderPlugins(regionPlugin{}, guardPlugin{}))
	client, base := srv.Client(), srv.URL

	tok, _ := testsupport.RegisterAndLogin(t, client, base, "plug-pol")
	productID := seedCatalog(t, client, base, "plugin-region", tok)

	// Create path: the plugin rejects a policy without region.
	code, body := createSubscriptionAs(t, client, base, tok, productID, `{"mode":"seat"}`, 0)
	if code != http.StatusUnprocessableEntity {
		t.Fatalf("create without region: status = %d, want 422: %v", code, body)
	}
	code, body = createSubscriptionAs(t, client, base, tok, productID, `{"mode":"seat","region":"US"}`, 0)
	if code != http.StatusCreated {
		t.Fatalf("create with region: status = %d: %v", code, body)
	}
	subID, _ := body["id"].(string)

	// Generic providers stay untouched: SeedChain sets no policy at all.
	gtok, _, _ := testsupport.SeedChain(t, client, base, "plug-pol-generic", "10.00", 4)
	if code, body := testsupport.DoAuthJSON(t, client, http.MethodGet,
		base+"/api/v1/subscriptions", "", gtok); code != http.StatusOK {
		t.Fatalf("generic chain: status = %d: %v", code, body)
	}

	// Update path: the plugin re-validates when the policy changes.
	code, body = testsupport.DoAuthJSON(t, client, http.MethodPatch,
		base+"/api/v1/subscriptions/"+subID, `{"sharing_policy":{"mode":"family"}}`, tok)
	if code != http.StatusUnprocessableEntity {
		t.Fatalf("update without region: status = %d, want 422: %v", code, body)
	}
	code, body = testsupport.DoAuthJSON(t, client, http.MethodPatch,
		base+"/api/v1/subscriptions/"+subID, `{"sharing_policy":{"mode":"family","region":"EU"}}`, tok)
	if code != http.StatusOK {
		t.Fatalf("update with region: status = %d: %v", code, body)
	}
}

func TestPluginAdmissionGuard(t *testing.T) {
	srv := testsupport.NewServer(t, testsupport.NewDB(t), app.WithProviderPlugins(regionPlugin{}, guardPlugin{}))
	client, base := srv.Client(), srv.URL

	tok, _ := testsupport.RegisterAndLogin(t, client, base, "plug-guard")
	productID := seedCatalog(t, client, base, "plugin-guard", tok)
	code, body := createSubscriptionAs(t, client, base, tok, productID, "", 0)
	if code != http.StatusCreated {
		t.Fatalf("seed subscription: status = %d: %v", code, body)
	}
	subID, _ := body["id"].(string)
	coterieID := openCircle(t, client, base, tok, subID, "Guard Circle")

	// Owner + first invitee = 2 members: still allowed.
	code, body = joinViaInvitation(t, client, base, tok, coterieID, "plug-guard-m1")
	if code != http.StatusCreated {
		t.Fatalf("first join: status = %d: %v", code, body)
	}

	// The plugin refuses the third member.
	code, body = joinViaInvitation(t, client, base, tok, coterieID, "plug-guard-m2")
	if code != http.StatusConflict {
		t.Fatalf("guarded join: status = %d, want 409: %v", code, body)
	}
	if msg, _ := body["error"].(map[string]any)["message"].(string); msg != "this provider caps circles at 2 members" {
		t.Fatalf("guard message = %q", msg)
	}
}

func TestMemberLimitCore(t *testing.T) {
	srv := testsupport.NewServer(t, testsupport.NewDB(t))
	client, base := srv.Client(), srv.URL

	tok, _ := testsupport.RegisterAndLogin(t, client, base, "plug-limit")
	productID := seedCatalog(t, client, base, "plug-limit", tok)
	code, body := createSubscriptionAs(t, client, base, tok, productID, "", 2)
	if code != http.StatusCreated {
		t.Fatalf("seed subscription: status = %d: %v", code, body)
	}
	subID, _ := body["id"].(string)
	coterieID := openCircle(t, client, base, tok, subID, "Limited Circle")

	// Owner + one invitee fills max_members = 2.
	code, body = joinViaInvitation(t, client, base, tok, coterieID, "plug-limit-m1")
	if code != http.StatusCreated {
		t.Fatalf("first join: status = %d: %v", code, body)
	}

	code, body = joinViaInvitation(t, client, base, tok, coterieID, "plug-limit-m2")
	if code != http.StatusConflict {
		t.Fatalf("over-limit join: status = %d, want 409: %v", code, body)
	}
	if msg, _ := body["error"].(map[string]any)["message"].(string); msg != "membership limit reached (2 members)" {
		t.Fatalf("limit message = %q", msg)
	}
}

// meterPlugin is a UsageValidator: any single record over 100 units is
// refused, and the magic "weird" unit trips the plain-error path
// (Metering facet).
type meterPlugin struct{}

func (meterPlugin) Slug() string { return "plugin-meter" }

func (meterPlugin) ValidateUsage(_ context.Context, in provider.UsageInput) error {
	if in.Unit == "weird" {
		return fmt.Errorf("unit %q is not meterable on this provider", in.Unit)
	}
	var over bool
	_, err := fmt.Sscanf(in.Amount, "%v", new(float64))
	if err == nil {
		var v float64
		_, _ = fmt.Sscanf(in.Amount, "%g", &v)
		over = v > 100
	}
	if over {
		return api.Validation("invalid usage record",
			api.Detail{Field: "amount", Message: "a single record may not exceed 100 units"})
	}
	return nil
}

func TestPluginUsageValidation(t *testing.T) {
	srv := testsupport.NewServer(t, testsupport.NewDB(t), app.WithProviderPlugins(meterPlugin{}))
	client, base := srv.Client(), srv.URL

	tok, _ := testsupport.RegisterAndLogin(t, client, base, "plug-meter")
	productID := seedCatalog(t, client, base, "plugin-meter", tok)
	code, body := createSubscriptionAs(t, client, base, tok, productID, "", 0)
	var mems map[string]any
	if code != http.StatusCreated {
		t.Fatalf("seed subscription: status = %d: %v", code, body)
	}
	subID, _ := body["id"].(string)
	coterieID := openCircle(t, client, base, tok, subID, "Meter Circle")
	// Usage records key on the member row id; the owner joined first.
	code, mems = testsupport.DoAuthJSON(t, client, http.MethodGet,
		base+"/api/v1/coteries/"+coterieID+"/members", "", tok)
	if code != http.StatusOK {
		t.Fatalf("list members: status = %d: %v", code, mems)
	}
	memberItems, _ := mems["items"].([]any)
	if len(memberItems) == 0 {
		t.Fatalf("circle has no members: %v", mems)
	}
	ownerMemberID, _ := memberItems[0].(map[string]any)["id"].(string)

	record := func(memberID, amount, unit string) (int, map[string]any) {
		t.Helper()
		return testsupport.DoAuthJSON(t, client, http.MethodPost,
			fmt.Sprintf("%s/api/v1/subscriptions/%s/usage-records", base, subID),
			fmt.Sprintf(`{"member_id":%q,"amount":%q,"unit":%q}`, memberID, amount, unit), tok)
	}

	// Within the plugin's cap the record lands.
	code, body = record(ownerMemberID, "50", "credits")
	if code != http.StatusCreated {
		t.Fatalf("record usage: status = %d: %v", code, body)
	}

	// Over the cap the provider plugin refuses with its 422.
	code, body = record(ownerMemberID, "150", "credits")
	if code != http.StatusUnprocessableEntity {
		t.Fatalf("over-cap usage: status = %d, want 422: %v", code, body)
	}
	if msg, _ := body["error"].(map[string]any)["message"].(string); msg != "invalid usage record" {
		t.Fatalf("reject message = %q", msg)
	}

	// A plain (non-api) plugin error also degrades to 422.
	code, body = record(ownerMemberID, "1", "weird")
	if code != http.StatusUnprocessableEntity {
		t.Fatalf("weird unit: status = %d, want 422: %v", code, body)
	}

	// The plugin face leaves other providers untouched: the same record
	// set goes through on a generically seeded chain.
	chainTok, _, chainSub := testsupport.SeedChain(t, client, base, "plug-meter-generic", "10.00", 4)
	chainCircle := openCircle(t, client, base, chainTok, chainSub, "Generic Meter Circle")
	code, chainMems := testsupport.DoAuthJSON(t, client, http.MethodGet,
		base+"/api/v1/coteries/"+chainCircle+"/members", "", chainTok)
	if code != http.StatusOK {
		t.Fatalf("list chain members: status = %d: %v", code, chainMems)
	}
	chainItems, _ := chainMems["items"].([]any)
	chainMemberID, _ := chainItems[0].(map[string]any)["id"].(string)
	code, body = testsupport.DoAuthJSON(t, client, http.MethodPost,
		fmt.Sprintf("%s/api/v1/subscriptions/%s/usage-records", base, chainSub),
		fmt.Sprintf(`{"member_id":%q,"amount":"1000","unit":"credits"}`, chainMemberID), chainTok)
	if code != http.StatusCreated {
		t.Fatalf("generic provider usage: status = %d: %v", code, body)
	}
}
