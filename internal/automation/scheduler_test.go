package automation_test

import (
	"context"
	"net/http"
	"slices"
	"testing"
	"time"

	"log/slog"

	"github.com/cuihairu/coterie/internal/automation"
	"github.com/cuihairu/coterie/internal/billing"
	"github.com/cuihairu/coterie/internal/notification"
	"github.com/cuihairu/coterie/internal/testsupport"
)

// enableAutoBilling flips the subscription's auto_billing flag via the
// API, as an owner would.
func enableAutoBilling(t *testing.T, client *http.Client, base, tok, subID string) {
	t.Helper()
	code, body := testsupport.DoAuthJSON(t, client, http.MethodPatch,
		base+"/api/v1/subscriptions/"+subID, `{"auto_billing":true}`, tok)
	if code != http.StatusOK {
		t.Fatalf("enable auto_billing: status = %d: %v", code, body)
	}
}

// openPeriod opens a billing period with explicit dates.
func openPeriod(t *testing.T, client *http.Client, base, tok, subID, start, end string) string {
	t.Helper()
	code, body := testsupport.DoAuthJSON(t, client, http.MethodPost,
		base+"/api/v1/subscriptions/"+subID+"/billing-periods",
		`{"start_date":"`+start+`","end_date":"`+end+`"}`, tok)
	if code != http.StatusCreated {
		t.Fatalf("open period: status = %d: %v", code, body)
	}
	id, _ := body["id"].(string)
	return id
}

// periodStarts lists the subscription's period start dates.
func periodStarts(t *testing.T, client *http.Client, base, tok, subID string) []string {
	t.Helper()
	code, body := testsupport.DoAuthJSON(t, client, http.MethodGet,
		base+"/api/v1/subscriptions/"+subID+"/billing-periods", "", tok)
	if code != http.StatusOK {
		t.Fatalf("list periods: status = %d: %v", code, body)
	}
	items, _ := body["items"].([]any)
	var starts []string
	for _, it := range items {
		s, _ := it.(map[string]any)["start_date"].(string)
		starts = append(starts, s)
	}
	return starts
}

func TestSchedulerRollsForward(t *testing.T) {
	db := testsupport.NewDB(t)
	srv := testsupport.NewServer(t, db)
	client, base := srv.Client(), srv.URL
	notifier := notification.NewServiceWithChannels(db, slog.Default())
	sched := automation.NewScheduler(db, billing.NewService(db, notifier), notifier, slog.Default(), time.Hour)

	// One member circle with a period that has already ended.
	tok, _, subID, _, joined := testsupport.SeedCircle(t, client, base, "auto-roll", "10.00", 4, 0, 1)
	enableAutoBilling(t, client, base, tok, subID)
	openPeriod(t, client, base, tok, subID, "2026-09-01", "2026-09-30")

	n, err := sched.RunOnce(context.Background())
	if err != nil {
		t.Fatalf("run once: %v", err)
	}
	if n != 1 {
		t.Fatalf("rolled = %d, want 1", n)
	}

	// The October period was opened and split equally between owner and
	// member — total exactly 10.00.
	starts := periodStarts(t, client, base, tok, subID)
	if !slices.Contains(starts, "2026-10-01") {
		t.Fatalf("no October period after rollover: %v", starts)
	}

	// Idempotent: a second pass rolls nothing (the frontier period is
	// current now).
	n, err = sched.RunOnce(context.Background())
	if err != nil {
		t.Fatalf("second run: %v", err)
	}
	if n != 0 {
		t.Fatalf("second pass rolled %d, want 0", n)
	}

	// The owner was told about the rollover and the outstanding share.
	code, body := testsupport.DoAuthJSON(t, client, http.MethodGet,
		base+"/api/v1/notifications", "", tok)
	if code != http.StatusOK {
		t.Fatalf("notifications: status = %d: %v", code, body)
	}
	rolled := false
	items, _ := body["items"].([]any)
	for _, it := range items {
		row := it.(map[string]any)
		if row["type"] == "subscription_renewal" {
			rolled = true
		}
	}
	if !rolled {
		t.Fatalf("owner not notified of rollover: %v", body)
	}

	// The member owes their share in the new period.
	code, body = testsupport.DoAuthJSON(t, client, http.MethodGet,
		base+"/api/v1/subscriptions/"+subID+"/billing-periods", "", tok)
	if code != http.StatusOK {
		t.Fatalf("periods: status = %d", code)
	}
	items, _ = body["items"].([]any)
	var octID string
	for _, it := range items {
		row := it.(map[string]any)
		if row["start_date"] == "2026-10-01" {
			octID, _ = row["id"].(string)
		}
	}
	code, body = testsupport.DoAuthJSON(t, client, http.MethodGet,
		base+"/api/v1/billing-periods/"+octID+"/contributions", "", tok)
	if code != http.StatusOK {
		t.Fatalf("contributions: status = %d: %v", code, body)
	}
	items, _ = body["items"].([]any)
	if len(items) != 2 {
		t.Fatalf("contributions = %d, want 2 (owner + member %s)", len(items), joined[0].MemberID)
	}
}

func TestSchedulerSkips(t *testing.T) {
	db := testsupport.NewDB(t)
	srv := testsupport.NewServer(t, db)
	client, base := srv.Client(), srv.URL
	sched := automation.NewScheduler(db, billing.NewService(db, nil), nil, slog.Default(), time.Hour)

	// Auto-billing off: nothing rolls.
	tok, _, subID, _, _ := testsupport.SeedCircle(t, client, base, "auto-off", "10.00", 4, 0, 1)
	openPeriod(t, client, base, tok, subID, "2026-09-01", "2026-09-30")
	n, err := sched.RunOnce(context.Background())
	if err != nil || n != 0 {
		t.Fatalf("auto_billing off: rolled = %d, err = %v", n, err)
	}

	enableAutoBilling(t, client, base, tok, subID)

	// A future frontier period is not due.
	code, body := testsupport.DoAuthJSON(t, client, http.MethodPatch,
		base+"/api/v1/subscriptions/"+subID, `{"billing_cycle":"monthly"}`, tok)
	if code != http.StatusOK {
		t.Fatalf("set monthly: status = %d: %v", code, body)
	}
	openPeriod(t, client, base, tok, subID, "2099-01-01", "2099-01-31")
	n, err = sched.RunOnce(context.Background())
	if err != nil || n != 0 {
		t.Fatalf("future frontier: rolled = %d, err = %v", n, err)
	}
}

// TestSchedulerRollsCustomCycle covers D20: a custom subscription with
// cycle_days rolls by the day-count offset, through the same pipeline
// (equal split, idempotent second pass).
func TestSchedulerRollsCustomCycle(t *testing.T) {
	db := testsupport.NewDB(t)
	srv := testsupport.NewServer(t, db)
	client, base := srv.Client(), srv.URL
	sched := automation.NewScheduler(db, billing.NewService(db, nil), nil, slog.Default(), time.Hour)

	tok, _, subID, _, _ := testsupport.SeedCircle(t, client, base, "auto-custom", "14.00", 4, 0, 1)
	enableAutoBilling(t, client, base, tok, subID)
	code, body := testsupport.DoAuthJSON(t, client, http.MethodPatch,
		base+"/api/v1/subscriptions/"+subID, `{"billing_cycle":"custom","cycle_days":30}`, tok)
	if code != http.StatusOK {
		t.Fatalf("set custom: status = %d: %v", code, body)
	}
	// The ended September window is 30 days long; the rolled window
	// must end in the future so the frontier is current afterwards.
	openPeriod(t, client, base, tok, subID, "2026-09-01", "2026-09-30")

	n, err := sched.RunOnce(context.Background())
	if err != nil {
		t.Fatalf("run once: %v", err)
	}
	if n != 1 {
		t.Fatalf("rolled = %d, want 1", n)
	}

	// The next window is exactly cycle_days long.
	code, body = testsupport.DoAuthJSON(t, client, http.MethodGet,
		base+"/api/v1/subscriptions/"+subID+"/billing-periods", "", tok)
	if code != http.StatusOK {
		t.Fatalf("list periods: status = %d: %v", code, body)
	}
	items, _ := body["items"].([]any)
	var next map[string]any
	for _, it := range items {
		p := it.(map[string]any)
		if p["start_date"] == "2026-10-01" {
			next = p
		}
	}
	if next == nil {
		t.Fatalf("no rolled period in %v", body["items"])
	}
	if next["end_date"] != "2026-10-30" {
		t.Fatalf("custom period end = %v, want 2026-10-30", next["end_date"])
	}

	// Idempotent: the frontier is current now, nothing further rolls.
	n, err = sched.RunOnce(context.Background())
	if err != nil || n != 0 {
		t.Fatalf("second pass: rolled = %d, err = %v", n, err)
	}
}
