package app_test

// routes_consistency_test.go guards docs/api.md against the mux
// registrations. The patrol audits did this check by hand five times
// in one session and still drifted twice on derived numbers, so the
// parity check runs as a test instead: every route the server code
// registers must have a doc row, and every doc row must be
// registered, with the method matching. Routes are keyed by
// "METHOD path" — one path legitimately carries several methods.

import (
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strings"
	"testing"
)

var (
	codeRe = regexp.MustCompile(`"(GET|POST|PUT|PATCH|DELETE) (/api/v1/[^"]+)"`)
	// Root routes: a path whose first segment is not "api". Matches
	// /healthz but never /api/v1/... (its first segment starts with a).
	rootRe   = regexp.MustCompile(`"(GET|POST|PUT|PATCH|DELETE) (/[^"/][^"]*)"`)
	docRe    = regexp.MustCompile(`\|\s*(GET|POST|PUT|PATCH|DELETE)\s*\|\s*` + "`" + `([^` + "`" + `]+)` + "`" + `\s*\|`)
	pholdRe  = regexp.MustCompile(`\{[^/]+\}`)
	goSuffix = ".go"
)

// normalize collapses path placeholders so {subID} in code and {id}
// in the doc compare equal.
func normalize(path string) string {
	return pholdRe.ReplaceAllString(path, "{id}")
}

// repoRoot locates the checkout root from this file's own path, so
// the test works from any working directory (including CI).
func repoRoot(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("cannot resolve source path")
	}
	// internal/app/routes_consistency_test.go -> repo root is three up.
	return filepath.Dir(filepath.Dir(filepath.Dir(file)))
}

// codeRoutes walks internal/ for mux registrations in non-test Go
// files: "METHOD /api/v1/..." literals plus root routes like
// /healthz.
func codeRoutes(t *testing.T) map[string]bool {
	t.Helper()
	routes := map[string]bool{}
	err := filepath.Walk(filepath.Join(repoRoot(t), "internal"), func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() || !strings.HasSuffix(path, goSuffix) || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		body, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		for _, m := range codeRe.FindAllStringSubmatch(string(body), -1) {
			routes[m[1]+" "+normalize(m[2])] = true
		}
		for _, m := range rootRe.FindAllStringSubmatch(string(body), -1) {
			if strings.HasPrefix(m[2], "/api/") {
				continue
			}
			routes[m[1]+" "+normalize(m[2])] = true
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk internal: %v", err)
	}
	return routes
}

// docRoutes reads the docs/api.md endpoint tables. Rows are relative
// to /api/v1; a row is kept unprefixed only when its raw path is
// itself registered at the root (e.g. /healthz).
func docRoutes(t *testing.T, code map[string]bool) map[string]bool {
	t.Helper()
	body, err := os.ReadFile(filepath.Join(repoRoot(t), "docs", "api.md"))
	if err != nil {
		t.Fatalf("read docs/api.md: %v", err)
	}
	routes := map[string]bool{}
	for _, m := range docRe.FindAllStringSubmatch(string(body), -1) {
		path := "/api/v1" + m[2]
		if !code[m[1]+" "+normalize(path)] && code[m[1]+" "+normalize(m[2])] {
			path = m[2]
		}
		routes[m[1]+" "+normalize(path)] = true
	}
	return routes
}

func TestAPIDocMatchesRegisteredRoutes(t *testing.T) {
	code := codeRoutes(t)
	doc := docRoutes(t, code)

	var missingInDoc, missingInCode []string
	for r := range code {
		if !doc[r] {
			missingInDoc = append(missingInDoc, r)
		}
	}
	for r := range doc {
		if !code[r] {
			missingInCode = append(missingInCode, r)
		}
	}
	sort.Strings(missingInDoc)
	sort.Strings(missingInCode)
	for _, r := range missingInDoc {
		t.Errorf("code registers %s but docs/api.md has no matching row", r)
	}
	for _, r := range missingInCode {
		t.Errorf("docs/api.md documents %s but no code registers it", r)
	}
	if len(code) != len(doc) {
		t.Errorf("route count mismatch: code has %d, docs/api.md has %d", len(code), len(doc))
	}
}
