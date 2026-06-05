package provider

import (
	"encoding/json"

	"github.com/hashicorp/terraform-plugin-framework-jsontypes/jsontypes"
)

// mapFreeformJSON converts a free-form API map to a jsontypes.Normalized value.
// Returns null when the map is nil or empty.
func mapFreeformJSON(data *map[string]interface{}) jsontypes.Normalized {
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
// for API requests. Returns nil when the value is null or unknown.
func freeformJSONToMap(v jsontypes.Normalized) *map[string]interface{} {
	if v.IsNull() || v.IsUnknown() {
		return nil
	}
	raw := v.ValueString()
	if raw == "" {
		return nil
	}
	var result map[string]interface{}
	if err := json.Unmarshal([]byte(raw), &result); err != nil {
		return nil
	}
	return &result
}
