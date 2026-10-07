package product

import (
	"context"
	"errors"

	"gorm.io/gorm"

	"github.com/cuihairu/coterie/internal/provider"
	"github.com/cuihairu/coterie/pkg/api"
)

// Store holds the GORM queries for products.
type Store struct {
	db *gorm.DB
}

// NewStore builds a Store.
func NewStore(db *gorm.DB) *Store { return &Store{db: db} }

// Create inserts p.
func (s *Store) Create(ctx context.Context, p *Product) error {
	return s.db.WithContext(ctx).Create(p).Error
}

// Get returns the product with id, or nil when absent.
func (s *Store) Get(ctx context.Context, id string) (*Product, error) {
	var p Product
	err := s.db.WithContext(ctx).First(&p, "id = ?", id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &p, nil
}

// ProviderExists reports whether the provider exists; used to validate
// the create payload without a cross-aggregate write.
func (s *Store) ProviderExists(ctx context.Context, id string) (bool, error) {
	var n int64
	err := s.db.WithContext(ctx).Model(&provider.Provider{}).
		Where("id = ?", id).
		Count(&n).Error
	return n > 0, err
}

// List returns products newest first, optionally filtered by provider.
func (s *Store) List(ctx context.Context, providerID string, page api.Page) ([]Product, int64, error) {
	q := s.db.WithContext(ctx).Model(&Product{})
	if providerID != "" {
		q = q.Where("provider_id = ?", providerID)
	}
	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var products []Product
	err := q.Order("created_at DESC, id DESC").
		Limit(page.Limit).
		Offset(page.Offset).
		Find(&products).Error
	return products, total, err
}

// Update persists all fields of p.
func (s *Store) Update(ctx context.Context, p *Product) error {
	return s.db.WithContext(ctx).Save(p).Error
}

// Delete removes the product and reports whether a row was removed.
func (s *Store) Delete(ctx context.Context, id string) (int64, error) {
	res := s.db.WithContext(ctx).Delete(&Product{}, "id = ?", id)
	return res.RowsAffected, res.Error
}
