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
		`{"slug":"netflix","name":"Netflix","category":"video","metadata":{"region":"US"}}`, tok)
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
	if code != http.StatusOK || body["slug"] != "netflix" {
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
	if body["slug"] != "netflix" {
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
		`{"slug":"spotify","name":"Spotify","category":"music"}`, tok); code != http.StatusCreated {
		t.Fatalf("seed status = %d: %v", code, body)
	}

	code, body := testsupport.DoAuthJSON(t, client, http.MethodPost, srv.URL+providersPath,
		`{"slug":"SPOTIFY","name":"Spotify Again","category":"music"}`, tok)
	if code != http.StatusConflict {
		t.Fatalf("duplicate slug status = %d, want 409: %v", code, body)
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
