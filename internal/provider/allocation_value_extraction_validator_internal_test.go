package provider

import (
	"context"
	"testing"

	"github.com/doitintl/terraform-provider-doit/internal/provider/resource_allocation"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
)

func createTestValueExtractionObject(
	ctx context.Context,
	t *testing.T,
	onMissing basetypes.StringValue,
	fallback basetypes.StringValue,
) basetypes.ObjectValue {
	t.Helper()
	attrTypes := resource_allocation.ValueExtractionValue{}.AttributeTypes(ctx)
	sourcesType := attrTypes["sources"].(types.ListType).ElemType
	attributes := map[string]attr.Value{
		"fallback":   fallback,
		"on_missing": onMissing,
		"sources":    basetypes.NewListNull(sourcesType),
	}
	veVal := resource_allocation.NewValueExtractionValueMust(attrTypes, attributes)
	objVal, diags := veVal.ToObjectValue(ctx)
	if diags.HasError() {
		t.Fatalf("failed to create object value: %v", diags)
	}
	return objVal
}

func TestAllocationValueExtractionValidator(t *testing.T) {
	ctx := t.Context()

	testPath := path.Root("rules").AtListIndex(0).AtName("value_extraction")

	tests := []struct {
		name          string
		configValue   basetypes.ObjectValue
		expectError   bool
		expectedDiag  string
		expectedPath  string
		severityCount int
	}{
		{
			name:        "whole object null is skipped",
			configValue: basetypes.NewObjectNull(resource_allocation.ValueExtractionValue{}.AttributeTypes(ctx)),
			expectError: false,
		},
		{
			name:        "whole object unknown is skipped",
			configValue: basetypes.NewObjectUnknown(resource_allocation.ValueExtractionValue{}.AttributeTypes(ctx)),
			expectError: false,
		},
		{
			name: "on_missing unknown with fallback known is deferred",
			configValue: createTestValueExtractionObject(
				ctx,
				t,
				basetypes.NewStringUnknown(),
				basetypes.NewStringValue("default_val"),
			),
			expectError: false,
		},
		{
			name: "on_missing unknown with fallback null is deferred",
			configValue: createTestValueExtractionObject(
				ctx,
				t,
				basetypes.NewStringUnknown(),
				basetypes.NewStringNull(),
			),
			expectError: false,
		},
		{
			name: "on_missing useFallback with fallback unknown is deferred",
			configValue: createTestValueExtractionObject(
				ctx,
				t,
				basetypes.NewStringValue("useFallback"),
				basetypes.NewStringUnknown(),
			),
			expectError: false,
		},
		{
			name: "on_missing nextRule with fallback unknown is deferred",
			configValue: createTestValueExtractionObject(
				ctx,
				t,
				basetypes.NewStringValue("nextRule"),
				basetypes.NewStringUnknown(),
			),
			expectError: false,
		},
		{
			name: "on_missing null (defaults to useFallback) with fallback unknown is deferred",
			configValue: createTestValueExtractionObject(
				ctx,
				t,
				basetypes.NewStringNull(),
				basetypes.NewStringUnknown(),
			),
			expectError: false,
		},
		{
			name: "useFallback with valid fallback passes",
			configValue: createTestValueExtractionObject(
				ctx,
				t,
				basetypes.NewStringValue("useFallback"),
				basetypes.NewStringValue("my_fallback"),
			),
			expectError: false,
		},
		{
			name: "null on_missing (defaults to useFallback) with valid fallback passes",
			configValue: createTestValueExtractionObject(
				ctx,
				t,
				basetypes.NewStringNull(),
				basetypes.NewStringValue("my_fallback"),
			),
			expectError: false,
		},
		{
			name: "useFallback with null fallback fails",
			configValue: createTestValueExtractionObject(
				ctx,
				t,
				basetypes.NewStringValue("useFallback"),
				basetypes.NewStringNull(),
			),
			expectError:   true,
			expectedDiag:  "fallback is required when on_missing is 'useFallback'",
			expectedPath:  "rules[0].value_extraction.fallback",
			severityCount: 1,
		},
		{
			name: "useFallback with empty fallback fails",
			configValue: createTestValueExtractionObject(
				ctx,
				t,
				basetypes.NewStringValue("useFallback"),
				basetypes.NewStringValue(""),
			),
			expectError:   true,
			expectedDiag:  "fallback is required when on_missing is 'useFallback'",
			expectedPath:  "rules[0].value_extraction.fallback",
			severityCount: 1,
		},
		{
			name: "null on_missing with null fallback fails",
			configValue: createTestValueExtractionObject(
				ctx,
				t,
				basetypes.NewStringNull(),
				basetypes.NewStringNull(),
			),
			expectError:   true,
			expectedDiag:  "fallback is required when on_missing is 'useFallback'",
			expectedPath:  "rules[0].value_extraction.fallback",
			severityCount: 1,
		},
		{
			name: "nextRule with null fallback passes",
			configValue: createTestValueExtractionObject(
				ctx,
				t,
				basetypes.NewStringValue("nextRule"),
				basetypes.NewStringNull(),
			),
			expectError: false,
		},
		{
			name: "nextRule with configured fallback fails",
			configValue: createTestValueExtractionObject(
				ctx,
				t,
				basetypes.NewStringValue("nextRule"),
				basetypes.NewStringValue("not_allowed"),
			),
			expectError:   true,
			expectedDiag:  "fallback is not allowed when on_missing is 'nextRule'",
			expectedPath:  "rules[0].value_extraction.fallback",
			severityCount: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := validator.ObjectRequest{
				Path:        testPath,
				ConfigValue: tt.configValue,
			}
			resp := &validator.ObjectResponse{}

			v := allocationValueExtractionValidator{}
			v.ValidateObject(ctx, req, resp)

			if tt.expectError {
				if !resp.Diagnostics.HasError() {
					t.Fatalf("expected error, got none")
				}
				if len(resp.Diagnostics.Errors()) != tt.severityCount {
					t.Errorf("expected %d error(s), got %d: %v", tt.severityCount, len(resp.Diagnostics.Errors()), resp.Diagnostics)
				}
				found := false
				for _, d := range resp.Diagnostics.Errors() {
					if d.Detail() == tt.expectedDiag {
						if withPath, ok := d.(diag.DiagnosticWithPath); ok {
							if withPath.Path().String() == tt.expectedPath {
								found = true
								break
							}
						}
					}
				}
				if !found {
					t.Errorf("expected error diag %q at path %q, got: %v", tt.expectedDiag, tt.expectedPath, resp.Diagnostics)
				}
			} else {
				if resp.Diagnostics.HasError() {
					t.Fatalf("expected no error, got: %v", resp.Diagnostics)
				}
			}
		})
	}
}

func TestAllocationValueExtractionValidator_Descriptions(t *testing.T) {
	ctx := t.Context()
	v := allocationValueExtractionValidator{}
	if v.Description(ctx) == "" {
		t.Error("expected non-empty description")
	}
	if v.MarkdownDescription(ctx) == "" {
		t.Error("expected non-empty markdown description")
	}
}
