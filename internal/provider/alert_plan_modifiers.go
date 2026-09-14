package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
)

// useNullForIgnoreValuesRange proposes null for config.ignore_values_range
// when the attribute is omitted from configuration (config is null).
// This enables omitting the block to clear it in the API via config.IgnoreValuesRange.SetNull().
func useNullForIgnoreValuesRange() planmodifier.Object {
	return useNullForIgnoreValuesRangeModifier{}
}

type useNullForIgnoreValuesRangeModifier struct{}

func (m useNullForIgnoreValuesRangeModifier) Description(_ context.Context) string {
	return "Proposes null when config.ignore_values_range is null so that omitting the block clears it."
}

func (m useNullForIgnoreValuesRangeModifier) MarkdownDescription(ctx context.Context) string {
	return m.Description(ctx)
}

func (m useNullForIgnoreValuesRangeModifier) PlanModifyObject(_ context.Context, req planmodifier.ObjectRequest, resp *planmodifier.ObjectResponse) {
	if req.ConfigValue.IsNull() {
		resp.PlanValue = req.ConfigValue
	}
}
