package provider

import (
	"context"
	"testing"

	"github.com/doitintl/terraform-provider-doit/internal/provider/resource_allocation"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
)

func TestAllocationComponentsValidator(t *testing.T) {
	ctx := context.Background()

	makeComponent := func(key, mode, compType string) resource_allocation.ComponentsValue {
		val, diags := resource_allocation.NewComponentsValue(
			resource_allocation.ComponentsValue{}.AttributeTypes(ctx),
			map[string]attr.Value{
				"key":               types.StringValue(key),
				"mode":              types.StringValue(mode),
				"type":              types.StringValue(compType),
				"values":            types.ListValueMust(types.StringType, []attr.Value{types.StringValue("some-id")}),
				"case_insensitive":  types.BoolValue(false),
				"include_null":      types.BoolValue(false),
				"inverse_selection": types.BoolValue(false),
			},
		)
		if diags.HasError() {
			t.Fatalf("failed to create component value: %s", diags.Errors())
		}
		return val
	}

	makeList := func(components ...resource_allocation.ComponentsValue) basetypes.ListValue {
		elems := make([]attr.Value, len(components))
		for i, c := range components {
			elems[i] = c
		}
		list, diags := types.ListValueFrom(ctx, resource_allocation.ComponentsValue{}.Type(ctx), elems)
		if diags.HasError() {
			t.Fatalf("failed to create list value: %s", diags.Errors())
		}
		return list
	}

	tests := []struct {
		name        string
		components  basetypes.ListValue
		expectError bool
	}{
		{
			name:        "valid - allocation_rule with correct key and mode 'is'",
			components:  makeList(makeComponent("allocation_rule", "is", "allocation_rule")),
			expectError: false,
		},
		{
			name:        "valid - allocation_rule with correct key and mode 'contains'",
			components:  makeList(makeComponent("allocation_rule", "contains", "allocation_rule")),
			expectError: false,
		},
		{
			name:        "valid - non-allocation_rule type (no constraints)",
			components:  makeList(makeComponent("country", "starts_with", "fixed")),
			expectError: false,
		},
		{
			name:        "valid - mixed types (allocation_rule valid + fixed)",
			components:  makeList(makeComponent("allocation_rule", "is", "allocation_rule"), makeComponent("country", "regexp", "fixed")),
			expectError: false,
		},
		{
			name:        "invalid - allocation_rule with wrong key",
			components:  makeList(makeComponent("country", "is", "allocation_rule")),
			expectError: true,
		},
		{
			name:        "invalid - allocation_rule with unsupported mode 'starts_with'",
			components:  makeList(makeComponent("allocation_rule", "starts_with", "allocation_rule")),
			expectError: true,
		},
		{
			name:        "invalid - allocation_rule with unsupported mode 'regexp'",
			components:  makeList(makeComponent("allocation_rule", "regexp", "allocation_rule")),
			expectError: true,
		},
		{
			name:        "invalid - allocation_rule with unsupported mode 'ends_with'",
			components:  makeList(makeComponent("allocation_rule", "ends_with", "allocation_rule")),
			expectError: true,
		},
		{
			name:        "invalid - allocation_rule with wrong key and wrong mode",
			components:  makeList(makeComponent("country", "starts_with", "allocation_rule")),
			expectError: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			v := allocationComponentsValidator{}
			req := validator.ListRequest{
				ConfigValue: tc.components,
			}
			resp := &validator.ListResponse{}

			v.ValidateList(ctx, req, resp)

			if tc.expectError && !resp.Diagnostics.HasError() {
				t.Error("expected error but got none")
			}
			if !tc.expectError && resp.Diagnostics.HasError() {
				t.Errorf("expected no error but got: %s", resp.Diagnostics.Errors())
			}
		})
	}
}

func TestAllocationComponentsValidator_NullAndUnknown(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name  string
		value basetypes.ListValue
	}{
		{
			name:  "null list skips validation",
			value: types.ListNull(resource_allocation.ComponentsValue{}.Type(ctx)),
		},
		{
			name:  "unknown list skips validation",
			value: types.ListUnknown(resource_allocation.ComponentsValue{}.Type(ctx)),
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			v := allocationComponentsValidator{}
			req := validator.ListRequest{
				ConfigValue: tc.value,
			}
			resp := &validator.ListResponse{}

			v.ValidateList(ctx, req, resp)

			if resp.Diagnostics.HasError() {
				t.Errorf("expected no error for %s, got: %s", tc.name, resp.Diagnostics.Errors())
			}
		})
	}
}
