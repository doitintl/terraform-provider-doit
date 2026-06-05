package provider

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-framework-jsontypes/jsontypes"
)

func TestMapFreeformJSON(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		input    *map[string]interface{}
		wantNull bool
		wantJSON string
	}{
		"nil-map": {
			input:    nil,
			wantNull: true,
		},
		"empty-map": {
			input:    &map[string]interface{}{},
			wantNull: true,
		},
		"simple-map": {
			input:    &map[string]interface{}{"key": "value"},
			wantNull: false,
			wantJSON: `{"key":"value"}`,
		},
		"nested-map": {
			input: &map[string]interface{}{
				"nested": map[string]interface{}{"inner": 42.0},
				"list":   []interface{}{"a", "b"},
			},
			wantNull: false,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			got := mapFreeformJSON(tc.input)

			if tc.wantNull {
				if !got.IsNull() {
					t.Errorf("expected null, got %q", got.ValueString())
				}
				return
			}

			if got.IsNull() || got.IsUnknown() {
				t.Fatal("expected a known value, got null/unknown")
			}

			if tc.wantJSON != "" && got.ValueString() != tc.wantJSON {
				t.Errorf("expected %q, got %q", tc.wantJSON, got.ValueString())
			}
		})
	}
}

func TestFreeformJSONToMap(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		input   jsontypes.Normalized
		wantNil bool
		wantKey string
		wantVal interface{}
	}{
		"null-value": {
			input:   jsontypes.NewNormalizedNull(),
			wantNil: true,
		},
		"unknown-value": {
			input:   jsontypes.NewNormalizedUnknown(),
			wantNil: true,
		},
		"empty-string": {
			input:   jsontypes.NewNormalizedValue(""),
			wantNil: true,
		},
		"invalid-json": {
			input:   jsontypes.NewNormalizedValue("not json"),
			wantNil: true,
		},
		"valid-json": {
			input:   jsontypes.NewNormalizedValue(`{"key":"value"}`),
			wantNil: false,
			wantKey: "key",
			wantVal: "value",
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			got := freeformJSONToMap(tc.input)

			if tc.wantNil {
				if got != nil {
					t.Errorf("expected nil, got %v", *got)
				}
				return
			}

			if got == nil {
				t.Fatal("expected non-nil map, got nil")
			}

			if val, ok := (*got)[tc.wantKey]; !ok || val != tc.wantVal {
				t.Errorf("expected [%s]=%v, got %v", tc.wantKey, tc.wantVal, *got)
			}
		})
	}
}

func TestFreeformJSON_RoundTrip(t *testing.T) {
	t.Parallel()

	original := &map[string]interface{}{
		"region":       "us-east-1",
		"instanceType": "m5.large",
	}

	// Serialize to TF value
	normalized := mapFreeformJSON(original)
	if normalized.IsNull() {
		t.Fatal("expected known value after serialization")
	}

	// Deserialize back to map
	restored := freeformJSONToMap(normalized)
	if restored == nil {
		t.Fatal("expected non-nil map after deserialization")
	}

	// Verify values
	for k, v := range *original {
		got, ok := (*restored)[k]
		if !ok {
			t.Errorf("missing key %q after round-trip", k)
		}
		if got != v {
			t.Errorf("key %q: expected %v, got %v", k, v, got)
		}
	}
}
