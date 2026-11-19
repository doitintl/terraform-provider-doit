package overlaytest

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/path"
)

type volatileResource struct{}

type schemaResponse struct{}
type modifyPlanRequest struct{}
type modifyPlanResponse struct{}

func (r *volatileResource) Schema(ctx context.Context, _ struct{}, _ *schemaResponse) {
	s := VolatileResourceSchema(ctx)
	_ = s
}

func (r *volatileResource) ModifyPlan(ctx context.Context, req modifyPlanRequest, resp *modifyPlanResponse) {
	planVolatileAttributesUnknownOnUpdate(ctx, req, resp, path.Root("update_time"), path.Root("etag"))
}

func planVolatileAttributesUnknownOnUpdate(_ context.Context, _ modifyPlanRequest, _ *modifyPlanResponse, _ ...path.Path) {
}

func mapVolatileToModel(apiResp *ApiResponse, m *VolatileModel) {}

// --- GOOD: update_time is guarded and ModifyPlan marks it unknown on update ---
// --- BAD: last_alerted is guarded but ModifyPlan does not mark it unknown ---
// --- BAD: etag is marked unknown but guarded by IsNull(), not its own IsUnknown() ---

func overlayVolatileComputedFields(apiResp *ApiResponse, plan *VolatileModel) {
	resolved := *plan
	mapVolatileToModel(apiResp, &resolved)

	plan.Id = resolved.Id
	if plan.UpdateTime.IsUnknown() {
		plan.UpdateTime = resolved.UpdateTime
	}
	if plan.LastAlerted.IsUnknown() {
		plan.LastAlerted = resolved.LastAlerted // want `overlayVolatileComputedFields: Computed-only field "last_alerted" should be assigned unconditionally`
	}
	// etag is passed to the helper, but an IsNull() guard skips the
	// assignment when ModifyPlan has made it unknown.
	if plan.Etag.IsNull() {
		plan.Etag = resolved.Etag // want `overlayVolatileComputedFields: Computed-only field "etag" should be assigned unconditionally`
	}
	if plan.Description.IsUnknown() {
		plan.Description = resolved.Description
	}
}
