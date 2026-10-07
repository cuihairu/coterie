package provider

import (
	"context"
	"errors"

	"gorm.io/gorm"

	"github.com/cuihairu/coterie/pkg/api"
)

// Store holds the GORM queries for providers.
type Store struct {
	db *gorm.DB
}

// NewStore builds a Store.
func NewStore(db *gorm.DB) *Store { return &Store{db: db} }

// Create inserts p.
func (s *Store) Create(ctx context.Context, p *Provider) error {
	return s.db.WithContext(ctx).Create(p).Error
}

// Get returns the provider with id, or nil when absent.
func (s *Store) Get(ctx context.Context, id string) (*Provider, error) {
	var p Provider
	err := s.db.WithContext(ctx).First(&p, "id = ?", id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &p, nil
}

// List returns providers newest first, optionally filtered by category.
func (s *Store) List(ctx context.Context, category string, page api.Page) ([]Provider, int64, error) {
	q := s.db.WithContext(ctx).Model(&Provider{})
	if category != "" {
		q = q.Where("category = ?", category)
	}
	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var providers []Provider
	err := q.Order("created_at DESC, id DESC").
		Limit(page.Limit).
		Offset(page.Offset).
		Find(&providers).Error
	return providers, total, err
}

// Update persists all fields of p.
func (s *Store) Update(ctx context.Context, p *Provider) error {
	return s.db.WithContext(ctx).Save(p).Error
}

// Delete removes the provider and reports whether a row was removed.
func (s *Store) Delete(ctx context.Context, id string) (int64, error) {
	res := s.db.WithContext(ctx).Delete(&Provider{}, "id = ?", id)
	return res.RowsAffected, res.Error
}
