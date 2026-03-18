package provider

// dimensionsTypeAliases maps the new alias names to their canonical (old) names
// as returned by the API.
//
// Background: CMP-36999 introduced "allocation" and "allocation_rule" as aliases
// for "attribution_group" and "attribution" respectively. The API accepts both
// forms in requests but returns the canonical names in responses. This causes
// Terraform drift when a user configures the new alias but the API returns the
// old canonical name.
//
// The normalization function below preserves the user's value when the API
// response is semantically equivalent (i.e. they are aliases of each other).
var dimensionsTypeAliases = map[string]string{
	"allocation":        "attribution_group",
	"allocation_rule":   "attribution",
	"attribution_group": "allocation",
	"attribution":       "allocation_rule",
}

// normalizeDimensionsType returns the value that should be stored in Terraform
// state for a DimensionsTypes field. It compares the API response value with
// the value currently in state (which reflects what the user configured).
//
// If the two values are aliases of each other, the state value is preserved to
// avoid false Terraform drift. Otherwise the API value is used, which is the
// correct behavior for:
//   - Import (stateValue is empty) — state gets populated with whatever the API returns
//   - External changes — if someone changes the type outside Terraform, the API value
//     will NOT be an alias of the state value, so Terraform will correctly detect drift
//   - Non-alias values (e.g. "fixed", "label") — returned as-is
func normalizeDimensionsType(apiValue, stateValue string) string {
	if apiValue == stateValue {
		return apiValue
	}

	// Check if apiValue is the alias counterpart of stateValue
	if canonical, ok := dimensionsTypeAliases[stateValue]; ok && canonical == apiValue {
		return stateValue
	}

	return apiValue
}
