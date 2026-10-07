package user_test

import (
	"net/http"
	"testing"

	"github.com/cuihairu/coterie/internal/testsupport"
)

const usersPath = "/api/v1/users"

func TestUserLifecycle(t *testing.T) {
	db := testsupport.NewDB(t)
	srv := testsupport.NewServer(t, db)
	client := srv.Client()

	code, body := testsupport.DoJSON(t, client, http.MethodPost, srv.URL+usersPath,
		`{"username":"alice","email":"alice@example.com"}`)
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

	code, body = testsupport.DoJSON(t, client, http.MethodGet, srv.URL+usersPath+"/"+id, "")
	if code != http.StatusOK {
		t.Fatalf("get status = %d, want 200: %v", code, body)
	}
	if body["id"] != id {
		t.Fatalf("round-trip id = %v, want %v", body["id"], id)
	}

	code, body = testsupport.DoJSON(t, client, http.MethodGet, srv.URL+usersPath, "")
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

	if code, _ := testsupport.DoJSON(t, client, http.MethodPost, srv.URL+usersPath,
		`{"username":"bob","email":"bob@example.com"}`); code != http.StatusCreated {
		t.Fatalf("seed user status = %d, want 201", code)
	}

	code, body := testsupport.DoJSON(t, client, http.MethodPost, srv.URL+usersPath,
		`{"username":"bob","email":"other@example.com"}`)
	if code != http.StatusConflict {
		t.Fatalf("duplicate username status = %d, want 409: %v", code, body)
	}

	code, body = testsupport.DoJSON(t, client, http.MethodPost, srv.URL+usersPath,
		`{"username":"bob2","email":"BOB@example.com"}`)
	if code != http.StatusConflict {
		t.Fatalf("duplicate email (case-insensitive) status = %d, want 409: %v", code, body)
	}
}

func TestUserValidationErrors(t *testing.T) {
	db := testsupport.NewDB(t)
	srv := testsupport.NewServer(t, db)
	client := srv.Client()

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
			code, body := testsupport.DoJSON(t, client, http.MethodPost, srv.URL+usersPath, tc.body)
			if code != tc.want {
				t.Fatalf("status = %d, want %d: %v", code, tc.want, body)
			}
			if _, ok := body["error"]; !ok {
				t.Fatalf("expected error envelope: %v", body)
			}
		})
	}

	code, _ := testsupport.DoJSON(t, client, http.MethodGet, srv.URL+usersPath+"/00000000-0000-0000-0000-000000000000", "")
	if code != http.StatusNotFound {
		t.Fatalf("unknown user status = %d, want 404", code)
	}
}
