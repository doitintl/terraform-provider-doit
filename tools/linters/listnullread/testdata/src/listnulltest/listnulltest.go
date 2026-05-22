package listnulltest

import (
	"github.com/hashicorp/terraform-plugin-framework/types"
)

type myModel struct {
	Scopes  types.List
	Results types.List
}

// --- BAD: populateState uses ListNull for Optional+Computed list ---

func populateState(state *myModel) {
	state.Scopes = types.ListNull(types.StringType) // want `types.ListNull\(\) used for Optional/Optional\+Computed list field "scopes"`
}

// --- GOOD: populateState uses empty list for Optional+Computed list ---

func mapResourceToModel(state *myModel) {
	state.Scopes, _ = types.ListValue(types.StringType, []interface{}{})
}

// --- GOOD: ListNull for Computed-only list is OK ---

func mapResultsToModel(state *myModel) {
	state.Results = types.ListNull(types.StringType)
}

// --- Not a mapping function — should be ignored ---

func helperFunction(state *myModel) {
	state.Scopes = types.ListNull(types.StringType)
}
