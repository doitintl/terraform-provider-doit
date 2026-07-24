package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
)

// useRuleIdFromStateWhenConfigNull returns a plan modifier that preserves the rule ID
// from prior state when the config value is null (omitted from HCL configuration).
//
// For group allocations, in-line rules created with action = "create" receive API-generated
// IDs upon creation, stored in state as rules[*].id. When a user subsequently updates the
// group allocation in HCL without specifying rule IDs, this modifier carries over the
// existing rule IDs from prior state into the plan.
func useRuleIdFromStateWhenConfigNull() planmodifier.String {
	return useRuleIdFromStateWhenConfigNullModifier{}
}

type useRuleIdFromStateWhenConfigNullModifier struct{}

func (m useRuleIdFromStateWhenConfigNullModifier) Description(_ context.Context) string {
	return "Preserves existing rule ID from prior state when omitted from configuration."
}

func (m useRuleIdFromStateWhenConfigNullModifier) MarkdownDescription(ctx context.Context) string {
	return m.Description(ctx)
}

func (m useRuleIdFromStateWhenConfigNullModifier) PlanModifyString(_ context.Context, req planmodifier.StringRequest, resp *planmodifier.StringResponse) {
	// If the config value is null (user omitted rule ID in HCL),
	// and there IS a prior state value that is known, preserve the state rule ID.
	if req.ConfigValue.IsNull() && !req.StateValue.IsNull() && !req.StateValue.IsUnknown() {
		resp.PlanValue = req.StateValue
	}
}
