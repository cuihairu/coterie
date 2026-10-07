package product

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

// Service carries the product business rules on top of the store.
type Service struct {
	store *Store
	log   *slog.Logger
}

// NewService builds a Service.
func NewService(db *gorm.DB) *Service {
	return &Service{store: NewStore(db), log: slog.Default()}
}

// Create validates and inserts a product. The referenced provider must
// exist; (provider_id, name) uniqueness maps to 409.
func (s *Service) Create(ctx context.Context, req CreateProductRequest) (*Product, error) {
	req.Name = strings.TrimSpace(req.Name)

	var details []api.Detail
	if req.ProviderID == "" {
		details = append(details, api.Detail{Field: "provider_id", Message: "required"})
	}
	if d := requiredLen(req.Name, "name", 200); d != nil {
		details = append(details, *d)
	}
	if req.Tier != nil && len(*req.Tier) > 64 {
		details = append(details, api.Detail{Field: "tier", Message: "must be at most 64 characters"})
	}
	if len(req.Metadata) > 0 && !json.Valid(req.Metadata) {
		details = append(details, api.Detail{Field: "metadata", Message: "must be valid JSON"})
	}
	if len(details) > 0 {
		return nil, api.Validation("invalid product", details...)
	}

	if ok, err := s.store.ProviderExists(ctx, req.ProviderID); err != nil {
		return nil, err
	} else if !ok {
		return nil, api.Validation("referenced resources missing",
			api.Detail{Field: "provider_id", Message: "provider not found"})
	}

	p := &Product{
		ID:         uuid.NewString(),
		ProviderID: req.ProviderID,
		Name:       req.Name,
		Tier:       req.Tier,
		Metadata:   database.JSONB("{}"),
	}
	if len(req.Metadata) > 0 {
		p.Metadata = database.JSONB(req.Metadata)
	}

	if err := s.store.Create(ctx, p); err != nil {
		if database.IsUniqueViolation(err) {
			return nil, api.Conflict("product name already exists for this provider")
		}
		return nil, err
	}
	return p, nil
}

// Get returns the product, mapping absence to 404.
func (s *Service) Get(ctx context.Context, id string) (*Product, error) {
	p, err := s.store.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	if p == nil {
		return nil, api.NotFound("product %s not found", id)
	}
	return p, nil
}

// List returns a page of products, optionally filtered by provider.
func (s *Service) List(ctx context.Context, providerID string, page api.Page) ([]Product, int64, error) {
	return s.store.List(ctx, providerID, page)
}

// Update patches the product's mutable fields.
func (s *Service) Update(ctx context.Context, id string, req UpdateProductRequest) (*Product, error) {
	p, err := s.store.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	if p == nil {
		return nil, api.NotFound("product %s not found", id)
	}

	var details []api.Detail
	if req.Name != nil {
		if d := requiredLen(*req.Name, "name", 200); d != nil {
			details = append(details, *d)
		}
	}
	if req.Tier != nil && len(*req.Tier) > 64 {
		details = append(details, api.Detail{Field: "tier", Message: "must be at most 64 characters"})
	}
	if req.Metadata != nil && !json.Valid(*req.Metadata) {
		details = append(details, api.Detail{Field: "metadata", Message: "must be valid JSON"})
	}
	if len(details) > 0 {
		return nil, api.Validation("invalid product", details...)
	}

	if req.Name != nil {
		p.Name = strings.TrimSpace(*req.Name)
	}
	if req.Tier != nil {
		p.Tier = req.Tier
	}
	if req.Metadata != nil {
		p.Metadata = database.JSONB(*req.Metadata)
	}

	if err := s.store.Update(ctx, p); err != nil {
		if database.IsUniqueViolation(err) {
			return nil, api.Conflict("product name already exists for this provider")
		}
		return nil, err
	}
	return p, nil
}

// Delete removes the product; products still referenced by
// subscriptions map to 409.
func (s *Service) Delete(ctx context.Context, id string) error {
	rows, err := s.store.Delete(ctx, id)
	if err != nil {
		if database.IsFKViolation(err) {
			return api.Conflict("product has subscriptions and cannot be deleted")
		}
		return err
	}
	if rows == 0 {
		return api.NotFound("product %s not found", id)
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
