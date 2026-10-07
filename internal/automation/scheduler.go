// Package automation implements scheduler-driven billing rollover
// (design D13): subscriptions opted into auto_billing get their next
// billing period opened and split equally once the frontier period
// ends. The first period always stays manual — price and start date are
// owner decisions — and the scheduler acts on behalf of the owner
// through the same billing service the API uses, so every invariant
// (unique period start, one contribution set per period, amount
// conservation) holds unchanged.
package automation

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"gorm.io/gorm"

	"github.com/cuihairu/coterie/internal/billing"
	"github.com/cuihairu/coterie/internal/notification"
	"github.com/cuihairu/coterie/internal/user"
	"github.com/cuihairu/coterie/pkg/api"
)

// Scheduler rolls billing periods forward on a fixed interval.
type Scheduler struct {
	store    *Store
	billing  *billing.Service
	users    *user.Store
	notifier *notification.Service
	log      *slog.Logger
	interval time.Duration
}

// NewScheduler builds a Scheduler. notifier may be nil; interval is
// the pass cadence.
func NewScheduler(db *gorm.DB, b *billing.Service, notifier *notification.Service, log *slog.Logger, interval time.Duration) *Scheduler {
	return &Scheduler{
		store:    NewStore(db),
		billing:  b,
		users:    user.NewStore(db),
		notifier: notifier,
		log:      log,
		interval: interval,
	}
}

// Run ticks until the context is cancelled. Each pass is best-effort:
// a failing subscription is logged and the rest still roll.
func (s *Scheduler) Run(ctx context.Context) {
	t := time.NewTicker(s.interval)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			n, err := s.RunOnce(ctx)
			if err != nil {
				s.log.Warn("billing rollover pass failed", "error", err)
				continue
			}
			if n > 0 {
				s.log.Info("billing rollover", "periods_rolled", n)
			}
		}
	}
}

// RunOnce performs one rollover pass and returns how many periods were
// opened. Exported for tests and one-shot invocations.
func (s *Scheduler) RunOnce(ctx context.Context) (int, error) {
	today := time.Now().UTC().Truncate(24 * time.Hour)
	due, err := s.store.Due(ctx, today)
	if err != nil {
		return 0, err
	}
	rolled := 0
	for _, d := range due {
		ok, err := s.rollSubscription(ctx, d)
		if err != nil {
			s.log.Warn("billing rollover skipped a subscription",
				"subscription", d.ID, "error", err)
			continue
		}
		if ok {
			rolled++
		}
	}
	return rolled, nil
}

// rollSubscription opens the next period for one due subscription and
// generates its equal shares, acting as the owner. It reports whether
// a period was rolled; a custom cycle is skipped (the scheduler cannot
// infer its length), as is a period that already exists.
func (s *Scheduler) rollSubscription(ctx context.Context, d DueSubscription) (bool, error) {
	if d.BillingCycle != "monthly" && d.BillingCycle != "yearly" {
		s.log.Warn("auto_billing on a custom cycle rolls nothing; manage periods manually",
			"subscription", d.ID)
		return false, nil
	}
	owner, err := s.users.Get(ctx, d.OwnerUserID)
	if err != nil {
		return false, err
	}
	if owner == nil {
		return false, fmt.Errorf("owner %s no longer exists", d.OwnerUserID)
	}

	start := d.EndDate.Time.AddDate(0, 0, 1)
	end := start.AddDate(0, 1, 0)
	if d.BillingCycle == "yearly" {
		end = start.AddDate(1, 0, 0)
	}
	req := billing.CreatePeriodRequest{
		StartDate: start.Format("2006-01-02"),
		EndDate:   end.AddDate(0, 0, -1).Format("2006-01-02"),
	}
	p, err := s.billing.CreatePeriod(ctx, owner, d.ID, req)
	if err != nil {
		var apiErr *api.APIError
		if errors.As(err, &apiErr) && apiErr.Status == http.StatusConflict {
			return false, nil // already rolled — idempotent re-run
		}
		return false, err
	}
	if _, err := s.billing.Generate(ctx, owner, p.ID, billing.GenerateRequest{Mode: billing.SplitEqual}); err != nil {
		return false, fmt.Errorf("generate shares for %s: %w", p.ID, err)
	}

	if s.notifier != nil {
		pending, err := s.store.PendingContributions(ctx, d.PeriodID)
		if err != nil {
			return false, err
		}
		window := req.StartDate + " – " + req.EndDate
		body := fmt.Sprintf("Your %s billing period (%s) was opened and split equally. Your previous period still has %d unsettled share(s).",
			d.BillingCycle, window, pending)
		if pending == 0 {
			body = fmt.Sprintf("Your %s billing period (%s) was opened and split equally. The previous period is fully settled.",
				d.BillingCycle, window)
		}
		s.notifier.Notify(ctx, owner.ID, notification.TypeSubscriptionRenewal,
			"Billing period rolled over", body, "subscription", d.ID)
	}
	return true, nil
}
