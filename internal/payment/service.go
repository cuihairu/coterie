// Package payment implements the payment adapter seam (design §4.2):
// channels turn a receivable into a received — nothing more. The
// ledger is append-only, amounts always come from the contribution
// (invariant 5), and contribution status flips in the same transaction
// as the payment row.
package payment

import (
	"context"
	"fmt"
	"log/slog"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/cuihairu/coterie/internal/billing"
	"github.com/cuihairu/coterie/internal/notification"
	"github.com/cuihairu/coterie/internal/user"
	"github.com/cuihairu/coterie/pkg/api"
)

// maxExternalRefLen bounds the free-text channel reference.
const maxExternalRefLen = 200

// Service records payments through the configured adapters.
type Service struct {
	store    *Store
	adapters map[string]Adapter
	notifier *notification.Service
	log      *slog.Logger
}

// NewService builds a Service with the manual adapter registered.
// notifier may be nil in tests.
func NewService(db *gorm.DB, notifier *notification.Service) *Service {
	return NewServiceWithAdapters(db, notifier, Manual{})
}

// NewServiceWithAdapters builds a Service over an explicit adapter set
// (the seam for real channels later).
func NewServiceWithAdapters(db *gorm.DB, notifier *notification.Service, adapters ...Adapter) *Service {
	byName := make(map[string]Adapter, len(adapters))
	for _, a := range adapters {
		byName[a.Name()] = a
	}
	return &Service{
		store:    NewStore(db),
		adapters: byName,
		notifier: notifier,
		log:      slog.Default(),
	}
}

// Record charges the contribution's receivable through the requested
// adapter (default manual) and marks it paid. Allowed for the
// subscription owner and the paying member; settled or non-payable
// contributions 409.
func (s *Service) Record(ctx context.Context, actor *user.User, contributionID string, req RecordPaymentRequest) (*Payment, error) {
	c, err := s.store.ContributionForPayment(ctx, contributionID)
	if err != nil {
		return nil, err
	}
	if c == nil {
		return nil, api.NotFound("contribution %s not found", contributionID)
	}
	if actor.ID != c.OwnerUserID && actor.ID != c.MemberUserID {
		return nil, api.Forbidden("only the subscription owner or the paying member may record a payment")
	}
	if c.Status == billing.ContributionPaid {
		return nil, api.Conflict("contribution %s is already settled", c.ID)
	}
	if c.Status != billing.ContributionPending {
		return nil, api.Conflict("contribution %s is not payable while %s", c.ID, c.Status)
	}

	method := strings.TrimSpace(req.Method)
	if method == "" {
		method = Manual{}.Name()
	}
	adapter, ok := s.adapters[method]
	if !ok {
		return nil, api.Validation("invalid payment",
			api.Detail{Field: "method", Message: fmt.Sprintf("must be one of the registered payment methods (%s)", strings.Join(s.Methods(), ", "))})
	}
	externalRef := strings.TrimSpace(req.ExternalRef)
	if len(externalRef) > maxExternalRefLen {
		return nil, api.Validation("invalid payment",
			api.Detail{Field: "external_ref", Message: "must be at most 200 characters"})
	}

	paidAt := time.Now().UTC()
	p := &Payment{
		ID:             uuid.NewString(),
		ContributionID: c.ID,
		SubscriptionID: c.SubscriptionID,
		PayerUserID:    c.MemberUserID,
		Amount:         c.Amount,
		Currency:       c.Currency,
		Method:         method,
		Status:         StatusSucceeded,
		PaidAt:         paidAt,
	}
	if externalRef != "" {
		ref := externalRef
		p.ExternalRef = &ref
	}

	err = s.store.DB().WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// The adapter answers inside the settlement transaction: the
		// row only lands when the channel accepted the charge.
		receipt, err := adapter.Charge(ctx, Charge{
			ContributionID: c.ID,
			PayerUserID:    c.MemberUserID,
			Amount:         c.Amount,
			Currency:       c.Currency,
		})
		if err != nil {
			return err
		}
		if receipt.ExternalRef != "" && p.ExternalRef == nil {
			ref := receipt.ExternalRef
			p.ExternalRef = &ref
		}
		if err := tx.Create(p).Error; err != nil {
			return err
		}
		ok, err := s.store.SetContributionPaid(ctx, c.ID, paidAt)
		if err != nil {
			return err
		}
		if !ok {
			return api.Conflict("contribution %s is no longer payable", c.ID)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	if s.notifier != nil && actor.ID != c.OwnerUserID {
		s.notifier.Notify(ctx, c.OwnerUserID, notification.TypePaymentReceived,
			"Payment received",
			fmt.Sprintf("A payment of %s %s was recorded for your sharing circle.", p.Amount, p.Currency),
			"contribution", c.ID)
	}
	return p, nil
}

// Methods returns the registered adapter names — what POST payments
// may pass as method (manual is always present).
func (s *Service) Methods() []string {
	names := make([]string, 0, len(s.adapters))
	for name := range s.adapters {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// List returns the contribution's payments (any authenticated user,
// matching the billing read baseline).
func (s *Service) List(ctx context.Context, contributionID string, page api.Page) ([]Payment, int64, error) {
	if c, err := s.store.ContributionForPayment(ctx, contributionID); err != nil {
		return nil, 0, err
	} else if c == nil {
		return nil, 0, api.NotFound("contribution %s not found", contributionID)
	}
	return s.store.ListByContribution(ctx, contributionID, page)
}

// Get returns one payment.
func (s *Service) Get(ctx context.Context, id string) (*Payment, error) {
	p, err := s.store.PaymentByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if p == nil {
		return nil, api.NotFound("payment %s not found", id)
	}
	return p, nil
}
