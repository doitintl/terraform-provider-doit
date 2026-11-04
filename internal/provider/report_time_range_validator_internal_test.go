package provider

import (
	"context"
	"testing"

	"github.com/doitintl/terraform-provider-doit/internal/provider/resource_report"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func timeRangeTestValue(ctx context.Context, mode, unit types.String, amount types.Int64, includeCurrent types.Bool) resource_report.TimeRangeValue {
	return resource_report.NewTimeRangeValueMust(resource_report.TimeRangeValue{}.AttributeTypes(ctx), map[string]attr.Value{
		"amount":          amount,
		"include_current": includeCurrent,
		"mode":            mode,
		"unit":            unit,
	})
}

type wantTimeRangeDiag struct {
	summary string
	path    path.Path
}

func assertTimeRangeDiags(t *testing.T, diagnostics diag.Diagnostics, want []wantTimeRangeDiag) {
	t.Helper()
	if len(diagnostics) != len(want) {
		t.Fatalf("diagnostic count = %d, want %d: %v", len(diagnostics), len(want), diagnostics)
	}
	for i, w := range want {
		if diagnostics[i].Severity() != diag.SeverityError {
			t.Errorf("diagnostic %d severity = %s, want error", i, diagnostics[i].Severity())
		}
		if diagnostics[i].Summary() != w.summary {
			t.Errorf("diagnostic %d summary = %q, want %q", i, diagnostics[i].Summary(), w.summary)
		}
		withPath, ok := diagnostics[i].(diag.DiagnosticWithPath)
		if !ok {
			t.Fatalf("diagnostic %d %T has no path", i, diagnostics[i])
		}
		if got := withPath.Path(); !got.Equal(w.path) {
			t.Errorf("diagnostic %d path = %s, want %s", i, got, w.path)
		}
	}
}

func TestValidateReportTimeRangeRequiredFields(t *testing.T) {
	ctx := t.Context()
	s := types.StringValue
	i := types.Int64Value
	b := types.BoolValue
	nullString := types.StringNull()
	nullInt := types.Int64Null()
	nullBool := types.BoolNull()

	primary := secondaryTestCTR(ctx, s("2024-01-01T00:00:00Z"), s("2024-12-31T23:59:59Z"))
	nullCTR := resource_report.NewCustomTimeRangeValueNull()
	nullSec := resource_report.NewSecondaryTimeRangeValueNull()
	nullTR := resource_report.NewTimeRangeValueNull()
	lastMonths := timeRangeTestValue(ctx, s("last"), s("month"), i(3), b(true))
	yearShift := secondaryTestRelative(ctx, i(1), s("year"), b(false))

	timeRangePath := path.Root("config").AtName("time_range")
	secPath := path.Root("config").AtName("secondary_time_range")
	missingTR := wantTimeRangeDiag{"Missing Time Range Field", timeRangePath.AtName("mode")}
	missingUnit := wantTimeRangeDiag{"Missing Time Range Field", timeRangePath.AtName("unit")}
	missingSecAmount := wantTimeRangeDiag{"Missing Secondary Time Range Field", secPath.AtName("amount")}
	missingSecUnit := wantTimeRangeDiag{"Missing Secondary Time Range Field", secPath.AtName("unit")}
	missingSecIncludeCurrent := wantTimeRangeDiag{"Missing Secondary Time Range Field", secPath.AtName("include_current")}
	missingPrimary := wantTimeRangeDiag{"Missing Time Range", timeRangePath}

	tests := []struct {
		name string
		tr   resource_report.TimeRangeValue
		ctr  resource_report.CustomTimeRangeValue
		sec  resource_report.SecondaryTimeRangeValue
		want []wantTimeRangeDiag
	}{
		// Primary time_range.
		{name: "complete last range", tr: lastMonths, ctr: nullCTR, sec: nullSec},
		{name: "omitted amount and include_current", tr: timeRangeTestValue(ctx, s("last"), s("month"), nullInt, nullBool), ctr: nullCTR, sec: nullSec},
		{name: "omitted mode", tr: timeRangeTestValue(ctx, nullString, s("month"), i(3), b(true)), ctr: nullCTR, sec: nullSec, want: []wantTimeRangeDiag{missingTR}},
		{name: "omitted mode and unit reports only mode", tr: timeRangeTestValue(ctx, nullString, nullString, i(3), b(true)), ctr: nullCTR, sec: nullSec, want: []wantTimeRangeDiag{missingTR}},
		{name: "unknown mode defers the unit check", tr: timeRangeTestValue(ctx, types.StringUnknown(), nullString, i(3), b(true)), ctr: nullCTR, sec: nullSec},
		{name: "omitted unit with mode last", tr: timeRangeTestValue(ctx, s("last"), nullString, i(3), b(true)), ctr: nullCTR, sec: nullSec, want: []wantTimeRangeDiag{missingUnit}},
		{name: "omitted unit with mode current", tr: timeRangeTestValue(ctx, s("current"), nullString, nullInt, nullBool), ctr: nullCTR, sec: nullSec, want: []wantTimeRangeDiag{missingUnit}},
		{name: "omitted unit with mode custom", tr: timeRangeTestValue(ctx, s("custom"), nullString, nullInt, nullBool), ctr: primary, sec: nullSec},
		{name: "unknown unit", tr: timeRangeTestValue(ctx, s("last"), types.StringUnknown(), i(3), b(true)), ctr: nullCTR, sec: nullSec},
		{name: "omitted time_range", tr: nullTR, ctr: nullCTR, sec: nullSec},
		{name: "unknown time_range", tr: resource_report.NewTimeRangeValueUnknown(), ctr: nullCTR, sec: nullSec},

		// Relative secondary_time_range.
		{name: "complete relative secondary", tr: lastMonths, ctr: nullCTR, sec: yearShift},
		{name: "empty secondary block", tr: lastMonths, ctr: nullCTR, sec: secondaryTestRelative(ctx, nullInt, nullString, nullBool), want: []wantTimeRangeDiag{missingSecAmount, missingSecUnit, missingSecIncludeCurrent}},
		{name: "omitted secondary amount", tr: lastMonths, ctr: nullCTR, sec: secondaryTestRelative(ctx, nullInt, s("year"), b(false)), want: []wantTimeRangeDiag{missingSecAmount}},
		{name: "omitted secondary unit", tr: lastMonths, ctr: nullCTR, sec: secondaryTestRelative(ctx, i(1), nullString, b(false)), want: []wantTimeRangeDiag{missingSecUnit}},
		{name: "omitted secondary include_current", tr: lastMonths, ctr: nullCTR, sec: secondaryTestRelative(ctx, i(1), s("year"), nullBool), want: []wantTimeRangeDiag{missingSecIncludeCurrent}},
		{
			name: "unknown secondary amount does not hide an omitted unit",
			tr:   lastMonths, ctr: nullCTR, sec: secondaryTestRelative(ctx, types.Int64Unknown(), nullString, b(false)),
			want: []wantTimeRangeDiag{missingSecUnit},
		},
		{name: "custom secondary needs no relative fields", tr: lastMonths, ctr: nullCTR, sec: secondaryTestCustom(ctx, secondaryTestCTR(ctx, s("2020-01-01T00:00:00Z"), s("2020-12-31T23:59:59Z")))},
		{name: "unknown secondary custom_time_range", tr: lastMonths, ctr: nullCTR, sec: secondaryTestCustom(ctx, resource_report.NewCustomTimeRangeValueUnknown())},
		{name: "unknown secondary", tr: lastMonths, ctr: nullCTR, sec: resource_report.NewSecondaryTimeRangeValueUnknown()},

		// secondary_time_range requires time_range.
		{name: "secondary without time_range", tr: nullTR, ctr: nullCTR, sec: yearShift, want: []wantTimeRangeDiag{missingPrimary}},
		{name: "custom secondary without time_range", tr: nullTR, ctr: nullCTR, sec: secondaryTestCustom(ctx, secondaryTestCTR(ctx, s("2020-01-01T00:00:00Z"), s("2020-12-31T23:59:59Z"))), want: []wantTimeRangeDiag{missingPrimary}},
		{name: "secondary with unknown time_range", tr: resource_report.NewTimeRangeValueUnknown(), ctr: nullCTR, sec: yearShift},
		{name: "unknown secondary without time_range", tr: nullTR, ctr: nullCTR, sec: resource_report.NewSecondaryTimeRangeValueUnknown()},
		{
			name: "every check reports independently",
			tr:   timeRangeTestValue(ctx, nullString, s("month"), i(3), b(true)), ctr: nullCTR, sec: secondaryTestRelative(ctx, i(1), s("year"), nullBool),
			want: []wantTimeRangeDiag{missingTR, missingSecIncludeCurrent},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := buildSecondaryTestConfig(ctx, t, tt.tr, tt.ctr, tt.sec)
			var diagnostics diag.Diagnostics
			validateReportTimeRangeRequiredFields(ctx, config, &diagnostics)
			assertTimeRangeDiags(t, diagnostics, tt.want)
		})
	}
}

// TestValidateReportTimeRangeRequiredFields_UnknownConfig verifies that an
// unknown config wrapper defers validation.
func TestValidateReportTimeRangeRequiredFields_UnknownConfig(t *testing.T) {
	ctx := t.Context()
	var diagnostics diag.Diagnostics
	validateReportTimeRangeRequiredFields(ctx, buildUnknownReportConfig(ctx, t), &diagnostics)
	if len(diagnostics) != 0 {
		t.Fatalf("diagnostic count = %d, want 0: %v", len(diagnostics), diagnostics)
	}
}

// TestReportTimeRangeRequiredFieldsValidators verifies that the resource and
// data-source ConfigValidators report a missing field from the configuration alone.
func TestReportTimeRangeRequiredFieldsValidators(t *testing.T) {
	ctx := t.Context()
	config := buildSecondaryTestConfig(ctx, t,
		timeRangeTestValue(ctx, types.StringValue("last"), types.StringNull(), types.Int64Value(3), types.BoolValue(true)),
		resource_report.NewCustomTimeRangeValueNull(),
		resource_report.NewSecondaryTimeRangeValueNull(),
	)
	want := []wantTimeRangeDiag{{"Missing Time Range Field", path.Root("config").AtName("time_range").AtName("unit")}}

	resourceResp := &resource.ValidateConfigResponse{}
	reportTimeRangeRequiredFieldsValidator{}.ValidateResource(ctx, resource.ValidateConfigRequest{Config: config}, resourceResp)
	assertTimeRangeDiags(t, resourceResp.Diagnostics, want)

	dataSourceResp := &datasource.ValidateConfigResponse{}
	reportTimeRangeRequiredFieldsDataSourceValidator{}.ValidateDataSource(ctx, datasource.ValidateConfigRequest{Config: config}, dataSourceResp)
	assertTimeRangeDiags(t, dataSourceResp.Diagnostics, want)
}

// TestValidateReportTimestamps_CustomTimeRangeBounds verifies that a configured
// custom_time_range, primary or secondary, requires both from and to.
func TestValidateReportTimestamps_CustomTimeRangeBounds(t *testing.T) {
	ctx := t.Context()
	s := types.StringValue
	nullString := types.StringNull()
	unknownString := types.StringUnknown()

	trCustom := timeRangeTestValue(ctx, s("custom"), nullString, types.Int64Null(), types.BoolNull())
	trLast := timeRangeTestValue(ctx, s("last"), s("month"), types.Int64Value(3), types.BoolValue(true))
	from := s("2020-01-01T00:00:00Z")
	to := s("2020-12-31T23:59:59Z")

	ctrPath := path.Root("config").AtName("custom_time_range")
	secCTRPath := path.Root("config").AtName("secondary_time_range").AtName("custom_time_range")
	missing := func(p path.Path) wantTimeRangeDiag { return wantTimeRangeDiag{"Missing Custom Time Range Field", p} }

	tests := []struct {
		name string
		tr   resource_report.TimeRangeValue
		ctr  resource_report.CustomTimeRangeValue
		sec  resource_report.SecondaryTimeRangeValue
		want []wantTimeRangeDiag
	}{
		{name: "complete primary range", tr: trCustom, ctr: secondaryTestCTR(ctx, from, to), sec: resource_report.NewSecondaryTimeRangeValueNull()},
		{name: "primary range without to", tr: trCustom, ctr: secondaryTestCTR(ctx, from, nullString), sec: resource_report.NewSecondaryTimeRangeValueNull(), want: []wantTimeRangeDiag{missing(ctrPath.AtName("to"))}},
		{name: "primary range without from", tr: trCustom, ctr: secondaryTestCTR(ctx, nullString, to), sec: resource_report.NewSecondaryTimeRangeValueNull(), want: []wantTimeRangeDiag{missing(ctrPath.AtName("from"))}},
		{
			name: "empty primary range",
			tr:   trCustom, ctr: secondaryTestCTR(ctx, nullString, nullString), sec: resource_report.NewSecondaryTimeRangeValueNull(),
			want: []wantTimeRangeDiag{missing(ctrPath.AtName("from")), missing(ctrPath.AtName("to"))},
		},
		{name: "unknown from does not hide an omitted to", tr: trCustom, ctr: secondaryTestCTR(ctx, unknownString, nullString), sec: resource_report.NewSecondaryTimeRangeValueNull(), want: []wantTimeRangeDiag{missing(ctrPath.AtName("to"))}},
		{name: "unknown bounds", tr: trCustom, ctr: secondaryTestCTR(ctx, unknownString, unknownString), sec: resource_report.NewSecondaryTimeRangeValueNull()},
		{name: "unknown primary range", tr: trCustom, ctr: resource_report.NewCustomTimeRangeValueUnknown(), sec: resource_report.NewSecondaryTimeRangeValueNull()},
		{name: "complete secondary range", tr: trLast, ctr: resource_report.NewCustomTimeRangeValueNull(), sec: secondaryTestCustom(ctx, secondaryTestCTR(ctx, from, to))},
		{name: "secondary range without to", tr: trLast, ctr: resource_report.NewCustomTimeRangeValueNull(), sec: secondaryTestCustom(ctx, secondaryTestCTR(ctx, from, nullString)), want: []wantTimeRangeDiag{missing(secCTRPath.AtName("to"))}},
		{name: "secondary range without from", tr: trLast, ctr: resource_report.NewCustomTimeRangeValueNull(), sec: secondaryTestCustom(ctx, secondaryTestCTR(ctx, nullString, to)), want: []wantTimeRangeDiag{missing(secCTRPath.AtName("from"))}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := buildSecondaryTestConfig(ctx, t, tt.tr, tt.ctr, tt.sec)
			var diagnostics diag.Diagnostics
			validateReportTimestamps(ctx, config, &diagnostics)
			assertTimeRangeDiags(t, diagnostics, tt.want)
		})
	}
}

func TestPlanReportTimeRangeOmittedFields(t *testing.T) {
	ctx := t.Context()
	s := types.StringValue
	i := types.Int64Value
	b := types.BoolValue
	nullCTR := resource_report.NewCustomTimeRangeValueNull()
	nullSec := resource_report.NewSecondaryTimeRangeValueNull()

	omitted := timeRangeTestValue(ctx, s("current"), s("month"), types.Int64Null(), types.BoolNull())
	unknownLeaves := timeRangeTestValue(ctx, s("current"), s("month"), types.Int64Unknown(), types.BoolUnknown())
	priorLeaves := timeRangeTestValue(ctx, s("current"), s("month"), i(1), b(true))

	tests := []struct {
		name               string
		config             resource_report.TimeRangeValue
		plan               resource_report.TimeRangeValue
		wantAmount         types.Int64
		wantIncludeCurrent types.Bool
	}{
		{name: "omitted fields planned as unknown get the stored values", config: omitted, plan: unknownLeaves, wantAmount: i(0), wantIncludeCurrent: b(false)},
		{name: "omitted fields kept from prior state are left alone", config: omitted, plan: priorLeaves, wantAmount: i(1), wantIncludeCurrent: b(true)},
		{
			name:   "configured unknown fields are left alone",
			config: unknownLeaves, plan: unknownLeaves, wantAmount: types.Int64Unknown(), wantIncludeCurrent: types.BoolUnknown(),
		},
		{
			name:   "only the omitted field is planned",
			config: timeRangeTestValue(ctx, s("last"), s("month"), i(3), types.BoolNull()), plan: timeRangeTestValue(ctx, s("last"), s("month"), i(3), types.BoolUnknown()),
			wantAmount: i(3), wantIncludeCurrent: b(false),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			plan := tfsdk.Plan(buildSecondaryTestConfig(ctx, t, tt.plan, nullCTR, nullSec))
			req := resource.ModifyPlanRequest{Config: buildSecondaryTestConfig(ctx, t, tt.config, nullCTR, nullSec), Plan: plan}
			resp := &resource.ModifyPlanResponse{Plan: plan}
			planReportTimeRangeOmittedFields(ctx, req, resp)
			if resp.Diagnostics.HasError() {
				t.Fatalf("unexpected diagnostics: %v", resp.Diagnostics)
			}

			var got resource_report.TimeRangeValue
			if d := resp.Plan.GetAttribute(ctx, path.Root("config").AtName("time_range"), &got); d.HasError() {
				t.Fatalf("GetAttribute: %v", d)
			}
			if !got.Amount.Equal(tt.wantAmount) {
				t.Errorf("amount = %s, want %s", got.Amount, tt.wantAmount)
			}
			if !got.IncludeCurrent.Equal(tt.wantIncludeCurrent) {
				t.Errorf("include_current = %s, want %s", got.IncludeCurrent, tt.wantIncludeCurrent)
			}
		})
	}
}

// TestPlanReportTimeRangeOmittedFields_TimeRangeNotConfigured verifies that the
// plan is left alone when time_range is omitted or unknown in config.
func TestPlanReportTimeRangeOmittedFields_TimeRangeNotConfigured(t *testing.T) {
	ctx := t.Context()
	nullCTR := resource_report.NewCustomTimeRangeValueNull()
	nullSec := resource_report.NewSecondaryTimeRangeValueNull()
	plan := timeRangeTestValue(ctx, types.StringValue("current"), types.StringValue("month"), types.Int64Unknown(), types.BoolUnknown())

	for name, config := range map[string]resource_report.TimeRangeValue{
		"omitted": resource_report.NewTimeRangeValueNull(),
		"unknown": resource_report.NewTimeRangeValueUnknown(),
	} {
		t.Run(name, func(t *testing.T) {
			planValue := tfsdk.Plan(buildSecondaryTestConfig(ctx, t, plan, nullCTR, nullSec))
			req := resource.ModifyPlanRequest{Config: buildSecondaryTestConfig(ctx, t, config, nullCTR, nullSec), Plan: planValue}
			resp := &resource.ModifyPlanResponse{Plan: planValue}
			planReportTimeRangeOmittedFields(ctx, req, resp)
			if resp.Diagnostics.HasError() {
				t.Fatalf("unexpected diagnostics: %v", resp.Diagnostics)
			}

			var got resource_report.TimeRangeValue
			if d := resp.Plan.GetAttribute(ctx, path.Root("config").AtName("time_range"), &got); d.HasError() {
				t.Fatalf("GetAttribute: %v", d)
			}
			if !got.Amount.IsUnknown() || !got.IncludeCurrent.IsUnknown() {
				t.Errorf("amount = %s, include_current = %s, want both unknown", got.Amount, got.IncludeCurrent)
			}
		})
	}
}
