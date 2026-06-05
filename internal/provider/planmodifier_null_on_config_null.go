package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
)

// useNullForUnknownWhenConfigNull returns a plan modifier that proposes null when
// the config value is null (either explicitly set or omitted) and a prior state
// value exists.
//
// For Optional+Computed attributes, Terraform Core copies the prior state value
// into the ProposedNewState when the config value is null. The framework then
// skips its MarkComputedNilsAsUnknown phase because ProposedNewState already
// equals PriorState. The net effect is that the plan silently preserves the old
// value, making it impossible for users to clear the attribute.
//
// This modifier overrides that behavior: when the config is null and the state
// holds a value, it sets the planned value to null so the provider can omit or
// clear the field in the API request.
//
// Note: Terraform does not distinguish between "attribute omitted" and
// "attribute explicitly set to null" — both result in a null config value.
// Apply this modifier only to attributes where the API treats an absent field
// as "clear" (e.g. full-replacement PUT semantics) and users are not expected
// to rely on implicit preservation of server-computed defaults.
//
// See: https://github.com/hashicorp/terraform-plugin-framework/issues/603
func useNullForUnknownWhenConfigNull() planmodifier.String {
	return useNullForUnknownWhenConfigNullModifier{}
}

type useNullForUnknownWhenConfigNullModifier struct{}

func (m useNullForUnknownWhenConfigNullModifier) Description(_ context.Context) string {
	return "Proposes null when the config value is null (omitted or explicitly set) " +
		"and a prior state value exists, allowing the attribute to be cleared."
}

func (m useNullForUnknownWhenConfigNullModifier) MarkdownDescription(ctx context.Context) string {
	return m.Description(ctx)
}

func (m useNullForUnknownWhenConfigNullModifier) PlanModifyString(_ context.Context, req planmodifier.StringRequest, resp *planmodifier.StringResponse) {
	// If the config value is null (user explicitly set null or omitted the field),
	// and there IS a prior state value, propose null to allow clearing.
	// This overrides the default Optional+Computed behavior where Terraform Core
	// copies the prior state value into the ProposedNewState, making clearing
	// impossible.
	if req.ConfigValue.IsNull() && !req.StateValue.IsNull() {
		resp.PlanValue = req.ConfigValue // null
	}
}
