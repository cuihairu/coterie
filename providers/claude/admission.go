package claude

import (
	"context"

	"github.com/cuihairu/coterie/internal/provider"
	"github.com/cuihairu/coterie/pkg/api"
)

// CheckAdmission applies the plan's sharing contract before a member
// joins: pro is personal and takes nobody, max circles hold at most
// MaxMembers members including the owner.
func (Plugin) CheckAdmission(_ context.Context, in provider.GuardInput) error {
	if planOf(in.Policy) == PlanPro {
		return api.Conflict("claude pro is a personal plan; sharing requires the max plan")
	}
	if in.MemberCount >= MaxMembers {
		return api.Conflict("claude max circles hold at most %d members", MaxMembers)
	}
	return nil
}
