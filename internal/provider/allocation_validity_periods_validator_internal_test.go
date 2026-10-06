package provider

import (
	"context"
	"testing"

	"github.com/doitintl/terraform-provider-doit/internal/provider/resource_allocation"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
)

func createTestPeriod(
	ctx context.Context,
	t *testing.T,
	startDate basetypes.StringValue,
	endDate basetypes.StringValue,
) resource_allocation.ValidityPeriodsValue {
	t.Helper()
	attrTypes := resource_allocation.ValidityPeriodsValue{}.AttributeTypes(ctx)
	attributes := map[string]attr.Value{
		"start_date": startDate,
		"end_date":   endDate,
	}
	return resource_allocation.NewValidityPeriodsValueMust(attrTypes, attributes)
}

func TestAllocationValidityPeriodsValidator(t *testing.T) {
	ctx := t.Context()
	elemType := resource_allocation.ValidityPeriodsValue{}.Type(ctx)
	testPath := path.Root("rules").AtListIndex(0).AtName("validity_periods")

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
			name: "single period fully bounded passes",
			configValue: basetypes.NewListValueMust(elemType, []attr.Value{
				createTestPeriod(ctx, t, basetypes.NewStringValue("2026-01-01"), basetypes.NewStringValue("2026-01-31")),
			}),
			expectError: false,
		},
		{
			name: "single period open start passes",
			configValue: basetypes.NewListValueMust(elemType, []attr.Value{
				createTestPeriod(ctx, t, basetypes.NewStringNull(), basetypes.NewStringValue("2026-01-31")),
			}),
			expectError: false,
		},
		{
			name: "single period open end passes",
			configValue: basetypes.NewListValueMust(elemType, []attr.Value{
				createTestPeriod(ctx, t, basetypes.NewStringValue("2026-01-01"), basetypes.NewStringNull()),
			}),
			expectError: false,
		},
		{
			name: "single period unbounded passes",
			configValue: basetypes.NewListValueMust(elemType, []attr.Value{
				createTestPeriod(ctx, t, basetypes.NewStringNull(), basetypes.NewStringNull()),
			}),
			expectError: false,
		},
		{
			name: "single period unknown element passes",
			configValue: basetypes.NewListValueMust(elemType, []attr.Value{
				resource_allocation.NewValidityPeriodsValueUnknown(),
			}),
			expectError: false,
		},
		{
			name: "single period null element passes",
			configValue: basetypes.NewListValueMust(elemType, []attr.Value{
				resource_allocation.NewValidityPeriodsValueNull(),
			}),
			expectError: false,
		},
		{
			name: "single period unknown start_date is deferred",
			configValue: basetypes.NewListValueMust(elemType, []attr.Value{
				createTestPeriod(ctx, t, basetypes.NewStringUnknown(), basetypes.NewStringValue("2026-01-31")),
			}),
			expectError: false,
		},
		{
			name: "single period unknown end_date is deferred",
			configValue: basetypes.NewListValueMust(elemType, []attr.Value{
				createTestPeriod(ctx, t, basetypes.NewStringValue("2026-01-01"), basetypes.NewStringUnknown()),
			}),
			expectError: false,
		},
		{
			name: "single period start_date after end_date fails",
			configValue: basetypes.NewListValueMust(elemType, []attr.Value{
				createTestPeriod(ctx, t, basetypes.NewStringValue("2026-02-01"), basetypes.NewStringValue("2026-01-01")),
			}),
			expectError:   true,
			expectedDiag:  "validity_periods[0]: start_date (2026-02-01) must be on or before end_date (2026-01-01)",
			expectedPath:  "rules[0].validity_periods[0]",
			severityCount: 1,
		},
		{
			name: "single period start_date equal to end_date passes",
			configValue: basetypes.NewListValueMust(elemType, []attr.Value{
				createTestPeriod(ctx, t, basetypes.NewStringValue("2026-01-01"), basetypes.NewStringValue("2026-01-01")),
			}),
			expectError: false,
		},
		{
			name: "multi-period valid open outer boundaries passes",
			configValue: basetypes.NewListValueMust(elemType, []attr.Value{
				createTestPeriod(ctx, t, basetypes.NewStringNull(), basetypes.NewStringValue("2026-03-14")),
				createTestPeriod(ctx, t, basetypes.NewStringValue("2026-03-15"), basetypes.NewStringValue("2026-06-15")),
				createTestPeriod(ctx, t, basetypes.NewStringValue("2026-06-16"), basetypes.NewStringNull()),
			}),
			expectError: false,
		},
		{
			name: "multi-period missing start_date on second period fails",
			configValue: basetypes.NewListValueMust(elemType, []attr.Value{
				createTestPeriod(ctx, t, basetypes.NewStringValue("2026-01-01"), basetypes.NewStringValue("2026-03-14")),
				createTestPeriod(ctx, t, basetypes.NewStringNull(), basetypes.NewStringValue("2026-06-15")),
			}),
			expectError:   true,
			expectedDiag:  "validity_periods[1]: only the first period in the list may omit start_date",
			expectedPath:  "rules[0].validity_periods[1].start_date",
			severityCount: 1,
		},
		{
			name: "multi-period missing end_date on first period fails",
			configValue: basetypes.NewListValueMust(elemType, []attr.Value{
				createTestPeriod(ctx, t, basetypes.NewStringValue("2026-01-01"), basetypes.NewStringNull()),
				createTestPeriod(ctx, t, basetypes.NewStringValue("2026-06-16"), basetypes.NewStringValue("2026-08-01")),
			}),
			expectError:   true,
			expectedDiag:  "validity_periods[0]: only the last period in the list may omit end_date",
			expectedPath:  "rules[0].validity_periods[0].end_date",
			severityCount: 1,
		},
		{
			name: "multi-period overlapping consecutive periods fails",
			configValue: basetypes.NewListValueMust(elemType, []attr.Value{
				createTestPeriod(ctx, t, basetypes.NewStringValue("2026-01-01"), basetypes.NewStringValue("2026-03-15")),
				createTestPeriod(ctx, t, basetypes.NewStringValue("2026-03-15"), basetypes.NewStringValue("2026-06-15")),
			}),
			expectError:   true,
			expectedDiag:  "validity_periods[1]: start_date (2026-03-15) must be after previous period's end_date (2026-03-15)",
			expectedPath:  "rules[0].validity_periods[1].start_date",
			severityCount: 1,
		},
		{
			name: "multi-period out of order consecutive periods fails",
			configValue: basetypes.NewListValueMust(elemType, []attr.Value{
				createTestPeriod(ctx, t, basetypes.NewStringValue("2026-06-01"), basetypes.NewStringValue("2026-08-01")),
				createTestPeriod(ctx, t, basetypes.NewStringValue("2026-01-01"), basetypes.NewStringValue("2026-03-01")),
			}),
			expectError:   true,
			expectedDiag:  "validity_periods[1]: start_date (2026-01-01) must be after previous period's end_date (2026-08-01)",
			expectedPath:  "rules[0].validity_periods[1].start_date",
			severityCount: 1,
		},
		{
			name: "multi-period unknown element before invalid period still reports error",
			configValue: basetypes.NewListValueMust(elemType, []attr.Value{
				resource_allocation.NewValidityPeriodsValueUnknown(),
				createTestPeriod(ctx, t, basetypes.NewStringValue("2026-02-01"), basetypes.NewStringValue("2026-01-01")),
			}),
			expectError:   true,
			expectedDiag:  "validity_periods[1]: start_date (2026-02-01) must be on or before end_date (2026-01-01)",
			expectedPath:  "rules[0].validity_periods[1]",
			severityCount: 1,
		},
		{
			name: "multi-period unknown element after invalid period still reports error",
			configValue: basetypes.NewListValueMust(elemType, []attr.Value{
				createTestPeriod(ctx, t, basetypes.NewStringValue("2026-02-01"), basetypes.NewStringValue("2026-01-01")),
				resource_allocation.NewValidityPeriodsValueUnknown(),
			}),
			expectError:   true,
			expectedDiag:  "validity_periods[0]: start_date (2026-02-01) must be on or before end_date (2026-01-01)",
			expectedPath:  "rules[0].validity_periods[0]",
			severityCount: 1,
		},
		{
			name: "multi-period with unknown end_date defers consecutive check",
			configValue: basetypes.NewListValueMust(elemType, []attr.Value{
				createTestPeriod(ctx, t, basetypes.NewStringValue("2026-01-01"), basetypes.NewStringUnknown()),
				createTestPeriod(ctx, t, basetypes.NewStringValue("2026-03-15"), basetypes.NewStringValue("2026-06-15")),
			}),
			expectError: false,
		},
		{
			name: "multi-period with unknown start_date defers consecutive check",
			configValue: basetypes.NewListValueMust(elemType, []attr.Value{
				createTestPeriod(ctx, t, basetypes.NewStringValue("2026-01-01"), basetypes.NewStringValue("2026-03-14")),
				createTestPeriod(ctx, t, basetypes.NewStringUnknown(), basetypes.NewStringValue("2026-06-15")),
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

			v := allocationValidityPeriodsValidator{}
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

func TestAllocationValidityPeriodsValidator_Descriptions(t *testing.T) {
	ctx := t.Context()
	v := allocationValidityPeriodsValidator{}
	if v.Description(ctx) == "" {
		t.Error("expected non-empty description")
	}
	if v.MarkdownDescription(ctx) == "" {
		t.Error("expected non-empty markdown description")
	}
}
