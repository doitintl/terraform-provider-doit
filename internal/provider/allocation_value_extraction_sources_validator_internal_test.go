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

func createTestSource(
	ctx context.Context,
	t *testing.T,
	key basetypes.StringValue,
	sourceType basetypes.StringValue,
	providers basetypes.ListValue,
) resource_allocation.SourcesValue {
	t.Helper()
	attrTypes := resource_allocation.SourcesValue{}.AttributeTypes(ctx)
	attributes := map[string]attr.Value{
		"key":       key,
		"type":      sourceType,
		"providers": providers,
	}
	return resource_allocation.NewSourcesValueMust(attrTypes, attributes)
}

func TestAllocationValueExtractionSourcesValidator(t *testing.T) {
	ctx := t.Context()
	elemType := resource_allocation.SourcesValue{}.Type(ctx)
	testPath := path.Root("rules").AtListIndex(0).AtName("value_extraction").AtName("sources")

	tests := []struct {
		name          string
		configValue   basetypes.ListValue
		expectError   bool
		expectedDiag  string
		expectedPath  string
		severityCount int
	}{
		{
			name:        "whole list null is skipped",
			configValue: basetypes.NewListNull(elemType),
			expectError: false,
		},
		{
			name:        "whole list unknown is skipped",
			configValue: basetypes.NewListUnknown(elemType),
			expectError: false,
		},
		{
			name:        "empty list passes",
			configValue: basetypes.NewListValueMust(elemType, []attr.Value{}),
			expectError: false,
		},
		{
			name: "single valid source passes",
			configValue: basetypes.NewListValueMust(elemType, []attr.Value{
				createTestSource(ctx, t, types.StringValue("env"), types.StringValue("label"), types.ListNull(types.StringType)),
			}),
			expectError: false,
		},
		{
			name: "single valid source with providers passes",
			configValue: basetypes.NewListValueMust(elemType, []attr.Value{
				createTestSource(ctx, t, types.StringValue("env"), types.StringValue("tag"), types.ListValueMust(types.StringType, []attr.Value{types.StringValue("amazon-web-services")})),
			}),
			expectError: false,
		},
		{
			name: "single source null element fails",
			configValue: basetypes.NewListValueMust(elemType, []attr.Value{
				resource_allocation.NewSourcesValueNull(),
			}),
			expectError:   true,
			expectedDiag:  "sources[0]: null element is not permitted",
			expectedPath:  "rules[0].value_extraction.sources[0]",
			severityCount: 1,
		},
		{
			name: "single source unknown element passes",
			configValue: basetypes.NewListValueMust(elemType, []attr.Value{
				resource_allocation.NewSourcesValueUnknown(),
			}),
			expectError: false,
		},
		{
			name: "multi-source unknown element before null element still reports error",
			configValue: basetypes.NewListValueMust(elemType, []attr.Value{
				resource_allocation.NewSourcesValueUnknown(),
				resource_allocation.NewSourcesValueNull(),
			}),
			expectError:   true,
			expectedDiag:  "sources[1]: null element is not permitted",
			expectedPath:  "rules[0].value_extraction.sources[1]",
			severityCount: 1,
		},
		{
			name: "multi-source unknown element after null element still reports error",
			configValue: basetypes.NewListValueMust(elemType, []attr.Value{
				resource_allocation.NewSourcesValueNull(),
				resource_allocation.NewSourcesValueUnknown(),
			}),
			expectError:   true,
			expectedDiag:  "sources[0]: null element is not permitted",
			expectedPath:  "rules[0].value_extraction.sources[0]",
			severityCount: 1,
		},
		{
			name: "multiple valid sources pass",
			configValue: basetypes.NewListValueMust(elemType, []attr.Value{
				createTestSource(ctx, t, types.StringValue("env"), types.StringValue("label"), types.ListNull(types.StringType)),
				createTestSource(ctx, t, types.StringValue("project_id"), types.StringValue("fixed"), types.ListNull(types.StringType)),
			}),
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := validator.ListRequest{
				Path:        testPath,
				ConfigValue: tt.configValue,
			}
			resp := &validator.ListResponse{}

			v := allocationValueExtractionSourcesValidator{}
			v.ValidateList(ctx, req, resp)

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

func TestAllocationValueExtractionSourcesValidator_Descriptions(t *testing.T) {
	ctx := t.Context()
	v := allocationValueExtractionSourcesValidator{}
	if v.Description(ctx) == "" {
		t.Error("expected non-empty description")
	}
	if v.MarkdownDescription(ctx) == "" {
		t.Error("expected non-empty markdown description")
	}
}
