package setnestedtest

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/resource"
)

func planSetNestedAttributesFromState(_ context.Context, _ resource.ModifyPlanRequest, _ *resource.ModifyPlanResponse) {
}

// --- GOOD: ModifyPlan plans the set from the prior state ---

type plannedResource struct{}

func (r *plannedResource) Schema(ctx context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	s := PlannedResourceSchema(ctx)
	resp.Schema = s
}

func (r *plannedResource) ModifyPlan(ctx context.Context, req resource.ModifyPlanRequest, resp *resource.ModifyPlanResponse) {
	planSetNestedAttributesFromState(ctx, req, resp)
}

// --- BAD: Optional+Computed set element attribute, no ModifyPlan ---

type unplannedResource struct{}

func (r *unplannedResource) Schema(ctx context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) { // want `SetNestedAttribute "items" has Computed attributes in its elements`
	s := UnplannedResourceSchema(ctx)
	resp.Schema = s
}

// --- BAD: set inside an object, ModifyPlan without the helper ---

type nestedUnplannedResource struct{}

func (r *nestedUnplannedResource) Schema(ctx context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) { // want `SetNestedAttribute "config.items" has Computed attributes in its elements`
	s := NestedUnplannedResourceSchema(ctx)
	resp.Schema = s
}

func (r *nestedUnplannedResource) ModifyPlan(_ context.Context, _ resource.ModifyPlanRequest, _ *resource.ModifyPlanResponse) {
}

// --- GOOD: set elements without Computed attributes; Computed list elements are fine ---

type configOnlyResource struct{}

func (r *configOnlyResource) Schema(ctx context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	s := ConfigOnlyResourceSchema(ctx)
	resp.Schema = s
}
