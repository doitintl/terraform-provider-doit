package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
)

// useNullForUnknownObjectWhenConfigNull is the Object equivalent of
// useEmptyForUnknownWhenConfigNull. See that function's documentation for details.
//
// Proposes a true Null object value when the configuration value is null (omitted or explicitly set)
// and a prior state value exists, allowing the single-nested object attribute to be cleared.
func useNullForUnknownObjectWhenConfigNull() planmodifier.Object {
	return useNullForUnknownObjectWhenConfigNullModifier{}
}

type useNullForUnknownObjectWhenConfigNullModifier struct{}

func (m useNullForUnknownObjectWhenConfigNullModifier) Description(_ context.Context) string {
	return "Proposes a Null object when the config value is null (omitted or explicitly set) " +
		"and a prior state value exists, allowing the attribute to be cleared."
}

func (m useNullForUnknownObjectWhenConfigNullModifier) MarkdownDescription(ctx context.Context) string {
	return m.Description(ctx)
}

func (m useNullForUnknownObjectWhenConfigNullModifier) PlanModifyObject(ctx context.Context, req planmodifier.ObjectRequest, resp *planmodifier.ObjectResponse) {
	if req.ConfigValue.IsNull() {
		resp.PlanValue = req.ConfigValue
	}
}
