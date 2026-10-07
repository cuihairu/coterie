package provider

import (
	"context"
	"encoding/json"
	"log/slog"
	"strconv"
	"strings"

	"gorm.io/gorm"

	"github.com/cuihairu/coterie/internal/database"
	"github.com/cuihairu/coterie/pkg/api"

	"github.com/google/uuid"
)

// Service carries the provider business rules on top of the store.
type Service struct {
	store *Store
	log   *slog.Logger
}

// NewService builds a Service.
func NewService(db *gorm.DB) *Service {
	return &Service{store: NewStore(db), log: slog.Default()}
}

// Create validates and inserts a provider. Slug uniqueness is
// case-insensitive (enforced by the database); clashes map to 409.
func (s *Service) Create(ctx context.Context, req CreateProviderRequest) (*Provider, error) {
	req.Slug = strings.TrimSpace(req.Slug)

	var details []api.Detail
	if req.Slug == "" {
		details = append(details, api.Detail{Field: "slug", Message: "required"})
	} else if len(req.Slug) > 100 {
		details = append(details, api.Detail{Field: "slug", Message: "must be at most 100 characters"})
	}
	if d := requiredLen(req.Name, "name", 200); d != nil {
		details = append(details, *d)
	}
	if d := requiredLen(req.Category, "category", 64); d != nil {
		details = append(details, *d)
	}
	if len(req.Metadata) > 0 && !json.Valid(req.Metadata) {
		details = append(details, api.Detail{Field: "metadata", Message: "must be valid JSON"})
	}
	if len(details) > 0 {
		return nil, api.Validation("invalid provider", details...)
	}

	p := &Provider{
		ID:       uuid.NewString(),
		Slug:     req.Slug,
		Name:     req.Name,
		Category: req.Category,
		Metadata: database.JSONB("{}"),
	}
	if len(req.Metadata) > 0 {
		p.Metadata = database.JSONB(req.Metadata)
	}

	if err := s.store.Create(ctx, p); err != nil {
		if database.IsUniqueViolation(err) {
			return nil, api.Conflict("provider slug already exists")
		}
		return nil, err
	}
	return p, nil
}

// Get returns the provider, mapping absence to 404.
func (s *Service) Get(ctx context.Context, id string) (*Provider, error) {
	p, err := s.store.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	if p == nil {
		return nil, api.NotFound("provider %s not found", id)
	}
	return p, nil
}

// List returns a page of providers, optionally filtered by category.
func (s *Service) List(ctx context.Context, category string, page api.Page) ([]Provider, int64, error) {
	return s.store.List(ctx, category, page)
}

// Update patches the provider's mutable fields.
func (s *Service) Update(ctx context.Context, id string, req UpdateProviderRequest) (*Provider, error) {
	p, err := s.store.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	if p == nil {
		return nil, api.NotFound("provider %s not found", id)
	}

	var details []api.Detail
	if req.Name != nil {
		if d := requiredLen(*req.Name, "name", 200); d != nil {
			details = append(details, *d)
		}
	}
	if req.Category != nil {
		if d := requiredLen(*req.Category, "category", 64); d != nil {
			details = append(details, *d)
		}
	}
	if req.Metadata != nil && !json.Valid(*req.Metadata) {
		details = append(details, api.Detail{Field: "metadata", Message: "must be valid JSON"})
	}
	if len(details) > 0 {
		return nil, api.Validation("invalid provider", details...)
	}

	if req.Name != nil {
		p.Name = *req.Name
	}
	if req.Category != nil {
		p.Category = *req.Category
	}
	if req.Metadata != nil {
		p.Metadata = database.JSONB(*req.Metadata)
	}

	if err := s.store.Update(ctx, p); err != nil {
		if database.IsUniqueViolation(err) {
			return nil, api.Conflict("provider slug already exists")
		}
		return nil, err
	}
	return p, nil
}

// Delete removes the provider; providers still referenced by products
// map to 409.
func (s *Service) Delete(ctx context.Context, id string) error {
	rows, err := s.store.Delete(ctx, id)
	if err != nil {
		if database.IsFKViolation(err) {
			return api.Conflict("provider has products and cannot be deleted")
		}
		return err
	}
	if rows == 0 {
		return api.NotFound("provider %s not found", id)
	}
	return nil
}

func requiredLen(value, field string, max int) *api.Detail {
	v := strings.TrimSpace(value)
	if v == "" {
		return &api.Detail{Field: field, Message: "required"}
	}
	if len(v) > max {
		return &api.Detail{Field: field, Message: "must be at most " + strconv.Itoa(max) + " characters"}
	}
	return nil
}
