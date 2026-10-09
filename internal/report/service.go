package report

import (
	"context"
	"log/slog"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/cuihairu/coterie/internal/audit"
	"github.com/cuihairu/coterie/internal/user"
	"github.com/cuihairu/coterie/pkg/api"
)

// Service carries the report flow (design §6.6): users flag publicly
// listed coteries, the platform admin decides. Neither side notifies —
// reports are the operator's business, not the parties' inboxes.
type Service struct {
	store *Store
	log   *slog.Logger
}

// NewService builds a Service.
func NewService(db *gorm.DB) *Service {
	return &Service{store: NewStore(db), log: slog.Default()}
}

// Create files a report on a publicly listed coterie. Private and
// unknown circles both 404 — existence stays hidden like everywhere
// else; one open report per user and coterie; nobody is notified.
func (s *Service) Create(ctx context.Context, actor *user.User, coterieID string, req CreateReportRequest) (*ReportView, error) {
	reason := strings.TrimSpace(req.Reason)
	if reason == "" {
		return nil, api.Validation("invalid report",
			api.Detail{Field: "reason", Message: "is required"})
	}
	if utf8.RuneCountInString(reason) > maxReasonLen {
		return nil, api.Validation("invalid report",
			api.Detail{Field: "reason", Message: "must be at most 1000 characters"})
	}
	c, err := s.store.ListedCoterie(ctx, coterieID)
	if err != nil {
		return nil, err
	}
	if c == nil || c.Listing != "public" {
		return nil, api.NotFound("coterie %s not found", coterieID)
	}
	if existing, err := s.store.OpenByReporterAndCoterie(ctx, actor.ID, c.ID); err != nil {
		return nil, err
	} else if existing != nil {
		return nil, api.Conflict("you already have an open report for this coterie")
	}
	r := &Report{
		ID:         uuid.NewString(),
		CoterieID:  c.ID,
		ReporterID: actor.ID,
		Reason:     reason,
		Status:     StatusOpen,
		CreatedAt:  time.Now().UTC(),
	}
	if err := s.store.Create(ctx, r); err != nil {
		return nil, err
	}
	return s.store.View(ctx, r.ID)
}

// List is the admin inbox, newest first; an empty status lists every
// state. The admin gate itself lives on the route middleware.
func (s *Service) List(ctx context.Context, status string, page api.Page) ([]ReportView, int64, error) {
	switch status {
	case "", StatusOpen, StatusResolved, StatusDismissed:
	default:
		return nil, 0, api.Validation("invalid report filter",
			api.Detail{Field: "status", Message: "must be open, resolved, or dismissed"})
	}
	return s.store.List(ctx, status, page)
}

// Decide closes an open report as resolved or dismissed. The note is
// optional; the decision lands in the audit log and nowhere else — the
// reporter is deliberately not told (design §6.6).
func (s *Service) Decide(ctx context.Context, admin *user.User, id, status string, req DecideReportRequest) (*ReportView, error) {
	if status != StatusResolved && status != StatusDismissed {
		return nil, api.Validation("invalid report decision",
			api.Detail{Field: "status", Message: "must be resolved or dismissed"})
	}
	note := strings.TrimSpace(req.Note)
	if utf8.RuneCountInString(note) > maxNoteLen {
		return nil, api.Validation("invalid report decision",
			api.Detail{Field: "note", Message: "must be at most 1000 characters"})
	}
	r, err := s.store.ByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if r == nil {
		return nil, api.NotFound("report %s not found", id)
	}
	decided := time.Now().UTC()
	var notePtr *string
	if note != "" {
		notePtr = &note
	}
	err = s.store.DB().WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		st := NewStore(tx)
		ok, err := st.Decide(ctx, id, status, notePtr, decided)
		if err != nil {
			return err
		}
		if !ok {
			return api.Conflict("report %s is already decided", id)
		}
		return audit.Record(ctx, tx, audit.Input{
			ActorID:    admin.ID,
			Action:     audit.ActionReportDecided,
			EntityType: "report",
			EntityID:   id,
			CoterieID:  r.CoterieID,
			Before:     map[string]any{"status": StatusOpen},
			After:      map[string]any{"status": status, "resolution_note": note},
		})
	})
	if err != nil {
		return nil, err
	}
	return s.store.View(ctx, id)
}
