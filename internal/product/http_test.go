package product_test

import (
	"fmt"
	"net/http"
	"testing"

	"github.com/cuihairu/coterie/internal/testsupport"
)

const productsPath = "/api/v1/products"

func TestProductLifecycle(t *testing.T) {
	db := testsupport.NewDB(t)
	srv := testsupport.NewServer(t, db)
	client := srv.Client()
	tok, _ := testsupport.RegisterAndLogin(t, client, srv.URL, "prod-life")
	providerID := seedProvider(t, client, srv.URL, "main-provider", tok)

	code, body := testsupport.DoAuthJSON(t, client, http.MethodPost, srv.URL+productsPath,
		fmt.Sprintf(`{"provider_id":%q,"name":"Premium","tier":"4K","metadata":{"profiles":5}}`, providerID), tok)
	if code != http.StatusCreated {
		t.Fatalf("create status = %d, want 201: %v", code, body)
	}
	id, _ := body["id"].(string)
	if id == "" || body["provider_id"] != providerID {
		t.Fatalf("unexpected create body: %v", body)
	}
	if body["tier"] != "4K" {
		t.Fatalf("tier not echoed: %v", body)
	}

	code, body = testsupport.DoAuthJSON(t, client, http.MethodGet, srv.URL+productsPath+"/"+id, "", tok)
	if code != http.StatusOK || body["name"] != "Premium" {
		t.Fatalf("get status = %d body = %v", code, body)
	}

	code, body = testsupport.DoAuthJSON(t, client, http.MethodPatch, srv.URL+productsPath+"/"+id,
		`{"name":"Premium Plus"}`, tok)
	if code != http.StatusOK || body["name"] != "Premium Plus" {
		t.Fatalf("patch status = %d body = %v", code, body)
	}

	code, body = testsupport.DoAuthJSON(t, client, http.MethodGet,
		srv.URL+productsPath+"?provider_id="+providerID, "", tok)
	if code != http.StatusOK {
		t.Fatalf("list status = %d: %v", code, body)
	}
	items, _ := body["items"].([]any)
	if len(items) != 1 {
		t.Fatalf("provider filter items = %d, want 1: %v", len(items), body)
	}

	code, _ = testsupport.DoAuthJSON(t, client, http.MethodDelete, srv.URL+productsPath+"/"+id, "", tok)
	if code != http.StatusNoContent {
		t.Fatalf("delete status = %d, want 204", code)
	}
}

func TestProductProviderFilterIsolates(t *testing.T) {
	db := testsupport.NewDB(t)
	srv := testsupport.NewServer(t, db)
	client := srv.Client()
	tok, _ := testsupport.RegisterAndLogin(t, client, srv.URL, "prod-filter")
	providerA := seedProvider(t, client, srv.URL, "provider-a", tok)
	providerB := seedProvider(t, client, srv.URL, "provider-b", tok)

	for _, name := range []string{"Basic", "Standard"} {
		createProduct(t, client, srv.URL, providerA, name, tok)
	}
	createProduct(t, client, srv.URL, providerB, "Solo", tok)

	code, body := testsupport.DoAuthJSON(t, client, http.MethodGet,
		srv.URL+productsPath+"?provider_id="+providerA, "", tok)
	if code != http.StatusOK {
		t.Fatalf("list status = %d: %v", code, body)
	}
	items, _ := body["items"].([]any)
	if len(items) != 2 {
		t.Fatalf("provider A items = %d, want 2: %v", len(items), body)
	}
	if total, _ := body["meta"].(map[string]any)["total"].(float64); total != 2 {
		t.Fatalf("provider A total = %v, want 2", body["meta"])
	}
}

func TestProductValidationAndConflicts(t *testing.T) {
	db := testsupport.NewDB(t)
	srv := testsupport.NewServer(t, db)
	client := srv.Client()
	tok, _ := testsupport.RegisterAndLogin(t, client, srv.URL, "prod-conf")
	providerID := seedProvider(t, client, srv.URL, "conflict-provider", tok)

	// Unknown provider → 422 with a field detail.
	code, body := testsupport.DoAuthJSON(t, client, http.MethodPost, srv.URL+productsPath,
		`{"provider_id":"00000000-0000-0000-0000-000000000000","name":"Ghost"}`, tok)
	if code != http.StatusUnprocessableEntity {
		t.Fatalf("unknown provider status = %d, want 422: %v", code, body)
	}

	// Duplicate (provider_id, name) → 409.
	createProduct(t, client, srv.URL, providerID, "Dup", tok)
	code, body = testsupport.DoAuthJSON(t, client, http.MethodPost, srv.URL+productsPath,
		fmt.Sprintf(`{"provider_id":%q,"name":"Dup"}`, providerID), tok)
	if code != http.StatusConflict {
		t.Fatalf("duplicate name status = %d, want 409: %v", code, body)
	}

	// Missing name → 422.
	code, body = testsupport.DoAuthJSON(t, client, http.MethodPost, srv.URL+productsPath,
		fmt.Sprintf(`{"provider_id":%q}`, providerID), tok)
	if code != http.StatusUnprocessableEntity {
		t.Fatalf("missing name status = %d, want 422: %v", code, body)
	}

	// Deleting a provider that still has products → 409.
	code, body = testsupport.DoAuthJSON(t, client, http.MethodDelete,
		srv.URL+"/api/v1/providers/"+providerID, "", tok)
	if code != http.StatusConflict {
		t.Fatalf("delete provider with products status = %d, want 409: %v", code, body)
	}

	// Unknown product → 404.
	code, _ = testsupport.DoAuthJSON(t, client, http.MethodGet,
		srv.URL+productsPath+"/00000000-0000-0000-0000-000000000000", "", tok)
	if code != http.StatusNotFound {
		t.Fatalf("unknown product status = %d, want 404", code)
	}
}

func createProduct(t *testing.T, client *http.Client, base, providerID, name, tok string) string {
	t.Helper()
	code, body := testsupport.DoAuthJSON(t, client, http.MethodPost, base+productsPath,
		fmt.Sprintf(`{"provider_id":%q,"name":%q}`, providerID, name), tok)
	if code != http.StatusCreated {
		t.Fatalf("create product %s: status = %d: %v", name, code, body)
	}
	id, _ := body["id"].(string)
	if id == "" {
		t.Fatalf("create product %s: no id", name)
	}
	return id
}

// seedProvider creates a provider through the API and returns its id.
func seedProvider(t *testing.T, client *http.Client, base, slug, tok string) string {
	t.Helper()
	code, body := testsupport.DoAuthJSON(t, client, http.MethodPost, base+"/api/v1/providers",
		fmt.Sprintf(`{"slug":%q,"name":%q,"category":"video"}`, slug, slug), tok)
	if code != http.StatusCreated {
		t.Fatalf("seed provider %s: status = %d: %v", slug, code, body)
	}
	id, _ := body["id"].(string)
	if id == "" {
		t.Fatalf("seed provider %s: no id", slug)
	}
	return id
}
