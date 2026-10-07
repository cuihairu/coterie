package product

import (
	"encoding/json"
	"time"

	"github.com/cuihairu/coterie/internal/database"
)

// Product is a purchasable plan offered by a provider. It describes
// what can be bought — the actual price paid lives on the subscription.
type Product struct {
	ID         string         `gorm:"column:id;primaryKey;type:uuid" json:"id"`
	ProviderID string         `gorm:"column:provider_id;not null" json:"provider_id"`
	Name       string         `gorm:"column:name;not null" json:"name"`
	Tier       *string        `gorm:"column:tier" json:"tier,omitempty"`
	Metadata   database.JSONB `gorm:"column:metadata;not null" json:"metadata"`
	CreatedAt  time.Time      `gorm:"column:created_at" json:"created_at"`
	UpdatedAt  time.Time      `gorm:"column:updated_at" json:"updated_at"`
}

// TableName aligns the model with the hand-written migration schema.
func (Product) TableName() string { return "products" }

// CreateProductRequest is the payload for POST /products.
type CreateProductRequest struct {
	ProviderID string          `json:"provider_id"`
	Name       string          `json:"name"`
	Tier       *string         `json:"tier"`
	Metadata   json.RawMessage `json:"metadata"`
}

// UpdateProductRequest patches name/tier/metadata; nil fields are left
// unchanged.
type UpdateProductRequest struct {
	Name     *string          `json:"name"`
	Tier     *string          `json:"tier"`
	Metadata *json.RawMessage `json:"metadata"`
}
