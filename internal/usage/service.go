package usage

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/cuihairu/coterie/internal/database"
	"github.com/cuihairu/coterie/internal/provider"
	"github.com/cuihairu/coterie/internal/seat"
	"github.com/cuihairu/coterie/internal/subscription"
	"github.com/cuihairu/coterie/internal/user"
	"github.com/cuihairu/coterie/pkg/api"
)

// amountPattern is the exact range numeric(14,4) accepts. Negative
// values are corrections to the ledger, not errors.
var amountPattern = regexp.MustCompile(`^-?\d{1,10}(\.\d{1,4})?$`)

// Service records metered usage against subscriptions (design D9).
// Writes are owner-only like the rest of subscription management.
type Service struct {
	store   *Store
	plugins *provider.Registry
}

// NewService builds a Service without provider plugins.
func NewService(db *gorm.DB) *Service {
	return NewServiceWithPlugins(db, nil)
}

// NewServiceWithPlugins builds a Service that consults the provider
// plugin face (§5.1) on usage writes.
func NewServiceWithPlugins(db *gorm.DB, plugins *provider.Registry) *Service {
	return &Service{store: NewStore(db), plugins: plugins}
}

// Create appends a usage record and, when the record lands on a quota
// seat, recomputes that seat's metadata.used from the ledger. The whole
// write — attribution, insert, projection — is one transaction.
func (s *Service) Create(ctx context.Context, actor *user.User, subscriptionID string, req CreateRecordRequest) (*UsageRecord, error) {
	sub, err := s.loadSubscriptionForOwner(ctx, actor, subscriptionID)
	if err != nil {
		return nil, err
	}

	req.MemberID = strings.TrimSpace(req.MemberID)
	req.Unit = strings.TrimSpace(req.Unit)
	if req.MemberID == "" {
		return nil, api.Validation("invalid usage record",
			api.Detail{Field: "member_id", Message: "is required"})
	}
	if req.Unit == "" || len(req.Unit) > 32 {
		return nil, api.Validation("invalid usage record",
			api.Detail{Field: "unit", Message: "is required (1-32 characters, e.g. credits or GB)"})
	}
	if !amountPattern.MatchString(req.Amount) {
		return nil, api.Validation("invalid usage record",
			api.Detail{Field: "amount", Message: `must be a decimal string with at most 4 fraction digits (e.g. "12.5"; negatives are corrections)`})
	}
	req.Amount = canonicalAmount(req.Amount)
	var metadata database.JSONB
	if len(req.Metadata) > 0 {
		var probe map[string]any
		if err := json.Unmarshal(req.Metadata, &probe); err != nil || probe == nil {
			return nil, api.Validation("invalid usage record",
				api.Detail{Field: "metadata", Message: "must be a JSON object"})
		}
		metadata = database.JSONB(req.Metadata)
	} else {
		metadata = database.JSONB("{}")
	}
	var recordedAt time.Time
	if req.RecordedAt == "" {
		recordedAt = time.Now().UTC()
	} else {
		recordedAt, err = time.Parse(time.RFC3339, req.RecordedAt)
		if err != nil {
			return nil, api.Validation("invalid usage record",
				api.Detail{Field: "recorded_at", Message: `must be an RFC 3339 timestamp (e.g. "2026-10-08T09:30:00Z")`})
		}
		recordedAt = recordedAt.UTC()
	}

	tx := s.store.db.WithContext(ctx).Begin()
	if tx.Error != nil {
		return nil, tx.Error
	}
	defer func() {
		_ = tx.Rollback().Error
	}()
	tstore := NewStore(tx)

	active, err := tstore.ActiveMember(ctx, sub.ID, req.MemberID)
	if err != nil {
		return nil, err
	}
	if !active {
		return nil, api.Validation("invalid usage record",
			api.Detail{Field: "member_id", Message: "must be an active member of this subscription's coterie"})
	}

	var attributed *seat.Seat
	if req.SeatID != "" {
		st, err := tstore.Seat(ctx, req.SeatID)
		if err != nil {
			return nil, err
		}
		if st == nil || st.SubscriptionID != sub.ID || st.MemberID == nil || *st.MemberID != req.MemberID {
			return nil, api.Validation("invalid usage record",
				api.Detail{Field: "seat_id", Message: "must be a seat of this subscription occupied by the member"})
		}
		attributed = st
	} else {
		quotaSeats, err := tstore.QuotaSeatsOfMember(ctx, sub.ID, req.MemberID)
		if err != nil {
			return nil, err
		}
		switch len(quotaSeats) {
		case 0: // no quota seat — the record stays seatless
		case 1:
			attributed = &quotaSeats[0]
		default:
			return nil, api.Validation("invalid usage record",
				api.Detail{Field: "seat_id", Message: "the member holds multiple quota seats; name the target one"})
		}
	}

	now := time.Now().UTC()
	var seatID *string
	if attributed != nil {
		id := attributed.ID
		seatID = &id
	}
	if err := s.validateUsageWithPlugin(ctx, tstore, sub.ID, req, seatID); err != nil {
		return nil, err
	}
	if err := s.enforceUsageLimit(ctx, tstore, sub, req.MemberID, req.Unit, req.Amount, recordedAt); err != nil {
		return nil, err
	}
	record := &UsageRecord{
		ID:             uuid.NewString(),
		SubscriptionID: sub.ID,
		MemberID:       req.MemberID,
		SeatID:         seatID,
		Amount:         req.Amount,
		Unit:           req.Unit,
		Metadata:       metadata,
		RecordedAt:     recordedAt,
		CreatedAt:      now,
	}
	if err := tstore.Create(ctx, record); err != nil {
		return nil, err
	}
	if attributed != nil {
		if err := projectQuotaUsed(ctx, tstore, attributed.ID); err != nil {
			return nil, err
		}
	}
	if err := tx.Commit().Error; err != nil {
		return nil, err
	}
	return record, nil
}

// enforceUsageLimit applies the sharing policy's usage_limit (D21):
// per member, per billing period, per unit, the post-write ledger sum
// must stay within per_period. Judging the new sum rather than the
// increment lets negative corrections through naturally. A record the
// window doesn't cover, or a unit the cap doesn't name, is not
// constrained. All comparisons run on 1e4-scaled integers.
func (s *Service) enforceUsageLimit(ctx context.Context, store *Store, sub *subscription.Subscription, memberID, unit, amount string, recordedAt time.Time) error {
	var policy struct {
		UsageLimit *struct {
			Unit      string `json:"unit"`
			PerPeriod string `json:"per_period"`
		} `json:"usage_limit"`
	}
	if err := json.Unmarshal(sub.SharingPolicy, &policy); err != nil || policy.UsageLimit == nil {
		return nil
	}
	lim := policy.UsageLimit
	if lim.Unit != unit {
		return nil
	}
	limit, ok := scaled(lim.PerPeriod)
	if !ok {
		return nil // policy structure is validated at write time; never block on a malformed value
	}
	window, err := store.PeriodCovering(ctx, sub.ID, recordedAt)
	if err != nil {
		return err
	}
	if window == nil {
		return nil
	}
	sumStr, err := store.SumMemberUsage(ctx, sub.ID, memberID, unit, window)
	if err != nil {
		return err
	}
	sum, _ := scaled(sumStr)
	amount4, _ := scaled(amount)
	if sum+amount4 > limit {
		return api.Conflict(
			"usage limit exceeded: %s total for %s would pass %s %s in the covered billing period",
			decimal4(sum+amount4), memberID, lim.PerPeriod, unit)
	}
	return nil
}

// scaled parses a decimal string into a 1e4-scaled integer. ok is
// false for anything but a plain decimal — the ledger and policy
// values are validated upstream, so a malformed value never blocks a
// write through this path.
func scaled(v string) (int64, bool) {
	neg := false
	s := strings.TrimSpace(v)
	if strings.HasPrefix(s, "-") {
		neg, s = true, s[1:]
	}
	intPart, fracPart := s, ""
	if dot := strings.IndexByte(s, '.'); dot >= 0 {
		intPart, fracPart = s[:dot], s[dot+1:]
	}
	if intPart == "" {
		return 0, false
	}
	frac := fracPart
	if len(frac) > 4 {
		return 0, false
	}
	for len(frac) < 4 {
		frac += "0"
	}
	i, err := strconv.ParseInt(intPart, 10, 64)
	if err != nil || i < 0 {
		return 0, false
	}
	f, err := strconv.ParseInt(frac, 10, 64)
	if err != nil {
		return 0, false
	}
	out := i*10_000 + f
	if neg {
		out = -out
	}
	return out, true
}

// decimal4 renders a 1e4-scaled integer back to a decimal string.
func decimal4(v int64) string {
	sign := ""
	if v < 0 {
		sign, v = "-", -v
	}
	return fmt.Sprintf("%s%d.%04d", sign, v/10_000, v%10_000)
}

// validateUsageWithPlugin consults the provider plugin's metering
// facet, if one is registered for the subscription's provider. Plugin
// api errors pass through; anything else becomes a 422.
func (s *Service) validateUsageWithPlugin(ctx context.Context, st *Store, subscriptionID string, req CreateRecordRequest, seatID *string) error {
	slug, err := st.ProviderSlugBySubscription(ctx, subscriptionID)
	if err != nil {
		return err
	}
	v, ok := s.plugins.For(slug).(provider.UsageValidator)
	if !ok {
		return nil
	}
	sid := ""
	if seatID != nil {
		sid = *seatID
	}
	if err := v.ValidateUsage(ctx, provider.UsageInput{
		UserID: req.MemberID,
		Amount: req.Amount,
		Unit:   req.Unit,
		SeatID: sid,
	}); err != nil {
		var apiErr *api.APIError
		if errors.As(err, &apiErr) {
			return apiErr
		}
		return api.Validation("invalid usage record",
			api.Detail{Field: "amount", Message: "rejected by provider: " + err.Error()})
	}
	return nil
}

// Get returns the record, mapping absence to 404.
func (s *Service) Get(ctx context.Context, id string) (*UsageRecord, error) {
	r, err := s.store.RecordByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if r == nil {
		return nil, api.NotFound("usage record %s not found", id)
	}
	return r, nil
}

// List returns the subscription's records under the filters.
func (s *Service) List(ctx context.Context, subscriptionID string, f ListFilters, page api.Page) ([]UsageRecord, int64, error) {
	sub, err := s.store.SubscriptionByID(ctx, subscriptionID)
	if err != nil {
		return nil, 0, err
	}
	if sub == nil {
		return nil, 0, api.NotFound("subscription %s not found", subscriptionID)
	}
	return s.store.ListBySubscription(ctx, subscriptionID, f, page)
}

// ParseFilters validates the query parameters of the list endpoint;
// from and to are "YYYY-MM-DD" and to is inclusive.
func ParseFilters(memberID, unit, from, to string) (ListFilters, error) {
	f := ListFilters{MemberID: strings.TrimSpace(memberID), Unit: strings.TrimSpace(unit)}
	if from != "" {
		d, err := database.ParseDate(from)
		if err != nil {
			return f, api.Validation("invalid filter",
				api.Detail{Field: "from", Message: `must be a "YYYY-MM-DD" date`})
		}
		f.From = &d
	}
	if to != "" {
		d, err := database.ParseDate(to)
		if err != nil {
			return f, api.Validation("invalid filter",
				api.Detail{Field: "to", Message: `must be a "YYYY-MM-DD" date`})
		}
		f.To = &d
	}
	return f, nil
}

// projectQuotaUsed rewrites metadata.used as the ledger sum of the
// seat's attributed records, preserving every other metadata key. The
// caller must run inside the write transaction.
func projectQuotaUsed(ctx context.Context, store *Store, seatID string) error {
	st, err := store.SeatForUpdate(ctx, seatID)
	if err != nil {
		return err
	}
	if st == nil {
		return nil
	}
	var md map[string]any
	if err := json.Unmarshal(st.Metadata, &md); err != nil {
		return err
	}
	if _, ok := md["quota"]; !ok {
		return nil // not a quota seat — nothing to project
	}
	sum, err := store.SumBySeat(ctx, seatID)
	if err != nil {
		return err
	}
	md["used"] = json.Number(trimNumeric(sum))
	raw, err := json.Marshal(md)
	if err != nil {
		return err
	}
	return store.SetSeatMetadata(ctx, seatID, raw)
}

// canonicalAmount renders a conforming decimal at the column's scale
// (4), so create responses and later reads show the same text.
func canonicalAmount(v string) string {
	r, ok := new(big.Rat).SetString(v)
	if !ok {
		return v
	}
	scaled := new(big.Int).Quo(new(big.Int).Mul(r.Num(), big.NewInt(10000)), r.Denom())
	neg := scaled.Sign() < 0
	if neg {
		scaled.Neg(scaled)
	}
	digits := scaled.String()
	for len(digits) <= 4 {
		digits = "0" + digits
	}
	out := digits[:len(digits)-4] + "." + digits[len(digits)-4:]
	if neg {
		out = "-" + out
	}
	return out
}

// trimNumeric drops the scale digits numeric::text always emits
// ("301.5000" → "301.5", "300.0000" → "300") so used stays a plain JSON
// number.
func trimNumeric(s string) string {
	if i := strings.IndexByte(s, '.'); i >= 0 {
		s = strings.TrimRight(s[:i+1]+strings.TrimRight(s[i+1:], "0"), ".")
	}
	return s
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
		return nil, api.Forbidden("only the subscription owner may record usage")
	}
	return sub, nil
}
