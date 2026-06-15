package provider

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-framework-jsontypes/jsontypes"
)

func TestMapFreeformJSON(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		input    *map[string]any
		wantNull bool
		wantJSON string
	}{
		"nil-map": {
			input:    nil,
			wantNull: true,
		},
		"empty-map": {
			input:    &map[string]any{},
			wantNull: true,
		},
		"simple-map": {
			input:    &map[string]any{"key": "value"},
			wantNull: false,
			wantJSON: `{"key":"value"}`,
		},
		"nested-map": {
			input: &map[string]any{
				"nested": map[string]any{"inner": 42.0},
				"list":   []any{"a", "b"},
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
		input     jsontypes.Normalized
		wantNil   bool
		wantError bool
		wantKey   string
		wantVal   any
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
			input:     jsontypes.NewNormalizedValue("not json"),
			wantNil:   true,
			wantError: true,
		},
		"valid-json": {
			input:   jsontypes.NewNormalizedValue(`{"key":"value"}`),
			wantNil: false,
			wantKey: "key",
			wantVal: "value",
		},
		"empty-object": {
			input:   jsontypes.NewNormalizedValue(`{}`),
			wantNil: false,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			got, diags := freeformJSONToMap(tc.input)

			if tc.wantError {
				if !diags.HasError() {
					t.Error("expected diagnostic error, got none")
				}
			} else if diags.HasError() {
				t.Errorf("unexpected diagnostic error: %s", diags.Errors()[0].Detail())
			}

			if tc.wantNil {
				if got != nil {
					t.Errorf("expected nil, got %v", *got)
				}
				return
			}

			if got == nil {
				t.Fatal("expected non-nil map, got nil")
			}

			if tc.wantKey != "" {
				if val, ok := (*got)[tc.wantKey]; !ok || val != tc.wantVal {
					t.Errorf("expected [%s]=%v, got %v", tc.wantKey, tc.wantVal, *got)
				}
			}
		})
	}
}

func TestFreeformJSON_RoundTrip(t *testing.T) {
	t.Parallel()

	original := &map[string]any{
		"region":       "us-east-1",
		"instanceType": "m5.large",
	}

	// Serialize to TF value.
	normalized := mapFreeformJSON(original)
	if normalized.IsNull() {
		t.Fatal("expected known value after serialization")
	}

	// Deserialize back to map.
	restored, diags := freeformJSONToMap(normalized)
	if diags.HasError() {
		t.Fatalf("unexpected error: %s", diags.Errors()[0].Detail())
	}
	if restored == nil {
		t.Fatal("expected non-nil map after deserialization")
	}

	// Verify values.
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

func TestFreeformJSON_EmptyObjectRoundTrip(t *testing.T) {
	t.Parallel()

	// The API normalizes empty objects to null in responses, so mapFreeformJSON
	// collapses {} to null to prevent drift.
	original := &map[string]any{}

	normalized := mapFreeformJSON(original)
	if !normalized.IsNull() {
		t.Fatalf("expected null for empty object (API normalizes {} to null), got %q", normalized.ValueString())
	}
}
