package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// useNullForUnknownStringWhenConfigNull is a null-based String equivalent of
// the clearing modifier. Unlike useNullForUnknownWhenConfigNull (which proposes ""),
// this modifier proposes StringNull. Use it when the API returns nil (not "")
// after clearing, e.g. POST-replace endpoints that clear fields by omission.
func useNullForUnknownStringWhenConfigNull() planmodifier.String {
	return useNullForUnknownStringWhenConfigNullModifier{}
}

type useNullForUnknownStringWhenConfigNullModifier struct{}

func (m useNullForUnknownStringWhenConfigNullModifier) Description(_ context.Context) string {
	return "Proposes null when the config value is null (omitted or explicitly set) " +
		"and a prior state value exists, allowing the attribute to be cleared."
}

func (m useNullForUnknownStringWhenConfigNullModifier) MarkdownDescription(ctx context.Context) string {
	return m.Description(ctx)
}

func (m useNullForUnknownStringWhenConfigNullModifier) PlanModifyString(_ context.Context, req planmodifier.StringRequest, resp *planmodifier.StringResponse) {
	if req.ConfigValue.IsNull() && !req.StateValue.IsNull() {
		resp.PlanValue = types.StringNull()
	}
}

// useNullForUnknownBoolWhenConfigNull is the Bool equivalent of
// useNullForUnknownWhenConfigNull. See that function's documentation for details.
func useNullForUnknownBoolWhenConfigNull() planmodifier.Bool {
	return useNullForUnknownBoolWhenConfigNullModifier{}
}

type useNullForUnknownBoolWhenConfigNullModifier struct{}

func (m useNullForUnknownBoolWhenConfigNullModifier) Description(_ context.Context) string {
	return "Proposes null when the config value is null (omitted or explicitly set) " +
		"and a prior state value exists, allowing the attribute to be cleared."
}

func (m useNullForUnknownBoolWhenConfigNullModifier) MarkdownDescription(ctx context.Context) string {
	return m.Description(ctx)
}

func (m useNullForUnknownBoolWhenConfigNullModifier) PlanModifyBool(_ context.Context, req planmodifier.BoolRequest, resp *planmodifier.BoolResponse) {
	if req.ConfigValue.IsNull() && !req.StateValue.IsNull() {
		resp.PlanValue = types.BoolNull()
	}
}

// useNullForUnknownInt64WhenConfigNull is the Int64 equivalent of
// useNullForUnknownWhenConfigNull. See that function's documentation for details.
func useNullForUnknownInt64WhenConfigNull() planmodifier.Int64 {
	return useNullForUnknownInt64WhenConfigNullModifier{}
}

type useNullForUnknownInt64WhenConfigNullModifier struct{}

func (m useNullForUnknownInt64WhenConfigNullModifier) Description(_ context.Context) string {
	return "Proposes null when the config value is null (omitted or explicitly set) " +
		"and a prior state value exists, allowing the attribute to be cleared."
}

func (m useNullForUnknownInt64WhenConfigNullModifier) MarkdownDescription(ctx context.Context) string {
	return m.Description(ctx)
}

func (m useNullForUnknownInt64WhenConfigNullModifier) PlanModifyInt64(_ context.Context, req planmodifier.Int64Request, resp *planmodifier.Int64Response) {
	if req.ConfigValue.IsNull() && !req.StateValue.IsNull() {
		resp.PlanValue = types.Int64Null()
	}
}

// useNullForUnknownFloat64WhenConfigNull is the Float64 equivalent of
// useNullForUnknownWhenConfigNull. See that function's documentation for details.
func useNullForUnknownFloat64WhenConfigNull() planmodifier.Float64 {
	return useNullForUnknownFloat64WhenConfigNullModifier{}
}

type useNullForUnknownFloat64WhenConfigNullModifier struct{}

func (m useNullForUnknownFloat64WhenConfigNullModifier) Description(_ context.Context) string {
	return "Proposes null when the config value is null (omitted or explicitly set) " +
		"and a prior state value exists, allowing the attribute to be cleared."
}

func (m useNullForUnknownFloat64WhenConfigNullModifier) MarkdownDescription(ctx context.Context) string {
	return m.Description(ctx)
}

func (m useNullForUnknownFloat64WhenConfigNullModifier) PlanModifyFloat64(_ context.Context, req planmodifier.Float64Request, resp *planmodifier.Float64Response) {
	if req.ConfigValue.IsNull() && !req.StateValue.IsNull() {
		resp.PlanValue = types.Float64Null()
	}
}
