package provider

import (
	"encoding/json"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework-jsontypes/jsontypes"
	"github.com/hashicorp/terraform-plugin-framework/diag"
)

// mapFreeformJSON converts a free-form API map to a jsontypes.Normalized value.
// Returns null when the map pointer is nil (field absent) or the map is empty.
// The API normalizes empty objects to null in responses, so collapsing {} to null
// here prevents drift between the user's config and the API response.
func mapFreeformJSON(data *map[string]any) jsontypes.Normalized {
	if data == nil || len(*data) == 0 {
		return jsontypes.NewNormalizedNull()
	}
	jsonBytes, err := json.Marshal(*data)
	if err != nil {
		return jsontypes.NewNormalizedNull()
	}
	return jsontypes.NewNormalizedValue(string(jsonBytes))
}

// freeformJSONToMap converts a jsontypes.Normalized value to a free-form map
// for API requests. Returns nil and no error when the value is null or unknown
// (meaning the field should be omitted from the request). Returns a diagnostic
// error if the JSON cannot be parsed.
func freeformJSONToMap(v jsontypes.Normalized) (*map[string]any, diag.Diagnostics) {
	var diags diag.Diagnostics

	if v.IsNull() || v.IsUnknown() {
		return nil, diags
	}
	raw := v.ValueString()
	if raw == "" {
		return nil, diags
	}
	var result map[string]any
	if err := json.Unmarshal([]byte(raw), &result); err != nil {
		diags.AddError(
			"Invalid JSON in Freeform Attribute",
			fmt.Sprintf("Could not parse JSON value %q: %s", raw, err),
		)
		return nil, diags
	}
	return &result, diags
}
