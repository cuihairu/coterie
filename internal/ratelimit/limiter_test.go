package ratelimit

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// newFake builds a limiter with a controllable clock.
func newFake(limits map[string]int, window time.Duration) (*Limiter, *time.Time) {
	base := time.Unix(1_800_000_000, 0)
	l := New(limits, window)
	l.now = func() time.Time { return base }
	return l, &base
}

func TestFixedWindowCounts(t *testing.T) {
	l, clock := newFake(map[string]int{ClassLogin: 2}, time.Minute)

	for i := 0; i < 2; i++ {
		retry, ok := l.Allow(ClassLogin, "10.0.0.1")
		if !ok || retry != 0 {
			t.Fatalf("allow %d: ok=%v retry=%v", i, ok, retry)
		}
	}
	retry, ok := l.Allow(ClassLogin, "10.0.0.1")
	if ok {
		t.Fatal("third request must be denied")
	}
	if retry <= 0 || retry > time.Minute {
		t.Fatalf("retry window = %v, want (0, 1m]", retry)
	}

	// A different address has its own budget.
	if _, ok := l.Allow(ClassLogin, "10.0.0.2"); !ok {
		t.Fatal("second address must not share the first's budget")
	}

	// The window rolls: budget resets and denial clears.
	*clock = clock.Add(time.Minute)
	retry, ok = l.Allow(ClassLogin, "10.0.0.1")
	if !ok || retry != 0 {
		t.Fatalf("post-rollover allow: ok=%v retry=%v", ok, retry)
	}
}

func TestClassesIndependent(t *testing.T) {
	l, _ := newFake(map[string]int{ClassRegister: 1, ClassLogin: 1}, time.Minute)
	if _, ok := l.Allow(ClassRegister, "ip"); !ok {
		t.Fatal("register allow")
	}
	if _, ok := l.Allow(ClassLogin, "ip"); !ok {
		t.Fatal("login must not share register's budget")
	}
	if _, ok := l.Allow(ClassRegister, "ip"); ok {
		t.Fatal("register budget must be exhausted")
	}
}

func TestDisabledClassAlwaysAllows(t *testing.T) {
	l, _ := newFake(map[string]int{ClassRegister: 0}, time.Minute)
	for i := 0; i < 100; i++ {
		if _, ok := l.Allow(ClassRegister, "ip"); !ok {
			t.Fatalf("disabled class denied at request %d", i)
		}
	}
	// Unknown classes are disabled too.
	if _, ok := l.Allow("unknown", "ip"); !ok {
		t.Fatal("unknown class denied")
	}
}

func TestSweepDropsExpired(t *testing.T) {
	l, clock := newFake(map[string]int{ClassLogin: 1}, time.Minute)
	if _, ok := l.Allow(ClassLogin, "ip"); !ok {
		t.Fatal("allow")
	}
	*clock = clock.Add(2 * time.Minute) // one window past the sweep mark
	if _, ok := l.Allow(ClassLogin, "other"); !ok {
		t.Fatal("allow other")
	}
	if len(l.counts) != 1 {
		t.Fatalf("expired tallies kept: %d entries", len(l.counts))
	}
}

func TestGuard429Shape(t *testing.T) {
	l, _ := newFake(map[string]int{ClassRegister: 1, ClassLogin: 1}, time.Minute)
	h := l.Guard()(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	post := func(path string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodPost, path, nil)
		req.RemoteAddr = "203.0.113.9:44321"
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		return rec
	}

	if rec := post("/api/v1/auth/register"); rec.Code != http.StatusOK {
		t.Fatalf("first register: status = %d", rec.Code)
	}
	rec := post("/api/v1/auth/register")
	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("second register: status = %d, want 429", rec.Code)
	}
	if ra := rec.Header().Get("Retry-After"); ra == "" || ra == "0" {
		t.Fatalf("Retry-After = %q", ra)
	}
	if got := rec.Body.String(); !contains(got, `"rate_limited"`) {
		t.Fatalf("body = %s", got)
	}

	// A GET to the same path is outside the guarded surface.
	req := httptest.NewRequest(http.MethodGet, "/api/v1/auth/register", nil)
	req.RemoteAddr = "203.0.113.9:44321"
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET register: status = %d, want pass-through", rec.Code)
	}

	// Other paths stay untouched even when both budgets are spent.
	if _, ok := l.Allow(ClassLogin, "203.0.113.9"); !ok {
		t.Fatal("spend login budget")
	}
	req = httptest.NewRequest(http.MethodPost, "/api/v1/healthz", nil)
	req.RemoteAddr = "203.0.113.9:1"
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("unguarded path: status = %d", rec.Code)
	}
}

// contains reports whether substr appears in s (kept tiny to avoid
// importing strings for one check).
func contains(s, substr string) bool {
	for i := 0; i+len(substr) <= len(s); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
