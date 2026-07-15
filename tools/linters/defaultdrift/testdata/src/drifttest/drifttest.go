package drifttest

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/types"
)

// Ensure imports are used.
var _ context.Context

// --- Resource type with Schema() method ---

type driftTestResource struct{}

func (r *driftTestResource) Schema(_ context.Context, _ interface{}, resp *interface{}) {
	s := DriftTestResourceSchema(context.Background())
	_ = s
}

// --- BAD: PointerValue on defaulted fields ---

func (r *driftTestResource) mapDriftTestToModel(resp *ApiResponse, state *DriftTestModel) {
	state.Id = types.StringPointerValue(resp.Id)
	state.Description = types.StringPointerValue(resp.Description)          // want `PointerValue on field "description" with schema default ""; use nil-fallback pattern`
	state.Metric = types.StringPointerValue(resp.Metric)                    // want `PointerValue on field "metric" with schema default "cost"; use nil-fallback pattern`
	state.GrowthPerPeriod = types.Float64PointerValue(resp.GrowthPerPeriod) // want `PointerValue on field "growth_per_period" with schema default 0; use nil-fallback pattern`
	state.UsePrevSpend = types.BoolPointerValue(resp.UsePrevSpend)          // want `PointerValue on field "use_prev_spend" with schema default false; use nil-fallback pattern`
	state.MaxResults = types.Int64PointerValue(resp.MaxResults)             // want `PointerValue on field "max_results" with schema default 1000; use nil-fallback pattern`
	// Non-defaulted O+C — OK
	state.FolderId = types.StringPointerValue(resp.FolderId)
	// Computed-only — OK
	state.CreateTime = types.Int64PointerValue(resp.CreateTime)
}

// --- GOOD: nil-fallback pattern ---

func (r *driftTestResource) mapDriftTestToModelCorrect(resp *ApiResponse, state *DriftTestModel) {
	state.Id = types.StringPointerValue(resp.Id)
	if resp.Description != nil {
		state.Description = types.StringValue(*resp.Description)
	} else {
		state.Description = types.StringValue("")
	}
	if resp.Metric != nil {
		state.Metric = types.StringValue(*resp.Metric)
	} else {
		state.Metric = types.StringValue("cost")
	}
	if resp.GrowthPerPeriod != nil {
		state.GrowthPerPeriod = types.Float64Value(*resp.GrowthPerPeriod)
	} else {
		state.GrowthPerPeriod = types.Float64Value(0)
	}
	if resp.UsePrevSpend != nil {
		state.UsePrevSpend = types.BoolValue(*resp.UsePrevSpend)
	} else {
		state.UsePrevSpend = types.BoolValue(false)
	}
	if resp.MaxResults != nil {
		state.MaxResults = types.Int64Value(*resp.MaxResults)
	} else {
		state.MaxResults = types.Int64Value(1000)
	}
	state.FolderId = types.StringPointerValue(resp.FolderId)
	state.CreateTime = types.Int64PointerValue(resp.CreateTime)
}

// --- BAD: PointerValue on nested defaulted fields ---

func (r *driftTestResource) mapConfigToModel(resp *ConfigResponse, state *ConfigModel) {
	state.Enabled = types.BoolPointerValue(resp.Enabled)       // want `PointerValue on field "enabled" with schema default false; use nil-fallback pattern`
	state.SortOrder = types.StringPointerValue(resp.SortOrder) // want `PointerValue on field "sort_order" with schema default "desc"; use nil-fallback pattern`
	// Non-defaulted — OK
	state.Label = types.StringPointerValue(resp.Label)
}

// --- BAD: PointerValue on list-nested defaulted fields ---

func (r *driftTestResource) mapItemToModel(resp *ItemResponse, state *ItemModel) {
	state.Name = types.StringValue(resp.Name)
	state.IncludeNull = types.BoolPointerValue(resp.IncludeNull) // want `PointerValue on field "include_null" with schema default false; use nil-fallback pattern`
}

// --- Not a mapping function — should be ignored ---

func (r *driftTestResource) helperFunc(resp *ApiResponse, state *DriftTestModel) {
	state.Description = types.StringPointerValue(resp.Description)
}
