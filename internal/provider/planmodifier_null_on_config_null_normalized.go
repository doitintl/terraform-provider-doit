package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework-jsontypes/jsontypes"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
)

// useNullForUnknownNormalizedWhenConfigNull is a plan modifier for attributes
// using jsontypes.NormalizedType. It proposes a NormalizedNull when the config
// value is null and a prior state value exists, allowing the attribute to be
// cleared.
//
// This is the jsontypes-aware equivalent of useEmptyForUnknownWhenConfigNull.
// Using the standard string modifier with jsontypes.NormalizedType causes
// "Semantic Equality Check Error: EOF" because the framework cannot compare
// basetypes.StringValue against jsontypes.Normalized.
func useNullForUnknownNormalizedWhenConfigNull() planmodifier.String {
	return useNullForUnknownNormalizedWhenConfigNullModifier{}
}

type useNullForUnknownNormalizedWhenConfigNullModifier struct{}

func (m useNullForUnknownNormalizedWhenConfigNullModifier) Description(_ context.Context) string {
	return "Proposes a jsontypes.NormalizedNull when the config value is null " +
		"and a prior state value exists, allowing the attribute to be cleared."
}

func (m useNullForUnknownNormalizedWhenConfigNullModifier) MarkdownDescription(ctx context.Context) string {
	return m.Description(ctx)
}

func (m useNullForUnknownNormalizedWhenConfigNullModifier) PlanModifyString(_ context.Context, req planmodifier.StringRequest, resp *planmodifier.StringResponse) {
	if req.ConfigValue.IsNull() && !req.StateValue.IsNull() {
		resp.PlanValue = jsontypes.NewNormalizedNull().StringValue
	}
}
