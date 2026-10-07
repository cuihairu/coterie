package coterie

import (
	"context"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/cuihairu/coterie/internal/auth"
	"github.com/cuihairu/coterie/internal/seat"
	"github.com/cuihairu/coterie/internal/user"
	"github.com/cuihairu/coterie/pkg/api"
)

// invitationTTLBounds bounds the invitation lifetime in days.
const (
	defaultInvitationDays = 7
	maxInvitationDays     = 30
)

// transitions is the coterie lifecycle (design §1.5): Draft → Open →
// Active ⇄ Paused, every non-terminal state can be Closed; Closed is
// terminal. Full is not a state (decision D3).
var transitions = map[string]map[string]bool{
	StatusDraft:  {StatusOpen: true, StatusClosed: true},
	StatusOpen:   {StatusActive: true, StatusClosed: true},
	StatusActive: {StatusPaused: true, StatusClosed: true},
	StatusPaused: {StatusActive: true, StatusClosed: true},
	StatusClosed: {},
}

// Service carries the coterie business rules: strict 1:1 with the
// subscription (D1), owner identity (D5), the lifecycle state machine,
// derived full flag (D3), and invitation-based joining.
type Service struct {
	store *Store
	seats *seat.Service
	log   *slog.Logger
}

// NewService builds a Service. seats is shared with the app wiring so
// coterie creation can provision seats inside its transaction.
func NewService(db *gorm.DB, seats *seat.Service) *Service {
	return &Service{store: NewStore(db), seats: seats, log: slog.Default()}
}

// Create bootstraps a coterie for a subscription the actor owns: the
// coterie starts in draft, the owner becomes its first member (role
// owner), and capacity seats are provisioned in the same transaction.
func (s *Service) Create(ctx context.Context, actor *user.User, req CreateCoterieRequest) (*CoterieView, error) {
	if req.Name == "" {
		return nil, api.Validation("invalid coterie",
			api.Detail{Field: "name", Message: "is required"})
	}
	if req.Capacity < 0 {
		return nil, api.Validation("invalid coterie",
			api.Detail{Field: "capacity", Message: "must not be negative"})
	}
	sub, err := s.store.SubscriptionByID(ctx, req.SubscriptionID)
	if err != nil {
		return nil, err
	}
	if sub == nil {
		return nil, api.NotFound("subscription %s not found", req.SubscriptionID)
	}
	if sub.OwnerUserID != actor.ID {
		return nil, api.Forbidden("only the subscription owner may create its coterie")
	}
	if existing, err := s.store.CoterieBySubscription(ctx, sub.ID); err != nil {
		return nil, err
	} else if existing != nil {
		return nil, api.Conflict("subscription %s already has a coterie (strict 1:1)", sub.ID)
	}

	now := time.Now().UTC()
	c := &Coterie{
		ID:             uuid.NewString(),
		SubscriptionID: sub.ID,
		Name:           req.Name,
		Status:         StatusDraft,
		CreatedAt:      now,
		UpdatedAt:      now,
	}
	owner := &Member{
		ID:        uuid.NewString(),
		CoterieID: c.ID,
		UserID:    sub.OwnerUserID,
		Role:      RoleOwner,
		Status:    "active",
		JoinedAt:  now,
	}

	err = s.store.DB().WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(c).Error; err != nil {
			return err
		}
		if err := tx.Create(owner).Error; err != nil {
			return err
		}
		if req.Capacity > 0 {
			_, err := s.seats.ProvisionOn(ctx, tx, actor, sub.ID, seat.ProvisionRequest{Count: req.Capacity})
			return err
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return s.view(ctx, c)
}

// Get returns the coterie with its derived display state.
func (s *Service) Get(ctx context.Context, id string) (*CoterieView, error) {
	c, err := s.store.GetCoterie(ctx, id)
	if err != nil {
		return nil, err
	}
	if c == nil {
		return nil, api.NotFound("coterie %s not found", id)
	}
	return s.view(ctx, c)
}

// List returns coteries, optionally filtered by subscription.
func (s *Service) List(ctx context.Context, subscriptionID string, page api.Page) ([]CoterieView, int64, error) {
	items, total, err := s.store.ListCoteries(ctx, subscriptionID, page)
	if err != nil {
		return nil, 0, err
	}
	views := make([]CoterieView, len(items))
	for i := range items {
		v, err := s.view(ctx, &items[i])
		if err != nil {
			return nil, 0, err
		}
		views[i] = *v
	}
	return views, total, nil
}

// Update patches the name and drives the lifecycle state machine.
// Only the subscription owner (≡ coterie owner, D5) may call it.
func (s *Service) Update(ctx context.Context, actor *user.User, id string, req UpdateCoterieRequest) (*CoterieView, error) {
	c, err := s.loadForOwner(ctx, actor, id)
	if err != nil {
		return nil, err
	}
	if req.Name != nil {
		if *req.Name == "" {
			return nil, api.Validation("invalid coterie",
				api.Detail{Field: "name", Message: "must not be empty"})
		}
		c.Name = *req.Name
	}
	if req.Status != nil && *req.Status != c.Status {
		if !transitions[c.Status][*req.Status] {
			return nil, api.Conflict("cannot transition coterie from %s to %s", c.Status, *req.Status)
		}
		c.Status = *req.Status
	}
	c.UpdatedAt = time.Now().UTC()
	if err := s.store.UpdateCoterie(ctx, c); err != nil {
		return nil, err
	}
	return s.view(ctx, c)
}

// ListMembers returns the active members of the coterie.
func (s *Service) ListMembers(ctx context.Context, coterieID string, page api.Page) ([]Member, int64, error) {
	if _, err := s.Get(ctx, coterieID); err != nil {
		return nil, 0, err
	}
	return s.store.ListMembers(ctx, coterieID, page)
}

// Join accepts an invitation token: the invitation must be live, the
// coterie must be recruiting (open or active) with a free seat, and the
// user must not already be a member.
func (s *Service) Join(ctx context.Context, actor *user.User, req AcceptInvitationRequest) (*Member, error) {
	if req.Token == "" {
		return nil, api.Validation("invalid invitation",
			api.Detail{Field: "token", Message: "is required"})
	}
	inv, err := s.store.InvitationByToken(ctx, auth.HashToken(req.Token))
	if err != nil {
		return nil, err
	}
	if inv == nil {
		return nil, api.NotFound("invitation not found")
	}
	now := time.Now().UTC()
	if inv.AcceptedAt != nil {
		return nil, api.Conflict("invitation has already been used")
	}
	if inv.ExpireAt.Before(now) {
		return nil, api.Conflict("invitation has expired")
	}

	c, err := s.store.GetCoterie(ctx, inv.CoterieID)
	if err != nil {
		return nil, err
	}
	if c == nil {
		return nil, api.NotFound("coterie %s not found", inv.CoterieID)
	}
	if c.Status != StatusOpen && c.Status != StatusActive {
		return nil, api.Conflict("coterie is not accepting members while %s", c.Status)
	}
	if existing, err := s.store.ActiveMemberByUser(ctx, c.ID, actor.ID); err != nil {
		return nil, err
	} else if existing != nil {
		return nil, api.Conflict("already a member of this coterie")
	}

	total, free, err := s.store.SeatStats(ctx, c.SubscriptionID)
	if err != nil {
		return nil, err
	}
	if total > 0 && free == 0 {
		return nil, api.Conflict("coterie is full")
	}

	m := &Member{
		ID:        uuid.NewString(),
		CoterieID: c.ID,
		UserID:    actor.ID,
		Role:      inv.Role,
		Status:    "active",
		JoinedAt:  now,
	}
	accepted := false
	err = s.store.DB().WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(m).Error; err != nil {
			return err
		}
		ok, err := s.store.MarkInvitationAccepted(ctx, inv.ID, now)
		accepted = ok
		return err
	})
	if err != nil {
		return nil, err
	}
	if !accepted {
		return nil, api.Conflict("invitation has already been used")
	}
	return m, nil
}

// Leave marks the actor's membership as ended and releases their seats
// in the same transaction. The owner cannot leave (D5); closing the
// coterie is the way out.
func (s *Service) Leave(ctx context.Context, actor *user.User, coterieID string) error {
	c, err := s.store.GetCoterie(ctx, coterieID)
	if err != nil {
		return err
	}
	if c == nil {
		return api.NotFound("coterie %s not found", coterieID)
	}
	m, err := s.store.ActiveMemberByUser(ctx, c.ID, actor.ID)
	if err != nil {
		return err
	}
	if m == nil {
		return api.NotFound("you are not a member of coterie %s", coterieID)
	}
	if m.Role == RoleOwner {
		return api.Conflict("the owner cannot leave; close the coterie instead")
	}
	return s.retireMember(ctx, m)
}

// RemoveMember ends a membership by id (owner only) and releases the
// member's seats in the same transaction.
func (s *Service) RemoveMember(ctx context.Context, actor *user.User, memberID string) error {
	m, err := s.store.GetMember(ctx, memberID)
	if err != nil {
		return err
	}
	if m == nil {
		return api.NotFound("member %s not found", memberID)
	}
	if _, err := s.loadForOwner(ctx, actor, m.CoterieID); err != nil {
		return err
	}
	if m.LeftAt != nil {
		return api.Conflict("member %s has already left", memberID)
	}
	if m.Role == RoleOwner {
		return api.Conflict("the owner cannot be removed")
	}
	return s.retireMember(ctx, m)
}

// retireMember soft-ends the membership and frees its seats atomically.
func (s *Service) retireMember(ctx context.Context, m *Member) error {
	now := time.Now().UTC()
	return s.store.DB().WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := SoftLeftMember(tx, m.ID, now); err != nil {
			return err
		}
		_, err := s.seats.ReleaseMemberSeatsOn(ctx, tx, m.ID)
		return err
	})
}

// CreateInvitation mints a join token for the coterie. Owners and
// admins may invite (permission baseline); the raw token is returned
// exactly once — only its SHA-256 hash is stored.
func (s *Service) CreateInvitation(ctx context.Context, actor *user.User, coterieID string, req CreateInvitationRequest) (*InvitationView, error) {
	c, m, err := s.loadForRole(ctx, actor, coterieID, RoleOwner, RoleAdmin)
	if err != nil {
		return nil, err
	}
	role := req.Role
	if role == "" {
		role = RoleMember
	}
	if role != RoleAdmin && role != RoleMember {
		return nil, api.Validation("invalid invitation",
			api.Detail{Field: "role", Message: "must be admin or member"})
	}
	days := req.ExpiresInDays
	if days == 0 {
		days = defaultInvitationDays
	}
	if days < 1 || days > maxInvitationDays {
		return nil, api.Validation("invalid invitation",
			api.Detail{Field: "expires_in_days", Message: "must be between 1 and 30"})
	}

	raw, err := auth.NewToken()
	if err != nil {
		return nil, err
	}
	now := time.Now().UTC()
	inv := &Invitation{
		ID:        uuid.NewString(),
		CoterieID: c.ID,
		Token:     auth.HashToken(raw),
		Role:      role,
		ExpireAt:  now.AddDate(0, 0, days),
		CreatedBy: m.UserID,
		CreatedAt: now,
	}
	if err := s.store.CreateInvitation(ctx, inv); err != nil {
		return nil, err
	}
	return &InvitationView{Invitation: *inv, Token: raw}, nil
}

// ListInvitations returns the coterie's invitations (owners and admins
// only). Tokens are never included.
func (s *Service) ListInvitations(ctx context.Context, actor *user.User, coterieID string, page api.Page) ([]Invitation, int64, error) {
	if _, _, err := s.loadForRole(ctx, actor, coterieID, RoleOwner, RoleAdmin); err != nil {
		return nil, 0, err
	}
	return s.store.ListInvitations(ctx, coterieID, page)
}

// loadForOwner returns the coterie when the actor owns its subscription
// (D5) — 404 for unknown, 403 otherwise.
func (s *Service) loadForOwner(ctx context.Context, actor *user.User, coterieID string) (*Coterie, error) {
	c, err := s.store.GetCoterie(ctx, coterieID)
	if err != nil {
		return nil, err
	}
	if c == nil {
		return nil, api.NotFound("coterie %s not found", coterieID)
	}
	sub, err := s.store.SubscriptionByID(ctx, c.SubscriptionID)
	if err != nil {
		return nil, err
	}
	if sub == nil || sub.OwnerUserID != actor.ID {
		return nil, api.Forbidden("only the subscription owner may manage this coterie")
	}
	return c, nil
}

// loadForRole returns the coterie and the actor's active membership
// when their role is one of the given ones.
func (s *Service) loadForRole(ctx context.Context, actor *user.User, coterieID string, roles ...string) (*Coterie, *Member, error) {
	c, err := s.store.GetCoterie(ctx, coterieID)
	if err != nil {
		return nil, nil, err
	}
	if c == nil {
		return nil, nil, api.NotFound("coterie %s not found", coterieID)
	}
	m, err := s.store.ActiveMemberByUser(ctx, c.ID, actor.ID)
	if err != nil {
		return nil, nil, err
	}
	if m == nil {
		return nil, nil, api.Forbidden("you are not a member of this coterie")
	}
	allowed := false
	for _, r := range roles {
		if m.Role == r {
			allowed = true
			break
		}
	}
	if !allowed {
		return nil, nil, api.Forbidden("requires role %v", roles)
	}
	return c, m, nil
}

// view attaches the derived display state to a coterie.
func (s *Service) view(ctx context.Context, c *Coterie) (*CoterieView, error) {
	members, err := s.store.MemberCount(ctx, c.ID)
	if err != nil {
		return nil, err
	}
	total, free, err := s.store.SeatStats(ctx, c.SubscriptionID)
	if err != nil {
		return nil, err
	}
	return &CoterieView{
		Coterie:     *c,
		MemberCount: members,
		SeatsTotal:  total,
		SeatsFree:   free,
		// Full means no seat is idle. A circle without seats yet is
		// not full — there is nothing to fill (design §1.5).
		Full: total > 0 && free == 0,
	}, nil
}
