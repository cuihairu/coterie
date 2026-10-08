package audit

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"gorm.io/gorm"

	"github.com/cuihairu/coterie/internal/database"
)

// Input describes one audit entry. Before/After may be nil or any
// JSON-marshalable value — update-style actions carry only the changed
// fields. An empty ActorID records a system action.
type Input struct {
	ActorID        string
	Action         string
	EntityType     string
	EntityID       string
	SubscriptionID string
	CoterieID      string
	Before         any
	After          any
}

// Record appends one audit entry on db. Pass a *gorm.DB transaction to
// bind the entry to the mutation it describes; a plain handle records
// immediately. Marshal or insert failures return the error so the
// caller's transaction — and the mutation it carries — rolls back.
func Record(ctx context.Context, db *gorm.DB, in Input) error {
	e := Entry{
		Action:     in.Action,
		EntityType: in.EntityType,
		EntityID:   in.EntityID,
		CreatedAt:  time.Now().UTC(),
	}
	if in.ActorID != "" {
		id := in.ActorID
		e.ActorID = &id
	}
	if in.SubscriptionID != "" {
		id := in.SubscriptionID
		e.SubscriptionID = &id
	}
	if in.CoterieID != "" {
		id := in.CoterieID
		e.CoterieID = &id
	}
	var err error
	if e.Before, err = snapshot(in.Before); err != nil {
		return fmt.Errorf("audit before: %w", err)
	}
	if e.After, err = snapshot(in.After); err != nil {
		return fmt.Errorf("audit after: %w", err)
	}
	return db.WithContext(ctx).Create(&e).Error
}

// snapshot marshals a value into a JSONB pointer; nil stays nil so the
// column records SQL NULL rather than an empty object.
func snapshot(v any) (*database.JSONB, error) {
	if v == nil {
		return nil, nil
	}
	raw, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}
	jb := database.JSONB(raw)
	return &jb, nil
}
