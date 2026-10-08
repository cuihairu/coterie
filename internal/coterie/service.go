package coterie

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/cuihairu/coterie/internal/audit"
	"github.com/cuihairu/coterie/internal/auth"
	"github.com/cuihairu/coterie/internal/notification"
	"github.com/cuihairu/coterie/internal/provider"
	"github.com/cuihairu/coterie/internal/seat"
	"github.com/cuihairu/coterie/internal/subscription"
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
	store    *Store
	seats    *seat.Service
	notifier *notification.Service
	plugins  *provider.Registry
	log      *slog.Logger
}

// NewService builds a Service. seats is shared with the app wiring so
// coterie creation can provision seats inside its transaction; notifier
// may be nil in tests that do not exercise notifications; plugins is
// the provider plugin registry (nil — Generic behavior only).
func NewService(db *gorm.DB, seats *seat.Service, notifier *notification.Service, plugins *provider.Registry) *Service {
	return &Service{store: NewStore(db), seats: seats, notifier: notifier, plugins: plugins, log: slog.Default()}
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
		// Opt-in exposure (D10): circles start invisible to the
		// marketplace until the owner lists them.
		Listing:   ListingPrivate,
		CreatedAt: now,
		UpdatedAt: now,
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
			if _, err := s.seats.ProvisionOn(ctx, tx, actor, sub.ID, seat.ProvisionRequest{Count: req.Capacity}); err != nil {
				return err
			}
		}
		return audit.Record(ctx, tx, audit.Input{
			ActorID:        actor.ID,
			Action:         audit.ActionCoterieCreated,
			EntityType:     "coterie",
			EntityID:       c.ID,
			SubscriptionID: sub.ID,
			CoterieID:      c.ID,
			After: map[string]any{
				"name": c.Name, "status": c.Status,
				"listing": c.Listing, "capacity": req.Capacity,
			},
		})
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

	// Snapshot what the request touches before mutating — the audit
	// entry carries only the changed fields.
	before := map[string]any{}
	if req.Name != nil {
		before["name"] = c.Name
	}
	if req.Listing != nil {
		before["listing"] = c.Listing
	}
	if req.Status != nil {
		before["status"] = c.Status
	}

	if req.Name != nil {
		if *req.Name == "" {
			return nil, api.Validation("invalid coterie",
				api.Detail{Field: "name", Message: "must not be empty"})
		}
		c.Name = *req.Name
	}
	if req.Listing != nil && *req.Listing != c.Listing {
		if *req.Listing != ListingPrivate && *req.Listing != ListingPublic {
			return nil, api.Validation("invalid coterie",
				api.Detail{Field: "listing", Message: "must be private or public"})
		}
		c.Listing = *req.Listing
	}
	if req.Status != nil && *req.Status != c.Status {
		if !transitions[c.Status][*req.Status] {
			return nil, api.Conflict("cannot transition coterie from %s to %s", c.Status, *req.Status)
		}
		c.Status = *req.Status
	}
	c.UpdatedAt = time.Now().UTC()
	after := map[string]any{}
	for field := range before {
		switch field {
		case "name":
			after[field] = c.Name
		case "listing":
			after[field] = c.Listing
		case "status":
			after[field] = c.Status
		}
	}
	err = s.store.DB().WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Save(c).Error; err != nil {
			return err
		}
		return audit.Record(ctx, tx, audit.Input{
			ActorID:        actor.ID,
			Action:         audit.ActionCoterieUpdated,
			EntityType:     "coterie",
			EntityID:       c.ID,
			SubscriptionID: c.SubscriptionID,
			CoterieID:      c.ID,
			Before:         before,
			After:          after,
		})
	})
	if err != nil {
		return nil, err
	}
	if req.Status != nil && c.Status == StatusClosed && s.notifier != nil {
		if ids, err := s.store.ActiveMemberUserIDs(ctx, c.ID); err == nil {
			for _, uid := range ids {
				s.notifier.Notify(ctx, uid, notification.TypeCoterieClosed,
					"Your sharing circle was closed",
					c.Name+" is now closed.",
					"coterie", c.ID)
			}
		}
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
	m, err := s.admitUser(ctx, s.store.DB().WithContext(ctx), c, actor.ID, inv.Role)
	if err != nil {
		return nil, err
	}
	accepted := false
	err = s.store.DB().WithContext(ctx).Transaction(func(tx *gorm.DB) error {
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

// AdmitMember admits a user into the coterie directly — the marketplace
// join-request accept path. The operator must be the owner or an admin;
// the admitted user runs the same checks as invitation acceptance
// (recruiting status, not full, not already a member).
func (s *Service) AdmitMember(ctx context.Context, operator *user.User, coterieID string, admitted *user.User) (*Member, error) {
	if _, _, err := s.LoadForRole(ctx, operator, coterieID, RoleOwner, RoleAdmin); err != nil {
		return nil, err
	}
	c, err := s.store.GetCoterie(ctx, coterieID)
	if err != nil {
		return nil, err
	}
	if c == nil {
		return nil, api.NotFound("coterie %s not found", coterieID)
	}
	return s.admitUser(ctx, s.store.DB().WithContext(ctx), c, admitted.ID, RoleMember)
}

// admitUser runs the shared admission checks and inserts the active
// member. db allows callers to keep larger transactions around it.
func (s *Service) admitUser(ctx context.Context, db *gorm.DB, c *Coterie, userID, role string) (*Member, error) {
	if c.Status != StatusOpen && c.Status != StatusActive {
		return nil, api.Conflict("coterie is not accepting members while %s", c.Status)
	}
	st := NewStore(db)
	if existing, err := st.ActiveMemberByUser(ctx, c.ID, userID); err != nil {
		return nil, err
	} else if existing != nil {
		return nil, api.Conflict("already a member of this coterie")
	}
	sub, err := st.SubscriptionByID(ctx, c.SubscriptionID)
	if err != nil {
		return nil, err
	}
	if sub == nil {
		return nil, api.Conflict("subscription %s no longer exists", c.SubscriptionID)
	}
	memberCount, err := st.MemberCount(ctx, c.ID)
	if err != nil {
		return nil, err
	}
	// Sharing facet, core part (FR-10): max_members caps the circle
	// regardless of seats — seats bound devices, max_members bounds people.
	if sub.MaxMembers != nil && memberCount >= *sub.MaxMembers {
		return nil, api.Conflict("membership limit reached (%d members)", *sub.MaxMembers)
	}
	total, free, err := st.SeatStats(ctx, c.SubscriptionID)
	if err != nil {
		return nil, err
	}
	if total > 0 && free == 0 {
		return nil, api.Conflict("coterie is full")
	}
	if err := s.checkAdmissionGuard(ctx, st, sub, memberCount, userID, role); err != nil {
		return nil, err
	}
	m := &Member{
		ID:        uuid.NewString(),
		CoterieID: c.ID,
		UserID:    userID,
		Role:      role,
		Status:    "active",
		JoinedAt:  time.Now().UTC(),
	}
	if err := db.Create(m).Error; err != nil {
		return nil, err
	}
	return m, nil
}

// checkAdmissionGuard consults the subscription product provider's
// plugin, when one implements AdmissionGuard (design §5.1). Plain
// errors from plugins become 409s.
func (s *Service) checkAdmissionGuard(ctx context.Context, st *Store, sub *subscription.Subscription, memberCount int, userID, role string) error {
	slug, err := st.ProviderSlugBySubscription(ctx, sub.ID)
	if err != nil {
		return err
	}
	guard, ok := s.plugins.For(slug).(provider.AdmissionGuard)
	if !ok {
		return nil
	}
	in := provider.GuardInput{
		UserID:      userID,
		Role:        role,
		MemberCount: memberCount,
		Policy:      json.RawMessage(sub.SharingPolicy),
	}
	if err := guard.CheckAdmission(ctx, in); err != nil {
		var apiErr *api.APIError
		if errors.As(err, &apiErr) {
			return apiErr
		}
		return api.Conflict("admission refused by provider: %v", err)
	}
	return nil
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
	return s.retireMember(ctx, audit.ActionMemberLeft, actor.ID, c, m)
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
	c, err := s.loadForOwner(ctx, actor, m.CoterieID)
	if err != nil {
		return err
	}
	if m.LeftAt != nil {
		return api.Conflict("member %s has already left", memberID)
	}
	if m.Role == RoleOwner {
		return api.Conflict("the owner cannot be removed")
	}
	return s.retireMember(ctx, audit.ActionMemberRemoved, actor.ID, c, m)
}

// retireMember soft-ends the membership, frees its seats, and records
// the audit entry — all in the one transaction.
func (s *Service) retireMember(ctx context.Context, action, actorID string, c *Coterie, m *Member) error {
	now := time.Now().UTC()
	return s.store.DB().WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := SoftLeftMember(tx, m.ID, now); err != nil {
			return err
		}
		if _, err := s.seats.ReleaseMemberSeatsOn(ctx, tx, m.ID); err != nil {
			return err
		}
		return audit.Record(ctx, tx, audit.Input{
			ActorID:        actorID,
			Action:         action,
			EntityType:     "member",
			EntityID:       m.ID,
			SubscriptionID: c.SubscriptionID,
			CoterieID:      c.ID,
			After:          map[string]any{"user_id": m.UserID, "role": m.Role},
		})
	})
}

// CreateInvitation mints a join token for the coterie. Owners and
// admins may invite (permission baseline); the raw token is returned
// exactly once — only its SHA-256 hash is stored.
func (s *Service) CreateInvitation(ctx context.Context, actor *user.User, coterieID string, req CreateInvitationRequest) (*InvitationView, error) {
	c, m, err := s.LoadForRole(ctx, actor, coterieID, RoleOwner, RoleAdmin)
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
	if _, _, err := s.LoadForRole(ctx, actor, coterieID, RoleOwner, RoleAdmin); err != nil {
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

// LoadForRole returns the coterie and the actor's active membership
// when their role is one of the given ones. Exported for sibling
// modules (marketplace join requests) so permission semantics stay in
// one place.
func (s *Service) LoadForRole(ctx context.Context, actor *user.User, coterieID string, roles ...string) (*Coterie, *Member, error) {
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
