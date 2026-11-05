package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// planVolatileAttributesUnknownOnUpdate marks the given server-maintained
// attributes (etag, update_time) unknown when the plan updates the resource.
// The API changes them on every update, and the framework marks computed
// values unknown only when Terraform's proposed state differs from the prior
// state (fwserver/server_planresourcechange.go, v1.19.0). Removing an
// Optional+Computed attribute changes nothing in the proposed state, because
// Terraform proposes the stored value; the clearing plan modifier plans the
// update afterwards. Without this, state would keep the old etag and the next
// update without a refresh would send a stale If-Match and fail with 412.
//
// Call it from ModifyPlan of every resource that has a clearing modifier and a
// volatile Computed-only attribute.
func planVolatileAttributesUnknownOnUpdate(ctx context.Context, req resource.ModifyPlanRequest, resp *resource.ModifyPlanResponse, attributes ...path.Path) {
	if req.State.Raw.IsNull() || req.Plan.Raw.IsNull() || resp.Plan.Raw.Equal(req.State.Raw) {
		return // create, destroy, or no change
	}
	for _, attribute := range attributes {
		resp.Diagnostics.Append(resp.Plan.SetAttribute(ctx, attribute, types.StringUnknown())...)
	}
}
