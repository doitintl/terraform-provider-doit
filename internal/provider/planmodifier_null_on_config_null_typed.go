package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// useNullForUnknownStringWhenConfigNull is a null-based String equivalent of
// the clearing modifier. Unlike useEmptyForUnknownWhenConfigNull (which proposes ""),
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

// useNullForUnknownListNullWhenConfigNull is a null-based List equivalent of
// the clearing modifier. Unlike useNullForUnknownListWhenConfigNull (which proposes []),
// this modifier proposes ListNull. Use it when the API returns nil (not [])
// after clearing, e.g. for nested list attributes whose absence represents unrestricted/unbounded.
func useNullForUnknownListNullWhenConfigNull(elemType attr.Type) planmodifier.List {
	return useNullForUnknownListNullWhenConfigNullModifier{elemType: elemType}
}

type useNullForUnknownListNullWhenConfigNullModifier struct {
	elemType attr.Type
}

func (m useNullForUnknownListNullWhenConfigNullModifier) Description(_ context.Context) string {
	return "Proposes null when the config value is null (omitted or explicitly set) " +
		"and a prior state value exists, allowing the attribute to be cleared."
}

func (m useNullForUnknownListNullWhenConfigNullModifier) MarkdownDescription(ctx context.Context) string {
	return m.Description(ctx)
}

func (m useNullForUnknownListNullWhenConfigNullModifier) PlanModifyList(_ context.Context, req planmodifier.ListRequest, resp *planmodifier.ListResponse) {
	if req.ConfigValue.IsNull() && !req.StateValue.IsNull() {
		resp.PlanValue = types.ListNull(m.elemType)
	}
}
