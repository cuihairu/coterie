// Package claude is the first real provider plugin (design §5.2): a
// concrete implementation of the D12 plugin face for the seeded Claude
// catalog entry. It encodes the provider's sharing contract in the
// three facets —
//
//   - policy: a data-residency region (us | eu) is required; an
//     optional plan (pro | max, default max) states which Claude plan
//     the subscription is on;
//   - admission: pro is a personal plan and refuses every additional
//     member; max circles hold at most maxMembers;
//   - usage: metering is only accepted in tokens or requests, bounded
//     per record (corrections stay negative, as the core ledger allows).
//
// The plugin carries no state and never talks to the provider's
// network; the limits below are the sample's documented contract.
package claude

import (
	"github.com/cuihairu/coterie/internal/provider"
)

// Sharing contract enforced by this plugin.
const (
	Slug = "claude"

	// Region and plan enumerations for sharing_policy.
	RegionUS = "us"
	RegionEU = "eu"
	PlanPro  = "pro"
	PlanMax  = "max"
	// PlanDefault applies when the policy does not name a plan.
	PlanDefault = PlanMax

	// MaxMembers is the circle cap for the shareable plan.
	MaxMembers = 5
	// MaxTokensPerRecord bounds a single metered usage record.
	MaxTokensPerRecord = 10_000_000
	// MaxRequestsPerRecord bounds a single metered usage record.
	MaxRequestsPerRecord = 100_000

	UnitTokens   = "tokens"
	UnitRequests = "requests"
)

// Plugin implements the D12 face for slug "claude".
type Plugin struct{}

// Slug is the provider this plugin covers — the seeded catalog entry.
func (Plugin) Slug() string { return Slug }

// Compile-time proof the plugin implements every facet of the face.
var (
	_ provider.PolicyValidator = Plugin{}
	_ provider.AdmissionGuard  = Plugin{}
	_ provider.UsageValidator  = Plugin{}
)
