package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
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
// volatile Computed-only attribute. Each attribute is set to an unknown value
// of its own schema type, so it works for numbers and timestamps as well as
// strings.
func planVolatileAttributesUnknownOnUpdate(ctx context.Context, req resource.ModifyPlanRequest, resp *resource.ModifyPlanResponse, attributes ...path.Path) {
	if req.State.Raw.IsNull() || req.Plan.Raw.IsNull() || resp.Plan.Raw.Equal(req.State.Raw) {
		return // create, destroy, or no change
	}
	for _, attribute := range attributes {
		schemaAttr, d := resp.Plan.Schema.AttributeAtPath(ctx, attribute)
		resp.Diagnostics.Append(d...)
		if d.HasError() {
			continue
		}
		attrType := schemaAttr.GetType()
		unknown, err := attrType.ValueFromTerraform(ctx, tftypes.NewValue(attrType.TerraformType(ctx), tftypes.UnknownValue))
		if err != nil {
			resp.Diagnostics.AddAttributeError(attribute, "Error Planning Volatile Attribute", err.Error())
			continue
		}
		resp.Diagnostics.Append(resp.Plan.SetAttribute(ctx, attribute, unknown)...)
	}
}
