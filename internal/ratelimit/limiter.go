// Package ratelimit implements NFR-7 Phase 1 (design D19): fixed
// window counters for the public, unauthenticated write endpoints —
// the realistic abuse surface, bulk registration and password
// brute-forcing. Single-instance and in-memory by design (§7.4: no
// Redis in the first phase); a process restart resets the counters,
// and multiple instances count independently.
package ratelimit

import (
	"sync"
	"time"
)

// Endpoint classes covered in Phase 1.
const (
	ClassRegister = "register"
	ClassLogin    = "login"
)

// Phase 1 defaults; config and app wiring both start from these.
const (
	DefaultRegisterPerMin = 5
	DefaultLoginPerMin    = 10
)

// Limiter counts requests per (class, key) inside fixed windows. The
// zero value is not usable; build with New.
type Limiter struct {
	mu      sync.Mutex
	limits  map[string]int
	window  time.Duration
	now     func() time.Time
	counts  map[string]*tally
	sweepAt time.Time
}

// tally is the current window's count for one (class, key) pair.
type tally struct {
	start time.Time
	n     int
}

// New builds a Limiter over the given per-class per-window limits. A
// limit of 0 or less disables the class entirely.
func New(limits map[string]int, window time.Duration) *Limiter {
	if window <= 0 {
		window = time.Minute
	}
	copied := make(map[string]int, len(limits))
	for k, v := range limits {
		copied[k] = v
	}
	return &Limiter{
		limits: copied,
		window: window,
		now:    time.Now,
		counts: make(map[string]*tally),
	}
}

// Allow reports whether one more request of the class for the key fits
// the window; when it does not, it returns how long until the window
// rolls over and the budget resets.
func (l *Limiter) Allow(class, key string) (time.Duration, bool) {
	limit := l.limits[class]
	if limit <= 0 {
		return 0, true
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	now := l.now()
	l.sweep(now)

	id := class + "\x00" + key
	t := l.counts[id]
	if t == nil || now.Sub(t.start) >= l.window {
		t = &tally{start: now}
		l.counts[id] = t
	}
	if t.n >= limit {
		return l.window - now.Sub(t.start), false
	}
	t.n++
	return 0, true
}

// sweep drops expired tallies once per window so memory stays bounded
// even under spoofed-address traffic. Called with mu held.
func (l *Limiter) sweep(now time.Time) {
	if now.Before(l.sweepAt) {
		return
	}
	l.sweepAt = now.Add(l.window)
	for id, t := range l.counts {
		if now.Sub(t.start) >= l.window {
			delete(l.counts, id)
		}
	}
}
