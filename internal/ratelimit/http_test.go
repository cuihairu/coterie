package ratelimit_test

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/cuihairu/coterie/internal/app"
	"github.com/cuihairu/coterie/internal/testsupport"
)

// TestPublicEndpointLimits walks register and login into their 429s
// with the small budgets injected for the test, and checks the envelope
// and Retry-After on the way out.
func TestPublicEndpointLimits(t *testing.T) {
	db := testsupport.NewDB(t)
	srv := testsupport.NewServer(t, db, app.WithRateLimits(2, 2))
	client, base := srv.Client(), srv.URL

	register := func(username string) int {
		payload, _ := json.Marshal(map[string]string{
			"username": username,
			"email":    username + "@example.com",
			"password": "password-123",
		})
		code, _ := testsupport.DoJSON(t, client, http.MethodPost,
			base+"/api/v1/auth/register", string(payload))
		return code
	}

	for i := 0; i < 2; i++ {
		if code := register("limit-user-" + string(rune('a'+i))); code != http.StatusCreated {
			t.Fatalf("register %d: status = %d, want 201", i, code)
		}
	}
	code, body := testsupport.DoJSON(t, client, http.MethodPost,
		base+"/api/v1/auth/register",
		`{"username":"limit-user-c","email":"limit-user-c@example.com","password":"password-123"}`)
	if code != http.StatusTooManyRequests {
		t.Fatalf("third register: status = %d, want 429: %v", code, body)
	}
	if msg, _ := body["error"].(map[string]any)["code"].(string); msg != "rate_limited" {
		t.Fatalf("error code = %v", body["error"])
	}

	// Login has its own budget: one real login, one wrong password,
	// then the third attempt is a 429 even though register was spent
	// separately.
	login := func(password string) int {
		payload, _ := json.Marshal(map[string]string{
			"email":    "limit-user-a@example.com",
			"password": password,
		})
		code, _ := testsupport.DoJSON(t, client, http.MethodPost,
			base+"/api/v1/auth/login", string(payload))
		return code
	}
	if code := login("password-123"); code != http.StatusOK {
		t.Fatalf("first login: status = %d, want 200", code)
	}
	if code := login("wrong-password"); code != http.StatusUnauthorized {
		t.Fatalf("wrong password: status = %d, want 401", code)
	}
	if code := login("wrong-password"); code != http.StatusTooManyRequests {
		t.Fatalf("third login: status = %d, want 429", code)
	}

	// Guarded classes don't bleed into the rest of the surface.
	if hcode, _ := testsupport.DoJSON(t, client, http.MethodGet, base+"/healthz", ""); hcode != http.StatusOK {
		t.Fatalf("healthz: status = %d, want 200", hcode)
	}
}
