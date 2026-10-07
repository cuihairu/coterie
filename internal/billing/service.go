package billing

import (
	"context"
	"fmt"
	"log/slog"
	"math/big"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/cuihairu/coterie/internal/database"
	"github.com/cuihairu/coterie/internal/notification"
	"github.com/cuihairu/coterie/internal/subscription"
	"github.com/cuihairu/coterie/internal/user"
	"github.com/cuihairu/coterie/pkg/api"
)

// amountPattern is the exact range numeric(12,2) accepts, so invalid
// amounts are 422s instead of database errors.
var amountPattern = regexp.MustCompile(`^\d{1,10}(\.\d{1,2})?$`)

// Service implements Phase 1 billing: periods are composed and closed
// by the subscription owner, contributions record who owes what, and
// settlement is manual (design §4.2 — payment stays an adapter).
type Service struct {
	store    *Store
	notifier *notification.Service
	log      *slog.Logger
}

// NewService builds a Service. notifier may be nil in tests that do
// not exercise notifications.
func NewService(db *gorm.DB, notifier *notification.Service) *Service {
	return &Service{store: NewStore(db), notifier: notifier, log: slog.Default()}
}

// CreatePeriod opens a chargeable window. Explicit dates cover monthly,
// yearly, and custom cycles; (subscription_id, start_date) is unique.
func (s *Service) CreatePeriod(ctx context.Context, actor *user.User, subscriptionID string, req CreatePeriodRequest) (*BillingPeriod, error) {
	sub, err := s.loadSubscriptionForOwner(ctx, actor, subscriptionID)
	if err != nil {
		return nil, err
	}
	var details []api.Detail
	start, startErr := database.ParseDate(req.StartDate)
	if startErr != nil {
		details = append(details, api.Detail{Field: "start_date", Message: `must be a "YYYY-MM-DD" date (required)`})
	}
	end, endErr := database.ParseDate(req.EndDate)
	if endErr != nil {
		details = append(details, api.Detail{Field: "end_date", Message: `must be a "YYYY-MM-DD" date (required)`})
	}
	if startErr == nil && endErr == nil && !end.After(start.Time) {
		details = append(details, api.Detail{Field: "end_date", Message: "must be after start_date"})
	}
	if len(details) > 0 {
		return nil, api.Validation("invalid billing period", details...)
	}

	p := &BillingPeriod{
		ID:             uuid.NewString(),
		SubscriptionID: sub.ID,
		StartDate:      start,
		EndDate:        end,
		Status:         PeriodOpen,
	}
	if err := s.store.CreatePeriod(ctx, p); err != nil {
		if database.IsUniqueViolation(err) {
			return nil, api.Conflict("a billing period starting %s already exists for this subscription", req.StartDate)
		}
		return nil, err
	}
	return p, nil
}

// GetPeriod returns the period, mapping absence to 404.
func (s *Service) GetPeriod(ctx context.Context, id string) (*BillingPeriod, error) {
	p, err := s.store.PeriodByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if p == nil {
		return nil, api.NotFound("billing period %s not found", id)
	}
	return p, nil
}

// ListPeriods returns the subscription's periods.
func (s *Service) ListPeriods(ctx context.Context, subscriptionID string, page api.Page) ([]BillingPeriod, int64, error) {
	sub, err := s.store.SubscriptionByID(ctx, subscriptionID)
	if err != nil {
		return nil, 0, err
	}
	if sub == nil {
		return nil, 0, api.NotFound("subscription %s not found", subscriptionID)
	}
	return s.store.ListPeriods(ctx, subscriptionID, page)
}

// Close freezes a period: no further generation or amount edits. The
// period must be open; closing is one-way.
func (s *Service) Close(ctx context.Context, actor *user.User, periodID string) (*BillingPeriod, error) {
	p, err := s.GetPeriod(ctx, periodID)
	if err != nil {
		return nil, err
	}
	if _, err := s.loadSubscriptionForOwner(ctx, actor, p.SubscriptionID); err != nil {
		return nil, err
	}
	changed, err := s.store.ClosePeriod(ctx, p.ID)
	if err != nil {
		return nil, err
	}
	if !changed {
		return nil, api.Conflict("billing period %s is already closed", p.ID)
	}
	return s.GetPeriod(ctx, p.ID)
}

// Generate composes the period's contributions in one go. The split
// distributes the subscription price exactly — rounding remainders
// (single cents) go to the earliest-joined members.
func (s *Service) Generate(ctx context.Context, actor *user.User, periodID string, req GenerateRequest) ([]Contribution, error) {
	p, err := s.GetPeriod(ctx, periodID)
	if err != nil {
		return nil, err
	}
	sub, err := s.loadSubscriptionForOwner(ctx, actor, p.SubscriptionID)
	if err != nil {
		return nil, err
	}
	if p.Status != PeriodOpen {
		return nil, api.Conflict("billing period %s is closed", p.ID)
	}
	if n, err := s.store.CountContributions(ctx, p.ID); err != nil {
		return nil, err
	} else if n > 0 {
		return nil, api.Conflict("contributions for period %s already exist; adjust them individually", p.ID)
	}

	mode := req.Mode
	if mode == "" {
		mode = SplitEqual
	}
	if mode != SplitEqual && mode != SplitPerSeat && mode != SplitFixed && mode != SplitUsage && mode != SplitProrated {
		return nil, api.Validation("invalid split mode",
			api.Detail{Field: "mode", Message: "must be equal, per_seat, fixed, usage, or prorated"})
	}

	totalCents, ok := parseCents(sub.Price)
	if !ok {
		return nil, api.Internal()
	}

	members, err := s.store.ActiveMembers(ctx, sub.ID)
	if err != nil {
		return nil, err
	}

	var shares map[string]int64 // member_id → cents
	switch mode {
	case SplitEqual:
		if len(members) == 0 {
			return nil, api.Validation("nothing to split",
				api.Detail{Field: "mode", Message: "the coterie has no active members"})
		}
		shares = splitEqual(totalCents, members)

	case SplitPerSeat:
		holds, err := s.store.SeatHolds(ctx, sub.ID)
		if err != nil {
			return nil, err
		}
		shares, err = splitPerSeat(totalCents, members, holds)
		if err != nil {
			return nil, err
		}

	case SplitFixed:
		shares, err = s.splitFixed(ctx, sub.ID, req.Items)
		if err != nil {
			return nil, err
		}

	case SplitUsage:
		shares, err = s.splitUsage(ctx, sub, p, members)
		if err != nil {
			return nil, err
		}

	case SplitProrated:
		shares = splitProrated(totalCents, p, members)
		if len(shares) == 0 {
			return nil, api.Validation("nothing to split",
				api.Detail{Field: "mode", Message: "no active member was in the coterie during this period"})
		}
	}

	now := time.Now().UTC()
	contributions := make([]Contribution, 0, len(shares))
	for _, m := range members { // deterministic order for stable output
		cents, ok := shares[m.ID]
		if !ok {
			continue
		}
		contributions = append(contributions, Contribution{
			ID:              uuid.NewString(),
			BillingPeriodID: p.ID,
			MemberID:        m.ID,
			Amount:          formatCents(cents),
			Currency:        sub.Currency,
			Status:          ContributionPending,
			CreatedAt:       now,
			UpdatedAt:       now,
		})
	}
	if len(contributions) == 0 {
		return nil, api.Validation("nothing to split",
			api.Detail{Field: "mode", Message: "no member would receive a share"})
	}
	if err := s.store.CreateContributions(ctx, contributions); err != nil {
		if database.IsUniqueViolation(err) {
			return nil, api.Conflict("a member appears twice or already has a contribution in this period")
		}
		return nil, err
	}
	if s.notifier != nil {
		userByMember := make(map[string]string, len(members))
		for _, m := range members {
			userByMember[m.ID] = m.UserID
		}
		window := p.StartDate.Time.Format("2006-01-02") + " – " + p.EndDate.Time.Format("2006-01-02")
		for _, c := range contributions {
			if uid, ok := userByMember[c.MemberID]; ok {
				s.notifier.Notify(ctx, uid, notification.TypePaymentDue,
					"Payment due ("+window+")",
					"Your share for this period is "+c.Amount+" "+c.Currency+".",
					"billing_period", p.ID)
			}
		}
	}
	return contributions, nil
}

// UpdateContribution patches the amount (while the period is open and
// the contribution is pending) and drives settlement status. Moving in
// or out of paid maintains paid_at.
func (s *Service) UpdateContribution(ctx context.Context, actor *user.User, id string, req UpdateContributionRequest) (*Contribution, error) {
	c, err := s.store.ContributionByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if c == nil {
		return nil, api.NotFound("contribution %s not found", id)
	}
	p, err := s.store.PeriodByID(ctx, c.BillingPeriodID)
	if err != nil {
		return nil, err
	}
	if _, err := s.loadSubscriptionForOwner(ctx, actor, p.SubscriptionID); err != nil {
		return nil, err
	}

	if req.Amount != nil {
		if !amountPattern.MatchString(*req.Amount) {
			return nil, api.Validation("invalid contribution",
				api.Detail{Field: "amount", Message: `must be a non-negative decimal string with at most 2 fraction digits (e.g. "12.99")`})
		}
		if p.Status != PeriodOpen || c.Status != ContributionPending {
			return nil, api.Conflict("amount is frozen once the period is closed or the contribution is settled")
		}
		c.Amount = *req.Amount
	}
	if req.Status != nil {
		switch *req.Status {
		case ContributionPending, ContributionPaid, ContributionWaived, ContributionCancelled:
		default:
			return nil, api.Validation("invalid contribution",
				api.Detail{Field: "status", Message: "must be pending, paid, waived, or cancelled"})
		}
		c.Status = *req.Status
		if c.Status == ContributionPaid {
			if c.PaidAt == nil {
				now := time.Now().UTC()
				c.PaidAt = &now
			}
		} else {
			c.PaidAt = nil
		}
	}
	if err := s.store.UpdateContribution(ctx, c); err != nil {
		return nil, err
	}
	return c, nil
}

// ListContributions returns the period's contributions.
func (s *Service) ListContributions(ctx context.Context, periodID string, page api.Page) ([]Contribution, int64, error) {
	if _, err := s.GetPeriod(ctx, periodID); err != nil {
		return nil, 0, err
	}
	return s.store.ListContributions(ctx, periodID, page)
}

// splitEqual divides cents evenly; the first len%k members absorb one
// extra cent so the shares always sum to the total.
func splitEqual(total int64, members []MemberRef) map[string]int64 {
	k := int64(len(members))
	base := total / k
	rem := total % k
	shares := make(map[string]int64, k)
	for i, m := range members {
		shares[m.ID] = base
		if int64(i) < rem {
			shares[m.ID]++
		}
	}
	return shares
}

// splitPerSeat divides the total across occupied seats, then sums each
// member's seats. Members without seats pay nothing; with no occupied
// seats at all there is nothing to split.
func splitPerSeat(total int64, members []MemberRef, holds map[string]int) (map[string]int64, error) {
	occupied := 0
	for _, n := range holds {
		occupied += n
	}
	if occupied == 0 {
		return nil, api.Validation("nothing to split",
			api.Detail{Field: "mode", Message: "no seat is occupied; per-seat split needs assignees"})
	}
	base := total / int64(occupied)
	rem := total % int64(occupied)
	seatIndex := int64(0)
	shares := make(map[string]int64, len(members))
	for _, m := range members { // earliest joined take the remainder cents
		n := holds[m.ID]
		if n == 0 {
			continue
		}
		var sum int64
		for i := 0; i < n; i++ {
			sum += base
			if seatIndex < rem {
				sum++
			}
			seatIndex++
		}
		shares[m.ID] = sum
	}
	return shares, nil
}

// splitUsage shares the price proportionally to each active member's
// recorded usage in the period window [start, end). The ledger must
// speak a single unit and no member may be net-negative; cents go out
// by largest remainder (ties → earliest joined) so the shares sum
// exactly to the subscription price (design D9).
func (s *Service) splitUsage(ctx context.Context, sub *subscription.Subscription, p *BillingPeriod, members []MemberRef) (map[string]int64, error) {
	totals, units, err := s.store.UsageTotals(ctx, sub.ID, p.StartDate.Time, p.EndDate.Time)
	if err != nil {
		return nil, err
	}
	if units > 1 {
		return nil, api.Validation("unsupported usage split",
			api.Detail{Field: "mode", Message: "usage records in this period mix units; correct the ledger or split per unit"})
	}

	totalCents, ok := parseCents(sub.Price)
	if !ok {
		return nil, api.Internal()
	}

	sums := make(map[string]*big.Rat, len(members))
	totalUsage := new(big.Rat)
	for _, m := range members { // members are joined_at ASC — the tie order
		raw, ok := totals[m.ID]
		if !ok {
			continue
		}
		r, ok := parseUsageDecimal(raw)
		if !ok {
			return nil, api.Internal()
		}
		if r.Sign() < 0 {
			return nil, api.Validation("unsupported usage split",
				api.Detail{Field: "mode", Message: "member has negative net usage; correct the ledger with offsetting records first"})
		}
		sums[m.ID] = r
		totalUsage.Add(totalUsage, r)
	}
	if totalUsage.Sign() <= 0 {
		return nil, api.Validation("nothing to split",
			api.Detail{Field: "mode", Message: "no positive usage is recorded in this period's window"})
	}

	// Members order is joined_at ASC — the remainder tie order.
	weights := make(map[string]*big.Rat, len(sums))
	for id, r := range sums {
		weights[id] = r
	}
	return splitWeighted(totalCents, members, weights), nil
}

// splitProrated weights each active member by the days they were in the
// coterie during the period: from max(joined_at, period start) through
// the period end, inclusive. Members who joined after the period ended
// get no share; the split conserves the total exactly.
func splitProrated(totalCents int64, p *BillingPeriod, members []MemberRef) map[string]int64 {
	start := p.StartDate.Time.Truncate(24 * time.Hour)
	end := p.EndDate.Time.Truncate(24 * time.Hour)
	weights := make(map[string]*big.Rat, len(members))
	for _, m := range members {
		from := m.JoinedAt.UTC().Truncate(24 * time.Hour)
		if from.Before(start) {
			from = start
		}
		if from.After(end) {
			continue // joined after this period — not chargeable here
		}
		days := int64(end.Sub(from)/(24*time.Hour)) + 1
		weights[m.ID] = big.NewRat(days, 1)
	}
	if len(weights) == 0 {
		return nil
	}
	return splitWeighted(totalCents, members, weights)
}

// splitWeighted divides totalCents proportionally to the weights
// (largest-remainder method): every member's exact share is floored and
// the leftover cents go to the largest fractional remainders, ties
// broken by the members slice order (joined_at ASC). The shares sum to
// totalCents exactly.
func splitWeighted(totalCents int64, members []MemberRef, weights map[string]*big.Rat) map[string]int64 {
	var totalWeight *big.Rat
	for _, m := range members {
		if w, ok := weights[m.ID]; ok {
			if totalWeight == nil {
				totalWeight = new(big.Rat)
			}
			totalWeight.Add(totalWeight, w)
		}
	}
	if totalWeight == nil || totalWeight.Sign() <= 0 {
		return nil
	}

	type remainder struct {
		memberID string
		rest     *big.Rat
	}
	price := new(big.Rat).SetInt64(totalCents)
	shares := make(map[string]int64, len(weights))
	assigned := int64(0)
	var rems []remainder
	for _, m := range members {
		w, ok := weights[m.ID]
		if !ok {
			continue
		}
		exact := new(big.Rat).Mul(price, new(big.Rat).Quo(w, totalWeight))
		floor := new(big.Int).Quo(exact.Num(), exact.Denom()) // exact ≥ 0
		cents := floor.Int64()
		assigned += cents
		rems = append(rems, remainder{m.ID, new(big.Rat).Sub(exact, new(big.Rat).SetInt(floor))})
		shares[m.ID] = cents
	}
	remCents := totalCents - assigned // 0..len(rems)-1
	sort.SliceStable(rems, func(i, j int) bool { return rems[i].rest.Cmp(rems[j].rest) > 0 })
	for i := 0; i < int(remCents) && i < len(rems); i++ {
		shares[rems[i].memberID]++
	}
	return shares
}

// splitFixed validates the explicit items: every member must be active
// in this subscription's coterie and appear at most once.
func (s *Service) splitFixed(ctx context.Context, subscriptionID string, items []FixedShare) (map[string]int64, error) {
	if len(items) == 0 {
		return nil, api.Validation("invalid fixed split",
			api.Detail{Field: "items", Message: "fixed mode requires at least one member amount"})
	}
	shares := make(map[string]int64, len(items))
	var details []api.Detail
	for _, item := range items {
		if item.MemberID == "" {
			details = append(details, api.Detail{Field: "items", Message: "member_id is required"})
			continue
		}
		if !amountPattern.MatchString(item.Amount) {
			details = append(details, api.Detail{Field: "items", Message: "amount must be a non-negative decimal with at most 2 fraction digits"})
			continue
		}
		if _, dup := shares[item.MemberID]; dup {
			details = append(details, api.Detail{Field: "items", Message: "member appears more than once"})
			continue
		}
		ok, err := s.store.ActiveMember(ctx, subscriptionID, item.MemberID)
		if err != nil {
			return nil, err
		}
		if !ok {
			details = append(details, api.Detail{Field: "items", Message: "member must be active in this coterie"})
			continue
		}
		cents, _ := parseCents(item.Amount)
		shares[item.MemberID] = cents
	}
	if len(details) > 0 {
		return nil, api.Validation("invalid fixed split", details...)
	}
	return shares, nil
}

// loadSubscriptionForOwner returns the subscription when the actor owns
// it — 404 for unknown, 403 otherwise.
func (s *Service) loadSubscriptionForOwner(ctx context.Context, actor *user.User, subscriptionID string) (*subscription.Subscription, error) {
	sub, err := s.store.SubscriptionByID(ctx, subscriptionID)
	if err != nil {
		return nil, err
	}
	if sub == nil {
		return nil, api.NotFound("subscription %s not found", subscriptionID)
	}
	if sub.OwnerUserID != actor.ID {
		return nil, api.Forbidden("only the subscription owner may manage billing")
	}
	return sub, nil
}

// parseCents converts a decimal string to cents. ok is false when the
// value does not match amountPattern.
func parseCents(v string) (int64, bool) {
	if !amountPattern.MatchString(v) {
		return 0, false
	}
	intPart, fracPart := v, "0"
	if dot := strings.IndexByte(v, '.'); dot >= 0 {
		intPart, fracPart = v[:dot], v[dot+1:]
	}
	if len(fracPart) == 1 {
		fracPart += "0"
	}
	i, _ := strconv.ParseInt(intPart, 10, 64)
	f, _ := strconv.ParseInt(fracPart, 10, 64)
	return i*100 + f, true
}

// usageAmountPattern accepts the text forms numeric sums render as.
var usageAmountPattern = regexp.MustCompile(`^-?\d+(\.\d+)?$`)

// parseUsageDecimal converts a ledger sum into an exact rational.
func parseUsageDecimal(v string) (*big.Rat, bool) {
	if !usageAmountPattern.MatchString(v) {
		return nil, false
	}
	return new(big.Rat).SetString(v)
}

// formatCents renders cents as a decimal string with exactly two
// fraction digits, matching the numeric(12,2) column.
func formatCents(c int64) string {
	return fmt.Sprintf("%d.%02d", c/100, c%100)
}
