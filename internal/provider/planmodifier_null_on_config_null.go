package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
)

// useNullForUnknownWhenConfigNull returns a plan modifier that proposes null when
// the config value is explicitly null. This is needed for Optional+Computed string
// attributes where the user should be able to clear the value by setting it to null
// in their config. Without this modifier, Terraform preserves the prior state value
// when config is null, making it impossible to clear the attribute.
//
// See: https://github.com/hashicorp/terraform-plugin-framework/issues/603
func useNullForUnknownWhenConfigNull() planmodifier.String {
	return useNullForUnknownWhenConfigNullModifier{}
}

type useNullForUnknownWhenConfigNullModifier struct{}

func (m useNullForUnknownWhenConfigNullModifier) Description(_ context.Context) string {
	return "Proposes null when the config value is explicitly null, allowing the attribute to be cleared."
}

func (m useNullForUnknownWhenConfigNullModifier) MarkdownDescription(ctx context.Context) string {
	return m.Description(ctx)
}

func (m useNullForUnknownWhenConfigNullModifier) PlanModifyString(_ context.Context, req planmodifier.StringRequest, resp *planmodifier.StringResponse) {
	// If the config value is null (user explicitly set null or omitted the field),
	// and there IS a prior state value, propose null to allow clearing.
	// This overrides the default Optional+Computed behavior of preserving prior state.
	if req.ConfigValue.IsNull() && !req.StateValue.IsNull() {
		resp.PlanValue = req.ConfigValue // null
	}
}
