package volatiletest

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
)

func useEmptyForUnknownWhenConfigNull() {}

func planVolatileAttributesUnknownOnUpdate(_ context.Context, _ resource.ModifyPlanRequest, _ *resource.ModifyPlanResponse, _ ...path.Path) {
}

// --- GOOD: clearing modifier, ModifyPlan marks every volatile attribute unknown ---

type plannedResource struct{}

func (r *plannedResource) Schema(ctx context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	s := PlannedResourceSchema(ctx)
	useEmptyForUnknownWhenConfigNull()
	resp.Schema = s
}

func (r *plannedResource) ModifyPlan(ctx context.Context, req resource.ModifyPlanRequest, resp *resource.ModifyPlanResponse) {
	planVolatileAttributesUnknownOnUpdate(ctx, req, resp, path.Root("etag"), path.Root("update_time"))
}

// --- GOOD: stable attributes have UseStateForUnknown, in Schema() or in the generated schema ---

type stableResource struct{}

func (r *stableResource) Schema(ctx context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	s := StableResourceSchema(ctx)
	useEmptyForUnknownWhenConfigNull()
	if attr, ok := s.Attributes["id"].(schema.StringAttribute); ok {
		attr.PlanModifiers = append(attr.PlanModifiers, stringplanmodifier.UseStateForUnknown())
		s.Attributes["id"] = attr
	}
	resp.Schema = s
}

func (r *stableResource) ModifyPlan(ctx context.Context, req resource.ModifyPlanRequest, resp *resource.ModifyPlanResponse) {
	planVolatileAttributesUnknownOnUpdate(ctx, req, resp, path.Root("update_time"))
}

// --- BAD: clearing modifier and an etag, no ModifyPlan ---

type unplannedResource struct{}

func (r *unplannedResource) Schema(ctx context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) { // want `clearing plan modifier.*Computed-only attribute\(s\) etag `
	s := UnplannedResourceSchema(ctx)
	useEmptyForUnknownWhenConfigNull()
	resp.Schema = s
}

// --- BAD: the helper is called without any path ---

type noPathsResource struct{}

func (r *noPathsResource) Schema(ctx context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) { // want `Computed-only attribute\(s\) etag `
	s := NoPathsResourceSchema(ctx)
	useEmptyForUnknownWhenConfigNull()
	resp.Schema = s
}

func (r *noPathsResource) ModifyPlan(ctx context.Context, req resource.ModifyPlanRequest, resp *resource.ModifyPlanResponse) {
	planVolatileAttributesUnknownOnUpdate(ctx, req, resp)
}

// --- BAD: the helper misses some attributes; all of them are listed in one report ---

type partialResource struct{}

func (r *partialResource) Schema(ctx context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) { // want `Computed-only attribute\(s\) records, update_time `
	s := PartialResourceSchema(ctx)
	useEmptyForUnknownWhenConfigNull()
	resp.Schema = s
}

func (r *partialResource) ModifyPlan(ctx context.Context, req resource.ModifyPlanRequest, resp *resource.ModifyPlanResponse) {
	planVolatileAttributesUnknownOnUpdate(ctx, req, resp, path.Root("etag"))
}

// --- GOOD: without a clearing modifier the framework plans them unknown ---

type noClearingResource struct{}

func (r *noClearingResource) Schema(ctx context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	s := NoClearingResourceSchema(ctx)
	resp.Schema = s
}

// --- GOOD: the schema is assigned directly, without a local variable ---

type directResource struct{}

func (r *directResource) Schema(ctx context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	useEmptyForUnknownWhenConfigNull()
	resp.Schema = DirectResourceSchema(ctx)
}

func (r *directResource) ModifyPlan(ctx context.Context, req resource.ModifyPlanRequest, resp *resource.ModifyPlanResponse) {
	planVolatileAttributesUnknownOnUpdate(ctx, req, resp, path.Root("etag"))
}
