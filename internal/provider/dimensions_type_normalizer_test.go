package provider

import (
	"testing"
)

func Test_normalizeDimensionsType_SameValue(t *testing.T) {
	got := normalizeDimensionsType("fixed", "fixed")
	if got != "fixed" {
		t.Errorf("expected 'fixed', got '%s'", got)
	}
}

func Test_normalizeDimensionsType_AliasPreservesUserValue(t *testing.T) {
	// User set "allocation", API returned "attribution_group" → preserve user's value
	got := normalizeDimensionsType("attribution_group", "allocation")
	if got != "allocation" {
		t.Errorf("expected 'allocation', got '%s'", got)
	}

	// User set "allocation_rule", API returned "attribution" → preserve user's value
	got = normalizeDimensionsType("attribution", "allocation_rule")
	if got != "allocation_rule" {
		t.Errorf("expected 'allocation_rule', got '%s'", got)
	}

	// Reverse: User set "attribution_group", API returned "allocation" → preserve user's value
	got = normalizeDimensionsType("allocation", "attribution_group")
	if got != "attribution_group" {
		t.Errorf("expected 'attribution_group', got '%s'", got)
	}
}

func Test_normalizeDimensionsType_NonAliasUsesAPIValue(t *testing.T) {
	// User set "fixed", API returned "label" → external change, use API value
	got := normalizeDimensionsType("label", "fixed")
	if got != "label" {
		t.Errorf("expected 'label', got '%s'", got)
	}
}

func Test_normalizeDimensionsType_EmptyStateUsesAPIValue(t *testing.T) {
	// Import case: no existing state, use API value
	got := normalizeDimensionsType("attribution_group", "")
	if got != "attribution_group" {
		t.Errorf("expected 'attribution_group', got '%s'", got)
	}
}

func Test_normalizeDimensionsType_AttributionTierIsNotAnAlias(t *testing.T) {
	// Attribute attribution tiers are not aliases of the deprecated
	// "attribution" type, in either direction.
	for _, pair := range [][2]string{
		{"attribution_tier_1", "attribution"},
		{"attribution", "attribution_tier_1"},
		{"attribution_tier_2", "allocation_rule"},
		{"allocation_rule", "attribution_tier_3"},
	} {
		apiValue, stateValue := pair[0], pair[1]
		if got := normalizeDimensionsType(apiValue, stateValue); got != apiValue {
			t.Errorf("normalizeDimensionsType(%q, %q) = %q, want %q", apiValue, stateValue, got, apiValue)
		}
	}
}
