package push_test

import (
	"encoding/base64"
	"fmt"
	"net/http"
	"strings"
	"testing"

	"github.com/cuihairu/coterie/internal/testsupport"
)

const pushPath = "/api/v1/push/subscriptions"

func registerBody(endpoint, p256dh, auth string) string {
	return fmt.Sprintf(`{"endpoint":%q,"keys":{"p256dh":%q,"auth":%q}}`, endpoint, p256dh, auth)
}

func clientKeyMaterial(t *testing.T) (string, string) {
	t.Helper()
	point := append([]byte{4}, make([]byte, 64)...) // shape-valid uncompressed point
	auth := make([]byte, 16)
	enc := base64.RawURLEncoding
	return enc.EncodeToString(point), enc.EncodeToString(auth)
}

// TestPushSubscriptionLifecycle covers D22 registration: https endpoint
// + base64url keys required, the listing is self-scoped, delete is
// self-scoped with 404 for unknown and foreign rows.
func TestPushSubscriptionLifecycle(t *testing.T) {
	db := testsupport.NewDB(t)
	srv := testsupport.NewServer(t, db)
	client, base := srv.Client(), srv.URL
	tok, _ := testsupport.RegisterAndLogin(t, client, base, "push-life")
	p256dh, auth := clientKeyMaterial(t)

	// Validation: non-https endpoint, missing keys, bad base64.
	for _, tc := range []struct{ name, body string }{
		{"http endpoint", registerBody("http://push.example.com/x", p256dh, auth)},
		{"no endpoint", registerBody("", p256dh, auth)},
		{"no p256dh", registerBody("https://push.example.com/x", "", auth)},
		{"bad base64 auth", registerBody("https://push.example.com/x", p256dh, "!!")},
	} {
		t.Run(tc.name, func(t *testing.T) {
			code, body := testsupport.DoAuthJSON(t, client, http.MethodPost, base+pushPath, tc.body, tok)
			if code != http.StatusUnprocessableEntity {
				t.Fatalf("status = %d, want 422: %v", code, body)
			}
		})
	}

	code, body := testsupport.DoAuthJSON(t, client, http.MethodPost, base+pushPath,
		registerBody("https://push.example.com/reg/one", p256dh, auth), tok)
	if code != http.StatusCreated {
		t.Fatalf("register: status = %d: %v", code, body)
	}
	id, _ := body["id"].(string)
	if id == "" {
		t.Fatalf("no id in response: %v", body)
	}
	if _, has := body["auth"]; has || body["p256dh"] != nil {
		t.Fatalf("key material leaked in response: %v", body)
	}

	// Re-registering the same endpoint refreshes instead of duplicating.
	code, body = testsupport.DoAuthJSON(t, client, http.MethodPost, base+pushPath,
		registerBody("https://push.example.com/reg/one", p256dh, auth), tok)
	if code != http.StatusCreated || body["id"] != id {
		t.Fatalf("re-register: status = %d id = %v, want 201 same id %s: %v", code, body["id"], id, body)
	}

	// Listing shows the single row.
	code, list := testsupport.DoAuthJSON(t, client, http.MethodGet, base+pushPath, "", tok)
	if code != http.StatusOK {
		t.Fatalf("list: status = %d: %v", code, list)
	}
	items, _ := list["items"].([]any)
	if len(items) != 1 {
		t.Fatalf("list items = %d, want 1", len(items))
	}

	// Another user sees nothing and cannot delete the row.
	tok2, _ := testsupport.RegisterAndLogin(t, client, base, "push-other")
	code, list = testsupport.DoAuthJSON(t, client, http.MethodGet, base+pushPath, "", tok2)
	if code != http.StatusOK || len(list["items"].([]any)) != 0 {
		t.Fatalf("other user's list not empty: %d %v", code, list)
	}
	code, body = testsupport.DoAuthJSON(t, client, http.MethodDelete, base+pushPath+"/"+id, "", tok2)
	if code != http.StatusNotFound {
		t.Fatalf("foreign delete: status = %d, want 404: %v", code, body)
	}

	// Owner delete: 204 and gone.
	code, body = testsupport.DoAuthJSON(t, client, http.MethodDelete, base+pushPath+"/"+id, "", tok)
	if code != http.StatusNoContent {
		t.Fatalf("delete: status = %d: %v", code, body)
	}
	code, _ = testsupport.DoAuthJSON(t, client, http.MethodDelete, base+pushPath+"/"+id, "", tok)
	if code != http.StatusNotFound {
		t.Fatalf("repeat delete: status = %d, want 404", code)
	}
	code, list = testsupport.DoAuthJSON(t, client, http.MethodGet, base+pushPath, "", tok)
	if code != http.StatusOK || len(list["items"].([]any)) != 0 {
		t.Fatalf("post-delete list: %d %v", code, list)
	}
}

// TestPushRequiresAuth: the endpoints are behind the session wall.
func TestPushRequiresAuth(t *testing.T) {
	db := testsupport.NewDB(t)
	srv := testsupport.NewServer(t, db)
	client, base := srv.Client(), srv.URL
	p256dh, auth := clientKeyMaterial(t)

	code, _ := testsupport.DoJSON(t, client, http.MethodPost, base+pushPath,
		registerBody("https://push.example.com/x", p256dh, auth))
	if code != http.StatusUnauthorized {
		t.Fatalf("anon register: status = %d, want 401", code)
	}
	code, _ = testsupport.DoJSON(t, client, http.MethodGet, base+pushPath, "")
	if code != http.StatusUnauthorized {
		t.Fatalf("anon list: status = %d, want 401", code)
	}
	code, _ = testsupport.DoJSON(t, client, http.MethodDelete, base+pushPath+"/"+strings.Repeat("0", 8), "")
	if code != http.StatusUnauthorized {
		t.Fatalf("anon delete: status = %d, want 401", code)
	}
}
