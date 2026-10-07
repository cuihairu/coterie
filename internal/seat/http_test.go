package seat_test

import (
	"fmt"
	"net/http"
	"testing"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/cuihairu/coterie/internal/testsupport"
)

// seedSubscription creates provider → product → subscription through the
// API with the authenticated user as owner and returns the subscription id.
func seedSubscription(t *testing.T, client *http.Client, base, tag, tok, ownerUserID string, maxSeats int) string {
	t.Helper()
	code, body := testsupport.DoAuthJSON(t, client, http.MethodPost, base+"/api/v1/providers",
		fmt.Sprintf(`{"slug":"seat-%s","name":"Seat Provider %s","category":"video"}`, tag, tag), tok)
	if code != http.StatusCreated {
		t.Fatalf("seed provider: status = %d: %v", code, body)
	}
	providerID, _ := body["id"].(string)

	code, body = testsupport.DoAuthJSON(t, client, http.MethodPost, base+"/api/v1/products",
		fmt.Sprintf(`{"provider_id":%q,"name":"Seat Product %s"}`, providerID, tag), tok)
	if code != http.StatusCreated {
		t.Fatalf("seed product: status = %d: %v", code, body)
	}
	productID, _ := body["id"].(string)

	code, body = testsupport.DoAuthJSON(t, client, http.MethodPost, base+"/api/v1/subscriptions",
		fmt.Sprintf(`{"product_id":%q,"owner_user_id":%q,"billing_cycle":"monthly","price":"9.99","currency":"USD","start_date":"2026-10-01","max_seats":%d}`, productID, ownerUserID, maxSeats), tok)
	if code != http.StatusCreated {
		t.Fatalf("seed subscription: status = %d: %v", code, body)
	}
	subID, _ := body["id"].(string)
	return subID
}

// seedCoterieWithMember inserts a coterie and an active member directly
// (the coterie module itself ships in the next increment).
func seedCoterieWithMember(t *testing.T, db *gorm.DB, subID, memberUserID string) string {
	t.Helper()
	coterieID := uuid.NewString()
	if err := db.Exec(
		"INSERT INTO coteries (id, subscription_id, name) VALUES (?, ?, ?)",
		coterieID, subID, "Seat Test Circle",
	).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec(
		"INSERT INTO members (id, coterie_id, user_id, role) VALUES (?, ?, ?, 'member')",
		uuid.NewString(), coterieID, memberUserID,
	).Error; err != nil {
		t.Fatal(err)
	}
	return coterieID
}

func TestSeatProvisionAndList(t *testing.T) {
	db := testsupport.NewDB(t)
	srv := testsupport.NewServer(t, db)
	client := srv.Client()
	tok, ownerID := testsupport.RegisterAndLogin(t, client, srv.URL, "seat-prov")
	subID := seedSubscription(t, client, srv.URL, "prov", tok, ownerID, 3)

	// count 0 → 422.
	code, body := testsupport.DoAuthJSON(t, client, http.MethodPost,
		srv.URL+"/api/v1/subscriptions/"+subID+"/seats", `{"count":0}`, tok)
	if code != http.StatusUnprocessableEntity {
		t.Fatalf("count 0 status = %d, want 422: %v", code, body)
	}

	// Provision 3 of 3.
	code, body = testsupport.DoAuthJSON(t, client, http.MethodPost,
		srv.URL+"/api/v1/subscriptions/"+subID+"/seats", `{"count":3}`, tok)
	if code != http.StatusCreated {
		t.Fatalf("provision status = %d, want 201: %v", code, body)
	}
	items, _ := body["items"].([]any)
	if len(items) != 3 {
		t.Fatalf("provisioned = %d seats, want 3: %v", len(items), body)
	}
	first, _ := items[0].(map[string]any)
	if first["label"] != "Seat 1" || first["status"] != "free" {
		t.Fatalf("unexpected first seat: %v", first)
	}

	// Capacity is full → one more is a 409.
	code, body = testsupport.DoAuthJSON(t, client, http.MethodPost,
		srv.URL+"/api/v1/subscriptions/"+subID+"/seats", `{"count":1}`, tok)
	if code != http.StatusConflict {
		t.Fatalf("over-provision status = %d, want 409: %v", code, body)
	}

	// List shows all three.
	code, body = testsupport.DoAuthJSON(t, client, http.MethodGet,
		srv.URL+"/api/v1/subscriptions/"+subID+"/seats", "", tok)
	if code != http.StatusOK {
		t.Fatalf("list status = %d: %v", code, body)
	}
	if total, _ := body["meta"].(map[string]any)["total"].(float64); total != 3 {
		t.Fatalf("list total = %v, want 3", body["meta"])
	}
}

func TestSeatAssignRelease(t *testing.T) {
	db := testsupport.NewDB(t)
	srv := testsupport.NewServer(t, db)
	client := srv.Client()
	tok, ownerID := testsupport.RegisterAndLogin(t, client, srv.URL, "seat-asg")
	subID := seedSubscription(t, client, srv.URL, "asg", tok, ownerID, 2)

	code, seats := testsupport.DoAuthJSON(t, client, http.MethodPost,
		srv.URL+"/api/v1/subscriptions/"+subID+"/seats", `{"count":2}`, tok)
	if code != http.StatusCreated {
		t.Fatalf("provision status = %d: %v", code, seats)
	}
	seatList, _ := seats["items"].([]any)
	seatA, _ := seatList[0].(map[string]any)
	seatAID, _ := seatA["id"].(string)

	// A platform user to become the member.
	code, memberUser := testsupport.DoAuthJSON(t, client, http.MethodPost, srv.URL+"/api/v1/users",
		`{"username":"seat-member","email":"seat-member@example.com"}`, tok)
	if code != http.StatusCreated {
		t.Fatalf("seed member user: status = %d: %v", code, memberUser)
	}
	memberUserID, _ := memberUser["id"].(string)
	seedCoterieWithMember(t, db, subID, memberUserID)

	// Assign to a non-member → 422.
	code, body := testsupport.DoAuthJSON(t, client, http.MethodPost,
		srv.URL+"/api/v1/seats/"+seatAID+"/assign",
		fmt.Sprintf(`{"member_id":%q}`, ownerID), tok)
	if code != http.StatusUnprocessableEntity {
		t.Fatalf("assign non-member status = %d, want 422: %v", code, body)
	}

	// Assign the member → occupied with member_id.
	code, body = testsupport.DoAuthJSON(t, client, http.MethodPost,
		srv.URL+"/api/v1/seats/"+seatAID+"/assign",
		fmt.Sprintf(`{"member_id":%q}`, memberUserID), tok)
	if code != http.StatusOK {
		t.Fatalf("assign status = %d, want 200: %v", code, body)
	}
	if body["status"] != "occupied" || body["member_id"] != memberUserID {
		t.Fatalf("assign result wrong: %v", body)
	}

	// Second assign → 409.
	code, body = testsupport.DoAuthJSON(t, client, http.MethodPost,
		srv.URL+"/api/v1/seats/"+seatAID+"/assign",
		fmt.Sprintf(`{"member_id":%q}`, memberUserID), tok)
	if code != http.StatusConflict {
		t.Fatalf("double assign status = %d, want 409: %v", code, body)
	}

	// Release → free, no member.
	code, body = testsupport.DoAuthJSON(t, client, http.MethodPost,
		srv.URL+"/api/v1/seats/"+seatAID+"/release", "", tok)
	if code != http.StatusOK || body["status"] != "free" {
		t.Fatalf("release status = %d body = %v", code, body)
	}
	if _, ok := body["member_id"]; ok {
		t.Fatalf("released seat must drop member_id: %v", body)
	}
}

func TestSeatUpdateAndDisable(t *testing.T) {
	db := testsupport.NewDB(t)
	srv := testsupport.NewServer(t, db)
	client := srv.Client()
	tok, ownerID := testsupport.RegisterAndLogin(t, client, srv.URL, "seat-upd")
	subID := seedSubscription(t, client, srv.URL, "upd", tok, ownerID, 2)

	code, seats := testsupport.DoAuthJSON(t, client, http.MethodPost,
		srv.URL+"/api/v1/subscriptions/"+subID+"/seats", `{"count":1}`, tok)
	if code != http.StatusCreated {
		t.Fatalf("provision status = %d: %v", code, seats)
	}
	seatList, _ := seats["items"].([]any)
	seatA, _ := seatList[0].(map[string]any)
	seatAID, _ := seatA["id"].(string)

	// PATCH label and metadata.
	code, body := testsupport.DoAuthJSON(t, client, http.MethodPatch,
		srv.URL+"/api/v1/seats/"+seatAID,
		`{"label":"Quota A","metadata":{"quota":300,"unit":"credits","used":0}}`, tok)
	if code != http.StatusOK || body["label"] != "Quota A" {
		t.Fatalf("patch status = %d body = %v", code, body)
	}
	meta, _ := body["metadata"].(map[string]any)
	if meta == nil || meta["quota"] != float64(300) {
		t.Fatalf("metadata not echoed: %v", body)
	}

	// status occupied via PATCH → 422 (must use /assign).
	code, body = testsupport.DoAuthJSON(t, client, http.MethodPatch,
		srv.URL+"/api/v1/seats/"+seatAID, `{"status":"occupied"}`, tok)
	if code != http.StatusUnprocessableEntity {
		t.Fatalf("patch occupied status = %d, want 422: %v", code, body)
	}

	// Disable the free seat, then assignment fails with 409.
	code, body = testsupport.DoAuthJSON(t, client, http.MethodPatch,
		srv.URL+"/api/v1/seats/"+seatAID, `{"status":"disabled"}`, tok)
	if code != http.StatusOK || body["status"] != "disabled" {
		t.Fatalf("disable status = %d body = %v", code, body)
	}
	code, body = testsupport.DoAuthJSON(t, client, http.MethodPost,
		srv.URL+"/api/v1/seats/"+seatAID+"/assign", `{"member_id":"x"}`, tok)
	if code != http.StatusConflict {
		t.Fatalf("assign disabled status = %d, want 409: %v", code, body)
	}

	// Re-enable.
	code, body = testsupport.DoAuthJSON(t, client, http.MethodPatch,
		srv.URL+"/api/v1/seats/"+seatAID, `{"status":"free"}`, tok)
	if code != http.StatusOK || body["status"] != "free" {
		t.Fatalf("enable status = %d body = %v", code, body)
	}

	// Unknown seat → 404.
	code, _ = testsupport.DoAuthJSON(t, client, http.MethodGet,
		srv.URL+"/api/v1/seats/00000000-0000-0000-0000-000000000000", "", tok)
	if code != http.StatusNotFound {
		t.Fatalf("unknown seat status = %d, want 404", code)
	}
}

func TestSeatOwnerOnlyMutations(t *testing.T) {
	db := testsupport.NewDB(t)
	srv := testsupport.NewServer(t, db)
	client := srv.Client()
	tok, ownerID := testsupport.RegisterAndLogin(t, client, srv.URL, "seat-own")
	subID := seedSubscription(t, client, srv.URL, "own", tok, ownerID, 2)

	other, _ := testsupport.RegisterAndLogin(t, client, srv.URL, "seat-other")

	// Non-owner provision → 403.
	code, body := testsupport.DoAuthJSON(t, client, http.MethodPost,
		srv.URL+"/api/v1/subscriptions/"+subID+"/seats", `{"count":1}`, other)
	if code != http.StatusForbidden {
		t.Fatalf("non-owner provision status = %d, want 403: %v", code, body)
	}

	code, seats := testsupport.DoAuthJSON(t, client, http.MethodPost,
		srv.URL+"/api/v1/subscriptions/"+subID+"/seats", `{"count":1}`, tok)
	if code != http.StatusCreated {
		t.Fatalf("provision status = %d: %v", code, seats)
	}
	seatList, _ := seats["items"].([]any)
	seatA, _ := seatList[0].(map[string]any)
	seatAID, _ := seatA["id"].(string)

	// Non-owner release → 403.
	code, body = testsupport.DoAuthJSON(t, client, http.MethodPost,
		srv.URL+"/api/v1/seats/"+seatAID+"/release", "", other)
	if code != http.StatusForbidden {
		t.Fatalf("non-owner release status = %d, want 403: %v", code, body)
	}

	// Reads stay open to authenticated users.
	code, _ = testsupport.DoAuthJSON(t, client, http.MethodGet,
		srv.URL+"/api/v1/subscriptions/"+subID+"/seats", "", other)
	if code != http.StatusOK {
		t.Fatalf("non-owner read status = %d, want 200", code)
	}
}
