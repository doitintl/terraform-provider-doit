package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
)

// useNullForUnknownGcpFeatureInputsWhenConfigNull proposes null for
// feature_inputs when it is omitted from configuration. Omitting the block
// clears the stored inputs: the update request sends featureInputs: null.
func useNullForUnknownGcpFeatureInputsWhenConfigNull() planmodifier.Object {
	return useNullForUnknownGcpFeatureInputsWhenConfigNullModifier{}
}

type useNullForUnknownGcpFeatureInputsWhenConfigNullModifier struct{}

func (m useNullForUnknownGcpFeatureInputsWhenConfigNullModifier) Description(_ context.Context) string {
	return "Proposes null when feature_inputs is null so that omitting the block clears the stored inputs."
}

func (m useNullForUnknownGcpFeatureInputsWhenConfigNullModifier) MarkdownDescription(ctx context.Context) string {
	return m.Description(ctx)
}

func (m useNullForUnknownGcpFeatureInputsWhenConfigNullModifier) PlanModifyObject(_ context.Context, req planmodifier.ObjectRequest, resp *planmodifier.ObjectResponse) {
	if req.ConfigValue.IsNull() {
		resp.PlanValue = req.ConfigValue
	}
}
