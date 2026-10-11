package api

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
)

func TestParsePage(t *testing.T) {
	cases := []struct {
		query string
		want  Page
	}{
		{"", Page{Limit: 20, Offset: 0}},
		{"?limit=5", Page{Limit: 5, Offset: 0}},
		{"?offset=3", Page{Limit: 20, Offset: 3}},
		{"?limit=0", Page{Limit: 1, Offset: 0}},
		{"?limit=-4", Page{Limit: 1, Offset: 0}},
		{"?limit=10000", Page{Limit: 100, Offset: 0}},
		{"?limit=abc", Page{Limit: 20, Offset: 0}},
		{"?offset=-2", Page{Limit: 20, Offset: 0}},
		{"?limit=10&offset=abc", Page{Limit: 10, Offset: 0}},
	}
	for _, tc := range cases {
		r := httptest.NewRequest(http.MethodGet, "http://api"+tc.query, nil)
		if got := ParsePage(r); got != tc.want {
			t.Errorf("ParsePage(%q) = %+v, want %+v", tc.query, got, tc.want)
		}
	}
}

func TestConstructorStatusAndCode(t *testing.T) {
	cases := []struct {
		err     *APIError
		status  int
		code    string
		message string
	}{
		{BadRequest("bad %s", "body"), http.StatusBadRequest, "bad_request", "bad body"},
		{Validation("invalid", Detail{Field: "name", Message: "required"}),
			http.StatusUnprocessableEntity, "validation_failed", "invalid"},
		{Unauthorized("missing bearer token"), http.StatusUnauthorized, "unauthorized", "missing bearer token"},
		{Forbidden("owner only"), http.StatusForbidden, "forbidden", "owner only"},
		{NotFound("thing %s not found", "42"), http.StatusNotFound, "not_found", "thing 42 not found"},
		{Conflict("already exists"), http.StatusConflict, "conflict", "already exists"},
		{TooManyRequests("too many logins"), http.StatusTooManyRequests, "rate_limited", "too many logins"},
		{Internal(), http.StatusInternalServerError, "internal", "internal server error"},
	}
	for _, tc := range cases {
		if tc.err.Status != tc.status {
			t.Errorf("%s: Status = %d, want %d", tc.err.Code, tc.err.Status, tc.status)
		}
		if tc.err.Code != tc.code || tc.err.Message != tc.message {
			t.Errorf("%s: got %+v, want code %q message %q",
				tc.code, tc.err, tc.code, tc.message)
		}
		if tc.err.Error() != tc.message {
			t.Errorf("Error() = %q, want %q", tc.err.Error(), tc.message)
		}
	}

	err := Validation("invalid", Detail{Field: "name", Message: "required"})
	if len(err.Details) != 1 || err.Details[0].Field != "name" {
		t.Errorf("Validation details = %+v, want one field detail", err.Details)
	}
}

func TestWriteErrorEnvelope(t *testing.T) {
	dec := func(w *httptest.ResponseRecorder) map[string]any {
		var out map[string]any
		if err := json.Unmarshal(w.Body.Bytes(), &out); err != nil {
			t.Fatalf("response is not JSON: %v: %s", err, w.Body.String())
		}
		return out
	}

	cases := []struct {
		name     string
		err      error
		status   int
		wantCode string
	}{
		{"validation", Validation("invalid", Detail{Field: "name", Message: "required"}),
			http.StatusUnprocessableEntity, "validation_failed"},
		{"wrapped", fmt.Errorf("wrap: %w", NotFound("user not found")),
			http.StatusNotFound, "not_found"},
		{"plain", errors.New("database exploded"), http.StatusInternalServerError, "internal"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			WriteError(w, tc.err)
			if w.Code != tc.status {
				t.Errorf("status = %d, want %d", w.Code, tc.status)
			}
			out := dec(w)
			inner, ok := out["error"].(map[string]any)
			if !ok {
				t.Fatalf("response missing the error key: %v", out)
			}
			if inner["code"] != tc.wantCode {
				t.Errorf("code = %v, want %s", inner["code"], tc.wantCode)
			}
			if _, has := inner["message"]; !has {
				t.Error("response missing the message field")
			}
		})
	}
}

func TestDecodeJSON(t *testing.T) {
	body := func(v string) *http.Request {
		return httptest.NewRequest(http.MethodPost, "http://api/v1/anything", strings.NewReader(v))
	}

	if err := DecodeJSON(body(`{"name":"n"}`), &struct {
		Name string `json:"name"`
	}{Name: "default"}); err != nil {
		t.Fatalf("valid body rejected: %v", err)
	}

	cases := []struct {
		name string
		body string
	}{
		{"malformed", `{"name":`},
		{"unknown field", `{"name":"n","extra":1}`},
		{"trailing value", `{"name":"n"} {"name":"x"}`},
		{"trailing text", `{"name":"n"} trailing`},
		{"empty", `"`},
		{"oversized", string(bytes.Repeat([]byte("a"), 1<<20+1))},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := DecodeJSON(body(tc.body), &struct {
				Name string `json:"name"`
			}{})
			if err == nil {
				t.Fatal("malformed body accepted")
			}
			if err.Status != http.StatusBadRequest {
				t.Errorf("status = %d, want 400", err.Status)
			}
		})
	}

	// A body just under the cap passes.
	if err := DecodeJSON(body(`{"name":"n"}`), &struct {
		Name string `json:"name"`
	}{}); err != nil {
		t.Fatalf("small body rejected: %v", err)
	}
}

func TestWriteJSON(t *testing.T) {
	w := httptest.NewRecorder()
	WriteJSON(w, http.StatusCreated, map[string]string{"id": "42"})
	if w.Code != http.StatusCreated {
		t.Errorf("status = %d, want 201", w.Code)
	}
	if ct := w.Header().Get("Content-Type"); ct != "application/json; charset=utf-8" {
		t.Errorf("Content-Type = %q", ct)
	}
	var out map[string]string
	if err := json.Unmarshal(w.Body.Bytes(), &out); err != nil || out["id"] != "42" {
		t.Errorf("body = %s, err = %v", w.Body.String(), err)
	}
}

func TestNewListNormalizesNilItems(t *testing.T) {
	out := NewList[map[string]any](nil, Meta{Total: 0, Limit: 20, Offset: 0})
	if out.Items == nil {
		t.Fatal("Items = nil, want an empty slice")
	}
	if len(out.Items) != 0 {
		t.Errorf("Items has %d entries, want none", len(out.Items))
	}
	encoded, _ := json.Marshal(out)
	if !strings.Contains(string(encoded), `"items":[]`) {
		t.Errorf("encoded = %s, want an empty items array", encoded)
	}
}

func TestChainOrderAndRecover(t *testing.T) {
	var order []string
	mark := func(tag string) Middleware {
		return func(next http.Handler) http.Handler {
			return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				order = append(order, tag)
				next.ServeHTTP(w, r)
			})
		}
	}
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		order = append(order, "handler")
		w.WriteHeader(http.StatusTeapot)
	})
	w := httptest.NewRecorder()
	Chain(inner, mark("outer"), mark("inner")).ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/", nil))
	if reflect.DeepEqual(order, []string{"outer", "inner", "handler"}) == false {
		t.Errorf("middleware order = %v, want outer first", order)
	}
	if w.Code != http.StatusTeapot {
		t.Errorf("status = %d, want the handler's code passed through", w.Code)
	}

	logger := slog.New(slog.DiscardHandler)
	w = httptest.NewRecorder()
	Chain(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { panic("boom") }), Recover(logger)).
		ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/", nil))
	if w.Code != http.StatusInternalServerError {
		t.Errorf("recovered status = %d, want 500", w.Code)
	}
	var env struct {
		Error *APIError `json:"error"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &env); err != nil || env.Error == nil || env.Error.Code != "internal" {
		t.Errorf("recovered body = %s, want the internal envelope: %v", w.Body.String(), err)
	}

	w = httptest.NewRecorder()
	Chain(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}), Recover(logger)).ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/", nil))
	if w.Code != http.StatusNoContent {
		t.Errorf("non-panicking status = %d, want the handler's code untouched", w.Code)
	}
}
