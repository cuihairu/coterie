package dispute

import (
	"context"
	"log/slog"
	"strings"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/cuihairu/coterie/internal/billing"
	"github.com/cuihairu/coterie/internal/database"
	"github.com/cuihairu/coterie/internal/notification"
	"github.com/cuihairu/coterie/internal/user"
	"github.com/cuihairu/coterie/pkg/api"
)

// Reason and evidence bounds (mirrored by CHECK constraints).
const (
	maxReasonLen   = 1000
	maxEvidenceLen = 2000
	maxNoteLen     = 1000
)

// Service raises and decides disputes over the contribution ledger.
type Service struct {
	store    *Store
	notifier *notification.Service
	log      *slog.Logger
}

// NewService builds a Service. notifier may be nil in tests.
func NewService(db *gorm.DB, notifier *notification.Service) *Service {
	return &Service{store: NewStore(db), notifier: notifier, log: slog.Default()}
}

// Raise opens a dispute on a contribution. The owner and active coterie
// members may raise; one open dispute per contribution; cancelled
// contributions (nothing owed, nothing to challenge) are not disputable.
func (s *Service) Raise(ctx context.Context, actor *user.User, contributionID string, req RaiseRequest) (*Dispute, error) {
	cc, err := s.store.ContributionContext(ctx, contributionID)
	if err != nil {
		return nil, err
	}
	if cc == nil {
		return nil, api.NotFound("contribution %s not found", contributionID)
	}
	if actor.ID != cc.OwnerUserID && actor.ID != cc.MemberUserID {
		ok, err := s.store.IsCoterieMember(ctx, cc.SubscriptionID, actor.ID)
		if err != nil {
			return nil, err
		}
		if !ok {
			return nil, api.Forbidden("only the subscription's owner or members may raise a dispute")
		}
	}
	if cc.Status == billing.ContributionCancelled {
		return nil, api.Conflict("a cancelled contribution cannot be disputed")
	}
	if open, err := s.store.OpenByContribution(ctx, contributionID); err != nil {
		return nil, err
	} else if open != nil {
		return nil, api.Conflict("contribution %s already has an open dispute", contributionID)
	}

	reason := strings.TrimSpace(req.Reason)
	if reason == "" || len(reason) > maxReasonLen {
		return nil, api.Validation("invalid dispute",
			api.Detail{Field: "reason", Message: "is required (1-1000 characters)"})
	}
	evidence := strings.TrimSpace(req.Evidence)

	d := &Dispute{
		ID:             uuid.NewString(),
		ContributionID: cc.ContributionID,
		SubscriptionID: cc.SubscriptionID,
		RaisedBy:       actor.ID,
		Reason:         reason,
		Status:         StatusOpen,
		CreatedAt:      time.Now().UTC(),
	}
	if evidence != "" {
		if len(evidence) > maxEvidenceLen {
			return nil, api.Validation("invalid dispute",
				api.Detail{Field: "evidence", Message: "must be at most 2000 characters"})
		}
		d.Evidence = &evidence
	}
	if err := s.store.Create(ctx, d); err != nil {
		if database.IsUniqueViolation(err) {
			return nil, api.Conflict("contribution %s already has an open dispute", contributionID)
		}
		if database.IsCheckViolation(err, "disputes_reason_len", "disputes_evidence_len") {
			return nil, api.Validation("invalid dispute",
				api.Detail{Field: "reason", Message: "is required (1-1000 characters)"})
		}
		return nil, err
	}

	if s.notifier != nil && actor.ID != cc.OwnerUserID {
		s.notifier.Notify(ctx, cc.OwnerUserID, notification.TypeDisputeOpened,
			"Dispute opened",
			"A member disputed their share: "+truncate(reason, 120),
			"dispute", d.ID)
	}
	return d, nil
}

// Decide resolves or rejects an open dispute. Owner-only; deciding is
// one-way and never mutates the contribution — the owner adjusts
// settlement separately if the decision calls for it.
func (s *Service) Decide(ctx context.Context, actor *user.User, id string, req DecideRequest) (*Dispute, error) {
	d, err := s.store.ByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if d == nil {
		return nil, api.NotFound("dispute %s not found", id)
	}
	cc, err := s.store.ContributionContext(ctx, d.ContributionID)
	if err != nil {
		return nil, err
	}
	if cc == nil || actor.ID != cc.OwnerUserID {
		return nil, api.Forbidden("only the subscription owner may decide a dispute")
	}

	var decision string
	switch req.Decision {
	case DecideResolved, DecideRejected:
		decision = req.Decision
	default:
		return nil, api.Validation("invalid decision",
			api.Detail{Field: "decision", Message: "must be resolved or rejected"})
	}
	note := strings.TrimSpace(req.Note)
	if len(note) > maxNoteLen {
		return nil, api.Validation("invalid decision",
			api.Detail{Field: "note", Message: "must be at most 1000 characters"})
	}

	now := time.Now().UTC()
	changed, err := s.store.Decide(ctx, d.ID, decision, note, actor.ID, now)
	if err != nil {
		return nil, err
	}
	if !changed {
		return nil, api.Conflict("dispute %s is already decided", d.ID)
	}

	if s.notifier != nil && d.RaisedBy != actor.ID {
		s.notifier.Notify(ctx, d.RaisedBy, notification.TypeDisputeDecided,
			"Dispute "+decision,
			"Your dispute was "+decision+" by the subscription owner.",
			"dispute", d.ID)
	}
	return s.store.ByID(ctx, d.ID)
}

// Get returns one dispute to the owner, the raiser, or the disputed
// contribution's paying member.
func (s *Service) Get(ctx context.Context, actor *user.User, id string) (*Dispute, error) {
	d, err := s.store.ByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if d == nil {
		return nil, api.NotFound("dispute %s not found", id)
	}
	cc, err := s.store.ContributionContext(ctx, d.ContributionID)
	if err != nil {
		return nil, err
	}
	if cc == nil {
		return nil, api.NotFound("dispute %s not found", id)
	}
	if actor.ID != cc.OwnerUserID && actor.ID != d.RaisedBy && actor.ID != cc.MemberUserID {
		return nil, api.Forbidden("only the owner, the raiser, or the paying member may view a dispute")
	}
	return d, nil
}

// List returns the subscription's disputes (owner-only).
func (s *Service) List(ctx context.Context, actor *user.User, subscriptionID, status string, page api.Page) ([]Dispute, int64, error) {
	var ownerID string
	err := s.store.DB().WithContext(ctx).
		Table("subscriptions").
		Select("owner_user_id").
		Where("id = ?", subscriptionID).
		Scan(&ownerID).Error
	if err != nil {
		return nil, 0, err
	}
	if ownerID == "" {
		return nil, 0, api.NotFound("subscription %s not found", subscriptionID)
	}
	if actor.ID != ownerID {
		return nil, 0, api.Forbidden("only the subscription owner may list disputes")
	}
	if status != "" && status != StatusOpen && status != StatusResolved && status != StatusRejected {
		return nil, 0, api.Validation("invalid filter",
			api.Detail{Field: "status", Message: "must be open, resolved, or rejected"})
	}
	return s.store.ListBySubscription(ctx, subscriptionID, status, page)
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}
