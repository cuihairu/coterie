package auth_test

import (
	"net/http"
	"testing"

	"github.com/cuihairu/coterie/internal/testsupport"
)

const authPath = "/api/v1/auth"

func TestRegisterLoginMeLogout(t *testing.T) {
	db := testsupport.NewDB(t)
	srv := testsupport.NewServer(t, db)
	client := srv.Client()

	code, body := testsupport.DoJSON(t, client, http.MethodPost, srv.URL+authPath+"/register",
		`{"username":"auth-main","email":"auth-main@example.com","password":"password-123"}`)
	if code != http.StatusCreated {
		t.Fatalf("register status = %d, want 201: %v", code, body)
	}
	token, _ := body["token"].(string)
	if token == "" {
		t.Fatalf("register returned no token: %v", body)
	}
	created, _ := body["user"].(map[string]any)
	if created["username"] != "auth-main" {
		t.Fatalf("register user echo wrong: %v", created)
	}
	if _, ok := created["password_hash"]; ok {
		t.Fatalf("password_hash must not appear in JSON: %v", created)
	}

	// me → 200 with the registered user.
	code, body = testsupport.DoAuthJSON(t, client, http.MethodGet, srv.URL+authPath+"/me", "", token)
	if code != http.StatusOK || body["username"] != "auth-main" {
		t.Fatalf("me status = %d body = %v", code, body)
	}

	// logout → 204, then the token is dead.
	code, _ = testsupport.DoAuthJSON(t, client, http.MethodPost, srv.URL+authPath+"/logout", "", token)
	if code != http.StatusNoContent {
		t.Fatalf("logout status = %d, want 204", code)
	}
	code, body = testsupport.DoAuthJSON(t, client, http.MethodGet, srv.URL+authPath+"/me", "", token)
	if code != http.StatusUnauthorized {
		t.Fatalf("me after logout status = %d, want 401: %v", code, body)
	}
}

func TestLoginErrorsAreIndistinguishable(t *testing.T) {
	db := testsupport.NewDB(t)
	srv := testsupport.NewServer(t, db)
	client := srv.Client()

	code, body := testsupport.DoJSON(t, client, http.MethodPost, srv.URL+authPath+"/register",
		`{"username":"auth-login","email":"auth-login@example.com","password":"password-123"}`)
	if code != http.StatusCreated {
		t.Fatalf("register status = %d: %v", code, body)
	}

	code, body = testsupport.DoJSON(t, client, http.MethodPost, srv.URL+authPath+"/login",
		`{"email":"auth-login@example.com","password":"wrong-password"}`)
	if code != http.StatusUnauthorized {
		t.Fatalf("wrong password status = %d, want 401: %v", code, body)
	}
	wrongPasswordMsg, _ := body["error"].(map[string]any)["message"].(string)

	code, body = testsupport.DoJSON(t, client, http.MethodPost, srv.URL+authPath+"/login",
		`{"email":"nobody@example.com","password":"password-123"}`)
	if code != http.StatusUnauthorized {
		t.Fatalf("unknown email status = %d, want 401: %v", code, body)
	}
	unknownEmailMsg, _ := body["error"].(map[string]any)["message"].(string)

	if wrongPasswordMsg == "" || wrongPasswordMsg != unknownEmailMsg {
		t.Fatalf("messages must match to prevent enumeration: %q vs %q", wrongPasswordMsg, unknownEmailMsg)
	}

	// Correct password → 200 with a usable token.
	code, body = testsupport.DoJSON(t, client, http.MethodPost, srv.URL+authPath+"/login",
		`{"email":"auth-login@example.com","password":"password-123"}`)
	if code != http.StatusOK || body["token"] == "" {
		t.Fatalf("login status = %d body = %v", code, body)
	}
}

func TestProtectedRoutesRejectAnonymous(t *testing.T) {
	db := testsupport.NewDB(t)
	srv := testsupport.NewServer(t, db)
	client := srv.Client()

	// No token at all.
	code, body := testsupport.DoJSON(t, client, http.MethodGet, srv.URL+"/api/v1/users", "")
	if code != http.StatusUnauthorized {
		t.Fatalf("no token status = %d, want 401: %v", code, body)
	}
	if errCode, _ := body["error"].(map[string]any)["code"].(string); errCode != "unauthorized" {
		t.Fatalf("error code = %q, want unauthorized", errCode)
	}

	// Forged token.
	code, _ = testsupport.DoAuthJSON(t, client, http.MethodGet,
		srv.URL+"/api/v1/users", "", "forged-token")
	if code != http.StatusUnauthorized {
		t.Fatalf("forged token status = %d, want 401", code)
	}

	// healthz stays public.
	code, _ = testsupport.DoJSON(t, client, http.MethodGet, srv.URL+"/healthz", "")
	if code != http.StatusOK {
		t.Fatalf("healthz status = %d, want 200", code)
	}
}

func TestRegisterValidation(t *testing.T) {
	db := testsupport.NewDB(t)
	srv := testsupport.NewServer(t, db)
	client := srv.Client()

	// Duplicate email → 409.
	if code, _ := testsupport.DoJSON(t, client, http.MethodPost, srv.URL+authPath+"/register",
		`{"username":"auth-dup","email":"auth-dup@example.com","password":"password-123"}`); code != http.StatusCreated {
		t.Fatalf("seed register status = %d, want 201", code)
	}
	code, body := testsupport.DoJSON(t, client, http.MethodPost, srv.URL+authPath+"/register",
		`{"username":"auth-dup2","email":"auth-dup@example.com","password":"password-123"}`)
	if code != http.StatusConflict {
		t.Fatalf("duplicate email status = %d, want 409: %v", code, body)
	}

	// Weak password → 422.
	code, body = testsupport.DoJSON(t, client, http.MethodPost, srv.URL+authPath+"/register",
		`{"username":"auth-weak","email":"auth-weak@example.com","password":"short"}`)
	if code != http.StatusUnprocessableEntity {
		t.Fatalf("weak password status = %d, want 422: %v", code, body)
	}

	// Missing password → 422.
	code, body = testsupport.DoJSON(t, client, http.MethodPost, srv.URL+authPath+"/register",
		`{"username":"auth-nopass","email":"auth-nopass@example.com"}`)
	if code != http.StatusUnprocessableEntity {
		t.Fatalf("missing password status = %d, want 422: %v", code, body)
	}
}
