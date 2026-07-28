package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
)

// requiresReplaceWhenCleared forces resource replacement when an Optional+Computed
// attribute that CANNOT be cleared in place is present in prior state but removed
// from config.
//
// Some DoiT API objects are updated via a PATCH that merges config and cannot
// express "clear this field" (a nil pointer omits the field, and the field has no
// null representation to send). For such attributes, removing them from config can
// neither clear them nor round-trip cleanly:
//   - a nested Optional+Computed object is re-marked "known after apply" whenever it
//     is absent from config, producing perpetual drift; and
//   - a plan modifier cannot fix this because nested object plan modifiers on
//     custom-typed parents (e.g. config.*) do not fire.
//
// Rather than perpetual drift (or a confusing API error), surface the removal as a
// destroy+create by adding the attribute path to resp.RequiresReplace. This is the
// standard pattern for unclearable Optional+Computed attributes until the API adds
// a null/clear representation for them.
//
// T must be the attribute's generated custom value type (e.g.
// resource_report.CountValue). Reading a custom-typed SingleNestedAttribute into a
// plain types.Object instead crashes with a "Value Conversion Error", so the typed
// read here is required.
//
// Call from a resource's ModifyPlan after guarding create/destroy:
//
//	func (r *xResource) ModifyPlan(ctx context.Context, req resource.ModifyPlanRequest, resp *resource.ModifyPlanResponse) {
//	    if req.State.Raw.IsNull() || req.Plan.Raw.IsNull() {
//	        return // create or destroy — nothing to compare
//	    }
//	    requiresReplaceWhenCleared[resource_report.CountValue](ctx, req, resp, path.Root("config").AtName("count"))
//	}
func requiresReplaceWhenCleared[T attr.Value](ctx context.Context, req resource.ModifyPlanRequest, resp *resource.ModifyPlanResponse, p path.Path) {
	var stateVal T
	resp.Diagnostics.Append(req.State.GetAttribute(ctx, p, &stateVal)...)
	var configVal T
	resp.Diagnostics.Append(req.Config.GetAttribute(ctx, p, &configVal)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Set in prior state but removed from config → cannot be cleared in place.
	if !stateVal.IsNull() && configVal.IsNull() {
		resp.RequiresReplace = append(resp.RequiresReplace, p)
	}
}
