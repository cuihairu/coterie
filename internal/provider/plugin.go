// Provider plugin face (design §5.1): a plugin binds to one provider
// slug and may enable optional capabilities — policy validation when a
// subscription is created or edited, admission checks when someone
// joins a circle on that provider. The core platform depends on none of
// them: with an empty registry everything behaves as Generic (§5.3).
package provider

import (
	"context"
	"encoding/json"
)

// Plugin is the extension point implemented per provider slug.
type Plugin interface {
	// Slug is the provider this plugin covers.
	Slug() string
}

// PolicyValidator is the Validation facet: checks a subscription's
// sharing policy beyond the core mode check. Return an api error
// (typically 422) to reject the policy.
type PolicyValidator interface {
	ValidatePolicy(ctx context.Context, policy json.RawMessage) error
}

// GuardInput carries the admission decision context to a plugin.
type GuardInput struct {
	UserID      string          // candidate member
	Role        string          // role they would join with
	MemberCount int             // active members before the admission
	Policy      json.RawMessage // subscription sharing policy
}

// AdmissionGuard is the Sharing facet: consulted before a member is
// admitted to a coterie on this provider's subscription. Return an api
// error (typically 409) to refuse the admission.
type AdmissionGuard interface {
	CheckAdmission(ctx context.Context, in GuardInput) error
}

// Registry resolves provider slugs to plugins. A nil Registry behaves
// like an empty one, so callers need no nil checks.
type Registry struct {
	bySlug map[string]Plugin
}

// NewRegistry indexes the given plugins by slug (later wins on
// duplicates).
func NewRegistry(plugins ...Plugin) *Registry {
	r := &Registry{bySlug: make(map[string]Plugin, len(plugins))}
	for _, p := range plugins {
		r.bySlug[p.Slug()] = p
	}
	return r
}

// For returns the plugin for the slug, or nil when none is registered.
func (r *Registry) For(slug string) Plugin {
	if r == nil {
		return nil
	}
	return r.bySlug[slug]
}
