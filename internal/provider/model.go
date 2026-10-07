package provider

import (
	"encoding/json"
	"time"

	"github.com/cuihairu/coterie/internal/database"
)

// Provider is a digital service vendor in the catalog. It holds
// descriptive metadata only — plans and pricing belong to products and
// subscriptions, never here.
type Provider struct {
	ID        string         `gorm:"column:id;primaryKey;type:uuid" json:"id"`
	Slug      string         `gorm:"column:slug;not null" json:"slug"`
	Name      string         `gorm:"column:name;not null" json:"name"`
	Category  string         `gorm:"column:category;not null" json:"category"`
	Metadata  database.JSONB `gorm:"column:metadata;not null" json:"metadata"`
	CreatedAt time.Time      `gorm:"column:created_at" json:"created_at"`
	UpdatedAt time.Time      `gorm:"column:updated_at" json:"updated_at"`
}

// TableName aligns the model with the hand-written migration schema.
func (Provider) TableName() string { return "providers" }

// CreateProviderRequest is the payload for POST /providers.
type CreateProviderRequest struct {
	Slug     string          `json:"slug"`
	Name     string          `json:"name"`
	Category string          `json:"category"`
	Metadata json.RawMessage `json:"metadata"`
}

// UpdateProviderRequest patches name/category/metadata; nil fields are
// left unchanged.
type UpdateProviderRequest struct {
	Name     *string          `json:"name"`
	Category *string          `json:"category"`
	Metadata *json.RawMessage `json:"metadata"`
}
