package listnulltest

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

type myModel struct {
	Scopes  types.List
	Results types.List
}

// --- Resource type with Schema() method that references MyResourceSchema ---

type myResource struct{}

func (r *myResource) Schema(ctx context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	s := MyResourceSchema(ctx)
	resp.Schema = s
}

// --- BAD: populateState uses ListNull for Optional+Computed list ---

func (r *myResource) populateState(state *myModel) {
	state.Scopes = types.ListNull(types.StringType) // want `types.ListNull\(\) used for Optional/Optional\+Computed list field "scopes"`
}

// --- GOOD: mapResourceToModel uses empty list for Optional+Computed list ---

func (r *myResource) mapResourceToModel(state *myModel) {
	state.Scopes, _ = types.ListValue(types.StringType, []interface{}{})
}

// --- GOOD: ListNull for Computed-only list is OK ---

func (r *myResource) mapResultsToModel(state *myModel) {
	state.Results = types.ListNull(types.StringType)
}

// --- Not a mapping function — should be ignored ---

func (r *myResource) helperFunction(state *myModel) {
	state.Scopes = types.ListNull(types.StringType)
}

// --- Standalone function (no receiver) — should be ignored ---

func standalonePopulateState(state *myModel) {
	state.Scopes = types.ListNull(types.StringType)
}
