package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
)

// UseNullForUnknownObjectWhenConfigNull is the Object equivalent of
// useEmptyForUnknownWhenConfigNull. See that function's documentation for details.
//
// Proposes a true Null object value when the configuration value is null (omitted or explicitly set)
// and a prior state value exists, allowing the single-nested object attribute to be cleared.
func UseNullForUnknownObjectWhenConfigNull() planmodifier.Object {
	return UseNullForUnknownObjectWhenConfigNullModifier{}
}

type UseNullForUnknownObjectWhenConfigNullModifier struct{}

func (m UseNullForUnknownObjectWhenConfigNullModifier) Description(_ context.Context) string {
	return "Proposes a Null object when the config value is null (omitted or explicitly set) " +
		"and a prior state value exists, allowing the attribute to be cleared."
}

func (m UseNullForUnknownObjectWhenConfigNullModifier) MarkdownDescription(ctx context.Context) string {
	return m.Description(ctx)
}

func (m UseNullForUnknownObjectWhenConfigNullModifier) PlanModifyObject(ctx context.Context, req planmodifier.ObjectRequest, resp *planmodifier.ObjectResponse) {
	if req.ConfigValue.IsNull() && !req.StateValue.IsNull() {
		resp.PlanValue = req.ConfigValue
	}
}
