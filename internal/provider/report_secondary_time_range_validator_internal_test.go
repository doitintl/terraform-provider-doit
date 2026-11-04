package provider

import (
	"context"
	"strings"
	"testing"

	"github.com/doitintl/terraform-provider-doit/internal/provider/resource_report"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

func secondaryTestCTR(ctx context.Context, from, to types.String) resource_report.CustomTimeRangeValue {
	return resource_report.NewCustomTimeRangeValueMust(resource_report.CustomTimeRangeValue{}.AttributeTypes(ctx), map[string]attr.Value{
		"from": from,
		"to":   to,
	})
}

func secondaryTestTimeRange(ctx context.Context, mode, unit types.String, amount types.Int64) resource_report.TimeRangeValue {
	return resource_report.NewTimeRangeValueMust(resource_report.TimeRangeValue{}.AttributeTypes(ctx), map[string]attr.Value{
		"amount":          amount,
		"include_current": types.BoolValue(false),
		"mode":            mode,
		"unit":            unit,
	})
}

func secondaryTestRelative(ctx context.Context, amount types.Int64, unit types.String, includeCurrent types.Bool) resource_report.SecondaryTimeRangeValue {
	return resource_report.NewSecondaryTimeRangeValueMust(resource_report.SecondaryTimeRangeValue{}.AttributeTypes(ctx), map[string]attr.Value{
		"amount":            amount,
		"custom_time_range": resource_report.NewCustomTimeRangeValueNull(),
		"include_current":   includeCurrent,
		"unit":              unit,
	})
}

func secondaryTestCustom(ctx context.Context, ctr resource_report.CustomTimeRangeValue) resource_report.SecondaryTimeRangeValue {
	return resource_report.NewSecondaryTimeRangeValueMust(resource_report.SecondaryTimeRangeValue{}.AttributeTypes(ctx), map[string]attr.Value{
		"amount":            types.Int64Null(),
		"custom_time_range": ctr,
		"include_current":   types.BoolNull(),
		"unit":              types.StringNull(),
	})
}

func buildSecondaryTestConfig(ctx context.Context, t *testing.T, tr resource_report.TimeRangeValue, ctr resource_report.CustomTimeRangeValue, sec resource_report.SecondaryTimeRangeValue) tfsdk.Config {
	t.Helper()
	cfgMap := validCumulativeComparisonConfigMap(ctx, t)
	cfgMap["layout"] = types.StringValue("table")
	cfgMap["time_range"] = tr
	cfgMap["custom_time_range"] = ctr
	cfgMap["secondary_time_range"] = sec
	return buildReportConfigWithMap(ctx, t, cfgMap)
}

func TestValidateReportSecondaryTimeRange(t *testing.T) {
	ctx := t.Context()
	s := types.StringValue
	i := types.Int64Value

	// Primary range used by the custom-primary cases: all of 2024 (UTC).
	primary := secondaryTestCTR(ctx, s("2024-01-01T00:00:00Z"), s("2024-12-31T23:59:59Z"))
	trCustom := secondaryTestTimeRange(ctx, s("custom"), types.StringNull(), types.Int64Null())
	trLast := func(amount int64, unit string) resource_report.TimeRangeValue {
		return secondaryTestTimeRange(ctx, s("last"), s(unit), i(amount))
	}
	custom := func(from, to string) resource_report.SecondaryTimeRangeValue {
		return secondaryTestCustom(ctx, secondaryTestCTR(ctx, s(from), s(to)))
	}
	yearShift := secondaryTestRelative(ctx, i(1), s("year"), types.BoolValue(false))
	nullCTR := resource_report.NewCustomTimeRangeValueNull()
	nullSec := resource_report.NewSecondaryTimeRangeValueNull()
	nullTR := resource_report.NewTimeRangeValueNull()

	secCTRPath := path.Root("config").AtName("secondary_time_range").AtName("custom_time_range")
	secPath := path.Root("config").AtName("secondary_time_range")
	amountPath := secPath.AtName("amount")
	const overlap = "Overlapping Secondary Time Range"
	const badAmount = "Invalid Secondary Time Range Amount"
	const inheritedNote = "kept from the existing report"

	type prior struct {
		tr  resource_report.TimeRangeValue
		ctr resource_report.CustomTimeRangeValue
		sec resource_report.SecondaryTimeRangeValue
	}

	tests := []struct {
		name         string
		tr           resource_report.TimeRangeValue
		ctr          resource_report.CustomTimeRangeValue
		sec          resource_report.SecondaryTimeRangeValue
		prior        *prior
		wantCount    int
		wantSummary  string
		wantPath     path.Path
		wantInDetail string
	}{
		// Custom primary and custom secondary.
		{name: "custom secondary ending the day before the primary starts", tr: trCustom, ctr: primary, sec: custom("2023-01-01T00:00:00Z", "2023-12-31T23:59:59Z")},
		{name: "custom secondary with a negative offset staying on the day before", tr: trCustom, ctr: primary, sec: custom("2023-01-01T00:00:00-05:00", "2023-12-31T18:59:59-05:00")},
		{
			name: "custom secondary with a negative offset crossing into the primary's first UTC day",
			tr:   trCustom, ctr: primary, sec: custom("2023-01-01T00:00:00-05:00", "2023-12-31T23:59:59-05:00"),
			wantCount: 1, wantSummary: overlap, wantPath: secCTRPath, wantInDetail: "End the secondary range on 2023-12-31 or earlier",
		},
		{name: "custom secondary ending on the primary's first day", tr: trCustom, ctr: primary, sec: custom("2023-01-01T00:00:00Z", "2024-01-01T00:00:00Z"), wantCount: 1, wantSummary: overlap, wantPath: secCTRPath},
		{name: "custom secondary containing the primary", tr: trCustom, ctr: primary, sec: custom("2023-01-01T00:00:00Z", "2025-12-31T00:00:00Z"), wantCount: 1, wantSummary: overlap, wantPath: secCTRPath},
		{name: "custom secondary starting on the primary's last day", tr: trCustom, ctr: primary, sec: custom("2024-12-31T00:00:00Z", "2025-06-30T00:00:00Z"), wantCount: 1, wantSummary: overlap, wantPath: secCTRPath},
		{name: "custom secondary entirely after the primary", tr: trCustom, ctr: primary, sec: custom("2025-01-01T00:00:00Z", "2025-06-30T00:00:00Z")},
		{name: "custom secondary with an unparseable bound is left to the timestamp validator", tr: trCustom, ctr: primary, sec: custom("2024-02-01", "2024-03-01T00:00:00Z")},
		{name: "custom secondary with to before from is left to the API", tr: trCustom, ctr: primary, sec: custom("2024-03-01T00:00:00Z", "2024-02-01T00:00:00Z")},
		{name: "primary mode last is not compared with a custom secondary", tr: trLast(3, "month"), ctr: primary, sec: custom("2024-02-01T00:00:00Z", "2024-03-01T00:00:00Z")},
		{name: "null primary custom_time_range", tr: trCustom, ctr: nullCTR, sec: custom("2024-02-01T00:00:00Z", "2024-03-01T00:00:00Z")},
		{name: "null secondary", tr: trCustom, ctr: primary, sec: nullSec},
		{name: "null secondary from", tr: trCustom, ctr: primary, sec: secondaryTestCustom(ctx, secondaryTestCTR(ctx, types.StringNull(), s("2024-03-01T00:00:00Z")))},
		{name: "null primary to", tr: trCustom, ctr: secondaryTestCTR(ctx, s("2024-01-01T00:00:00Z"), types.StringNull()), sec: custom("2024-02-01T00:00:00Z", "2024-03-01T00:00:00Z")},

		// Unknown inputs defer.
		{name: "unknown primary from", tr: trCustom, ctr: secondaryTestCTR(ctx, types.StringUnknown(), s("2024-12-31T23:59:59Z")), sec: custom("2024-02-01T00:00:00Z", "2024-03-01T00:00:00Z")},
		{name: "unknown primary to", tr: trCustom, ctr: secondaryTestCTR(ctx, s("2024-01-01T00:00:00Z"), types.StringUnknown()), sec: custom("2024-02-01T00:00:00Z", "2024-03-01T00:00:00Z")},
		{name: "unknown secondary from", tr: trCustom, ctr: primary, sec: secondaryTestCustom(ctx, secondaryTestCTR(ctx, types.StringUnknown(), s("2024-03-01T00:00:00Z")))},
		{name: "unknown secondary to", tr: trCustom, ctr: primary, sec: secondaryTestCustom(ctx, secondaryTestCTR(ctx, s("2024-02-01T00:00:00Z"), types.StringUnknown()))},
		{name: "unknown primary custom_time_range", tr: trCustom, ctr: resource_report.NewCustomTimeRangeValueUnknown(), sec: custom("2024-02-01T00:00:00Z", "2024-03-01T00:00:00Z")},
		{name: "unknown secondary custom_time_range", tr: trCustom, ctr: primary, sec: secondaryTestCustom(ctx, resource_report.NewCustomTimeRangeValueUnknown())},
		{name: "unknown secondary", tr: trCustom, ctr: primary, sec: resource_report.NewSecondaryTimeRangeValueUnknown()},
		{name: "unknown time_range", tr: resource_report.NewTimeRangeValueUnknown(), ctr: primary, sec: custom("2024-02-01T00:00:00Z", "2024-03-01T00:00:00Z")},
		{name: "unknown time_range.mode", tr: secondaryTestTimeRange(ctx, types.StringUnknown(), types.StringNull(), types.Int64Null()), ctr: primary, sec: custom("2024-02-01T00:00:00Z", "2024-03-01T00:00:00Z")},

		// Prior state supplies omitted Category B attributes on update.
		{
			name: "omitted secondary inherited from state overlaps the configured primary",
			tr:   trCustom, ctr: primary, sec: nullSec,
			prior:     &prior{tr: trCustom, ctr: primary, sec: custom("2024-02-01T00:00:00Z", "2024-03-01T00:00:00Z")},
			wantCount: 1, wantSummary: overlap, wantPath: secCTRPath, wantInDetail: inheritedNote,
		},
		{
			name: "omitted time_range inherited from state with mode custom",
			tr:   nullTR, ctr: primary, sec: custom("2024-02-01T00:00:00Z", "2024-03-01T00:00:00Z"),
			prior:     &prior{tr: trCustom, ctr: primary, sec: nullSec},
			wantCount: 1, wantSummary: overlap, wantPath: secCTRPath, wantInDetail: inheritedNote,
		},
		{
			name: "omitted time_range.mode inherited from state as custom",
			tr:   secondaryTestTimeRange(ctx, types.StringNull(), types.StringNull(), types.Int64Null()), ctr: primary, sec: custom("2024-02-01T00:00:00Z", "2024-03-01T00:00:00Z"),
			prior:     &prior{tr: trCustom, ctr: primary, sec: nullSec},
			wantCount: 1, wantSummary: overlap, wantPath: secCTRPath, wantInDetail: inheritedNote,
		},
		{
			name: "omitted custom_time_range is cleared, not inherited from state",
			tr:   trCustom, ctr: nullCTR, sec: custom("2024-02-01T00:00:00Z", "2024-03-01T00:00:00Z"),
			prior: &prior{tr: trCustom, ctr: primary, sec: nullSec},
		},
		{
			name: "configured attributes need no inheritance note",
			tr:   trCustom, ctr: primary, sec: custom("2024-02-01T00:00:00Z", "2024-03-01T00:00:00Z"),
			prior:     &prior{tr: trCustom, ctr: primary, sec: custom("2023-01-01T00:00:00Z", "2023-12-31T00:00:00Z")},
			wantCount: 1, wantSummary: overlap, wantPath: secCTRPath,
		},
		{
			name: "omitted secondary with no secondary in state",
			tr:   trCustom, ctr: primary, sec: nullSec,
			prior: &prior{tr: trCustom, ctr: primary, sec: nullSec},
		},

		// Relative secondary amount.
		{name: "amount zero", tr: trLast(7, "day"), ctr: nullCTR, sec: secondaryTestRelative(ctx, i(0), s("day"), types.BoolValue(false)), wantCount: 1, wantSummary: badAmount, wantPath: amountPath},
		{name: "amount negative", tr: trLast(7, "day"), ctr: nullCTR, sec: secondaryTestRelative(ctx, i(-1), s("day"), types.BoolValue(false)), wantCount: 1, wantSummary: badAmount, wantPath: amountPath},
		{name: "amount zero is reported while time_range is unknown", tr: resource_report.NewTimeRangeValueUnknown(), ctr: nullCTR, sec: secondaryTestRelative(ctx, i(0), s("day"), types.BoolValue(false)), wantCount: 1, wantSummary: badAmount, wantPath: amountPath},
		{name: "unknown amount", tr: trLast(7, "day"), ctr: nullCTR, sec: secondaryTestRelative(ctx, types.Int64Unknown(), s("day"), types.BoolValue(false))},
		{name: "positive amount", tr: trLast(7, "day"), ctr: nullCTR, sec: secondaryTestRelative(ctx, i(7), s("day"), types.BoolValue(false))},
		{
			name: "amount zero is ignored by a custom secondary",
			tr:   trCustom, ctr: primary,
			sec: resource_report.NewSecondaryTimeRangeValueMust(resource_report.SecondaryTimeRangeValue{}.AttributeTypes(ctx), map[string]attr.Value{
				"amount":            i(0),
				"custom_time_range": secondaryTestCTR(ctx, s("2023-01-01T00:00:00Z"), s("2023-12-31T00:00:00Z")),
				"include_current":   types.BoolNull(),
				"unit":              types.StringNull(),
			}),
		},

		// Relative secondary on a "last" primary longer than one year.
		{name: "last 366 days", tr: trLast(366, "day"), ctr: nullCTR, sec: yearShift, wantCount: 1, wantSummary: overlap, wantPath: secPath},
		{name: "last 53 weeks", tr: trLast(53, "week"), ctr: nullCTR, sec: yearShift, wantCount: 1, wantSummary: overlap, wantPath: secPath},
		{name: "last 13 months", tr: trLast(13, "month"), ctr: nullCTR, sec: yearShift, wantCount: 1, wantSummary: overlap, wantPath: secPath},
		{name: "last 5 quarters", tr: trLast(5, "quarter"), ctr: nullCTR, sec: yearShift, wantCount: 1, wantSummary: overlap, wantPath: secPath},
		{name: "last 2 years", tr: trLast(2, "year"), ctr: nullCTR, sec: yearShift, wantCount: 1, wantSummary: overlap, wantPath: secPath, wantInDetail: "last 2 year(s)"},
		{name: "last 365 days", tr: trLast(365, "day"), ctr: nullCTR, sec: yearShift},
		{name: "last 52 weeks", tr: trLast(52, "week"), ctr: nullCTR, sec: yearShift},
		{name: "last 12 months", tr: trLast(12, "month"), ctr: nullCTR, sec: yearShift},
		{name: "last 4 quarters", tr: trLast(4, "quarter"), ctr: nullCTR, sec: yearShift},
		{name: "last 1 year", tr: trLast(1, "year"), ctr: nullCTR, sec: yearShift},
		{name: "current mode is never longer than a year", tr: secondaryTestTimeRange(ctx, s("current"), s("year"), i(2)), ctr: nullCTR, sec: yearShift},
		{name: "custom secondary on a primary longer than a year", tr: trLast(2, "year"), ctr: nullCTR, sec: custom("2020-01-01T00:00:00Z", "2020-12-31T00:00:00Z")},
		{name: "unknown secondary custom_time_range on a primary longer than a year", tr: trLast(2, "year"), ctr: nullCTR, sec: secondaryTestCustom(ctx, resource_report.NewCustomTimeRangeValueUnknown())},
		{name: "unknown primary amount", tr: secondaryTestTimeRange(ctx, s("last"), s("year"), types.Int64Unknown()), ctr: nullCTR, sec: yearShift},
		{name: "unknown primary unit", tr: secondaryTestTimeRange(ctx, s("last"), types.StringUnknown(), i(2)), ctr: nullCTR, sec: yearShift},
		{
			name: "omitted time_range inherited from state as last 2 years",
			tr:   nullTR, ctr: nullCTR, sec: yearShift,
			prior:     &prior{tr: trLast(2, "year"), ctr: nullCTR, sec: nullSec},
			wantCount: 1, wantSummary: overlap, wantPath: secPath, wantInDetail: inheritedNote,
		},

		// One-year shift of a custom primary.
		{name: "one-year shift of a one-year custom primary", tr: trCustom, ctr: primary, sec: yearShift},
		{
			name: "one-year shift of a 13-month custom primary",
			tr:   trCustom, ctr: secondaryTestCTR(ctx, s("2023-12-01T00:00:00Z"), s("2024-12-31T00:00:00Z")), sec: yearShift,
			wantCount: 1, wantSummary: overlap, wantPath: secPath, wantInDetail: "gives 2022-12-01 to 2023-12-31",
		},
		{
			name: "one-year shift of a custom primary ending on a leap day lands on its first day",
			tr:   trCustom, ctr: secondaryTestCTR(ctx, s("2023-03-01T00:00:00Z"), s("2024-02-29T00:00:00Z")), sec: yearShift,
			wantCount: 1, wantSummary: overlap, wantPath: secPath,
		},
		{name: "one-year shift of a calendar-quarter custom primary", tr: trCustom, ctr: secondaryTestCTR(ctx, s("2024-01-01T00:00:00Z"), s("2024-03-31T00:00:00Z")), sec: yearShift},
		{name: "include_current one-year shift is left to the API", tr: trCustom, ctr: secondaryTestCTR(ctx, s("2023-01-01T00:00:00Z"), s("2024-12-31T00:00:00Z")), sec: secondaryTestRelative(ctx, i(1), s("year"), types.BoolValue(true))},
		{name: "two-year shift is left to the API", tr: trCustom, ctr: secondaryTestCTR(ctx, s("2023-01-01T00:00:00Z"), s("2024-12-31T00:00:00Z")), sec: secondaryTestRelative(ctx, i(2), s("year"), types.BoolValue(false))},
		{name: "null include_current is left to the API", tr: trCustom, ctr: secondaryTestCTR(ctx, s("2023-01-01T00:00:00Z"), s("2024-12-31T00:00:00Z")), sec: secondaryTestRelative(ctx, i(1), s("year"), types.BoolNull())},
		{name: "unknown shift unit", tr: trCustom, ctr: secondaryTestCTR(ctx, s("2023-01-01T00:00:00Z"), s("2024-12-31T00:00:00Z")), sec: secondaryTestRelative(ctx, i(1), types.StringUnknown(), types.BoolValue(false))},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := buildSecondaryTestConfig(ctx, t, tt.tr, tt.ctr, tt.sec)

			var priorState attributeGetter
			if tt.prior != nil {
				priorState = tfsdk.State(buildSecondaryTestConfig(ctx, t, tt.prior.tr, tt.prior.ctr, tt.prior.sec))
			}

			var diagnostics diag.Diagnostics
			validateReportSecondaryTimeRange(ctx, config, priorState, &diagnostics)

			if len(diagnostics) != tt.wantCount {
				t.Fatalf("diagnostic count = %d, want %d: %v", len(diagnostics), tt.wantCount, diagnostics)
			}
			if tt.wantCount == 0 {
				return
			}
			diagnostic := diagnostics[0]
			if diagnostic.Severity() != diag.SeverityError {
				t.Errorf("severity = %s, want error", diagnostic.Severity())
			}
			if diagnostic.Summary() != tt.wantSummary {
				t.Errorf("summary = %q, want %q", diagnostic.Summary(), tt.wantSummary)
			}
			withPath, ok := diagnostic.(diag.DiagnosticWithPath)
			if !ok {
				t.Fatalf("diagnostic %T has no path", diagnostic)
			}
			if got := withPath.Path(); !got.Equal(tt.wantPath) {
				t.Errorf("path = %s, want %s", got, tt.wantPath)
			}
			if tt.wantInDetail != "" && !strings.Contains(diagnostic.Detail(), tt.wantInDetail) {
				t.Errorf("detail %q does not contain %q", diagnostic.Detail(), tt.wantInDetail)
			}
			if tt.wantInDetail != inheritedNote && strings.Contains(diagnostic.Detail(), inheritedNote) {
				t.Errorf("detail %q mentions inheritance, but every checked attribute is configured", diagnostic.Detail())
			}
		})
	}
}

// TestValidateReportSecondaryTimeRange_UnknownConfig verifies that an unknown
// config wrapper defers validation.
func TestValidateReportSecondaryTimeRange_UnknownConfig(t *testing.T) {
	ctx := t.Context()
	schema := resource_report.ReportResourceSchema(ctx)
	schemaType := schema.Type().TerraformType(ctx)
	objType, ok := schemaType.(tftypes.Object)
	if !ok {
		t.Fatalf("expected schema to be tftypes.Object, got %T", schemaType)
	}
	attrValues := make(map[string]tftypes.Value, len(objType.AttributeTypes))
	for name, attrType := range objType.AttributeTypes {
		attrValues[name] = tftypes.NewValue(attrType, nil)
	}
	attrValues["config"] = tftypes.NewValue(objType.AttributeTypes["config"], tftypes.UnknownValue)
	config := tfsdk.Config{Schema: schema, Raw: tftypes.NewValue(schemaType, attrValues)}

	var diagnostics diag.Diagnostics
	validateReportSecondaryTimeRange(ctx, config, nil, &diagnostics)
	if len(diagnostics) != 0 {
		t.Fatalf("diagnostic count = %d, want 0: %v", len(diagnostics), diagnostics)
	}
}

// TestReportSecondaryTimeRangeValidator_ValidateResource verifies that the
// resource ConfigValidator reports an overlap from the configuration alone.
func TestReportSecondaryTimeRangeValidator_ValidateResource(t *testing.T) {
	ctx := t.Context()
	config := buildSecondaryTestConfig(ctx, t,
		secondaryTestTimeRange(ctx, types.StringValue("custom"), types.StringNull(), types.Int64Null()),
		secondaryTestCTR(ctx, types.StringValue("2024-01-01T00:00:00Z"), types.StringValue("2024-12-31T23:59:59Z")),
		secondaryTestCustom(ctx, secondaryTestCTR(ctx, types.StringValue("2023-01-01T00:00:00-05:00"), types.StringValue("2023-12-31T23:59:59-05:00"))),
	)

	resp := &resource.ValidateConfigResponse{}
	reportSecondaryTimeRangeValidator{}.ValidateResource(ctx, resource.ValidateConfigRequest{Config: config}, resp)
	if len(resp.Diagnostics) != 1 || resp.Diagnostics[0].Summary() != "Overlapping Secondary Time Range" {
		t.Fatalf("diagnostics = %v, want one Overlapping Secondary Time Range error", resp.Diagnostics)
	}
}
