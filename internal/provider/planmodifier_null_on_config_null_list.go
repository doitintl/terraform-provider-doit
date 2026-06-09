package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// useNullForUnknownListWhenConfigNull is the List equivalent of
// useNullForUnknownWhenConfigNull. See that function's documentation for details.
//
// Instead of proposing null (which would require state-aware Read paths to
// avoid null↔[] drift), this modifier proposes an empty list. The empty list
// signals "clear this field" to the Update handler, and because the Read path
// already maps nil API responses to [], the modifier's value matches the
// post-Update state without any additional logic.
func useNullForUnknownListWhenConfigNull() planmodifier.List {
	return useNullForUnknownListWhenConfigNullModifier{}
}

type useNullForUnknownListWhenConfigNullModifier struct{}

func (m useNullForUnknownListWhenConfigNullModifier) Description(_ context.Context) string {
	return "Proposes an empty list when the config value is null (omitted or explicitly set) " +
		"and a prior state value exists, allowing the attribute to be cleared."
}

func (m useNullForUnknownListWhenConfigNullModifier) MarkdownDescription(ctx context.Context) string {
	return m.Description(ctx)
}

func (m useNullForUnknownListWhenConfigNullModifier) PlanModifyList(ctx context.Context, req planmodifier.ListRequest, resp *planmodifier.ListResponse) {
	if req.ConfigValue.IsNull() && !req.StateValue.IsNull() {
		emptyList, diags := types.ListValue(req.StateValue.ElementType(ctx), []attr.Value{})
		resp.Diagnostics.Append(diags...)
		resp.PlanValue = emptyList
	}
}
