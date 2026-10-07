package subscription

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"regexp"

	"gorm.io/gorm"

	"github.com/cuihairu/coterie/internal/database"
	"github.com/cuihairu/coterie/internal/provider"
	"github.com/cuihairu/coterie/pkg/api"

	"github.com/google/uuid"
)

// pricePattern allows up to 10 integer digits and 2 fraction digits —
// the exact range numeric(12,2) accepts, so validation failures are
// 422s instead of database errors. The database additionally enforces
// price >= 0.
var pricePattern = regexp.MustCompile(`^\d{1,10}(\.\d{1,2})?$`)

// currencyPattern matches ISO 4217 alpha-3 uppercase codes; the same
// check exists as a database constraint.
var currencyPattern = regexp.MustCompile(`^[A-Z]{3}$`)

// Service carries the subscription business rules on top of the store.
type Service struct {
	store   *Store
	plugins *provider.Registry
	log     *slog.Logger
}

// NewService builds a Service without provider plugins.
func NewService(db *gorm.DB) *Service {
	return NewServiceWithPlugins(db, nil)
}

// NewServiceWithPlugins builds a Service that consults the provider
// plugin registry when policies are set (Validation facet). A nil
// registry means no plugins — plain Generic behavior.
func NewServiceWithPlugins(db *gorm.DB, plugins *provider.Registry) *Service {
	return &Service{store: NewStore(db), plugins: plugins, log: slog.Default()}
}

// Create validates and inserts a subscription. Field-level rules fail
// with 422 before referenced resources are checked; database uniqueness
// or FK clashes surface as 409.
func (s *Service) Create(ctx context.Context, req CreateSubscriptionRequest) (*Subscription, error) {
	details := validateFields(req.BillingCycle, req.Price, req.Currency, req.MaxSeats, req.MaxMembers, req.SharingPolicy)

	start, startErr := database.ParseDate(req.StartDate)
	if startErr != nil {
		details = append(details, api.Detail{
			Field:   "start_date",
			Message: `must be a "YYYY-MM-DD" date (required)`,
		})
	}

	var renewal *database.Date
	if req.RenewalDate != nil && *req.RenewalDate != "" {
		d, err := database.ParseDate(*req.RenewalDate)
		if err != nil {
			details = append(details, api.Detail{
				Field:   "renewal_date",
				Message: `must be a "YYYY-MM-DD" date`,
			})
		} else {
			renewal = &d
		}
	}
	if renewal != nil && startErr == nil && renewal.Before(start.Time) {
		details = append(details, api.Detail{
			Field:   "renewal_date",
			Message: "must not be before start_date",
		})
	}

	if len(details) > 0 {
		return nil, api.Validation("invalid subscription", details...)
	}

	if ok, err := s.store.ProductExists(ctx, req.ProductID); err != nil {
		return nil, err
	} else if !ok {
		return nil, api.Validation("referenced resources missing",
			api.Detail{Field: "product_id", Message: "product not found"})
	}
	if ok, err := s.store.UserExists(ctx, req.OwnerUserID); err != nil {
		return nil, err
	} else if !ok {
		return nil, api.Validation("referenced resources missing",
			api.Detail{Field: "owner_user_id", Message: "user not found"})
	}
	if err := s.validatePolicyWithPlugin(ctx, req.ProductID, req.SharingPolicy); err != nil {
		return nil, err
	}

	sub := &Subscription{
		ID:            uuid.NewString(),
		ProductID:     req.ProductID,
		OwnerUserID:   req.OwnerUserID,
		BillingCycle:  req.BillingCycle,
		Price:         req.Price,
		Currency:      req.Currency,
		StartDate:     start,
		RenewalDate:   renewal,
		Status:        StatusActive,
		MaxSeats:      req.MaxSeats,
		MaxMembers:    req.MaxMembers,
		SharingPolicy: database.JSONB("{}"),
		AutoBilling:   req.AutoBilling,
	}
	if len(req.SharingPolicy) > 0 {
		sub.SharingPolicy = database.JSONB(req.SharingPolicy)
	}

	if err := s.store.Create(ctx, sub); err != nil {
		if database.IsFKViolation(err) {
			return nil, api.Conflict("referenced product or user no longer exists")
		}
		return nil, err
	}
	return sub, nil
}

// Get returns the subscription, mapping absence to 404.
func (s *Service) Get(ctx context.Context, id string) (*Subscription, error) {
	sub, err := s.store.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	if sub == nil {
		return nil, api.NotFound("subscription %s not found", id)
	}
	return sub, nil
}

// List returns a page of subscriptions, optionally filtered by owner
// and product.
func (s *Service) List(ctx context.Context, ownerUserID, productID string, page api.Page) ([]Subscription, int64, error) {
	return s.store.List(ctx, ownerUserID, productID, page)
}

// Update patches the subscription's mutable fields.
func (s *Service) Update(ctx context.Context, id string, req UpdateSubscriptionRequest) (*Subscription, error) {
	sub, err := s.store.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	if sub == nil {
		return nil, api.NotFound("subscription %s not found", id)
	}

	var details []api.Detail
	if req.BillingCycle != nil && !BillingCycles[*req.BillingCycle] {
		details = append(details, detailCycle())
	}
	if req.Price != nil && !pricePattern.MatchString(*req.Price) {
		details = append(details, detailPrice())
	}
	if req.Status != nil && !statusAllowed(*req.Status) {
		details = append(details, detailStatus())
	}
	if req.MaxSeats != nil && *req.MaxSeats <= 0 {
		details = append(details, detailSeats())
	}
	if req.MaxMembers != nil && *req.MaxMembers <= 0 {
		details = append(details, detailMembers())
	}
	if req.SharingPolicy != nil {
		if d := detailSharingPolicy(*req.SharingPolicy); d != nil {
			details = append(details, *d)
		}
	}
	var renewal *database.Date
	if req.RenewalDate != nil && *req.RenewalDate != "" {
		d, err := database.ParseDate(*req.RenewalDate)
		if err != nil {
			details = append(details, api.Detail{
				Field:   "renewal_date",
				Message: `must be a "YYYY-MM-DD" date`,
			})
		} else {
			renewal = &d
		}
	}
	if renewal != nil && renewal.Before(sub.StartDate.Time) {
		details = append(details, api.Detail{
			Field:   "renewal_date",
			Message: "must not be before start_date",
		})
	}
	if len(details) > 0 {
		return nil, api.Validation("invalid subscription", details...)
	}

	if req.BillingCycle != nil {
		sub.BillingCycle = *req.BillingCycle
	}
	if req.Price != nil {
		sub.Price = *req.Price
	}
	if req.Status != nil {
		sub.Status = *req.Status
	}
	if req.MaxSeats != nil {
		sub.MaxSeats = *req.MaxSeats
	}
	if req.MaxMembers != nil {
		sub.MaxMembers = req.MaxMembers
	}
	if req.AutoBilling != nil {
		sub.AutoBilling = *req.AutoBilling
	}
	if req.SharingPolicy != nil {
		if err := s.validatePolicyWithPlugin(ctx, sub.ProductID, *req.SharingPolicy); err != nil {
			return nil, err
		}
		sub.SharingPolicy = database.JSONB(*req.SharingPolicy)
	}
	if req.RenewalDate != nil {
		if *req.RenewalDate == "" {
			sub.RenewalDate = nil
		} else {
			sub.RenewalDate = renewal
		}
	}

	if err := s.store.Update(ctx, sub); err != nil {
		return nil, err
	}
	return sub, nil
}

// Delete removes the subscription; subscriptions still referenced by
// coteries or seats map to 409.
func (s *Service) Delete(ctx context.Context, id string) error {
	rows, err := s.store.Delete(ctx, id)
	if err != nil {
		if database.IsFKViolation(err) {
			return api.Conflict("subscription has coteries or seats and cannot be deleted")
		}
		return err
	}
	if rows == 0 {
		return api.NotFound("subscription %s not found", id)
	}
	return nil
}

func statusAllowed(status string) bool {
	switch status {
	case StatusActive, StatusPaused, StatusCancelled, StatusExpired:
		return true
	}
	return false
}

// validateFields covers the rules shared by Create; Update repeats the
// per-field checks inline for its optional fields.
func validateFields(cycle, price, currency string, maxSeats int, maxMembers *int, sharingPolicy json.RawMessage) []api.Detail {
	var details []api.Detail
	if !BillingCycles[cycle] {
		details = append(details, detailCycle())
	}
	if !pricePattern.MatchString(price) {
		details = append(details, detailPrice())
	}
	if !currencyPattern.MatchString(currency) {
		details = append(details, api.Detail{
			Field:   "currency",
			Message: "must be a 3-letter ISO 4217 code (uppercase)",
		})
	}
	if maxSeats <= 0 {
		details = append(details, detailSeats())
	}
	if maxMembers != nil && *maxMembers <= 0 {
		details = append(details, detailMembers())
	}
	if d := detailSharingPolicy(sharingPolicy); d != nil {
		details = append(details, *d)
	}
	return details
}

func detailCycle() api.Detail {
	return api.Detail{Field: "billing_cycle", Message: "must be one of monthly, yearly, custom"}
}

func detailPrice() api.Detail {
	return api.Detail{Field: "price", Message: "must be a non-negative decimal string with at most 2 fraction digits (e.g. \"12.99\")"}
}

func detailStatus() api.Detail {
	return api.Detail{Field: "status", Message: "must be one of active, paused, cancelled, expired"}
}

// validatePolicyWithPlugin hands the policy to the product provider's
// plugin, when one implements PolicyValidator (design §5.1). Plain
// errors from plugins become 422s.
func (s *Service) validatePolicyWithPlugin(ctx context.Context, productID string, policy json.RawMessage) error {
	if s.plugins == nil || len(policy) == 0 {
		return nil
	}
	slug, err := s.store.ProviderSlugByProduct(ctx, productID)
	if err != nil {
		return err
	}
	p := s.plugins.For(slug)
	v, ok := p.(provider.PolicyValidator)
	if !ok {
		return nil
	}
	if err := v.ValidatePolicy(ctx, policy); err != nil {
		var apiErr *api.APIError
		if errors.As(err, &apiErr) {
			return apiErr
		}
		return api.Validation("policy rejected by provider",
			api.Detail{Field: "sharing_policy", Message: err.Error()})
	}
	return nil
}

func detailSeats() api.Detail {
	return api.Detail{Field: "max_seats", Message: "must be greater than 0"}
}

func detailMembers() api.Detail {
	return api.Detail{Field: "max_members", Message: "must be greater than 0"}
}

// detailSharingPolicy checks that the policy is a JSON object and that
// mode, when present, is a known sharing mode.
func detailSharingPolicy(raw json.RawMessage) *api.Detail {
	if len(raw) == 0 {
		return nil
	}
	if !json.Valid(raw) {
		return &api.Detail{Field: "sharing_policy", Message: "must be valid JSON"}
	}
	var sp struct {
		Mode string `json:"mode"`
	}
	if err := json.Unmarshal(raw, &sp); err != nil {
		return &api.Detail{Field: "sharing_policy", Message: "must be a JSON object"}
	}
	if sp.Mode != "" && !SharingModes[sp.Mode] {
		return &api.Detail{
			Field:   "sharing_policy",
			Message: "unknown mode; want account, seat, family, quota, or resource",
		}
	}
	return nil
}
