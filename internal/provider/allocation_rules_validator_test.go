package provider

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func TestAllocationRulesValidator(t *testing.T) {
	ctx := context.Background()

	// Define the object type for a rule element
	ruleAttrTypes := map[string]attr.Type{
		"action":      types.StringType,
		"name":        types.StringType,
		"description": types.StringType,
		"id":          types.StringType,
		"formula":     types.StringType,
		"components":  types.ListType{ElemType: types.ObjectType{AttrTypes: map[string]attr.Type{}}},
	}

	tests := []struct {
		name        string
		rules       []map[string]attr.Value
		expectError bool
		errorMatch  string
	}{
		{
			name: "valid - create action with name",
			rules: []map[string]attr.Value{
				{
					"action":      types.StringValue("create"),
					"name":        types.StringValue("My Rule"),
					"description": types.StringNull(),
					"id":          types.StringNull(),
					"formula":     types.StringValue("A"),
					"components":  types.ListNull(types.ObjectType{AttrTypes: map[string]attr.Type{}}),
				},
			},
			expectError: false,
		},
		{
			name: "valid - select action without name",
			rules: []map[string]attr.Value{
				{
					"action":      types.StringValue("select"),
					"name":        types.StringNull(),
					"description": types.StringNull(),
					"id":          types.StringValue("some-id"),
					"formula":     types.StringNull(),
					"components":  types.ListNull(types.ObjectType{AttrTypes: map[string]attr.Type{}}),
				},
			},
			expectError: false,
		},
		{
			name: "invalid - create action without name",
			rules: []map[string]attr.Value{
				{
					"action":      types.StringValue("create"),
					"name":        types.StringNull(),
					"description": types.StringNull(),
					"id":          types.StringNull(),
					"formula":     types.StringValue("A"),
					"components":  types.ListNull(types.ObjectType{AttrTypes: map[string]attr.Type{}}),
				},
			},
			expectError: true,
			errorMatch:  "'name' is required when action is 'create'",
		},
		{
			name: "invalid - update action without name",
			rules: []map[string]attr.Value{
				{
					"action":      types.StringValue("update"),
					"name":        types.StringNull(),
					"description": types.StringNull(),
					"id":          types.StringValue("some-id"),
					"formula":     types.StringValue("A"),
					"components":  types.ListNull(types.ObjectType{AttrTypes: map[string]attr.Type{}}),
				},
			},
			expectError: true,
			errorMatch:  "'name' is required when action is 'update'",
		},
		{
			name: "invalid - create action with empty name",
			rules: []map[string]attr.Value{
				{
					"action":      types.StringValue("create"),
					"name":        types.StringValue(""),
					"description": types.StringNull(),
					"id":          types.StringNull(),
					"formula":     types.StringValue("A"),
					"components":  types.ListNull(types.ObjectType{AttrTypes: map[string]attr.Type{}}),
				},
			},
			expectError: true,
			errorMatch:  "'name' is required when action is 'create'",
		},
		{
			name: "valid - mixed rules with valid names",
			rules: []map[string]attr.Value{
				{
					"action":      types.StringValue("create"),
					"name":        types.StringValue("Rule 1"),
					"description": types.StringNull(),
					"id":          types.StringNull(),
					"formula":     types.StringValue("A"),
					"components":  types.ListNull(types.ObjectType{AttrTypes: map[string]attr.Type{}}),
				},
				{
					"action":      types.StringValue("select"),
					"name":        types.StringNull(),
					"description": types.StringNull(),
					"id":          types.StringValue("existing-id"),
					"formula":     types.StringNull(),
					"components":  types.ListNull(types.ObjectType{AttrTypes: map[string]attr.Type{}}),
				},
			},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Build the list of rule objects
			elements := make([]attr.Value, len(tt.rules))
			for i, rule := range tt.rules {
				objVal, diags := types.ObjectValue(ruleAttrTypes, rule)
				if diags.HasError() {
					t.Fatalf("failed to create object value: %v", diags)
				}
				elements[i] = objVal
			}

			listVal, diags := types.ListValue(types.ObjectType{AttrTypes: ruleAttrTypes}, elements)
			if diags.HasError() {
				t.Fatalf("failed to create list value: %v", diags)
			}

			req := validator.ListRequest{
				Path:        path.Root("rules"),
				ConfigValue: listVal,
			}
			resp := &validator.ListResponse{}

			v := allocationRulesValidator{}
			v.ValidateList(ctx, req, resp)

			hasError := resp.Diagnostics.HasError()
			if hasError != tt.expectError {
				t.Errorf("expected error=%v, got error=%v, diagnostics: %v", tt.expectError, hasError, resp.Diagnostics)
			}

			if tt.expectError && tt.errorMatch != "" {
				found := false
				for _, diag := range resp.Diagnostics {
					if contains(diag.Detail(), tt.errorMatch) {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("expected error containing %q, got: %v", tt.errorMatch, resp.Diagnostics)
				}
			}
		})
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 ||
		(len(s) > 0 && len(substr) > 0 && findSubstring(s, substr)))
}

func findSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
