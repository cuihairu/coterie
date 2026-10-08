// Package marketplace implements Phase 2 marketplace: a public,
// opt-in directory of recruiting coteries plus the join-request inbox
// (design D10). It stays decoupled from the core sharing model —
// admission goes through coterie.Service.AdmitMember so the checks
// match invitation acceptance exactly.
package marketplace

import (
	"context"
	"log/slog"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/cuihairu/coterie/internal/coterie"
	"github.com/cuihairu/coterie/internal/notification"
	"github.com/cuihairu/coterie/internal/reputation"
	"github.com/cuihairu/coterie/internal/user"
	"github.com/cuihairu/coterie/pkg/api"
)

// maxMessageLen bounds the free-text note sent with a join request.
const maxMessageLen = 500

// Service serves the directory and the join-request lifecycle.
type Service struct {
	store    *Store
	coteries *coterie.Service
	users    *user.Store
	rep      *reputation.Service
	notifier *notification.Service
	log      *slog.Logger
}

// NewService builds a Service. notifier may be nil in tests.
func NewService(db *gorm.DB, coteries *coterie.Service, notifier *notification.Service) *Service {
	return &Service{
		store:    NewStore(db),
		coteries: coteries,
		users:    user.NewStore(db),
		rep:      reputation.NewService(db),
		notifier: notifier,
		log:      slog.Default(),
	}
}

// Directory lists publicly listed, recruiting coteries; the product
// filter is optional. Public endpoint — no auth. Each entry carries
// the owner's derived reputation badge (D18).
func (s *Service) Directory(ctx context.Context, productID string, page api.Page) ([]DirectoryEntry, int64, error) {
	entries, total, err := s.store.Directory(ctx, strings.TrimSpace(productID), page)
	if err != nil {
		return nil, 0, err
	}
	ownerIDs := make([]string, 0, len(entries))
	seen := map[string]bool{}
	for _, e := range entries {
		if e.OwnerID != "" && !seen[e.OwnerID] {
			seen[e.OwnerID] = true
			ownerIDs = append(ownerIDs, e.OwnerID)
		}
	}
	reports, err := s.rep.Reports(ctx, ownerIDs)
	if err != nil {
		return nil, 0, err
	}
	for i := range entries {
		e := &entries[i]
		e.Full = e.SeatsTotal > 0 && e.SeatsFree == 0
		e.ShareEstimate = shareEstimate(e.Price, e.MemberCount)
		if rep, ok := reports[e.OwnerID]; ok {
			e.Owner = &OwnerBadge{
				UserID:        rep.UserID,
				Username:      e.OwnerUsername,
				Contributions: rep.Contributions,
				PaymentRatio:  rep.PaymentRatio,
			}
		}
	}
	return entries, total, nil
}

// CreateJoinRequest files a pending request for a publicly listed
// coterie. One live request per user and coterie; private, closed,
// full, or already-member situations are rejected (private circles 404
// so existence stays hidden).
func (s *Service) CreateJoinRequest(ctx context.Context, actor *user.User, coterieID string, req CreateJoinRequestRequest) (*JoinRequest, error) {
	c, err := s.lookupListed(ctx, coterieID)
	if err != nil {
		return nil, err
	}
	if _, member, err := s.store.ActiveMemberRole(ctx, c.ID, actor.ID); err != nil {
		return nil, err
	} else if member {
		return nil, api.Conflict("already a member of this coterie")
	}
	if existing, err := s.store.PendingByUserAndCoterie(ctx, c.ID, actor.ID); err != nil {
		return nil, err
	} else if existing != nil {
		return nil, api.Conflict("a join request is already pending for this coterie")
	}
	if total, free, err := s.store.SeatStats(ctx, c.SubscriptionID); err != nil {
		return nil, err
	} else if total > 0 && free == 0 {
		return nil, api.Conflict("coterie is full")
	}

	message := strings.TrimSpace(req.Message)
	if len(message) > maxMessageLen {
		return nil, api.Validation("invalid join request",
			api.Detail{Field: "message", Message: "must be at most 500 characters"})
	}
	r := &JoinRequest{
		ID:        uuid.NewString(),
		CoterieID: c.ID,
		UserID:    actor.ID,
		Message:   message,
		Status:    RequestPending,
		CreatedAt: time.Now().UTC(),
	}
	if err := s.store.Create(ctx, r); err != nil {
		return nil, err
	}
	if s.notifier != nil {
		if owner, err := s.store.CoterieOwnerUserID(ctx, c.ID); err == nil && owner != "" {
			s.notifier.Notify(ctx, owner, notification.TypeJoinRequested,
				"New join request for "+c.Name,
				displayName(actor)+" wants to join. Review it in the coterie's request inbox.",
				"coterie", c.ID)
		} else if err != nil {
			s.log.Warn("marketplace: locate coterie owner", "err", err)
		}
	}
	return r, nil
}

// ListJoinRequests returns the coterie's requests — owner or admin
// only. Status filter is optional.
func (s *Service) ListJoinRequests(ctx context.Context, actor *user.User, coterieID, status string, page api.Page) ([]JoinRequestView, int64, error) {
	if _, _, err := s.coteries.LoadForRole(ctx, actor, coterieID, coterie.RoleOwner, coterie.RoleAdmin); err != nil {
		return nil, 0, err
	}
	return s.store.ListByCoterie(ctx, coterieID, strings.TrimSpace(status), page)
}

// AcceptJoinRequest admits the requester as a member (same checks as
// invitation acceptance) and closes the request. Capacity is
// re-checked at admission time; the request stays pending when the
// circle filled up in the meantime, so the decision can be retried
// after a seat frees up.
func (s *Service) AcceptJoinRequest(ctx context.Context, actor *user.User, requestID string) (*JoinRequest, error) {
	r, c, err := s.loadRequest(ctx, actor, requestID)
	if err != nil {
		return nil, err
	}
	requester, err := s.users.Get(ctx, r.UserID)
	if err != nil {
		return nil, err
	}
	if requester == nil {
		return nil, api.NotFound("user %s not found", r.UserID)
	}
	if _, err := s.coteries.AdmitMember(ctx, actor, c.ID, requester); err != nil {
		return nil, err
	}
	ok, err := s.store.SetStatus(ctx, r.ID, RequestAccepted)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, api.Conflict("join request %s is no longer pending", r.ID)
	}
	s.decided(r, requester, c, true)
	return s.reload(ctx, r.ID)
}

// DeclineJoinRequest refuses a pending request — owner or admin only.
func (s *Service) DeclineJoinRequest(ctx context.Context, actor *user.User, requestID string) (*JoinRequest, error) {
	r, c, err := s.loadRequest(ctx, actor, requestID)
	if err != nil {
		return nil, err
	}
	ok, err := s.store.SetStatus(ctx, r.ID, RequestDeclined)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, api.Conflict("join request %s is no longer pending", r.ID)
	}
	if requester, err := s.users.Get(ctx, r.UserID); err == nil && requester != nil {
		s.decided(r, requester, c, false)
	}
	return s.reload(ctx, r.ID)
}

// CancelJoinRequest withdraws the actor's own pending request.
func (s *Service) CancelJoinRequest(ctx context.Context, actor *user.User, requestID string) error {
	r, err := s.store.RequestByID(ctx, requestID)
	if err != nil {
		return err
	}
	if r == nil {
		return api.NotFound("join request %s not found", requestID)
	}
	if r.UserID != actor.ID {
		return api.Forbidden("only the requester may withdraw a join request")
	}
	ok, err := s.store.SetStatus(ctx, r.ID, RequestCancelled)
	if err != nil {
		return err
	}
	if !ok {
		return api.Conflict("join request %s is no longer pending", r.ID)
	}
	return nil
}

// loadRequest returns the request plus its coterie after checking the
// actor may decide it (owner or admin).
func (s *Service) loadRequest(ctx context.Context, actor *user.User, requestID string) (*JoinRequest, *coterie.Coterie, error) {
	r, err := s.store.RequestByID(ctx, requestID)
	if err != nil {
		return nil, nil, err
	}
	if r == nil {
		return nil, nil, api.NotFound("join request %s not found", requestID)
	}
	c, _, err := s.coteries.LoadForRole(ctx, actor, r.CoterieID, coterie.RoleOwner, coterie.RoleAdmin)
	if err != nil {
		return nil, nil, err
	}
	return r, c, nil
}

// lookupListed returns the coterie only when it is publicly listed —
// private circles 404 so their existence stays hidden.
func (s *Service) lookupListed(ctx context.Context, coterieID string) (*coterie.Coterie, error) {
	c, err := s.store.CoterieByID(ctx, coterieID)
	if err != nil {
		return nil, err
	}
	if c == nil || c.Listing != coterie.ListingPublic {
		return nil, api.NotFound("coterie %s not found", coterieID)
	}
	return c, nil
}

func (s *Service) reload(ctx context.Context, id string) (*JoinRequest, error) {
	r, err := s.store.RequestByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if r == nil {
		return nil, api.Internal()
	}
	return r, nil
}

func (s *Service) decided(r *JoinRequest, requester *user.User, c *coterie.Coterie, accepted bool) {
	if s.notifier == nil {
		return
	}
	title, body := "Your join request was declined", "Your request to join "+c.Name+" was declined."
	if accepted {
		title, body = "You joined "+c.Name, "Your join request was accepted. Welcome!"
	}
	s.notifier.Notify(context.Background(), requester.ID, notification.TypeJoinDecided,
		title, body, "coterie", c.ID)
}

// shareEstimate floors the price over the member count under the equal
// split; format follows the money conventions (two fraction digits).
// Display-only: real shares come from the billing module.
func shareEstimate(price string, members int) string {
	if members <= 0 {
		return price
	}
	cents, ok := parseCents(price)
	if !ok {
		return price
	}
	base := cents / int64(members)
	return strconv.FormatInt(base/100, 10) + "." + fmtPad2(base%100)
}

func fmtPad2(v int64) string {
	if v < 10 {
		return "0" + strconv.FormatInt(v, 10)
	}
	return strconv.FormatInt(v, 10)
}

// parseCents converts a decimal string to cents. ok is false when the
// value is not a plain non-negative decimal.
func parseCents(v string) (int64, bool) {
	intPart, fracPart := v, "0"
	if dot := strings.IndexByte(v, '.'); dot >= 0 {
		intPart, fracPart = v[:dot], v[dot+1:]
	}
	if intPart == "" || len(fracPart) > 2 {
		return 0, false
	}
	if len(fracPart) == 1 {
		fracPart += "0"
	}
	i, err := strconv.ParseInt(intPart, 10, 64)
	if err != nil || i < 0 {
		return 0, false
	}
	f, err := strconv.ParseInt(fracPart, 10, 64)
	if err != nil {
		return 0, false
	}
	return i*100 + f, true
}

func displayName(u *user.User) string {
	if u.Username != "" {
		return u.Username
	}
	return u.Email
}
