package provider_test

import (
	"fmt"
	"net/http"
	"testing"

	"github.com/cuihairu/coterie/internal/testsupport"
)

const providersPath = "/api/v1/providers"

func TestProviderLifecycle(t *testing.T) {
	db := testsupport.NewDB(t)
	srv := testsupport.NewServer(t, db)
	client := srv.Client()
	tok, _ := testsupport.RegisterAndLogin(t, client, srv.URL, "prov-life")

	code, body := testsupport.DoAuthJSON(t, client, http.MethodPost, srv.URL+providersPath,
		`{"slug":"ts-netflix","name":"Netflix Fixture","category":"video","metadata":{"region":"US"}}`, tok)
	if code != http.StatusCreated {
		t.Fatalf("create status = %d, want 201: %v", code, body)
	}
	id, _ := body["id"].(string)
	if id == "" {
		t.Fatal("expected non-empty id")
	}
	meta, _ := body["metadata"].(map[string]any)
	if meta == nil || meta["region"] != "US" {
		t.Fatalf("metadata not echoed: %v", body)
	}

	code, body = testsupport.DoAuthJSON(t, client, http.MethodGet, srv.URL+providersPath+"/"+id, "", tok)
	if code != http.StatusOK || body["slug"] != "ts-netflix" {
		t.Fatalf("get status = %d body = %v", code, body)
	}

	code, body = testsupport.DoAuthJSON(t, client, http.MethodPatch, srv.URL+providersPath+"/"+id,
		`{"name":"Netflix Inc.","metadata":{"region":"EU"}}`, tok)
	if code != http.StatusOK || body["name"] != "Netflix Inc." {
		t.Fatalf("patch status = %d body = %v", code, body)
	}
	if meta, _ = body["metadata"].(map[string]any); meta == nil || meta["region"] != "EU" {
		t.Fatalf("patched metadata wrong: %v", body)
	}
	if body["slug"] != "ts-netflix" {
		t.Fatalf("patch must not touch slug: %v", body)
	}

	code, body = testsupport.DoAuthJSON(t, client, http.MethodGet, srv.URL+providersPath+"?category=video", "", tok)
	if code != http.StatusOK {
		t.Fatalf("list status = %d: %v", code, body)
	}
	items, _ := body["items"].([]any)
	if len(items) < 1 {
		t.Fatalf("category filter returned nothing: %v", body)
	}

	code, _ = testsupport.DoAuthJSON(t, client, http.MethodDelete, srv.URL+providersPath+"/"+id, "", tok)
	if code != http.StatusNoContent {
		t.Fatalf("delete status = %d, want 204", code)
	}
	code, _ = testsupport.DoAuthJSON(t, client, http.MethodGet, srv.URL+providersPath+"/"+id, "", tok)
	if code != http.StatusNotFound {
		t.Fatalf("get after delete status = %d, want 404", code)
	}
}

func TestProviderSlugConflictIsCaseInsensitive(t *testing.T) {
	db := testsupport.NewDB(t)
	srv := testsupport.NewServer(t, db)
	client := srv.Client()
	tok, _ := testsupport.RegisterAndLogin(t, client, srv.URL, "prov-conf")

	if code, body := testsupport.DoAuthJSON(t, client, http.MethodPost, srv.URL+providersPath,
		`{"slug":"ts-spotify","name":"Spotify Fixture","category":"music"}`, tok); code != http.StatusCreated {
		t.Fatalf("seed status = %d: %v", code, body)
	}

	code, body := testsupport.DoAuthJSON(t, client, http.MethodPost, srv.URL+providersPath,
		`{"slug":"TS-SPOTIFY","name":"Spotify Again","category":"music"}`, tok)
	if code != http.StatusConflict {
		t.Fatalf("duplicate slug status = %d, want 409: %v", code, body)
	}
}

// TestCatalogPublicReadsAndSeeds covers the Provider Catalog increment:
// a fresh database ships the seeded registry (design §5.4), catalog
// reads are public, and catalog mutations still need a session.
func TestCatalogPublicReadsAndSeeds(t *testing.T) {
	db := testsupport.NewDB(t)
	srv := testsupport.NewServer(t, db)
	client := srv.Client()

	// Anonymous browse finds the seeded AI providers.
	code, body := testsupport.DoJSON(t, client, http.MethodGet, srv.URL+providersPath+"?category=ai", "")
	if code != http.StatusOK {
		t.Fatalf("anonymous category list: status = %d: %v", code, body)
	}
	items, _ := body["items"].([]any)
	slugs := map[string]string{} // slug -> id
	for _, it := range items {
		p := it.(map[string]any)
		slugs[p["slug"].(string)] = p["id"].(string)
	}
	if slugs["chatgpt"] == "" || slugs["claude"] == "" {
		t.Fatalf("seeded ai providers missing: %v", slugs)
	}

	// Anonymous product list under a seeded provider shows its plans.
	code, body = testsupport.DoJSON(t, client, http.MethodGet,
		srv.URL+"/api/v1/products?provider_id="+slugs["chatgpt"], "")
	if code != http.StatusOK {
		t.Fatalf("anonymous product list: status = %d: %v", code, body)
	}
	items, _ = body["items"].([]any)
	names := map[string]bool{}
	for _, it := range items {
		p := it.(map[string]any)
		names[p["name"].(string)] = true
	}
	if !names["Plus"] || !names["Pro"] {
		t.Fatalf("seeded chatgpt plans missing: %v", names)
	}

	// Anonymous single reads work too.
	code, _ = testsupport.DoJSON(t, client, http.MethodGet, srv.URL+providersPath+"/"+slugs["chatgpt"], "")
	if code != http.StatusOK {
		t.Fatalf("anonymous provider get: status = %d", code)
	}

	// Mutations stay closed: 401 without a session.
	code, _ = testsupport.DoJSON(t, client, http.MethodPost, srv.URL+providersPath,
		`{"slug":"anon","name":"Anon","category":"video"}`)
	if code != http.StatusUnauthorized {
		t.Fatalf("anonymous create: status = %d, want 401", code)
	}
	code, _ = testsupport.DoJSON(t, client, http.MethodDelete, srv.URL+providersPath+"/"+slugs["claude"], "")
	if code != http.StatusUnauthorized {
		t.Fatalf("anonymous delete: status = %d, want 401", code)
	}
}

func TestProviderValidationErrors(t *testing.T) {
	db := testsupport.NewDB(t)
	srv := testsupport.NewServer(t, db)
	client := srv.Client()
	tok, _ := testsupport.RegisterAndLogin(t, client, srv.URL, "prov-valid")

	cases := []struct {
		name string
		body string
		want int
	}{
		{"missing slug", `{"name":"X","category":"video"}`, http.StatusUnprocessableEntity},
		{"missing name", `{"slug":"x1","category":"video"}`, http.StatusUnprocessableEntity},
		{"missing category", `{"slug":"x2","name":"X"}`, http.StatusUnprocessableEntity},
		{"bad metadata", `{"slug":"x3","name":"X","category":"video","metadata":{"broken"}`, http.StatusBadRequest},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			code, body := testsupport.DoAuthJSON(t, client, http.MethodPost, srv.URL+providersPath, tc.body, tok)
			if code != tc.want {
				t.Fatalf("status = %d, want %d: %v", code, tc.want, body)
			}
		})
	}

	code, _ := testsupport.DoAuthJSON(t, client, http.MethodDelete,
		srv.URL+providersPath+"/00000000-0000-0000-0000-000000000000", "", tok)
	if code != http.StatusNotFound {
		t.Fatalf("delete unknown status = %d, want 404", code)
	}
}

// seedProvider creates a provider through the API and returns its id.
func seedProvider(t *testing.T, client *http.Client, base, slug, tok string) string {
	t.Helper()
	code, body := testsupport.DoAuthJSON(t, client, http.MethodPost, base+providersPath,
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
