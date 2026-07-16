package provider

import (
	"context"
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

// ---------------------------------------------------------------------------
// TestReportTimestampValidator_NoCustomTimeRange
// ---------------------------------------------------------------------------

// TestReportTimestampValidator_NoCustomTimeRange verifies that the validator
// produces no errors/warnings when custom_time_range is not set.
// This tests whether GetAttribute on missing nested paths generates diagnostics
// that would confuse users if propagated.
func TestReportTimestampValidator_NoCustomTimeRange(t *testing.T) {
	ctx := context.Background()
	schema := resource_report.ReportResourceSchema(ctx)

	// Create an empty config with the report schema (no attributes set).
	config := tfsdk.Config{Schema: schema}

	// Test what GetAttribute returns for a missing nested path.
	var val types.String
	diags := config.GetAttribute(ctx, path.Root("config").AtName("custom_time_range").AtName("from"), &val)

	t.Logf("GetAttribute on missing path: hasError=%v, count=%d, isNull=%v, isUnknown=%v",
		diags.HasError(), len(diags), val.IsNull(), val.IsUnknown())
	for i, d := range diags {
		t.Logf("  diag[%d]: severity=%s summary=%q detail=%q", i, d.Severity(), d.Summary(), d.Detail())
	}

	// Now run the full validator and verify it produces clean output.
	v := reportTimestampValidator{}
	req := resource.ValidateConfigRequest{Config: config}
	resp := &resource.ValidateConfigResponse{}
	v.ValidateResource(ctx, req, resp)

	t.Logf("ValidateResource diagnostics: hasError=%v, count=%d", resp.Diagnostics.HasError(), len(resp.Diagnostics))
	for i, d := range resp.Diagnostics {
		t.Logf("  resp diag[%d]: severity=%s summary=%q", i, d.Severity(), d.Summary())
	}

	if resp.Diagnostics.HasError() {
		t.Fatalf("expected no errors when custom_time_range is not set, got: %v", resp.Diagnostics)
	}
	if len(resp.Diagnostics) > 0 {
		t.Errorf("expected zero diagnostics when custom_time_range is not set, got %d", len(resp.Diagnostics))
	}
}

// ---------------------------------------------------------------------------
// TestIsNAFallback
// ---------------------------------------------------------------------------

func TestIsNAFallback(t *testing.T) {
	tests := []struct {
		value string
		want  bool
	}{
		// Known NullFallback sentinels from the Cloud Analytics keymap.
		{"[Service N/A]", true},
		{"[Folder N/A]", true},
		{"[Customer N/A]", true},
		{"[Project/Account ID N/A]", true},
		{"[Attribution N/A]", true},
		{"[Allocation N/A]", true},
		{"[Value N/A]", true},
		{"[Region N/A]", true},
		// NOT sentinels.
		{"Compute Engine", false},
		{"[N/A]", false}, // NotApplicable constant — no space before N/A.
		{"", false},
		{"[something]", false},
		{"N/A", false},
		{"Service N/A", false},         // Missing leading bracket.
		{"[Service N/A] extra", false}, // Trailing characters after closing bracket.
	}
	for _, tt := range tests {
		t.Run(tt.value, func(t *testing.T) {
			if got := isNAFallback(tt.value); got != tt.want {
				t.Errorf("isNAFallback(%q) = %v, want %v", tt.value, got, tt.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// TestWarnNAFilterValues
// ---------------------------------------------------------------------------

// buildTestFilter constructs a resource_report.FiltersValue with the given values
// slice. Other fields are set to safe defaults so the constructor does not error.
func buildTestFilter(ctx context.Context, t *testing.T, values []string) resource_report.FiltersValue {
	t.Helper()
	elems := make([]attr.Value, len(values))
	for i, v := range values {
		elems[i] = types.StringValue(v)
	}
	f, diags := resource_report.NewFiltersValue(
		resource_report.FiltersValue{}.AttributeTypes(ctx),
		map[string]attr.Value{
			"case_insensitive": types.BoolValue(false),
			"id":               types.StringValue("service_description"),
			"include_null":     types.BoolValue(false),
			"inverse":          types.BoolNull(),
			"mode":             types.StringValue("is"),
			"type":             types.StringValue("fixed"),
			"values":           types.ListValueMust(types.StringType, elems),
		},
	)
	if diags.HasError() {
		t.Fatalf("buildTestFilter: %v", diags)
	}
	return f
}

func countWarnings(d diag.Diagnostics) int {
	n := 0
	for _, item := range d {
		if item.Severity() == diag.SeverityWarning {
			n++
		}
	}
	return n
}

func TestWarnNAFilterValues(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name         string
		filters      [][]string // outer = filters, inner = values per filter
		wantWarnings int
	}{
		{
			name:         "no filters",
			filters:      nil,
			wantWarnings: 0,
		},
		{
			name:         "real values only",
			filters:      [][]string{{"Compute Engine", "Cloud Storage"}},
			wantWarnings: 0,
		},
		{
			name:         "pure sentinel",
			filters:      [][]string{{"[Service N/A]"}},
			wantWarnings: 1,
		},
		{
			name:         "mixed real and sentinel",
			filters:      [][]string{{"Compute Engine", "[Service N/A]"}},
			wantWarnings: 1,
		},
		{
			name:         "multiple filters, one sentinel",
			filters:      [][]string{{"Compute Engine"}, {"[Folder N/A]"}},
			wantWarnings: 1,
		},
		{
			name:         "multiple filters, each with one sentinel",
			filters:      [][]string{{"[Service N/A]"}, {"[Folder N/A]"}},
			wantWarnings: 2,
		},
		{
			name:         "two sentinels in same filter",
			filters:      [][]string{{"[Service N/A]", "[Folder N/A]"}},
			wantWarnings: 2,
		},
		{
			name:         "[N/A] is not a sentinel (NotApplicable constant, no space)",
			filters:      [][]string{{"[N/A]"}},
			wantWarnings: 0,
		},
		{
			name:         "empty values list",
			filters:      [][]string{{}},
			wantWarnings: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var filterVals []resource_report.FiltersValue
			for _, vals := range tt.filters {
				filterVals = append(filterVals, buildTestFilter(ctx, t, vals))
			}
			var d diag.Diagnostics
			warnNAFilterValues(ctx, filterVals, &d)
			got := countWarnings(d)
			if got != tt.wantWarnings {
				t.Errorf("wantWarnings=%d got=%d diags=%v", tt.wantWarnings, got, d)
			}
			if d.HasError() {
				t.Errorf("unexpected errors: %v", d)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// TestReportFilterNAValidator_EmptyConfig
// ---------------------------------------------------------------------------

// TestReportFilterNAValidator_EmptyConfig verifies that the validator produces
// no diagnostics when the config has no filters set (null).
func TestReportFilterNAValidator_EmptyConfig(t *testing.T) {
	ctx := context.Background()
	config := tfsdk.Config{Schema: resource_report.ReportResourceSchema(ctx)}
	v := reportFilterNAValidator{}
	req := resource.ValidateConfigRequest{Config: config}
	resp := &resource.ValidateConfigResponse{}
	v.ValidateResource(ctx, req, resp)
	if resp.Diagnostics.HasError() {
		t.Fatalf("expected no errors with empty config, got: %v", resp.Diagnostics)
	}
	if len(resp.Diagnostics) > 0 {
		t.Errorf("expected zero diagnostics with empty config, got %d", len(resp.Diagnostics))
	}
}

// ---------------------------------------------------------------------------
// TestWarnNASentinels — unknown element handling
// ---------------------------------------------------------------------------

// TestWarnNASentinels_UnknownElement verifies that unknown elements in a
// filter/scope values list do not crash the validator. This reproduces the
// original "Value Conversion Error" where ElementsAs to []string crashed
// on unknown values. With []basetypes.StringValue, unknown elements are
// silently skipped instead.
func TestWarnNASentinels_UnknownElement(t *testing.T) {
	ctx := context.Background()

	unknownList := types.ListValueMust(types.StringType, []attr.Value{
		types.StringValue("known-allocation-id"),
		types.StringUnknown(), // simulates a cross-resource reference during plan
	})

	var diags diag.Diagnostics
	warnNASentinels(ctx, path.Root("test"), []types.List{unknownList}, &diags)

	if diags.HasError() {
		t.Fatalf("warnNASentinels crashed with unknown element: %v", diags)
	}
	if countWarnings(diags) > 0 {
		t.Errorf("expected no warnings when no sentinel values present, got %d", countWarnings(diags))
	}
}

// TestWarnNASentinels_MixedUnknownAndSentinel verifies that known sentinel
// values still produce deprecation warnings even when other elements in the
// same list are unknown. This is the key advantage of using
// []basetypes.StringValue over the skip-all approach.
func TestWarnNASentinels_MixedUnknownAndSentinel(t *testing.T) {
	ctx := context.Background()

	mixedList := types.ListValueMust(types.StringType, []attr.Value{
		types.StringValue("[Service N/A]"), // sentinel — should warn
		types.StringUnknown(),              // unknown — should be skipped
		types.StringValue("normal-value"),  // normal — no warning
	})

	var diags diag.Diagnostics
	warnNASentinels(ctx, path.Root("test"), []types.List{mixedList}, &diags)

	if diags.HasError() {
		t.Fatalf("unexpected error: %v", diags)
	}
	if got := countWarnings(diags); got != 1 {
		t.Errorf("expected 1 warning for sentinel in mixed list, got %d", got)
	}
}

// TestWarnNASentinels_AllUnknown reproduces the crash when ALL elements in a
// values list are unknown (e.g. values = [doit_allocation.xxx.id]).
func TestWarnNASentinels_AllUnknown(t *testing.T) {
	ctx := context.Background()

	unknownList := types.ListValueMust(types.StringType, []attr.Value{
		types.StringUnknown(),
	})

	var diags diag.Diagnostics
	warnNASentinels(ctx, path.Root("test"), []types.List{unknownList}, &diags)

	if diags.HasError() {
		t.Fatalf("warnNASentinels crashed with all-unknown elements: %v", diags)
	}
}

// TestWarnNAFilterValues_UnknownElement reproduces the crash through the full
// warnNAFilterValues path (the report validator's entry point).
func TestWarnNAFilterValues_UnknownElement(t *testing.T) {
	ctx := context.Background()

	f, fDiags := resource_report.NewFiltersValue(
		resource_report.FiltersValue{}.AttributeTypes(ctx),
		map[string]attr.Value{
			"case_insensitive": types.BoolValue(false),
			"id":               types.StringValue("attribution"),
			"include_null":     types.BoolValue(false),
			"inverse":          types.BoolNull(),
			"mode":             types.StringValue("is"),
			"type":             types.StringValue("allocation_rule"),
			"values": types.ListValueMust(types.StringType, []attr.Value{
				types.StringUnknown(), // simulates doit_allocation.xxx.id during plan
			}),
		},
	)
	if fDiags.HasError() {
		t.Fatalf("NewFiltersValue: %v", fDiags)
	}

	var diags diag.Diagnostics
	warnNAFilterValues(ctx, []resource_report.FiltersValue{f}, &diags)

	if diags.HasError() {
		t.Fatalf("warnNAFilterValues crashed with unknown element: %v", diags)
	}
}

// TestWarnNASentinels_KnownValues_StillWarns verifies that the sentinel warning
// still fires for fully-known values containing sentinels (guard against
// false positives from the unknown-element fix).
func TestWarnNASentinels_KnownValues_StillWarns(t *testing.T) {
	ctx := context.Background()

	knownList := types.ListValueMust(types.StringType, []attr.Value{
		types.StringValue("[Service N/A]"),
		types.StringValue("normal-value"),
	})

	var diags diag.Diagnostics
	warnNASentinels(ctx, path.Root("test"), []types.List{knownList}, &diags)

	if diags.HasError() {
		t.Fatalf("unexpected error: %v", diags)
	}
	if got := countWarnings(diags); got != 1 {
		t.Errorf("expected 1 warning for sentinel value, got %d", got)
	}
}

// ---------------------------------------------------------------------------
// TestReportTimestampValidator_ForecastSettings
// ---------------------------------------------------------------------------

func buildReportConfigWithForecastSettings(ctx context.Context, t *testing.T, futureFrom, futureTo, histFrom, histTo string) tfsdk.Config {
	t.Helper()
	schema := resource_report.ReportResourceSchema(ctx)

	// Construct the forecast_settings values
	fcdrMap := map[string]attr.Value{
		"from": types.StringNull(),
		"to":   types.StringNull(),
	}
	if futureFrom != "" {
		if futureFrom == "UNKNOWN" {
			fcdrMap["from"] = types.StringUnknown()
		} else {
			fcdrMap["from"] = types.StringValue(futureFrom)
		}
	}
	if futureTo != "" {
		if futureTo == "UNKNOWN" {
			fcdrMap["to"] = types.StringUnknown()
		} else {
			fcdrMap["to"] = types.StringValue(futureTo)
		}
	}
	fcdrVal, diags := resource_report.NewFutureCustomDateRangeValue(resource_report.FutureCustomDateRangeValue{}.AttributeTypes(ctx), fcdrMap)
	if diags.HasError() {
		t.Fatalf("NewFutureCustomDateRangeValue: %v", diags)
	}

	hcdrMap := map[string]attr.Value{
		"from": types.StringNull(),
		"to":   types.StringNull(),
	}
	if histFrom != "" {
		if histFrom == "UNKNOWN" {
			hcdrMap["from"] = types.StringUnknown()
		} else {
			hcdrMap["from"] = types.StringValue(histFrom)
		}
	}
	if histTo != "" {
		if histTo == "UNKNOWN" {
			hcdrMap["to"] = types.StringUnknown()
		} else {
			hcdrMap["to"] = types.StringValue(histTo)
		}
	}
	hcdrVal, diags := resource_report.NewHistoricalCustomDateRangeValue(resource_report.HistoricalCustomDateRangeValue{}.AttributeTypes(ctx), hcdrMap)
	if diags.HasError() {
		t.Fatalf("NewHistoricalCustomDateRangeValue: %v", diags)
	}

	fsMap := map[string]attr.Value{
		"future_custom_date_range":     fcdrVal,
		"future_time_intervals":        types.Int64Null(),
		"historical_custom_date_range": hcdrVal,
		"historical_time_intervals":    types.Int64Null(),
		"mode":                         types.StringValue("totals"),
	}
	fsVal, diags := resource_report.NewForecastSettingsValue(resource_report.ForecastSettingsValue{}.AttributeTypes(ctx), fsMap)
	if diags.HasError() {
		t.Fatalf("NewForecastSettingsValue: %v", diags)
	}

	configMap := map[string]attr.Value{
		"aggregation":                 types.StringNull(),
		"currency":                    types.StringNull(),
		"data_source":                 types.StringNull(),
		"display_values":              types.StringNull(),
		"include_promotional_credits": types.BoolNull(),
		"include_subtotals":           types.BoolNull(),
		"layout":                      types.StringNull(),
		"sort_dimensions":             types.StringNull(),
		"sort_groups":                 types.StringNull(),
		"time_interval":               types.StringNull(),
		"forecast_settings":           fsVal,
		"custom_time_range":           resource_report.NewCustomTimeRangeValueNull(),
		"secondary_time_range":        resource_report.NewSecondaryTimeRangeValueNull(),
		"time_range":                  resource_report.NewTimeRangeValueNull(),
		"advanced_analysis":           resource_report.NewAdvancedAnalysisValueNull(),
		"display_settings":            resource_report.NewDisplaySettingsValueNull(),
		"metric":                      resource_report.NewMetricValueNull(),
		"metric_filter":               resource_report.NewMetricFilterValueNull(),
		"dimensions":                  types.ListNull(resource_report.DimensionsValue{}.Type(ctx)),
		"filters":                     types.ListNull(resource_report.FiltersValue{}.Type(ctx)),
		"group":                       types.ListNull(resource_report.GroupValue{}.Type(ctx)),
		"metrics":                     types.ListNull(resource_report.MetricsValue{}.Type(ctx)),
		"splits":                      types.ListNull(resource_report.SplitsValue{}.Type(ctx)),
	}
	configVal, diags := resource_report.NewConfigValue(resource_report.ConfigValue{}.AttributeTypes(ctx), configMap)
	if diags.HasError() {
		t.Fatalf("NewConfigValue: %v", diags)
	}

	// Build the tftypes.Value representing the top-level report resource structure
	schemaType := schema.Type().TerraformType(ctx)
	objType, ok := schemaType.(tftypes.Object)
	if !ok {
		t.Fatalf("expected schema to be tftypes.Object, got %T", schemaType)
	}

	attrValues := make(map[string]tftypes.Value, len(objType.AttributeTypes))
	for name, attrType := range objType.AttributeTypes {
		attrValues[name] = tftypes.NewValue(attrType, nil) // default to null
	}

	// Convert config value to terraform value
	configTFVal, err := configVal.ToTerraformValue(ctx)
	if err != nil {
		t.Fatalf("Config ToTerraformValue: %v", err)
	}
	attrValues["config"] = configTFVal

	rawValue := tftypes.NewValue(schemaType, attrValues)
	return tfsdk.Config{
		Schema: schema,
		Raw:    rawValue,
	}
}

func TestReportTimestampValidator_ForecastSettings(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name       string
		futureFrom string
		futureTo   string
		histFrom   string
		histTo     string
		expectErr  bool
	}{
		{
			name:       "valid RFC3339 timestamps",
			futureFrom: "2024-02-02T00:00:00Z",
			futureTo:   "2024-08-02T00:00:00+02:00",
			histFrom:   "2023-01-01T00:00:00-05:00",
			histTo:     "2023-12-31T23:59:59Z",
			expectErr:  false,
		},
		{
			name:       "invalid future_custom_date_range.from format",
			futureFrom: "2024-02-02 00:00:00", // space instead of T, missing offset
			futureTo:   "2024-08-02T00:00:00Z",
			histFrom:   "2023-01-01T00:00:00Z",
			histTo:     "2023-12-31T23:59:59Z",
			expectErr:  true,
		},
		{
			name:       "invalid historical_custom_date_range.to format",
			futureFrom: "2024-02-02T00:00:00Z",
			futureTo:   "2024-08-02T00:00:00Z",
			histFrom:   "2023-01-01T00:00:00Z",
			histTo:     "not-a-timestamp",
			expectErr:  true,
		},
		{
			name:       "empty custom date range (both from and to are empty/null)",
			futureFrom: "",
			futureTo:   "",
			histFrom:   "2023-01-01T00:00:00Z",
			histTo:     "2023-12-31T23:59:59Z",
			expectErr:  true,
		},
		{
			name:       "valid unresolved unknown timestamp during planning (future from)",
			futureFrom: "UNKNOWN",
			futureTo:   "2024-08-02T00:00:00Z",
			histFrom:   "2023-01-01T00:00:00Z",
			histTo:     "2023-12-31T23:59:59Z",
			expectErr:  false,
		},
		{
			name:       "valid unresolved unknown timestamp during planning (historical to)",
			futureFrom: "2024-02-02T00:00:00Z",
			futureTo:   "2024-08-02T00:00:00Z",
			histFrom:   "2023-01-01T00:00:00Z",
			histTo:     "UNKNOWN",
			expectErr:  false,
		},
		{
			name:       "valid unresolved unknown timestamp during planning (only future from set as unknown, to null)",
			futureFrom: "UNKNOWN",
			futureTo:   "",
			histFrom:   "2023-01-01T00:00:00Z",
			histTo:     "2023-12-31T23:59:59Z",
			expectErr:  false,
		},
		{
			name:       "valid unresolved unknown timestamp during planning (only historical to set as unknown, from null)",
			futureFrom: "2024-02-02T00:00:00Z",
			futureTo:   "2024-08-02T00:00:00Z",
			histFrom:   "",
			histTo:     "UNKNOWN",
			expectErr:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := buildReportConfigWithForecastSettings(ctx, t, tt.futureFrom, tt.futureTo, tt.histFrom, tt.histTo)
			v := reportTimestampValidator{}
			req := resource.ValidateConfigRequest{Config: config}
			resp := &resource.ValidateConfigResponse{}
			v.ValidateResource(ctx, req, resp)

			if tt.expectErr {
				if !resp.Diagnostics.HasError() {
					t.Fatalf("expected validation error but got none")
				}
			} else {
				if resp.Diagnostics.HasError() {
					t.Fatalf("expected no validation error but got: %v", resp.Diagnostics)
				}
			}
		})
	}
}
