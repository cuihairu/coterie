package claude

import (
	"context"
	"encoding/json"

	"github.com/cuihairu/coterie/pkg/api"
)

// planPolicy is the slice of sharing_policy this plugin owns. The core
// has already checked mode; everything here is Claude-specific.
type planPolicy struct {
	Region string `json:"region"`
	Plan   string `json:"plan"`
}

// ValidatePolicy enforces the residency region on every policy and
// checks the plan enum. Without this plugin's region the subscription
// is rejected at create and update time (422).
func (Plugin) ValidatePolicy(_ context.Context, raw json.RawMessage) error {
	var sp planPolicy
	if err := json.Unmarshal(raw, &sp); err != nil {
		return api.Validation("policy rejected by provider",
			api.Detail{Field: "sharing_policy", Message: "must be a JSON object"})
	}
	switch sp.Region {
	case RegionUS, RegionEU:
	default:
		return api.Validation("policy rejected by provider",
			api.Detail{Field: "sharing_policy", Message: "region is required and must be us or eu (data residency)"})
	}
	switch sp.Plan {
	case "", PlanPro, PlanMax:
	default:
		return api.Validation("policy rejected by provider",
			api.Detail{Field: "sharing_policy", Message: "plan must be pro or max"})
	}
	return nil
}

// planOf returns the policy's plan, applying the default.
func planOf(raw json.RawMessage) string {
	var sp planPolicy
	_ = json.Unmarshal(raw, &sp)
	if sp.Plan == "" {
		return PlanDefault
	}
	return sp.Plan
}
