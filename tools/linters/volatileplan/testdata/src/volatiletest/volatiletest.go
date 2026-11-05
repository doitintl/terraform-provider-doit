package volatiletest

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
)

func useEmptyForUnknownWhenConfigNull() {}

func planVolatileAttributesUnknownOnUpdate(_ context.Context, _ resource.ModifyPlanRequest, _ *resource.ModifyPlanResponse, _ ...path.Path) {
}

// --- GOOD: clearing modifier and etag, ModifyPlan marks it unknown ---

type plannedResource struct{}

func (r *plannedResource) Schema(ctx context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	useEmptyForUnknownWhenConfigNull()
	resp.Schema = PlannedResourceSchema(ctx)
}

func (r *plannedResource) ModifyPlan(ctx context.Context, req resource.ModifyPlanRequest, resp *resource.ModifyPlanResponse) {
	planVolatileAttributesUnknownOnUpdate(ctx, req, resp, path.Root("etag"), path.Root("update_time"))
}

// --- GOOD: etag alone is enough ---

type etagOnlyResource struct{}

func (r *etagOnlyResource) Schema(ctx context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	useEmptyForUnknownWhenConfigNull()
	resp.Schema = EtagOnlyResourceSchema(ctx)
}

func (r *etagOnlyResource) ModifyPlan(ctx context.Context, req resource.ModifyPlanRequest, resp *resource.ModifyPlanResponse) {
	planVolatileAttributesUnknownOnUpdate(ctx, req, resp, path.Root("etag"))
}

// --- BAD: clearing modifier and etag, no ModifyPlan ---

type unplannedResource struct{}

func (r *unplannedResource) Schema(ctx context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) { // want `clearing plan modifier and a Computed etag`
	useEmptyForUnknownWhenConfigNull()
	resp.Schema = UnplannedResourceSchema(ctx)
}

// --- BAD: the helper is called without any path ---

type noPathsResource struct{}

func (r *noPathsResource) Schema(ctx context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) { // want `clearing plan modifier and a Computed etag`
	useEmptyForUnknownWhenConfigNull()
	resp.Schema = NoPathsResourceSchema(ctx)
}

func (r *noPathsResource) ModifyPlan(ctx context.Context, req resource.ModifyPlanRequest, resp *resource.ModifyPlanResponse) {
	planVolatileAttributesUnknownOnUpdate(ctx, req, resp)
}

// --- BAD: the helper is called with update_time only ---

type timestampOnlyResource struct{}

func (r *timestampOnlyResource) Schema(ctx context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) { // want `clearing plan modifier and a Computed etag`
	useEmptyForUnknownWhenConfigNull()
	resp.Schema = TimestampOnlyResourceSchema(ctx)
}

func (r *timestampOnlyResource) ModifyPlan(ctx context.Context, req resource.ModifyPlanRequest, resp *resource.ModifyPlanResponse) {
	planVolatileAttributesUnknownOnUpdate(ctx, req, resp, path.Root("update_time"))
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
