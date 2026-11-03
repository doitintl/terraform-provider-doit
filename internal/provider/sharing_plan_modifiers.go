package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// sharingAllocationsResourceType is the resource_type value of allocation
// (single and group) sharing.
const sharingAllocationsResourceType = "allocations"

// sharingPublicClearable plans the `public` attribute of doit_sharing.
//
//   - Allocations always have at least "viewer" public access and the API
//     cannot clear it, so an omitted `public` keeps the prior value.
//   - Every other resource type stores null as "no public access", so an
//     omitted `public` clears it, via useNullForUnknownStringWhenConfigNull.
//   - Otherwise an unknown planned value (the framework marks computed
//     attributes unknown whenever another attribute changes) is replaced with
//     the prior state value, like stringplanmodifier.UseStateForUnknown. This is
//     done here instead of with a separate UseStateForUnknown modifier, which
//     would run after the unknown resource_type branch below and restore a prior
//     value belonging to the old resource type.
//
// While resource_type is still unknown the type cannot be classified, so the
// plan value is left unknown and resolved from the API response after apply.
func sharingPublicClearable() planmodifier.String {
	return sharingPublicClearableModifier{clearing: useNullForUnknownStringWhenConfigNull()}
}

type sharingPublicClearableModifier struct {
	clearing planmodifier.String
}

func (m sharingPublicClearableModifier) Description(_ context.Context) string {
	return "Clears public access when it is omitted from the configuration, except for allocations, " +
		"which always keep at least viewer access and retain their current value."
}

func (m sharingPublicClearableModifier) MarkdownDescription(ctx context.Context) string {
	return m.Description(ctx)
}

func (m sharingPublicClearableModifier) PlanModifyString(ctx context.Context, req planmodifier.StringRequest, resp *planmodifier.StringResponse) {
	if !req.ConfigValue.IsNull() {
		return
	}

	var resourceType types.String
	resp.Diagnostics.Append(req.Plan.GetAttribute(ctx, path.Root("resource_type"), &resourceType)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if resourceType.IsUnknown() {
		resp.PlanValue = types.StringUnknown()
		return
	}

	if resourceType.ValueString() != sharingAllocationsResourceType {
		m.clearing.PlanModifyString(ctx, req, resp)
	}

	// Same conditions as stringplanmodifier.UseStateForUnknown.
	if resp.PlanValue.IsUnknown() && !req.State.Raw.IsNull() {
		resp.PlanValue = req.StateValue
	}
}
