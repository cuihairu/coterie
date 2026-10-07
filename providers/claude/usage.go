package claude

import (
	"context"
	"math"
	"strconv"
	"strings"

	"github.com/cuihairu/coterie/internal/provider"
	"github.com/cuihairu/coterie/pkg/api"
)

// ValidateUsage restricts metering to the units this provider bills in
// and bounds a single record. Negative amounts pass through — they are
// the ledger's correction mechanism — as long as their magnitude stays
// within the same bounds.
func (Plugin) ValidateUsage(_ context.Context, in provider.UsageInput) error {
	limit := int64(0)
	switch strings.ToLower(in.Unit) {
	case UnitTokens:
		limit = MaxTokensPerRecord
	case UnitRequests:
		limit = MaxRequestsPerRecord
	default:
		return api.Validation("invalid usage record",
			api.Detail{Field: "unit", Message: "claude usage must be recorded in tokens or requests"})
	}
	amount, err := strconv.ParseFloat(strings.TrimSpace(in.Amount), 64)
	if err != nil || math.IsNaN(amount) || math.IsInf(amount, 0) {
		return api.Validation("invalid usage record",
			api.Detail{Field: "amount", Message: "must be a decimal number"})
	}
	if math.Abs(amount) > float64(limit) {
		return api.Validation("invalid usage record",
			api.Detail{Field: "amount", Message: "a single record may not exceed " + strconv.FormatInt(limit, 10) + " " + in.Unit})
	}
	return nil
}
