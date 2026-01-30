package provider

import (
	"context"
	"strings"
	"testing"

	"github.com/doitintl/terraform-provider-doit/internal/provider/resource_allocation"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
)

// createRulesValue creates a RulesValue for testing purposes using NewRulesValueMust
// to ensure the state field is properly initialized (vs zero-value which means null).
func createRulesValue(ctx context.Context, action, name string, nameIsNull bool) resource_allocation.RulesValue {
	var nameVal basetypes.StringValue
	if nameIsNull {
		nameVal = basetypes.NewStringNull()
	} else {
		nameVal = basetypes.NewStringValue(name)
	}

	// Get attribute types from RulesValue
	attrTypes := resource_allocation.RulesValue{}.AttributeTypes(ctx)

	// Build attributes map
	attributes := map[string]attr.Value{
		"action":      basetypes.NewStringValue(action),
		"name":        nameVal,
		"description": basetypes.NewStringNull(),
		"id":          basetypes.NewStringNull(),
		"formula":     basetypes.NewStringValue("A"),
		"components":  basetypes.NewListNull(attrTypes["components"].(types.ListType).ElemType),
	}

	return resource_allocation.NewRulesValueMust(attrTypes, attributes)
}

// createRulesListValue creates a types.List containing RulesValue elements for unit testing.
func createRulesListValue(ctx context.Context, rules []resource_allocation.RulesValue) (types.List, bool) {
	// Convert RulesValue to attr.Value slice
	elements := make([]attr.Value, len(rules))
	for i, rule := range rules {
		elements[i] = rule
	}

	// Get the RulesType from a RulesValue
	rulesType := resource_allocation.RulesType{
		ObjectType: basetypes.ObjectType{
			AttrTypes: resource_allocation.RulesValue{}.AttributeTypes(ctx),
		},
	}

	listVal, diags := types.ListValue(rulesType, elements)
	return listVal, diags.HasError()
}

// ruleSpec is a simplified spec for creating RulesValue in tests.
type ruleSpec struct {
	action     string
	name       string
	nameIsNull bool
}

func TestAllocationRulesValidator(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name        string
		ruleSpecs   []ruleSpec
		expectError bool
		errorMatch  string
	}{
		{
			name: "valid - create action with name",
			ruleSpecs: []ruleSpec{
				{action: "create", name: "My Rule", nameIsNull: false},
			},
			expectError: false,
		},
		{
			name: "valid - select action without name",
			ruleSpecs: []ruleSpec{
				{action: "select", name: "", nameIsNull: true},
			},
			expectError: false,
		},
		{
			name: "invalid - create action without name (null)",
			ruleSpecs: []ruleSpec{
				{action: "create", name: "", nameIsNull: true},
			},
			expectError: true,
			errorMatch:  "'name' is required when action is 'create'",
		},
		{
			name: "invalid - update action without name",
			ruleSpecs: []ruleSpec{
				{action: "update", name: "", nameIsNull: true},
			},
			expectError: true,
			errorMatch:  "'name' is required when action is 'update'",
		},
		{
			name: "invalid - create action with empty name",
			ruleSpecs: []ruleSpec{
				{action: "create", name: "", nameIsNull: false},
			},
			expectError: true,
			errorMatch:  "'name' is required when action is 'create'",
		},
		{
			name: "valid - mixed rules with create having name",
			ruleSpecs: []ruleSpec{
				{action: "create", name: "Rule 1", nameIsNull: false},
				{action: "select", name: "", nameIsNull: true},
			},
			expectError: false,
		},
		{
			name: "invalid - second rule missing name",
			ruleSpecs: []ruleSpec{
				{action: "create", name: "Rule 1", nameIsNull: false},
				{action: "create", name: "", nameIsNull: true}, // missing name
			},
			expectError: true,
			errorMatch:  "rules[1]: 'name' is required when action is 'create'",
		},
		{
			name: "valid - update action with name",
			ruleSpecs: []ruleSpec{
				{action: "update", name: "Updated Rule", nameIsNull: false},
			},
			expectError: false,
		},
		{
			name:        "valid - empty rules list",
			ruleSpecs:   []ruleSpec{},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Build rules from specs
			rules := make([]resource_allocation.RulesValue, len(tt.ruleSpecs))
			for i, spec := range tt.ruleSpecs {
				rules[i] = createRulesValue(ctx, spec.action, spec.name, spec.nameIsNull)
			}

			listVal, hasErr := createRulesListValue(ctx, rules)
			if hasErr {
				t.Fatalf("failed to create list value")
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
					if strings.Contains(diag.Detail(), tt.errorMatch) {
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

func TestAllocationRulesValidator_NullAndUnknown(t *testing.T) {
	ctx := context.Background()
	v := allocationRulesValidator{}

	t.Run("null list skips validation", func(t *testing.T) {
		rulesType := resource_allocation.RulesType{
			ObjectType: basetypes.ObjectType{
				AttrTypes: resource_allocation.RulesValue{}.AttributeTypes(ctx),
			},
		}
		req := validator.ListRequest{
			Path:        path.Root("rules"),
			ConfigValue: types.ListNull(rulesType),
		}
		resp := &validator.ListResponse{}
		v.ValidateList(ctx, req, resp)

		if resp.Diagnostics.HasError() {
			t.Errorf("expected no error for null list, got: %v", resp.Diagnostics)
		}
	})

	t.Run("unknown list skips validation", func(t *testing.T) {
		rulesType := resource_allocation.RulesType{
			ObjectType: basetypes.ObjectType{
				AttrTypes: resource_allocation.RulesValue{}.AttributeTypes(ctx),
			},
		}
		req := validator.ListRequest{
			Path:        path.Root("rules"),
			ConfigValue: types.ListUnknown(rulesType),
		}
		resp := &validator.ListResponse{}
		v.ValidateList(ctx, req, resp)

		if resp.Diagnostics.HasError() {
			t.Errorf("expected no error for unknown list, got: %v", resp.Diagnostics)
		}
	})
}
