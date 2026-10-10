package coterie_test

import (
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/cuihairu/coterie/internal/auth"
	"github.com/cuihairu/coterie/internal/testsupport"
)

// seedOwnedSubscription creates provider → product → subscription with
// the authenticated user as owner and returns the subscription id.
func seedOwnedSubscription(t *testing.T, client *http.Client, base, tag, tok, ownerUserID string, maxSeats int) string {
	t.Helper()
	code, body := testsupport.DoAuthJSON(t, client, http.MethodPost, base+"/api/v1/providers",
		fmt.Sprintf(`{"slug":"cot-%s","name":"Coterie Provider %s","category":"video"}`, tag, tag), tok)
	if code != http.StatusCreated {
		t.Fatalf("seed provider: status = %d: %v", code, body)
	}
	providerID, _ := body["id"].(string)

	code, body = testsupport.DoAuthJSON(t, client, http.MethodPost, base+"/api/v1/products",
		fmt.Sprintf(`{"provider_id":%q,"name":"Coterie Product %s"}`, providerID, tag), tok)
	if code != http.StatusCreated {
		t.Fatalf("seed product: status = %d: %v", code, body)
	}
	productID, _ := body["id"].(string)

	code, body = testsupport.DoAuthJSON(t, client, http.MethodPost, base+"/api/v1/subscriptions",
		fmt.Sprintf(`{"product_id":%q,"owner_user_id":%q,"billing_cycle":"monthly","price":"19.99","currency":"USD","start_date":"2026-10-01","max_seats":%d}`, productID, ownerUserID, maxSeats), tok)
	if code != http.StatusCreated {
		t.Fatalf("seed subscription: status = %d: %v", code, body)
	}
	subID, _ := body["id"].(string)
	return subID
}

// createCoterie POSTs a coterie and returns the view.
func createCoterie(t *testing.T, client *http.Client, base, tok, subID, name string, capacity int) map[string]any {
	t.Helper()
	code, body := testsupport.DoAuthJSON(t, client, http.MethodPost, base+"/api/v1/coteries",
		fmt.Sprintf(`{"subscription_id":%q,"name":%q,"capacity":%d}`, subID, name, capacity), tok)
	if code != http.StatusCreated {
		t.Fatalf("create coterie: status = %d: %v", code, body)
	}
	return body
}

// invite mints an invitation and returns the raw token.
func invite(t *testing.T, client *http.Client, base, tok, coterieID, role string) string {
	t.Helper()
	code, body := testsupport.DoAuthJSON(t, client, http.MethodPost,
		base+"/api/v1/coteries/"+coterieID+"/invitations",
		fmt.Sprintf(`{"role":%q}`, role), tok)
	if code != http.StatusCreated {
		t.Fatalf("create invitation: status = %d: %v", code, body)
	}
	token, _ := body["token"].(string)
	if token == "" {
		t.Fatalf("invitation returned no token: %v", body)
	}
	if _, ok := body["id"]; !ok {
		t.Fatalf("invitation missing id: %v", body)
	}
	return token
}

// accept joins via token and returns the member payload.
func accept(t *testing.T, client *http.Client, base, tok, token string) map[string]any {
	t.Helper()
	code, body := testsupport.DoAuthJSON(t, client, http.MethodPost,
		base+"/api/v1/invitations/accept", fmt.Sprintf(`{"token":%q}`, token), tok)
	if code != http.StatusCreated {
		t.Fatalf("accept invitation: status = %d: %v", code, body)
	}
	return body
}

func TestCoterieCreationAndLifecycle(t *testing.T) {
	db := testsupport.NewDB(t)
	srv := testsupport.NewServer(t, db)
	client := srv.Client()
	tok, ownerID := testsupport.RegisterAndLogin(t, client, srv.URL, "cot-life")
	subID := seedOwnedSubscription(t, client, srv.URL, "life", tok, ownerID, 4)

	other, _ := testsupport.RegisterAndLogin(t, client, srv.URL, "cot-life-other")

	// Non-owner → 403.
	code, body := testsupport.DoAuthJSON(t, client, http.MethodPost, srv.URL+"/api/v1/coteries",
		fmt.Sprintf(`{"subscription_id":%q,"name":"Nope","capacity":1}`, subID), other)
	if code != http.StatusForbidden {
		t.Fatalf("non-owner create status = %d, want 403: %v", code, body)
	}

	// Capacity above max_seats → 409 and nothing persists.
	code, body = testsupport.DoAuthJSON(t, client, http.MethodPost, srv.URL+"/api/v1/coteries",
		fmt.Sprintf(`{"subscription_id":%q,"name":"Too Big","capacity":5}`, subID), tok)
	if code != http.StatusConflict {
		t.Fatalf("over-capacity create status = %d, want 409: %v", code, body)
	}
	code, body = testsupport.DoAuthJSON(t, client, http.MethodGet,
		srv.URL+"/api/v1/coteries?subscription_id="+subID, "", tok)
	if code != http.StatusOK {
		t.Fatalf("list after failed create status = %d", code)
	}
	if items, _ := body["items"].([]any); len(items) != 0 {
		t.Fatalf("failed create must roll back, got %d coteries", len(items))
	}

	// Whitespace-only names are rejected before anything persists.
	code, body = testsupport.DoAuthJSON(t, client, http.MethodPost, srv.URL+"/api/v1/coteries",
		fmt.Sprintf(`{"subscription_id":%q,"name":"   "}`, subID), tok)
	if code != http.StatusUnprocessableEntity {
		t.Fatalf("blank name create status = %d, want 422: %v", code, body)
	}

	// Happy path: draft, owner auto-joined, 3 seats provisioned.
	c := createCoterie(t, client, srv.URL, tok, subID, "Launch Circle", 3)
	id, _ := c["id"].(string)
	if c["status"] != "draft" || c["member_count"] != float64(1) || c["seats_total"] != float64(3) {
		t.Fatalf("unexpected create view: %v", c)
	}
	if c["full"] != false {
		t.Fatalf("fresh circle must not be full: %v", c)
	}

	// Strict 1:1.
	code, body = testsupport.DoAuthJSON(t, client, http.MethodPost, srv.URL+"/api/v1/coteries",
		fmt.Sprintf(`{"subscription_id":%q,"name":"Second"}`, subID), tok)
	if code != http.StatusConflict {
		t.Fatalf("second coterie status = %d, want 409: %v", code, body)
	}

	// Illegal transitions.
	for _, step := range []string{"active", "paused"} {
		code, body = testsupport.DoAuthJSON(t, client, http.MethodPatch,
			srv.URL+"/api/v1/coteries/"+id, fmt.Sprintf(`{"status":%q}`, step), tok)
		if code != http.StatusConflict {
			t.Fatalf("draft→%s status = %d, want 409: %v", step, code, body)
		}
	}

	// Owner-only management.
	code, body = testsupport.DoAuthJSON(t, client, http.MethodPatch,
		srv.URL+"/api/v1/coteries/"+id, `{"name":"Hijack"}`, other)
	if code != http.StatusForbidden {
		t.Fatalf("non-owner patch status = %d, want 403: %v", code, body)
	}

	// Blank renames are rejected; padded renames are trimmed.
	code, body = testsupport.DoAuthJSON(t, client, http.MethodPatch,
		srv.URL+"/api/v1/coteries/"+id, `{"name":"   "}`, tok)
	if code != http.StatusUnprocessableEntity {
		t.Fatalf("blank rename status = %d, want 422: %v", code, body)
	}
	code, body = testsupport.DoAuthJSON(t, client, http.MethodPatch,
		srv.URL+"/api/v1/coteries/"+id, `{"name":"  Renamed  "}`, tok)
	if code != http.StatusOK || body["name"] != "Renamed" {
		t.Fatalf("trim rename: status = %d body = %v", code, body)
	}

	// Legal walk: draft→open→active⇄paused→closed.
	for _, step := range []string{"open", "active", "paused", "active", "closed"} {
		code, body = testsupport.DoAuthJSON(t, client, http.MethodPatch,
			srv.URL+"/api/v1/coteries/"+id, fmt.Sprintf(`{"status":%q}`, step), tok)
		if code != http.StatusOK || body["status"] != step {
			t.Fatalf("transition to %s: status = %d body = %v", step, code, body)
		}
	}

	// Closed is terminal.
	code, body = testsupport.DoAuthJSON(t, client, http.MethodPatch,
		srv.URL+"/api/v1/coteries/"+id, `{"status":"open"}`, tok)
	if code != http.StatusConflict {
		t.Fatalf("closed→open status = %d, want 409: %v", code, body)
	}
}

// TestCoterieListScopedToMembers pins the contract behind GET /coteries:
// the list only contains the caller's own circles, never the whole
// table (docs/api.md: 自己参与的圈).
func TestCoterieListScopedToMembers(t *testing.T) {
	db := testsupport.NewDB(t)
	srv := testsupport.NewServer(t, db)
	client := srv.Client()
	tok, ownerID := testsupport.RegisterAndLogin(t, client, srv.URL, "cot-list")
	subID := seedOwnedSubscription(t, client, srv.URL, "list", tok, ownerID, 3)
	c := createCoterie(t, client, srv.URL, tok, subID, "Scoped Circle", 3)
	id, _ := c["id"].(string)

	other, _ := testsupport.RegisterAndLogin(t, client, srv.URL, "cot-list-b")

	// A fresh user sees none of the existing circles.
	for _, q := range []string{"", "?subscription_id=" + subID} {
		code, body := testsupport.DoAuthJSON(t, client, http.MethodGet,
			srv.URL+"/api/v1/coteries"+q, "", other)
		if code != http.StatusOK {
			t.Fatalf("outsider list status = %d: %v", code, body)
		}
		if items, _ := body["items"].([]any); len(items) != 0 {
			t.Fatalf("outsider list must be empty, got %d items", len(items))
		}
	}

	// The owner sees their circle, filtered and unfiltered.
	code, body := testsupport.DoAuthJSON(t, client, http.MethodGet,
		srv.URL+"/api/v1/coteries", "", tok)
	if code != http.StatusOK {
		t.Fatalf("owner list status = %d: %v", code, body)
	}
	items, _ := body["items"].([]any)
	if len(items) != 1 {
		t.Fatalf("owner list = %d items, want 1", len(items))
	}

	// Joining puts the circle in the member's list; leaving drops it.
	code, body = testsupport.DoAuthJSON(t, client, http.MethodPatch,
		srv.URL+"/api/v1/coteries/"+id, `{"status":"open"}`, tok)
	if code != http.StatusOK {
		t.Fatalf("publish status = %d: %v", code, body)
	}
	token := invite(t, client, srv.URL, tok, id, "member")
	accept(t, client, srv.URL, other, token)
	code, body = testsupport.DoAuthJSON(t, client, http.MethodGet,
		srv.URL+"/api/v1/coteries", "", other)
	if code != http.StatusOK {
		t.Fatalf("member list status = %d: %v", code, body)
	}
	if items, _ := body["items"].([]any); len(items) != 1 {
		t.Fatalf("member list = %d items, want 1", len(items))
	}

	code, body = testsupport.DoAuthJSON(t, client, http.MethodPost,
		srv.URL+"/api/v1/coteries/"+id+"/leave", "", other)
	if code != http.StatusNoContent {
		t.Fatalf("leave status = %d: %v", code, body)
	}
	code, body = testsupport.DoAuthJSON(t, client, http.MethodGet,
		srv.URL+"/api/v1/coteries", "", other)
	if code != http.StatusOK {
		t.Fatalf("post-leave list status = %d: %v", code, body)
	}
	if items, _ := body["items"].([]any); len(items) != 0 {
		t.Fatalf("post-leave list must be empty, got %d items", len(items))
	}
}

func TestInvitationJoinLeave(t *testing.T) {
	db := testsupport.NewDB(t)
	srv := testsupport.NewServer(t, db)
	client := srv.Client()
	tok, ownerID := testsupport.RegisterAndLogin(t, client, srv.URL, "cot-inv")
	subID := seedOwnedSubscription(t, client, srv.URL, "inv", tok, ownerID, 2)
	c := createCoterie(t, client, srv.URL, tok, subID, "Invite Circle", 2)
	id, _ := c["id"].(string)

	// Draft circles do not accept members.
	token := invite(t, client, srv.URL, tok, id, "member")
	u2, _ := testsupport.RegisterAndLogin(t, client, srv.URL, "cot-inv-u2")
	code, body := testsupport.DoAuthJSON(t, client, http.MethodPost,
		srv.URL+"/api/v1/invitations/accept", fmt.Sprintf(`{"token":%q}`, token), u2)
	if code != http.StatusConflict {
		t.Fatalf("join draft status = %d, want 409: %v", code, body)
	}

	// Publish, then join works.
	code, _ = testsupport.DoAuthJSON(t, client, http.MethodPatch,
		srv.URL+"/api/v1/coteries/"+id, `{"status":"open"}`, tok)
	if code != http.StatusOK {
		t.Fatalf("publish status = %d", code)
	}
	m2 := accept(t, client, srv.URL, u2, token)
	if m2["role"] != "member" {
		t.Fatalf("joined role wrong: %v", m2)
	}

	// Token is single-use.
	code, body = testsupport.DoAuthJSON(t, client, http.MethodPost,
		srv.URL+"/api/v1/invitations/accept", fmt.Sprintf(`{"token":%q}`, token), u2)
	if code != http.StatusConflict {
		t.Fatalf("token reuse status = %d, want 409: %v", code, body)
	}

	// Members cannot invite.
	code, body = testsupport.DoAuthJSON(t, client, http.MethodPost,
		srv.URL+"/api/v1/coteries/"+id+"/invitations", `{"role":"member"}`, u2)
	if code != http.StatusForbidden {
		t.Fatalf("member invite status = %d, want 403: %v", code, body)
	}

	// Fill both seats → full; a third join is rejected.
	code, members := testsupport.DoAuthJSON(t, client, http.MethodGet,
		srv.URL+"/api/v1/coteries/"+id+"/members", "", tok)
	if code != http.StatusOK {
		t.Fatalf("list members status = %d: %v", code, members)
	}
	items, _ := members["items"].([]any)
	if len(items) != 2 {
		t.Fatalf("members = %d, want 2", len(items))
	}
	seatIDs := freeSeats(t, client, srv.URL, tok, subID, 2)
	assigned := 0
	for _, raw := range items {
		m, _ := raw.(map[string]any)
		memberID, _ := m["id"].(string)
		if _, err := assignSeat(t, client, srv.URL, tok, seatIDs[assigned], memberID); err != nil {
			t.Fatal(err)
		}
		assigned++
	}
	code, body = testsupport.DoAuthJSON(t, client, http.MethodGet,
		srv.URL+"/api/v1/coteries/"+id, "", tok)
	if code != http.StatusOK || body["full"] != true || body["seats_free"] != float64(0) {
		t.Fatalf("full circle view wrong: status = %d body = %v", code, body)
	}

	u3, _ := testsupport.RegisterAndLogin(t, client, srv.URL, "cot-inv-u3")
	code, body = testsupport.DoAuthJSON(t, client, http.MethodPost,
		srv.URL+"/api/v1/invitations/accept", fmt.Sprintf(`{"token":%q}`, invite(t, client, srv.URL, tok, id, "member")), u3)
	if code != http.StatusConflict {
		t.Fatalf("join full status = %d, want 409: %v", code, body)
	}

	// Owner cannot leave; a member can, and their seat is freed.
	code, body = testsupport.DoAuthJSON(t, client, http.MethodPost,
		srv.URL+"/api/v1/coteries/"+id+"/leave", "", tok)
	if code != http.StatusConflict {
		t.Fatalf("owner leave status = %d, want 409: %v", code, body)
	}
	code, _ = testsupport.DoAuthJSON(t, client, http.MethodPost,
		srv.URL+"/api/v1/coteries/"+id+"/leave", "", u2)
	if code != http.StatusNoContent {
		t.Fatalf("member leave status = %d, want 204", code)
	}
	code, body = testsupport.DoAuthJSON(t, client, http.MethodGet,
		srv.URL+"/api/v1/coteries/"+id, "", tok)
	if code != http.StatusOK || body["member_count"] != float64(1) || body["seats_free"] != float64(1) || body["full"] != false {
		t.Fatalf("post-leave view wrong: status = %d body = %v", code, body)
	}

	// A fresh invitation lets u3 in.
	accept(t, client, srv.URL, u3, invite(t, client, srv.URL, tok, id, "member"))
}

func TestMemberRemovalReleasesSeats(t *testing.T) {
	db := testsupport.NewDB(t)
	srv := testsupport.NewServer(t, db)
	client := srv.Client()
	tok, ownerID := testsupport.RegisterAndLogin(t, client, srv.URL, "cot-rm")
	subID := seedOwnedSubscription(t, client, srv.URL, "rm", tok, ownerID, 2)
	c := createCoterie(t, client, srv.URL, tok, subID, "Removal Circle", 1)
	id, _ := c["id"].(string)
	testsupport.DoAuthJSON(t, client, http.MethodPatch,
		srv.URL+"/api/v1/coteries/"+id, `{"status":"open"}`, tok)

	u2, _ := testsupport.RegisterAndLogin(t, client, srv.URL, "cot-rm-u2")
	m2 := accept(t, client, srv.URL, u2, invite(t, client, srv.URL, tok, id, "member"))
	m2ID, _ := m2["id"].(string)

	// Give u2 the only seat, then verify removal frees it.
	seatIDs := freeSeats(t, client, srv.URL, tok, subID, 1)
	if _, err := assignSeat(t, client, srv.URL, tok, seatIDs[0], m2ID); err != nil {
		t.Fatal(err)
	}
	_, body := testsupport.DoAuthJSON(t, client, http.MethodGet,
		srv.URL+"/api/v1/coteries/"+id, "", tok)
	if body["full"] != true {
		t.Fatalf("expected full before removal: %v", body)
	}

	// Non-owner removal → 403.
	code, body := testsupport.DoAuthJSON(t, client, http.MethodDelete,
		srv.URL+"/api/v1/members/"+m2ID, "", u2)
	if code != http.StatusForbidden {
		t.Fatalf("non-owner remove status = %d, want 403: %v", code, body)
	}

	// Owner removes the member → seat released.
	code, _ = testsupport.DoAuthJSON(t, client, http.MethodDelete,
		srv.URL+"/api/v1/members/"+m2ID, "", tok)
	if code != http.StatusNoContent {
		t.Fatalf("remove member status = %d, want 204", code)
	}
	_, body = testsupport.DoAuthJSON(t, client, http.MethodGet,
		srv.URL+"/api/v1/coteries/"+id, "", tok)
	if body["member_count"] != float64(1) || body["seats_free"] != float64(1) || body["full"] != false {
		t.Fatalf("post-removal view wrong: %v", body)
	}

	// Owner member cannot be removed; unknown member 404s.
	_, members := testsupport.DoAuthJSON(t, client, http.MethodGet,
		srv.URL+"/api/v1/coteries/"+id+"/members", "", tok)
	items, _ := members["items"].([]any)
	ownerMember, _ := items[0].(map[string]any)
	if ownerMember["user_id"] != ownerID {
		t.Fatalf("expected owner as first member: %v", ownerMember)
	}
	ownerMemberID, _ := ownerMember["id"].(string)
	code, body = testsupport.DoAuthJSON(t, client, http.MethodDelete,
		srv.URL+"/api/v1/members/"+ownerMemberID, "", tok)
	if code != http.StatusConflict {
		t.Fatalf("remove owner status = %d, want 409: %v", code, body)
	}
	code, _ = testsupport.DoAuthJSON(t, client, http.MethodDelete,
		srv.URL+"/api/v1/members/00000000-0000-0000-0000-000000000000", "", tok)
	if code != http.StatusNotFound {
		t.Fatalf("remove unknown member status = %d, want 404", code)
	}
}

func TestInvitationValidationAndExpiry(t *testing.T) {
	db := testsupport.NewDB(t)
	srv := testsupport.NewServer(t, db)
	client := srv.Client()
	tok, ownerID := testsupport.RegisterAndLogin(t, client, srv.URL, "cot-exp")
	subID := seedOwnedSubscription(t, client, srv.URL, "exp", tok, ownerID, 2)
	c := createCoterie(t, client, srv.URL, tok, subID, "Expiry Circle", 0)
	id, _ := c["id"].(string)

	// Bad role / TTL → 422.
	code, body := testsupport.DoAuthJSON(t, client, http.MethodPost,
		srv.URL+"/api/v1/coteries/"+id+"/invitations", `{"role":"owner"}`, tok)
	if code != http.StatusUnprocessableEntity {
		t.Fatalf("owner-role invitation status = %d, want 422: %v", code, body)
	}
	code, body = testsupport.DoAuthJSON(t, client, http.MethodPost,
		srv.URL+"/api/v1/coteries/"+id+"/invitations", `{"expires_in_days":31}`, tok)
	if code != http.StatusUnprocessableEntity {
		t.Fatalf("31-day invitation status = %d, want 422: %v", code, body)
	}

	// Unknown token → 404.
	u2, _ := testsupport.RegisterAndLogin(t, client, srv.URL, "cot-exp-u2")
	code, body = testsupport.DoAuthJSON(t, client, http.MethodPost,
		srv.URL+"/api/v1/invitations/accept", `{"token":"no-such-token"}`, u2)
	if code != http.StatusNotFound {
		t.Fatalf("unknown token status = %d, want 404: %v", code, body)
	}

	// Expired invitation (seeded directly with its hash) → 409.
	expired, err := auth.NewToken()
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Exec(
		"INSERT INTO invitations (id, coterie_id, token, role, expire_at, created_by) VALUES (?, ?, ?, 'member', ?, ?)",
		"11111111-1111-1111-1111-111111111111", id, auth.HashToken(expired),
		time.Now().UTC().Add(-time.Hour), ownerID,
	).Error; err != nil {
		t.Fatal(err)
	}
	code, body = testsupport.DoAuthJSON(t, client, http.MethodPost,
		srv.URL+"/api/v1/invitations/accept", fmt.Sprintf(`{"token":%q}`, expired), u2)
	if code != http.StatusConflict {
		t.Fatalf("expired invitation status = %d, want 409: %v", code, body)
	}

	// A circle without seats still accepts members (nothing is full).
	testsupport.DoAuthJSON(t, client, http.MethodPatch,
		srv.URL+"/api/v1/coteries/"+id, `{"status":"open"}`, tok)
	live := invite(t, client, srv.URL, tok, id, "member")
	m := accept(t, client, srv.URL, u2, live)
	if m["role"] != "member" {
		t.Fatalf("seatless join wrong: %v", m)
	}

	// Listing invitations never leaks tokens.
	code, list := testsupport.DoAuthJSON(t, client, http.MethodGet,
		srv.URL+"/api/v1/coteries/"+id+"/invitations", "", tok)
	if code != http.StatusOK {
		t.Fatalf("list invitations status = %d: %v", code, list)
	}
	raw, _ := list["items"].([]any)
	for _, item := range raw {
		if _, has := item.(map[string]any)["token"]; has {
			t.Fatalf("invitation list leaked a token: %v", item)
		}
	}
}

// freeSeats lists the subscription's free seats and returns n ids —
// creating a coterie already provisions its capacity in seats.
func freeSeats(t *testing.T, client *http.Client, base, tok, subID string, n int) []string {
	t.Helper()
	code, body := testsupport.DoAuthJSON(t, client, http.MethodGet,
		base+"/api/v1/subscriptions/"+subID+"/seats", "", tok)
	if code != http.StatusOK {
		t.Fatalf("list seats: status = %d: %v", code, body)
	}
	items, _ := body["items"].([]any)
	ids := make([]string, 0, n)
	for _, raw := range items {
		s, _ := raw.(map[string]any)
		if s["status"] != "free" {
			continue
		}
		id, _ := s["id"].(string)
		ids = append(ids, id)
	}
	if len(ids) < n {
		t.Fatalf("free seats = %d, want %d: %v", len(ids), n, body)
	}
	return ids[:n]
}

// assignSeat assigns memberID to the seat and fails the test on error.
func assignSeat(t *testing.T, client *http.Client, base, tok, seatID, memberID string) (map[string]any, error) {
	t.Helper()
	code, body := testsupport.DoAuthJSON(t, client, http.MethodPost,
		base+"/api/v1/seats/"+seatID+"/assign", fmt.Sprintf(`{"member_id":%q}`, memberID), tok)
	if code != http.StatusOK {
		t.Fatalf("assign seat: status = %d: %v", code, body)
	}
	return body, nil
}

func TestCoterieBlocks(t *testing.T) {
	db := testsupport.NewDB(t)
	srv := testsupport.NewServer(t, db)
	client := srv.Client()
	tok, ownerID := testsupport.RegisterAndLogin(t, client, srv.URL, "cot-blk")
	subID := seedOwnedSubscription(t, client, srv.URL, "blk", tok, ownerID, 3)
	c := createCoterie(t, client, srv.URL, tok, subID, "Block Circle", 3)
	id, _ := c["id"].(string)

	// Recruit publicly so join requests are in play.
	code, body := testsupport.DoAuthJSON(t, client, http.MethodPatch,
		srv.URL+"/api/v1/coteries/"+id, `{"status":"open","listing":"public"}`, tok)
	if code != http.StatusOK {
		t.Fatalf("publish status = %d: %v", code, body)
	}

	// A member joins first — blocking later must not remove them.
	mtok, mid := testsupport.RegisterAndLogin(t, client, srv.URL, "cot-blk-m")
	accept(t, client, srv.URL, mtok, invite(t, client, srv.URL, tok, id, "member"))

	// An outsider to block.
	utok, uid := testsupport.RegisterAndLogin(t, client, srv.URL, "cot-blk-u")
	blockURL := srv.URL + "/api/v1/coteries/" + id + "/blocks/"

	// Only the owner manages the list.
	code, _ = testsupport.DoAuthJSON(t, client, http.MethodPut, blockURL+uid, "", utok)
	if code != http.StatusForbidden {
		t.Fatalf("non-owner block status = %d, want 403", code)
	}
	// Nonsense targets.
	code, _ = testsupport.DoAuthJSON(t, client, http.MethodPut, blockURL+ownerID, "", tok)
	if code != http.StatusUnprocessableEntity {
		t.Fatalf("self-block status = %d, want 422", code)
	}
	code, _ = testsupport.DoAuthJSON(t, client, http.MethodPut,
		blockURL+"00000000-0000-0000-0000-000000000000", "", tok)
	if code != http.StatusNotFound {
		t.Fatalf("unknown user block status = %d, want 404", code)
	}

	// Owner blocks the outsider: 201 with identity.
	code, body = testsupport.DoAuthJSON(t, client, http.MethodPut, blockURL+uid, "", tok)
	if code != http.StatusCreated {
		t.Fatalf("block status = %d, want 201: %v", code, body)
	}
	if body["user_id"] != uid || body["username"] == "" || body["email"] == "" {
		t.Fatalf("block entry identity wrong: %v", body)
	}
	created, _ := body["created_at"].(string)
	// PUT replay is idempotent: 200, same row.
	code, body = testsupport.DoAuthJSON(t, client, http.MethodPut, blockURL+uid, "", tok)
	if code != http.StatusOK || body["created_at"] != created {
		t.Fatalf("replay status = %d created_at = %v, want 200 same row", code, body["created_at"])
	}

	// The list is owner-only.
	code, _ = testsupport.DoAuthJSON(t, client, http.MethodGet, blockURL[:len(blockURL)-1], "", utok)
	if code != http.StatusForbidden {
		t.Fatalf("outsider list status = %d, want 403", code)
	}
	code, body = testsupport.DoAuthJSON(t, client, http.MethodGet, blockURL[:len(blockURL)-1], "", tok)
	if code != http.StatusOK {
		t.Fatalf("owner list status = %d: %v", code, body)
	}
	if meta, _ := body["meta"].(map[string]any); meta == nil || meta["total"] != float64(1) {
		t.Fatalf("block list total wrong: %v", body)
	}
	if items, _ := body["items"].([]any); len(items) != 1 || items[0].(map[string]any)["user_id"] != uid {
		t.Fatalf("block list items wrong: %v", body["items"])
	}

	// Join requests are refused while blocked.
	code, body = testsupport.DoAuthJSON(t, client, http.MethodPost,
		srv.URL+"/api/v1/coteries/"+id+"/join-requests", `{"message":"let me in"}`, utok)
	if code != http.StatusForbidden {
		t.Fatalf("blocked join request status = %d, want 403: %v", code, body)
	}
	// So is invitation acceptance.
	code, body = testsupport.DoAuthJSON(t, client, http.MethodPost,
		srv.URL+"/api/v1/invitations/accept",
		fmt.Sprintf(`{"token":%q}`, invite(t, client, srv.URL, tok, id, "member")), utok)
	if code != http.StatusForbidden {
		t.Fatalf("blocked invite accept status = %d, want 403: %v", code, body)
	}

	// The blocked member stays a member (no auto-removal).
	code, body = testsupport.DoAuthJSON(t, client, http.MethodGet,
		srv.URL+"/api/v1/coteries/"+id+"/members", "", tok)
	if code != http.StatusOK {
		t.Fatalf("members status = %d: %v", code, body)
	}
	if meta, _ := body["meta"].(map[string]any); meta == nil || meta["total"] != float64(2) {
		t.Fatalf("members total = %v, want owner + blocked member", body["meta"])
	}

	// Unblock reopens the paths; delete is not idempotent.
	code, _ = testsupport.DoAuthJSON(t, client, http.MethodDelete, blockURL+mid, "", tok)
	if code != http.StatusNotFound {
		t.Fatalf("delete absent block status = %d, want 404", code)
	}
	code, _ = testsupport.DoAuthJSON(t, client, http.MethodDelete, blockURL+uid, "", tok)
	if code != http.StatusNoContent {
		t.Fatalf("unblock status = %d, want 204", code)
	}
	code, _ = testsupport.DoAuthJSON(t, client, http.MethodDelete, blockURL+uid, "", tok)
	if code != http.StatusNotFound {
		t.Fatalf("re-unblock status = %d, want 404", code)
	}
	code, body = testsupport.DoAuthJSON(t, client, http.MethodPost,
		srv.URL+"/api/v1/coteries/"+id+"/join-requests", `{"message":"try again"}`, utok)
	if code != http.StatusCreated {
		t.Fatalf("join request after unblock status = %d, want 201: %v", code, body)
	}
}
