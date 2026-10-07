package usage

import (
	"encoding/json"
	"time"

	"github.com/cuihairu/coterie/internal/database"
)

// UsageRecord is one metered consumption entry (design D9). Rows are
// append-only: corrections are negative records, never edits.
type UsageRecord struct {
	ID             string         `gorm:"column:id;primaryKey;type:uuid" json:"id"`
	SubscriptionID string         `gorm:"column:subscription_id;not null" json:"subscription_id"`
	MemberID       string         `gorm:"column:member_id;not null" json:"member_id"`
	SeatID         *string        `gorm:"column:seat_id" json:"seat_id,omitempty"`
	Amount         string         `gorm:"column:amount;not null" json:"amount"`
	Unit           string         `gorm:"column:unit;not null" json:"unit"`
	Metadata       database.JSONB `gorm:"column:metadata;not null" json:"metadata"`
	RecordedAt     time.Time      `gorm:"column:recorded_at;not null" json:"recorded_at"`
	CreatedAt      time.Time      `gorm:"column:created_at;not null" json:"created_at"`
}

// TableName aligns the model with the hand-written migration schema.
func (UsageRecord) TableName() string { return "usage_records" }

// CreateRecordRequest is the payload for POST
// /subscriptions/{id}/usage-records. Amount is a decimal string with up
// to 4 fraction digits; negative values are corrections. seat_id pins
// the attribution; when empty the record lands on the member's single
// quota seat if they hold exactly one (design D9).
type CreateRecordRequest struct {
	MemberID   string          `json:"member_id"`
	SeatID     string          `json:"seat_id"`
	Amount     string          `json:"amount"`
	Unit       string          `json:"unit"`
	RecordedAt string          `json:"recorded_at"`
	Metadata   json.RawMessage `json:"metadata"`
}

// ListFilters narrows GET /subscriptions/{id}/usage-records.
type ListFilters struct {
	MemberID string
	Unit     string
	From     *database.Date
	To       *database.Date
}
