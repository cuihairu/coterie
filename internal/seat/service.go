package seat

import (
	"context"
	"encoding/json"
	"log/slog"
	"strconv"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/cuihairu/coterie/internal/database"
	"github.com/cuihairu/coterie/internal/subscription"
	"github.com/cuihairu/coterie/internal/user"
	"github.com/cuihairu/coterie/pkg/api"
)

// maxProvisionBounds how many seats one call may create.
const maxProvision = 100

// Service enforces the seat rules: capacity comes from the subscription
// (D2), mutations belong to the subscription owner (D5), and an assignee
// must be an active member of the subscription's coterie (invariant 4).
type Service struct {
	store *Store
	log   *slog.Logger
}

// NewService builds a Service.
func NewService(db *gorm.DB) *Service {
	return &Service{store: NewStore(db), log: slog.Default()}
}

// Provision creates count seats for the subscription, named "Seat N"
// continuing from the existing ones. Total seats must stay within
// subscription.max_seats.
func (s *Service) Provision(ctx context.Context, actor *user.User, subscriptionID string, req ProvisionRequest) ([]Seat, error) {
	sub, err := s.loadSubscription(ctx, subscriptionID)
	if err != nil {
		return nil, err
	}
	if err := requireOwner(sub, actor); err != nil {
		return nil, err
	}
	if req.Count <= 0 {
		return nil, api.Validation("invalid seat provision",
			api.Detail{Field: "count", Message: "must be greater than 0"})
	}

	existing, err := s.store.CountBySubscription(ctx, sub.ID)
	if err != nil {
		return nil, err
	}
	if int(existing)+req.Count > sub.MaxSeats {
		return nil, api.Conflict(
			"provisioning %d seats would exceed subscription max_seats (%d in use of %d)",
			req.Count, existing, sub.MaxSeats)
	}

	now := time.Now().UTC()
	seats := make([]Seat, req.Count)
	for i := range seats {
		seats[i] = Seat{
			ID:             uuid.NewString(),
			SubscriptionID: sub.ID,
			Label:          labelFor(int(existing) + i + 1),
			Status:         StatusFree,
			Metadata:       database.JSONB("{}"),
			CreatedAt:      now,
			UpdatedAt:      now,
		}
	}
	if err := s.store.Create(ctx, seats); err != nil {
		if database.IsUniqueViolation(err) {
			return nil, api.Conflict("seat label already exists for this subscription")
		}
		return nil, err
	}
	return seats, nil
}

// Get returns the seat, mapping absence to 404.
func (s *Service) Get(ctx context.Context, id string) (*Seat, error) {
	seat, err := s.store.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	if seat == nil {
		return nil, api.NotFound("seat %s not found", id)
	}
	return seat, nil
}

// List returns the subscription's seats.
func (s *Service) List(ctx context.Context, subscriptionID string, page api.Page) ([]Seat, int64, error) {
	sub, err := s.loadSubscription(ctx, subscriptionID)
	if err != nil {
		return nil, 0, err
	}
	return s.store.ListBySubscription(ctx, sub.ID, page)
}

// Assign gives a free seat to an active member of the subscription's
// coterie. Occupying an occupied or disabled seat is a 409.
func (s *Service) Assign(ctx context.Context, actor *user.User, seatID string, req AssignRequest) (*Seat, error) {
	seat, err := s.Get(ctx, seatID)
	if err != nil {
		return nil, err
	}
	sub, err := s.loadSubscription(ctx, seat.SubscriptionID)
	if err != nil {
		return nil, err
	}
	if err := requireOwner(sub, actor); err != nil {
		return nil, err
	}
	if req.MemberID == "" {
		return nil, api.Validation("invalid assignment",
			api.Detail{Field: "member_id", Message: "is required"})
	}
	switch seat.Status {
	case StatusOccupied:
		return nil, api.Conflict("seat %s is already occupied; release it first", seat.ID)
	case StatusDisabled:
		return nil, api.Conflict("seat %s is disabled", seat.ID)
	}
	ok, err := s.store.ActiveMemberInCoterie(ctx, sub.ID, req.MemberID)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, api.Validation("referenced resources missing",
			api.Detail{Field: "member_id", Message: "must be an active member of this subscription's coterie"})
	}
	changed, err := s.store.Occupy(ctx, seat.ID, req.MemberID)
	if err != nil {
		return nil, err
	}
	if !changed {
		return nil, api.Conflict("seat %s is no longer free", seat.ID)
	}
	return s.Get(ctx, seat.ID)
}

// Release frees an occupied seat. Releasing a free seat is a no-op;
// disabled seats must be re-enabled first.
func (s *Service) Release(ctx context.Context, actor *user.User, seatID string) (*Seat, error) {
	seat, err := s.Get(ctx, seatID)
	if err != nil {
		return nil, err
	}
	sub, err := s.loadSubscription(ctx, seat.SubscriptionID)
	if err != nil {
		return nil, err
	}
	if err := requireOwner(sub, actor); err != nil {
		return nil, err
	}
	if seat.Status == StatusDisabled {
		return nil, api.Conflict("seat %s is disabled", seat.ID)
	}
	if _, err := s.store.Release(ctx, seat.ID); err != nil {
		return nil, err
	}
	return s.Get(ctx, seat.ID)
}

// Update patches label, metadata, and the free⇄disabled toggle. The
// occupied status is owned by assign/release, not by PATCH.
func (s *Service) Update(ctx context.Context, actor *user.User, seatID string, req UpdateSeatRequest) (*Seat, error) {
	seat, err := s.Get(ctx, seatID)
	if err != nil {
		return nil, err
	}
	sub, err := s.loadSubscription(ctx, seat.SubscriptionID)
	if err != nil {
		return nil, err
	}
	if err := requireOwner(sub, actor); err != nil {
		return nil, err
	}

	var details []api.Detail
	if req.Label != nil && *req.Label == "" {
		details = append(details, api.Detail{Field: "label", Message: "must not be empty"})
	}
	if req.Metadata != nil && !json.Valid(req.Metadata) {
		details = append(details, api.Detail{Field: "metadata", Message: "must be valid JSON"})
	}
	if req.Status != nil {
		switch *req.Status {
		case StatusFree, StatusDisabled:
		case StatusOccupied:
			details = append(details, api.Detail{Field: "status", Message: "use /assign to occupy a seat"})
		default:
			details = append(details, api.Detail{Field: "status", Message: "must be free or disabled"})
		}
	}
	if len(details) > 0 {
		return nil, api.Validation("invalid seat", details...)
	}
	if req.Status != nil && *req.Status == StatusDisabled && seat.Status == StatusOccupied {
		return nil, api.Conflict("seat %s is occupied; release it before disabling", seat.ID)
	}

	if req.Label != nil {
		seat.Label = *req.Label
	}
	if req.Metadata != nil {
		seat.Metadata = database.JSONB(req.Metadata)
	}
	if req.Status != nil {
		seat.Status = *req.Status
		if seat.Status == StatusFree {
			seat.MemberID = nil
		}
	}
	if err := s.store.Update(ctx, seat); err != nil {
		if database.IsUniqueViolation(err) {
			return nil, api.Conflict("seat label already exists for this subscription")
		}
		return nil, err
	}
	return seat, nil
}

// loadSubscription returns the subscription or 404.
func (s *Service) loadSubscription(ctx context.Context, id string) (*subscription.Subscription, error) {
	sub, err := s.store.SubscriptionByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if sub == nil {
		return nil, api.NotFound("subscription %s not found", id)
	}
	return sub, nil
}

// requireOwner enforces decision D5: the subscription owner manages its
// capacity.
func requireOwner(sub *subscription.Subscription, actor *user.User) error {
	if sub.OwnerUserID != actor.ID {
		return api.Forbidden("only the subscription owner may manage seats")
	}
	return nil
}

func labelFor(n int) string {
	return "Seat " + strconv.Itoa(n)
}
