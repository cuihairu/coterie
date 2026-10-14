// run_test.go covers the scheduler's production entry point. apps/server
// starts it with `go sched.Run(ctx)`, so Run is the automation code that
// actually runs unattended; RunOnce had tests, Run had none.

package automation_test

import (
	"context"
	"log/slog"
	"net/http"
	"slices"
	"strings"
	"testing"
	"time"

	"gorm.io/gorm"

	"github.com/cuihairu/coterie/internal/automation"
	"github.com/cuihairu/coterie/internal/billing"
	"github.com/cuihairu/coterie/internal/notification"
	"github.com/cuihairu/coterie/internal/testsupport"
)

// waitFor polls until cond holds or the deadline passes. The ticker
// drives the roll on its own schedule, so the tests wait for it instead
// of asserting immediately.
func waitFor(t *testing.T, what string, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for %s", what)
}

// schedulerWithNotifier builds the same scheduler apps/server builds:
// a billing service and a notifier over the shared database.
func schedulerWithNotifier(t *testing.T, db *gorm.DB, interval time.Duration) *automation.Scheduler {
	t.Helper()
	notifier := notification.NewServiceWithChannels(db, slog.Default())
	return automation.NewScheduler(db, billing.NewService(db, notifier), notifier,
		slog.Default(), interval)
}

// TestRunRollsOnItsOwnTicks drives the loop the way production does —
// no RunOnce call from the test, only the ticker — then cancels and
// requires the goroutine to return so shutdown cannot leak.
func TestRunRollsOnItsOwnTicks(t *testing.T) {
	db := testsupport.NewDB(t)
	srv := testsupport.NewServer(t, db)
	client, base := srv.Client(), srv.URL
	sched := schedulerWithNotifier(t, db, 20*time.Millisecond)

	tok, _, subID, _, _ := testsupport.SeedCircle(t, client, base, "auto-run", "10.00", 4, 0, 1)
	enableAutoBilling(t, client, base, tok, subID)
	openPeriod(t, client, base, tok, subID, "2026-09-01", "2026-09-30")

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		sched.Run(ctx)
		close(done)
	}()

	// The ticker alone must open the October period.
	waitFor(t, "the ticker to roll the frontier period", func() bool {
		return slices.Contains(periodStarts(t, client, base, tok, subID), "2026-10-01")
	})

	cancel()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("Run did not return after its context was cancelled")
	}
}

// TestRunReturnsOnAlreadyCancelledContext covers the shutdown race: main
// may cancel the context before or just as the goroutine starts, so Run
// must notice and exit rather than block on its ticker.
func TestRunReturnsOnAlreadyCancelledContext(t *testing.T) {
	db := testsupport.NewDB(t)
	sched := schedulerWithNotifier(t, db, time.Hour)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	done := make(chan struct{})
	go func() {
		sched.Run(ctx)
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("Run blocked on an already-cancelled context")
	}
}

// TestRunOnceReportsSettledPreviousPeriod pins the other rollover
// wording: with no unsettled share left in the previous period the owner
// is told it is fully settled, not that shares are outstanding.
func TestRunOnceReportsSettledPreviousPeriod(t *testing.T) {
	db := testsupport.NewDB(t)
	srv := testsupport.NewServer(t, db)
	client, base := srv.Client(), srv.URL
	sched := schedulerWithNotifier(t, db, time.Hour)

	tok, _, subID, _, _ := testsupport.SeedCircle(t, client, base, "auto-settled", "10.00", 4, 0, 1)
	enableAutoBilling(t, client, base, tok, subID)
	sept := openPeriod(t, client, base, tok, subID, "2026-09-01", "2026-09-30")

	code, body := testsupport.DoAuthJSON(t, client, http.MethodPost,
		base+"/api/v1/billing-periods/"+sept+"/contributions/generate", `{"mode":"equal"}`, tok)
	if code != http.StatusCreated {
		t.Fatalf("generate shares: status = %d: %v", code, body)
	}

	// Settle every share, so the rollover finds a fully paid period.
	code, body = testsupport.DoAuthJSON(t, client, http.MethodGet,
		base+"/api/v1/billing-periods/"+sept+"/contributions", "", tok)
	if code != http.StatusOK {
		t.Fatalf("list contributions: status = %d: %v", code, body)
	}
	items, _ := body["items"].([]any)
	if len(items) != 2 {
		t.Fatalf("contributions = %d, want one per member", len(items))
	}
	for _, raw := range items {
		id, _ := raw.(map[string]any)["id"].(string)
		code, body = testsupport.DoAuthJSON(t, client, http.MethodPatch,
			base+"/api/v1/contributions/"+id, `{"status":"paid"}`, tok)
		if code != http.StatusOK {
			t.Fatalf("settle %s: status = %d: %v", id, code, body)
		}
	}

	if n, err := sched.RunOnce(context.Background()); err != nil || n != 1 {
		t.Fatalf("run once: rolled = %d, err = %v", n, err)
	}

	code, body = testsupport.DoAuthJSON(t, client, http.MethodGet,
		base+"/api/v1/notifications", "", tok)
	if code != http.StatusOK {
		t.Fatalf("notifications: status = %d: %v", code, body)
	}
	rows, _ := body["items"].([]any)
	rollover := ""
	for _, raw := range rows {
		row, _ := raw.(map[string]any)
		if row["type"] == "subscription_renewal" {
			rollover, _ = row["body"].(string)
		}
	}
	if rollover == "" {
		t.Fatalf("owner was not told about the rollover: %v", body["items"])
	}
	if !strings.Contains(rollover, "fully settled") {
		t.Errorf("rollover notice = %q, want the fully settled wording", rollover)
	}
	if strings.Contains(rollover, "unsettled share") {
		t.Errorf("rollover notice = %q, want no outstanding shares reported", rollover)
	}
}
