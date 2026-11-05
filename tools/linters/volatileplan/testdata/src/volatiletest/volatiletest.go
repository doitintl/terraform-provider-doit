package volatiletest

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/resource"
)

func useEmptyForUnknownWhenConfigNull() {}

func planVolatileAttributesUnknownOnUpdate(_ context.Context, _ resource.ModifyPlanRequest, _ *resource.ModifyPlanResponse) {
}

// --- GOOD: clearing modifier and etag, ModifyPlan marks it unknown ---

type plannedResource struct{}

func (r *plannedResource) Schema(ctx context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	useEmptyForUnknownWhenConfigNull()
	resp.Schema = PlannedResourceSchema(ctx)
}

func (r *plannedResource) ModifyPlan(ctx context.Context, req resource.ModifyPlanRequest, resp *resource.ModifyPlanResponse) {
	planVolatileAttributesUnknownOnUpdate(ctx, req, resp)
}

// --- BAD: clearing modifier and etag, no ModifyPlan ---

type unplannedResource struct{}

func (r *unplannedResource) Schema(ctx context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) { // want `clearing plan modifier and a Computed etag`
	useEmptyForUnknownWhenConfigNull()
	resp.Schema = UnplannedResourceSchema(ctx)
}

// --- GOOD: update_time alone is replaced by the next Read ---

type noEtagResource struct{}

func (r *noEtagResource) Schema(ctx context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	useEmptyForUnknownWhenConfigNull()
	resp.Schema = NoEtagResourceSchema(ctx)
}

// --- GOOD: etag without a clearing modifier is already planned unknown ---

type noClearingResource struct{}

func (r *noClearingResource) Schema(ctx context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = NoClearingResourceSchema(ctx)
}
