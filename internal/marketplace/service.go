// Package marketplace implements Phase 2 marketplace: a public,
// opt-in directory of recruiting coteries plus the join-request inbox
// (design D10). It stays decoupled from the core sharing model —
// admission goes through coterie.Service.AdmitMember so the checks
// match invitation acceptance exactly.
package marketplace

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/cuihairu/coterie/internal/audit"
	"github.com/cuihairu/coterie/internal/coterie"
	"github.com/cuihairu/coterie/internal/notification"
	"github.com/cuihairu/coterie/internal/payment"
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
	adapters map[string]payment.Adapter
	log      *slog.Logger
}

// NewService builds a Service. notifier may be nil in tests; the
// adapters back the payment gate (D25) — manual arrives always-on.
func NewService(db *gorm.DB, coteries *coterie.Service, notifier *notification.Service, adapters ...payment.Adapter) *Service {
	byName := make(map[string]payment.Adapter, len(adapters)+1)
	byName[(payment.Manual{}).Name()] = payment.Manual{}
	for _, a := range adapters {
		byName[a.Name()] = a
	}
	return &Service{
		store:    NewStore(db),
		coteries: coteries,
		users:    user.NewStore(db),
		rep:      reputation.NewService(db),
		notifier: notifier,
		adapters: byName,
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
	// The owner's block list (design D26) silences requests before they
	// reach the inbox; existing members are not touched by blocks.
	if blocked, err := s.store.IsBlocked(ctx, c.ID, actor.ID); err != nil {
		return nil, err
	} else if blocked {
		return nil, api.Forbidden("you are blocked from this coterie")
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

// ListMyJoinRequests returns the caller's own requests across circles,
// newest first. The scope is strictly the caller — nothing is taken
// from the coterie, so there is no capacity or role check here.
func (s *Service) ListMyJoinRequests(ctx context.Context, actor *user.User, page api.Page) ([]MyJoinRequestView, int64, error) {
	return s.store.ListMine(ctx, actor.ID, page)
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
	if r.Status != RequestPending {
		return nil, api.Conflict("join request %s is %s, not pending", r.ID, r.Status)
	}
	requester, err := s.users.Get(ctx, r.UserID)
	if err != nil {
		return nil, err
	}
	if requester == nil {
		return nil, api.NotFound("user %s not found", r.UserID)
	}

	// Payment gate (D25): the owner's consent is recorded and admission
	// is held until the requester's charge confirms. Manual owners
	// finish the same way — their payments call settles and admits.
	if c.PaymentGate {
		ok, err := s.store.SetStatus(ctx, r.ID, RequestAwaitingPayment)
		if err != nil {
			return nil, err
		}
		if !ok {
			return nil, api.Conflict("join request %s is no longer pending", r.ID)
		}
		if s.notifier != nil {
			s.notifier.Notify(ctx, r.UserID, notification.TypeAdmissionDue,
				"Complete your payment to join "+c.Name,
				"Your join request was accepted. Pay your share to finish joining.",
				"coterie", c.ID)
		}
		return s.reload(ctx, r.ID)
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
	if err := s.rejectWhileCharging(ctx, r); err != nil {
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
	if err := s.rejectWhileCharging(ctx, r); err != nil {
		return err
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

// rejectWhileCharging refuses a decision while a charge for the
// request is in flight — money may already have left the payer.
func (s *Service) rejectWhileCharging(ctx context.Context, r *JoinRequest) error {
	charge, err := s.store.LiveChargeByRequest(ctx, r.ID)
	if err != nil {
		return err
	}
	if charge != nil && charge.Status == ChargePending {
		return api.Conflict("an admission payment is still in flight for this request")
	}
	return nil
}

// StartAdmissionCharge charges the gate's estimated share for a
// request held at awaiting_payment (D25). The requester pays their own
// way (the owner may too); manual receipts are owner-only — the
// requester must never self-certify a payment. Sync channels settle
// and admit in one transaction; async ones land a pending charge the
// channel's webhook confirms.
func (s *Service) StartAdmissionCharge(ctx context.Context, actor *user.User, requestID string, req StartAdmissionChargeRequest) (*AdmissionCharge, error) {
	r, err := s.store.RequestByID(ctx, requestID)
	if err != nil {
		return nil, err
	}
	if r == nil {
		return nil, api.NotFound("join request %s not found", requestID)
	}
	c, err := s.store.CoterieByID(ctx, r.CoterieID)
	if err != nil {
		return nil, err
	}
	if c == nil {
		return nil, api.NotFound("coterie %s not found", r.CoterieID)
	}

	method := strings.TrimSpace(req.Method)
	if method == "" {
		method = (payment.Manual{}).Name()
	}
	adapter, ok := s.adapters[method]
	if !ok {
		return nil, api.Validation("invalid payment",
			api.Detail{Field: "method", Message: fmt.Sprintf("must be one of the registered payment methods (%s)", strings.Join(s.Methods(), ", "))})
	}

	isRequester := actor.ID == r.UserID
	isOwner := false
	if !isRequester {
		if _, m, err := s.coteries.LoadForRole(ctx, actor, c.ID, coterie.RoleOwner, coterie.RoleAdmin); err != nil {
			return nil, err
		} else if m != nil {
			isOwner = true
		}
	}
	if !isRequester && !isOwner {
		return nil, api.Forbidden("only the requester or the coterie owner may start an admission charge")
	}
	if method == (payment.Manual{}).Name() && !isOwner {
		return nil, api.Forbidden("only the coterie owner may record a manual admission charge")
	}

	if r.Status != RequestAwaitingPayment {
		return nil, api.Conflict("join request %s is %s, not awaiting payment", r.ID, r.Status)
	}
	if existing, err := s.store.LiveChargeByRequest(ctx, r.ID); err != nil {
		return nil, err
	} else if existing != nil {
		return nil, api.Conflict("an admission charge is already %s for this request", existing.Status)
	}

	// The amount is the platform's estimate: an equal split of the
	// price over the members after admission — same math the directory
	// shows, never above the price itself.
	price, currency, members, ok, err := s.store.GateQuote(ctx, c.ID)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, api.NotFound("coterie %s not found", c.ID)
	}
	if total, free, err := s.store.SeatStats(ctx, c.SubscriptionID); err != nil {
		return nil, err
	} else if total > 0 && free == 0 {
		return nil, api.Conflict("coterie is full")
	}
	amount := shareEstimate(price, members+1)

	charge := &AdmissionCharge{
		ID:            uuid.NewString(),
		CoterieID:     c.ID,
		UserID:        r.UserID,
		JoinRequestID: r.ID,
		Amount:        amount,
		Currency:      currency,
		Method:        method,
		Status:        ChargePending,
		CreatedAt:     time.Now().UTC(),
	}
	direction := payment.Charge{
		PayerUserID: r.UserID,
		Amount:      amount,
		Currency:    currency,
		Description: fmt.Sprintf("Coterie admission %s", r.ID),
	}

	if async, ok := adapter.(payment.AsyncAdapter); ok {
		pending, err := async.ChargeAsync(ctx, direction)
		if err != nil {
			return nil, err
		}
		ref := pending.ExternalRef
		charge.ExternalRef = &ref
		if err := s.store.DB().WithContext(ctx).Transaction(func(tx *gorm.DB) error {
			if err := tx.Create(charge).Error; err != nil {
				return err
			}
			return s.auditCharge(ctx, tx, actor.ID, charge, c, ChargePending)
		}); err != nil {
			return nil, err
		}
		charge.ClientSecret = pending.ClientSecret
		return charge, nil
	}

	// Sync channel: the charge answers inside the settlement
	// transaction — receipt, member, and the request's flip land
	// together or not at all.
	receipt, err := adapter.Charge(ctx, direction)
	if err != nil {
		return nil, err
	}
	now := time.Now().UTC()
	charge.Status = ChargeSucceeded
	charge.ConfirmedAt = &now
	if receipt.ExternalRef != "" {
		ref := receipt.ExternalRef
		charge.ExternalRef = &ref
	} else if note := strings.TrimSpace(req.ExternalRef); note != "" {
		charge.ExternalRef = &note
	}
	if err := s.store.DB().WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if _, err := s.coteries.AdmitUserInTx(ctx, tx, c, r.UserID, coterie.RoleMember); err != nil {
			return err
		}
		if err := tx.Create(charge).Error; err != nil {
			return err
		}
		if _, err := NewStore(tx).SetStatusFromAwaiting(ctx, r.ID, RequestAccepted); err != nil {
			return err
		}
		return s.auditCharge(ctx, tx, actor.ID, charge, c, ChargeSucceeded)
	}); err != nil {
		return nil, err
	}
	if requester, err := s.users.Get(ctx, r.UserID); err == nil && requester != nil {
		s.decided(r, requester, c, true)
	}
	return charge, nil
}

// ConfirmAdmission consumes a channel's outbound confirmation for an
// admission charge (D25). Success runs the full admission checks and
// inserts the member in the same transaction as the charge flip — a
// full circle rolls back and the channel's retries re-admit once a
// seat frees. Failures only flip the charge; the request stays
// awaiting payment and a fresh charge may be started.
func (s *Service) ConfirmAdmission(ctx context.Context, method string, header http.Header, payload []byte) (*AdmissionCharge, error) {
	adapter, ok := s.adapters[method]
	if !ok {
		return nil, api.NotFound("unknown payment method %s", method)
	}
	async, ok := adapter.(payment.AsyncAdapter)
	if !ok {
		return nil, api.Validation("invalid webhook",
			api.Detail{Field: "method", Message: "channel does not confirm asynchronously"})
	}
	event, ok, err := async.ParseWebhook(header, payload)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, nil // uninteresting event kind — 200, nothing to do
	}
	if event.ExternalRef == "" {
		return nil, api.Validation("invalid webhook",
			api.Detail{Field: "payload", Message: "event carries no external reference"})
	}

	seen, err := s.store.ChargeByExternalRef(ctx, event.ExternalRef)
	if err != nil {
		return nil, err
	}
	if seen == nil {
		return nil, api.NotFound("no admission charge for external reference %s", event.ExternalRef)
	}
	if seen.Status != ChargePending {
		return seen, nil // replay — already settled
	}

	now := time.Now().UTC()
	var admitted *coterie.Coterie
	err = s.store.DB().WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		locked, err := NewStore(tx).ChargeByExternalRefForUpdate(ctx, event.ExternalRef)
		if err != nil {
			return err
		}
		if locked == nil || locked.Status != ChargePending {
			return nil // raced with a concurrent webhook — nothing to do
		}
		r, err := NewStore(tx).RequestByIDTx(ctx, tx, locked.JoinRequestID)
		if err != nil {
			return err
		}
		if r == nil {
			return api.NotFound("join request %s not found", locked.JoinRequestID)
		}
		if r.Status != RequestAwaitingPayment {
			return api.Conflict("join request %s is %s, not awaiting payment", r.ID, r.Status)
		}
		c, err := NewStore(tx).CoterieByID(ctx, locked.CoterieID)
		if err != nil {
			return err
		}
		if c == nil {
			return api.NotFound("coterie %s not found", locked.CoterieID)
		}
		if event.Succeeded {
			if _, err := s.coteries.AdmitUserInTx(ctx, tx, c, locked.UserID, coterie.RoleMember); err != nil {
				return err
			}
			locked.Status = ChargeSucceeded
			locked.ConfirmedAt = &now
			if _, err := NewStore(tx).SetStatusFromAwaiting(ctx, r.ID, RequestAccepted); err != nil {
				return err
			}
		} else {
			locked.Status = ChargeFailed
		}
		if err := tx.Save(locked).Error; err != nil {
			return err
		}
		if err := s.auditCharge(ctx, tx, locked.UserID, locked, c, locked.Status); err != nil {
			return err
		}
		admitted = c
		return nil
	})
	if err != nil {
		return nil, err
	}
	charge, err := s.store.ChargeByExternalRef(ctx, event.ExternalRef)
	if err != nil {
		return nil, err
	}
	if charge == nil {
		return nil, api.Internal()
	}
	if s.notifier != nil && admitted != nil {
		if event.Succeeded {
			s.notifier.Notify(ctx, charge.UserID, notification.TypeJoinDecided,
				"You joined "+admitted.Name,
				"Your payment was confirmed. Welcome!",
				"coterie", admitted.ID)
		} else {
			s.notifier.Notify(ctx, charge.UserID, notification.TypeAdmissionDue,
				"Admission payment failed",
				"The payment for joining "+admitted.Name+" did not go through. Start a new payment to complete joining.",
				"coterie", admitted.ID)
		}
	}
	return charge, nil
}

// Methods lists the registered adapters (the gate's chargeable names).
func (s *Service) Methods() []string {
	names := make([]string, 0, len(s.adapters))
	for name := range s.adapters {
		names = append(names, name)
	}
	return names
}

// auditCharge records the charge's ledger event.
func (s *Service) auditCharge(ctx context.Context, tx *gorm.DB, actorID string, charge *AdmissionCharge, c *coterie.Coterie, status string) error {
	return audit.Record(ctx, tx, audit.Input{
		ActorID:        actorID,
		Action:         audit.ActionPaymentRecorded,
		EntityType:     "admission_charge",
		EntityID:       charge.ID,
		SubscriptionID: c.SubscriptionID,
		CoterieID:      c.ID,
		After: map[string]any{
			"amount": charge.Amount, "currency": charge.Currency,
			"method": charge.Method, "join_request_id": charge.JoinRequestID,
			"status": status,
		},
	})
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
