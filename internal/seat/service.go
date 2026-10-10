package seat

import (
	"context"
	"encoding/json"
	"log/slog"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/cuihairu/coterie/internal/audit"
	"github.com/cuihairu/coterie/internal/database"
	"github.com/cuihairu/coterie/internal/notification"
	"github.com/cuihairu/coterie/internal/subscription"
	"github.com/cuihairu/coterie/internal/user"
	"github.com/cuihairu/coterie/pkg/api"
)

// Service enforces the seat rules: capacity comes from the subscription
// (D2), mutations belong to the subscription owner (D5), and an assignee
// must be an active member of the subscription's coterie (invariant 4).
type Service struct {
	store    *Store
	notifier *notification.Service
	log      *slog.Logger
}

// NewService builds a Service. notifier may be nil in tests that do not
// exercise notifications.
func NewService(db *gorm.DB, notifier *notification.Service) *Service {
	return &Service{store: NewStore(db), notifier: notifier, log: slog.Default()}
}

// Provision creates count seats for the subscription, named "Seat N"
// continuing from the existing ones. Total seats must stay within
// subscription.max_seats.
func (s *Service) Provision(ctx context.Context, actor *user.User, subscriptionID string, req ProvisionRequest) ([]Seat, error) {
	return s.ProvisionOn(ctx, s.store.db, actor, subscriptionID, req)
}

// ProvisionOn is Provision against an explicit database handle so the
// coterie module can include seat provisioning inside its own
// transaction when bootstrapping a circle.
func (s *Service) ProvisionOn(ctx context.Context, db *gorm.DB, actor *user.User, subscriptionID string, req ProvisionRequest) ([]Seat, error) {
	store := NewStore(db)
	sub, err := store.SubscriptionByID(ctx, subscriptionID)
	if err != nil {
		return nil, err
	}
	if sub == nil {
		return nil, api.NotFound("subscription %s not found", subscriptionID)
	}
	if err := requireOwner(sub, actor); err != nil {
		return nil, err
	}
	if req.Count <= 0 {
		return nil, api.Validation("invalid seat provision",
			api.Detail{Field: "count", Message: "must be greater than 0"})
	}

	existing, err := store.CountBySubscription(ctx, sub.ID)
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
	if err := store.Create(ctx, seats); err != nil {
		if database.IsUniqueViolation(err) {
			return nil, api.Conflict("seat label already exists for this subscription")
		}
		return nil, err
	}
	return seats, nil
}

// ReleaseMemberSeatsOn frees every seat the member occupies, on the
// given database handle. Coterie membership removal calls this inside
// its transaction so a departing member never keeps a seat.
func (s *Service) ReleaseMemberSeatsOn(ctx context.Context, db *gorm.DB, memberID string) (int64, error) {
	return NewStore(db).ReleaseByMember(ctx, memberID)
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
	coterieID, err := s.store.CoterieIDBySubscription(ctx, sub.ID)
	if err != nil {
		return nil, err
	}
	err = s.store.DB().WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		ok, err := NewStore(tx).Occupy(ctx, seat.ID, req.MemberID)
		if err != nil {
			return err
		}
		if !ok {
			return api.Conflict("seat %s is no longer free", seat.ID)
		}
		return audit.Record(ctx, tx, audit.Input{
			ActorID:        actor.ID,
			Action:         audit.ActionSeatAssigned,
			EntityType:     "seat",
			EntityID:       seat.ID,
			SubscriptionID: seat.SubscriptionID,
			CoterieID:      coterieID,
			After:          map[string]any{"member_id": req.MemberID, "status": StatusOccupied},
		})
	})
	if err != nil {
		return nil, err
	}
	if s.notifier != nil {
		if userID, err := s.store.MemberUser(ctx, req.MemberID); err == nil && userID != "" {
			s.notifier.Notify(ctx, userID, notification.TypeSeatAssigned,
				"A seat was assigned to you",
				"You now hold "+seat.Label+" in your sharing circle.",
				"seat", seat.ID)
		}
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
	coterieID, err := s.store.CoterieIDBySubscription(ctx, sub.ID)
	if err != nil {
		return nil, err
	}
	err = s.store.DB().WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		changed, err := NewStore(tx).Release(ctx, seat.ID)
		if err != nil {
			return err
		}
		if !changed {
			return nil // releasing a free seat is a no-op
		}
		return audit.Record(ctx, tx, audit.Input{
			ActorID:        actor.ID,
			Action:         audit.ActionSeatReleased,
			EntityType:     "seat",
			EntityID:       seat.ID,
			SubscriptionID: seat.SubscriptionID,
			CoterieID:      coterieID,
			Before:         map[string]any{"member_id": seat.MemberID, "status": StatusOccupied},
			After:          map[string]any{"member_id": nil, "status": StatusFree},
		})
	})
	if err != nil {
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
	if req.Label != nil {
		// Labels are display names: trim and store the trimmed value so
		// whitespace-only or padded labels never land.
		label := strings.TrimSpace(*req.Label)
		if label == "" {
			details = append(details, api.Detail{Field: "label", Message: "must not be empty"})
		} else {
			req.Label = &label
		}
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

	// Snapshot what the request touches before mutating — the audit
	// entry carries only the changed fields.
	before := map[string]any{}
	if req.Label != nil {
		before["label"] = seat.Label
	}
	if req.Metadata != nil {
		before["metadata"] = json.RawMessage(seat.Metadata)
	}
	if req.Status != nil {
		before["status"] = seat.Status
		if seat.Status == StatusOccupied {
			before["member_id"] = seat.MemberID
		}
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

	after := map[string]any{}
	for field := range before {
		switch field {
		case "label":
			after[field] = seat.Label
		case "metadata":
			after[field] = json.RawMessage(seat.Metadata)
		case "status":
			after[field] = seat.Status
		case "member_id":
			after[field] = seat.MemberID
		}
	}

	coterieID, err := s.store.CoterieIDBySubscription(ctx, seat.SubscriptionID)
	if err != nil {
		return nil, err
	}
	err = s.store.DB().WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Save(seat).Error; err != nil {
			if database.IsUniqueViolation(err) {
				return api.Conflict("seat label already exists for this subscription")
			}
			return err
		}
		return audit.Record(ctx, tx, audit.Input{
			ActorID:        actor.ID,
			Action:         audit.ActionSeatUpdated,
			EntityType:     "seat",
			EntityID:       seat.ID,
			SubscriptionID: seat.SubscriptionID,
			CoterieID:      coterieID,
			Before:         before,
			After:          after,
		})
	})
	if err != nil {
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
