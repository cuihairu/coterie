package report

import (
	"context"
	"errors"
	"time"

	"gorm.io/gorm"

	"github.com/cuihairu/coterie/pkg/api"
)

// Store holds the GORM queries for the report ledger.
type Store struct {
	db *gorm.DB
}

// NewStore builds a Store.
func NewStore(db *gorm.DB) *Store { return &Store{db: db} }

// DB exposes the raw handle for transaction scoping.
func (s *Store) DB() *gorm.DB { return s.db }

// listedCoterie is the slice of a coterie the report flow needs.
type listedCoterie struct {
	ID      string
	Name    string
	Listing string
}

// ListedCoterie returns the coterie when it exists, or nil — the
// service decides whether the listing makes it reportable.
func (s *Store) ListedCoterie(ctx context.Context, id string) (*listedCoterie, error) {
	var lc listedCoterie
	err := s.db.WithContext(ctx).Table("coteries").
		Select("id, name, listing").Where("id = ?", id).Scan(&lc).Error
	if err != nil {
		return nil, err
	}
	if lc.ID == "" {
		return nil, nil
	}
	return &lc, nil
}

// Create inserts the report.
func (s *Store) Create(ctx context.Context, r *Report) error {
	return s.db.WithContext(ctx).Create(r).Error
}

// ByID returns the report with id, or nil when absent.
func (s *Store) ByID(ctx context.Context, id string) (*Report, error) {
	var r Report
	err := s.db.WithContext(ctx).First(&r, "id = ?", id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &r, nil
}

// OpenByReporterAndCoterie returns the user's open report on the
// coterie, or nil.
func (s *Store) OpenByReporterAndCoterie(ctx context.Context, reporterID, coterieID string) (*Report, error) {
	var r Report
	err := s.db.WithContext(ctx).
		First(&r, "reporter_id = ? AND coterie_id = ? AND status = ?",
			reporterID, coterieID, StatusOpen).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &r, nil
}

// viewSelect is the joined shape the inbox and single-report reads
// share.
const viewSelect = `r.id, r.coterie_id, r.reporter_id, r.reason, r.status,
	r.resolution_note, r.decided_at, r.created_at,
	c.name AS coterie_name, c.listing AS coterie_listing,
	u.username AS reporter_username, u.email AS reporter_email`

// List returns reports newest first, optionally filtered by status.
func (s *Store) List(ctx context.Context, status string, page api.Page) ([]ReportView, int64, error) {
	q := s.db.WithContext(ctx).Table("reports r").
		Joins("JOIN coteries c ON c.id = r.coterie_id").
		Joins("JOIN users u ON u.id = r.reporter_id")
	if status != "" {
		q = q.Where("r.status = ?", status)
	}
	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var items []ReportView
	err := q.Select(viewSelect).
		Order("r.created_at DESC, r.id DESC").
		Limit(page.Limit).
		Offset(page.Offset).
		Scan(&items).Error
	return items, total, err
}

// View returns one report with identities, or nil when absent.
func (s *Store) View(ctx context.Context, id string) (*ReportView, error) {
	var v ReportView
	err := s.db.WithContext(ctx).Table("reports r").
		Select(viewSelect).
		Joins("JOIN coteries c ON c.id = r.coterie_id").
		Joins("JOIN users u ON u.id = r.reporter_id").
		Where("r.id = ?", id).
		Scan(&v).Error
	if err != nil {
		return nil, err
	}
	if v.ID == "" {
		return nil, nil
	}
	return &v, nil
}

// Decide flips an open report to the given status with the optional
// note and reports whether the report was still open.
func (s *Store) Decide(ctx context.Context, id, status string, note *string, decidedAt time.Time) (bool, error) {
	res := s.db.WithContext(ctx).Model(&Report{}).
		Where("id = ? AND status = ?", id, StatusOpen).
		Updates(map[string]any{
			"status":          status,
			"resolution_note": note,
			"decided_at":      decidedAt,
		})
	return res.RowsAffected > 0, res.Error
}
