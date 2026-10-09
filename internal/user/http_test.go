package user_test

import (
	"context"
	"net/http"
	"strings"
	"testing"

	"github.com/cuihairu/coterie/internal/testsupport"
	"github.com/cuihairu/coterie/internal/user"
)

const usersPath = "/api/v1/users"

func TestUserLifecycle(t *testing.T) {
	db := testsupport.NewDB(t)
	srv := testsupport.NewServer(t, db)
	client := srv.Client()
	tok, _ := testsupport.RegisterAndLogin(t, client, srv.URL, "user-life")

	code, body := testsupport.DoAuthJSON(t, client, http.MethodPost, srv.URL+usersPath,
		`{"username":"alice","email":"alice@example.com"}`, tok)
	if code != http.StatusCreated {
		t.Fatalf("create status = %d, want 201: %v", code, body)
	}
	id, _ := body["id"].(string)
	if id == "" {
		t.Fatal("expected non-empty id")
	}
	if body["username"] != "alice" || body["email"] != "alice@example.com" {
		t.Fatalf("unexpected echo: %v", body)
	}
	if body["locale"] != "en" || body["timezone"] != "UTC" || body["status"] != "active" {
		t.Fatalf("defaults not applied: %v", body)
	}

	code, body = testsupport.DoAuthJSON(t, client, http.MethodGet, srv.URL+usersPath+"/"+id, "", tok)
	if code != http.StatusOK {
		t.Fatalf("get status = %d, want 200: %v", code, body)
	}
	if body["id"] != id {
		t.Fatalf("round-trip id = %v, want %v", body["id"], id)
	}

	code, body = testsupport.DoAuthJSON(t, client, http.MethodGet, srv.URL+usersPath, "", tok)
	if code != http.StatusOK {
		t.Fatalf("list status = %d, want 200: %v", code, body)
	}
	items, _ := body["items"].([]any)
	meta, _ := body["meta"].(map[string]any)
	if items == nil || meta == nil {
		t.Fatalf("list envelope missing items/meta: %v", body)
	}
	if total, _ := meta["total"].(float64); total < 1 {
		t.Fatalf("list total = %v, want >= 1", meta["total"])
	}
}

func TestUserConflicts(t *testing.T) {
	db := testsupport.NewDB(t)
	srv := testsupport.NewServer(t, db)
	client := srv.Client()
	tok, _ := testsupport.RegisterAndLogin(t, client, srv.URL, "user-conf")

	if code, _ := testsupport.DoAuthJSON(t, client, http.MethodPost, srv.URL+usersPath,
		`{"username":"bob","email":"bob@example.com"}`, tok); code != http.StatusCreated {
		t.Fatalf("seed user status = %d, want 201", code)
	}

	code, body := testsupport.DoAuthJSON(t, client, http.MethodPost, srv.URL+usersPath,
		`{"username":"bob","email":"other@example.com"}`, tok)
	if code != http.StatusConflict {
		t.Fatalf("duplicate username status = %d, want 409: %v", code, body)
	}

	code, body = testsupport.DoAuthJSON(t, client, http.MethodPost, srv.URL+usersPath,
		`{"username":"bob2","email":"BOB@example.com"}`, tok)
	if code != http.StatusConflict {
		t.Fatalf("duplicate email (case-insensitive) status = %d, want 409: %v", code, body)
	}
}

func TestUserValidationErrors(t *testing.T) {
	db := testsupport.NewDB(t)
	srv := testsupport.NewServer(t, db)
	client := srv.Client()
	tok, _ := testsupport.RegisterAndLogin(t, client, srv.URL, "user-valid")

	cases := []struct {
		name string
		body string
		want int
	}{
		{"missing username", `{"email":"x@example.com"}`, http.StatusUnprocessableEntity},
		{"missing email", `{"username":"carol"}`, http.StatusUnprocessableEntity},
		{"bad email", `{"username":"carol","email":"not-an-email"}`, http.StatusUnprocessableEntity},
		{"malformed json", `{"username":`, http.StatusBadRequest},
		{"unknown field", `{"username":"carol","email":"c@example.com","admin":true}`, http.StatusBadRequest},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			code, body := testsupport.DoAuthJSON(t, client, http.MethodPost, srv.URL+usersPath, tc.body, tok)
			if code != tc.want {
				t.Fatalf("status = %d, want %d: %v", code, tc.want, body)
			}
			if _, ok := body["error"]; !ok {
				t.Fatalf("expected error envelope: %v", body)
			}
		})
	}

	code, _ := testsupport.DoAuthJSON(t, client, http.MethodGet,
		srv.URL+usersPath+"/00000000-0000-0000-0000-000000000000", "", tok)
	if code != http.StatusNotFound {
		t.Fatalf("unknown user status = %d, want 404", code)
	}
}

// The admin role is invisible at registration and appears only through
// the ADMIN_EMAILS bootstrap (design D26).
func TestAdminRoleBootstrap(t *testing.T) {
	srv := testsupport.NewServer(t, testsupport.NewDB(t))
	client, base := srv.Client(), srv.URL
	tok, userID := testsupport.RegisterAndLogin(t, client, base, "role-boot")

	// Fresh users are plain 'user'.
	code, body := testsupport.DoAuthJSON(t, client, http.MethodGet,
		base+"/api/v1/auth/me", "", tok)
	if code != http.StatusOK {
		t.Fatalf("me: status = %d: %v", code, body)
	}
	if body["role"] != "user" {
		t.Fatalf("role = %v, want user", body["role"])
	}

	// The bootstrap is idempotent and case-insensitive on email.
	code, body = testsupport.DoAuthJSON(t, client, http.MethodGet,
		base+"/api/v1/auth/me", "", tok)
	email, _ := body["email"].(string)
	n, err := user.EnsureAdmins(context.Background(), testsupport.NewDB(t), []string{strings.ToUpper(email)})
	if err != nil || n != 1 {
		t.Fatalf("EnsureAdmins = %d, %v; want 1 promoted", n, err)
	}
	if n2, err := user.EnsureAdmins(context.Background(), testsupport.NewDB(t), []string{email}); err != nil || n2 != 0 {
		t.Fatalf("second EnsureAdmins = %d, %v; want idempotent 0", n2, err)
	}

	// The promoted user now carries the role.
	code, body = testsupport.DoAuthJSON(t, client, http.MethodGet,
		base+"/api/v1/auth/me", "", tok)
	if body["role"] != "admin" {
		t.Fatalf("role after bootstrap = %v, want admin", body["role"])
	}
	_ = userID
}
