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
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
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
	ctx := t.Context()
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
	ctx := t.Context()

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
	ctx := t.Context()
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
	ctx := t.Context()

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
	ctx := t.Context()

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
	ctx := t.Context()

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
	ctx := t.Context()

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
	ctx := t.Context()

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
		"limit_aggregation":           types.StringNull(),
		"sort_dimensions":             types.StringNull(),
		"sort_groups":                 types.StringNull(),
		"time_interval":               types.StringNull(),
		"forecast_settings":           fsVal,
		"custom_time_range":           resource_report.NewCustomTimeRangeValueNull(),
		"secondary_time_range":        resource_report.NewSecondaryTimeRangeValueNull(),
		"time_range":                  resource_report.NewTimeRangeValueNull(),
		"advanced_analysis":           resource_report.NewAdvancedAnalysisValueNull(),
		"display_settings":            resource_report.NewDisplaySettingsValueNull(),
		"metric_filter":               resource_report.NewMetricFilterValueNull(),
		"limit_by_change":             resource_report.NewLimitByChangeValueNull(),
		"dimensions":                  types.ListNull(resource_report.DimensionsValue{}.Type(ctx)),
		"filters":                     types.ListNull(resource_report.FiltersValue{}.Type(ctx)),
		"group":                       types.ListNull(resource_report.GroupValue{}.Type(ctx)),
		"metrics":                     types.ListNull(resource_report.MetricsValue{}.Type(ctx)),
		"splits":                      types.ListNull(resource_report.SplitsValue{}.Type(ctx)),
		"count":                       resource_report.NewCountValueNull(),
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
	ctx := t.Context()

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

// buildForecastConfigValue builds a ConfigValue with forecast_settings set and,
// when forecast is non-nil, advanced_analysis.forecast set to that value.
func buildForecastConfigValue(ctx context.Context, t *testing.T, forecast *bool) resource_report.ConfigValue {
	t.Helper()

	fsMap := map[string]attr.Value{
		"future_custom_date_range":     resource_report.NewFutureCustomDateRangeValueNull(),
		"future_time_intervals":        types.Int64Value(12),
		"historical_custom_date_range": resource_report.NewHistoricalCustomDateRangeValueNull(),
		"historical_time_intervals":    types.Int64Null(),
		"mode":                         types.StringValue("totals"),
	}
	fsVal, diags := resource_report.NewForecastSettingsValue(resource_report.ForecastSettingsValue{}.AttributeTypes(ctx), fsMap)
	if diags.HasError() {
		t.Fatalf("NewForecastSettingsValue: %v", diags)
	}

	advVal := resource_report.NewAdvancedAnalysisValueNull()
	if forecast != nil {
		advVal, diags = resource_report.NewAdvancedAnalysisValue(resource_report.AdvancedAnalysisValue{}.AttributeTypes(ctx), map[string]attr.Value{
			"forecast":      types.BoolValue(*forecast),
			"not_trending":  types.BoolNull(),
			"trending_down": types.BoolNull(),
			"trending_up":   types.BoolNull(),
		})
		if diags.HasError() {
			t.Fatalf("NewAdvancedAnalysisValue: %v", diags)
		}
	}

	configMap := map[string]attr.Value{
		"aggregation":                 types.StringNull(),
		"currency":                    types.StringNull(),
		"data_source":                 types.StringNull(),
		"display_values":              types.StringNull(),
		"include_promotional_credits": types.BoolNull(),
		"include_subtotals":           types.BoolNull(),
		"layout":                      types.StringNull(),
		"limit_aggregation":           types.StringNull(),
		"sort_dimensions":             types.StringNull(),
		"sort_groups":                 types.StringNull(),
		"time_interval":               types.StringNull(),
		"forecast_settings":           fsVal,
		"custom_time_range":           resource_report.NewCustomTimeRangeValueNull(),
		"secondary_time_range":        resource_report.NewSecondaryTimeRangeValueNull(),
		"time_range":                  resource_report.NewTimeRangeValueNull(),
		"advanced_analysis":           advVal,
		"display_settings":            resource_report.NewDisplaySettingsValueNull(),
		"metric_filter":               resource_report.NewMetricFilterValueNull(),
		"limit_by_change":             resource_report.NewLimitByChangeValueNull(),
		"dimensions":                  types.ListNull(resource_report.DimensionsValue{}.Type(ctx)),
		"filters":                     types.ListNull(resource_report.FiltersValue{}.Type(ctx)),
		"group":                       types.ListNull(resource_report.GroupValue{}.Type(ctx)),
		"metrics":                     types.ListNull(resource_report.MetricsValue{}.Type(ctx)),
		"splits":                      types.ListNull(resource_report.SplitsValue{}.Type(ctx)),
		"count":                       resource_report.NewCountValueNull(),
	}
	configVal, diags := resource_report.NewConfigValue(resource_report.ConfigValue{}.AttributeTypes(ctx), configMap)
	if diags.HasError() {
		t.Fatalf("NewConfigValue: %v", diags)
	}
	return configVal
}

// TestToExternalConfig_ForecastFalseWithSettings verifies that when
// advanced_analysis.forecast=false accompanies forecast_settings, toExternalConfig
// serializes both the false flag and the non-null settings (the API tolerates this
// and enables forecasting; see TestAccReport_ForecastSettings_RetainedForecastFalse).
func TestToExternalConfig_ForecastFalseWithSettings(t *testing.T) {
	ctx := t.Context()
	forecastFalse := false

	cfg := buildForecastConfigValue(ctx, t, &forecastFalse)
	ext, diags := toExternalConfig(ctx, cfg)
	if diags.HasError() {
		t.Fatalf("toExternalConfig: %v", diags)
	}

	if !ext.ForecastSettings.IsSpecified() || ext.ForecastSettings.IsNull() {
		t.Fatalf("expected forecast_settings to be sent, got specified=%v null=%v",
			ext.ForecastSettings.IsSpecified(), ext.ForecastSettings.IsNull())
	}
	if ext.AdvancedAnalysis == nil || ext.AdvancedAnalysis.Forecast == nil || *ext.AdvancedAnalysis.Forecast {
		t.Fatalf("expected serialized request to carry advanced_analysis.forecast=false alongside forecast_settings")
	}
}

// ---------------------------------------------------------------------------
// TestReportCumulativeComparisonValidator
// ---------------------------------------------------------------------------

func buildReportConfigWithMap(ctx context.Context, t *testing.T, configMap map[string]attr.Value) tfsdk.Config {
	t.Helper()
	schema := resource_report.ReportResourceSchema(ctx)
	configVal, diags := resource_report.NewConfigValue(resource_report.ConfigValue{}.AttributeTypes(ctx), configMap)
	if diags.HasError() {
		t.Fatalf("NewConfigValue: %v", diags)
	}

	schemaType := schema.Type().TerraformType(ctx)
	objType, ok := schemaType.(tftypes.Object)
	if !ok {
		t.Fatalf("expected schema to be tftypes.Object, got %T", schemaType)
	}

	attrValues := make(map[string]tftypes.Value, len(objType.AttributeTypes))
	for name, attrType := range objType.AttributeTypes {
		attrValues[name] = tftypes.NewValue(attrType, nil)
	}

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

func validCumulativeComparisonConfigMap(ctx context.Context, t *testing.T) map[string]attr.Value {
	t.Helper()

	dim1 := resource_report.NewDimensionsValueMust(resource_report.DimensionsValue{}.AttributeTypes(ctx), map[string]attr.Value{
		"id":   types.StringValue("year"),
		"type": types.StringValue("datetime"),
	})
	dim2 := resource_report.NewDimensionsValueMust(resource_report.DimensionsValue{}.AttributeTypes(ctx), map[string]attr.Value{
		"id":   types.StringValue("month"),
		"type": types.StringValue("datetime"),
	})
	dim3 := resource_report.NewDimensionsValueMust(resource_report.DimensionsValue{}.AttributeTypes(ctx), map[string]attr.Value{
		"id":   types.StringValue("day"),
		"type": types.StringValue("datetime"),
	})
	dimsVal, diags := types.ListValueFrom(ctx, resource_report.DimensionsValue{}.Type(ctx), []resource_report.DimensionsValue{dim1, dim2, dim3})
	if diags.HasError() {
		t.Fatalf("ListValueFrom dimensions: %v", diags)
	}

	m1 := resource_report.NewMetricsValueMust(resource_report.MetricsValue{}.AttributeTypes(ctx), map[string]attr.Value{
		"type":  types.StringValue("basic"),
		"value": types.StringValue("cost"),
	})
	metricsVal, diags := types.ListValueFrom(ctx, resource_report.MetricsValue{}.Type(ctx), []resource_report.MetricsValue{m1})
	if diags.HasError() {
		t.Fatalf("ListValueFrom metrics: %v", diags)
	}

	secTrVal := resource_report.NewSecondaryTimeRangeValueMust(resource_report.SecondaryTimeRangeValue{}.AttributeTypes(ctx), map[string]attr.Value{
		"amount":            types.Int64Value(1),
		"custom_time_range": resource_report.NewCustomTimeRangeValueNull(),
		"include_current":   types.BoolValue(false),
		"unit":              types.StringValue("month"),
	})

	return map[string]attr.Value{
		"aggregation":                 types.StringValue("total"),
		"currency":                    types.StringNull(),
		"data_source":                 types.StringNull(),
		"display_values":              types.StringNull(),
		"include_promotional_credits": types.BoolNull(),
		"include_subtotals":           types.BoolNull(),
		"layout":                      types.StringValue("cumulative_comparison"),
		"limit_aggregation":           types.StringNull(),
		"sort_dimensions":             types.StringNull(),
		"sort_groups":                 types.StringNull(),
		"time_interval":               types.StringNull(),
		"forecast_settings":           resource_report.NewForecastSettingsValueNull(),
		"custom_time_range":           resource_report.NewCustomTimeRangeValueNull(),
		"secondary_time_range":        secTrVal,
		"time_range":                  resource_report.NewTimeRangeValueNull(),
		"advanced_analysis":           resource_report.NewAdvancedAnalysisValueNull(),
		"display_settings":            resource_report.NewDisplaySettingsValueNull(),
		"metric_filter":               resource_report.NewMetricFilterValueNull(),
		"limit_by_change":             resource_report.NewLimitByChangeValueNull(),
		"dimensions":                  dimsVal,
		"filters":                     types.ListNull(resource_report.FiltersValue{}.Type(ctx)),
		"group":                       types.ListNull(resource_report.GroupValue{}.Type(ctx)),
		"metrics":                     metricsVal,
		"splits":                      types.ListNull(resource_report.SplitsValue{}.Type(ctx)),
		"count":                       resource_report.NewCountValueNull(),
	}
}

func TestReportCumulativeComparisonValidator(t *testing.T) {
	ctx := t.Context()

	dimAttrTypes := resource_report.DimensionsValue{}.AttributeTypes(ctx)
	dimYear := resource_report.NewDimensionsValueMust(dimAttrTypes, map[string]attr.Value{
		"id":   types.StringValue("year"),
		"type": types.StringValue("datetime"),
	})
	dimMonth := resource_report.NewDimensionsValueMust(dimAttrTypes, map[string]attr.Value{
		"id":   types.StringValue("month"),
		"type": types.StringValue("datetime"),
	})
	dimDay := resource_report.NewDimensionsValueMust(dimAttrTypes, map[string]attr.Value{
		"id":   types.StringValue("day"),
		"type": types.StringValue("datetime"),
	})
	dimWeek := resource_report.NewDimensionsValueMust(dimAttrTypes, map[string]attr.Value{
		"id":   types.StringValue("week"),
		"type": types.StringValue("datetime"),
	})
	dimFixedYear := resource_report.NewDimensionsValueMust(dimAttrTypes, map[string]attr.Value{
		"id":   types.StringValue("year"),
		"type": types.StringValue("fixed"),
	})
	dimService := resource_report.NewDimensionsValueMust(dimAttrTypes, map[string]attr.Value{
		"id":   types.StringValue("service_description"),
		"type": types.StringValue("fixed"),
	})

	metricAttrTypes := resource_report.MetricsValue{}.AttributeTypes(ctx)
	m1 := resource_report.NewMetricsValueMust(metricAttrTypes, map[string]attr.Value{
		"type":  types.StringValue("basic"),
		"value": types.StringValue("cost"),
	})
	m2 := resource_report.NewMetricsValueMust(metricAttrTypes, map[string]attr.Value{
		"type":  types.StringValue("basic"),
		"value": types.StringValue("usage"),
	})

	tests := []struct {
		name      string
		mutate    func(cfg map[string]attr.Value)
		expectErr bool
	}{
		{
			name:      "valid cumulative comparison configuration",
			mutate:    func(cfg map[string]attr.Value) {},
			expectErr: false,
		},
		{
			name: "non-cumulative layout table with missing requirements",
			mutate: func(cfg map[string]attr.Value) {
				cfg["layout"] = types.StringValue("table")
				cfg["dimensions"] = types.ListNull(resource_report.DimensionsValue{}.Type(ctx))
				cfg["aggregation"] = types.StringNull()
				cfg["secondary_time_range"] = resource_report.NewSecondaryTimeRangeValueNull()
			},
			expectErr: false,
		},
		{
			name: "layout is null",
			mutate: func(cfg map[string]attr.Value) {
				cfg["layout"] = types.StringNull()
				cfg["secondary_time_range"] = resource_report.NewSecondaryTimeRangeValueNull()
			},
			expectErr: false,
		},
		{
			name: "layout is unknown",
			mutate: func(cfg map[string]attr.Value) {
				cfg["layout"] = types.StringUnknown()
				cfg["secondary_time_range"] = resource_report.NewSecondaryTimeRangeValueNull()
			},
			expectErr: false,
		},
		{
			name: "missing dimensions (null)",
			mutate: func(cfg map[string]attr.Value) {
				cfg["dimensions"] = types.ListNull(resource_report.DimensionsValue{}.Type(ctx))
			},
			expectErr: true,
		},
		{
			name: "empty dimensions list",
			mutate: func(cfg map[string]attr.Value) {
				emptyDims, d := types.ListValueFrom(ctx, resource_report.DimensionsValue{}.Type(ctx), []resource_report.DimensionsValue{})
				if d.HasError() {
					t.Fatalf("ListValueFrom emptyDims: %v", d)
				}
				cfg["dimensions"] = emptyDims
			},
			expectErr: true,
		},
		{
			name: "insufficient dimensions (only year and month)",
			mutate: func(cfg map[string]attr.Value) {
				twoDims, d := types.ListValueFrom(ctx, resource_report.DimensionsValue{}.Type(ctx), []resource_report.DimensionsValue{dimYear, dimMonth})
				if d.HasError() {
					t.Fatalf("ListValueFrom twoDims: %v", d)
				}
				cfg["dimensions"] = twoDims
			},
			expectErr: true,
		},
		{
			name: "excess dimensions (4 dimensions)",
			mutate: func(cfg map[string]attr.Value) {
				fourDims, d := types.ListValueFrom(ctx, resource_report.DimensionsValue{}.Type(ctx), []resource_report.DimensionsValue{dimYear, dimMonth, dimDay, dimService})
				if d.HasError() {
					t.Fatalf("ListValueFrom fourDims: %v", d)
				}
				cfg["dimensions"] = fourDims
			},
			expectErr: true,
		},
		{
			name: "wrong dimension type (year of type fixed)",
			mutate: func(cfg map[string]attr.Value) {
				wrongTypeDims, d := types.ListValueFrom(ctx, resource_report.DimensionsValue{}.Type(ctx), []resource_report.DimensionsValue{dimFixedYear, dimMonth, dimDay})
				if d.HasError() {
					t.Fatalf("ListValueFrom wrongTypeDims: %v", d)
				}
				cfg["dimensions"] = wrongTypeDims
			},
			expectErr: true,
		},
		{
			name: "wrong dimension id (year, month, week)",
			mutate: func(cfg map[string]attr.Value) {
				wrongIdDims, d := types.ListValueFrom(ctx, resource_report.DimensionsValue{}.Type(ctx), []resource_report.DimensionsValue{dimYear, dimMonth, dimWeek})
				if d.HasError() {
					t.Fatalf("ListValueFrom wrongIdDims: %v", d)
				}
				cfg["dimensions"] = wrongIdDims
			},
			expectErr: true,
		},
		{
			name: "wrong dimension order (day, month, year)",
			mutate: func(cfg map[string]attr.Value) {
				wrongOrderDims, d := types.ListValueFrom(ctx, resource_report.DimensionsValue{}.Type(ctx), []resource_report.DimensionsValue{dimDay, dimMonth, dimYear})
				if d.HasError() {
					t.Fatalf("ListValueFrom wrongOrderDims: %v", d)
				}
				cfg["dimensions"] = wrongOrderDims
			},
			expectErr: true,
		},
		{
			name: "missing aggregation (null)",
			mutate: func(cfg map[string]attr.Value) {
				cfg["aggregation"] = types.StringNull()
			},
			expectErr: true,
		},
		{
			name: "wrong aggregation (percent_total)",
			mutate: func(cfg map[string]attr.Value) {
				cfg["aggregation"] = types.StringValue("percent_total")
			},
			expectErr: true,
		},
		{
			name: "multiple metrics (2 metrics)",
			mutate: func(cfg map[string]attr.Value) {
				twoMetrics, d := types.ListValueFrom(ctx, resource_report.MetricsValue{}.Type(ctx), []resource_report.MetricsValue{m1, m2})
				if d.HasError() {
					t.Fatalf("ListValueFrom twoMetrics: %v", d)
				}
				cfg["metrics"] = twoMetrics
			},
			expectErr: true,
		},
		{
			name: "missing secondary time range (null)",
			mutate: func(cfg map[string]attr.Value) {
				cfg["secondary_time_range"] = resource_report.NewSecondaryTimeRangeValueNull()
			},
			expectErr: true,
		},
		{
			name: "comparative display values percentage_change",
			mutate: func(cfg map[string]attr.Value) {
				cfg["display_values"] = types.StringValue("percentage_change")
			},
			expectErr: true,
		},
		{
			name: "comparative display values absolute_change",
			mutate: func(cfg map[string]attr.Value) {
				cfg["display_values"] = types.StringValue("absolute_change")
			},
			expectErr: true,
		},
		{
			name: "comparative display values absolute_and_percentage",
			mutate: func(cfg map[string]attr.Value) {
				cfg["display_values"] = types.StringValue("absolute_and_percentage")
			},
			expectErr: true,
		},
		{
			name: "valid display values actuals_only",
			mutate: func(cfg map[string]attr.Value) {
				cfg["display_values"] = types.StringValue("actuals_only")
			},
			expectErr: false,
		},
		{
			name: "forecast settings set",
			mutate: func(cfg map[string]attr.Value) {
				fsVal, diags := resource_report.NewForecastSettingsValue(resource_report.ForecastSettingsValue{}.AttributeTypes(ctx), map[string]attr.Value{
					"future_custom_date_range":     resource_report.NewFutureCustomDateRangeValueNull(),
					"future_time_intervals":        types.Int64Value(6),
					"historical_custom_date_range": resource_report.NewHistoricalCustomDateRangeValueNull(),
					"historical_time_intervals":    types.Int64Null(),
					"mode":                         types.StringValue("totals"),
				})
				if diags.HasError() {
					t.Fatalf("NewForecastSettingsValue: %v", diags)
				}
				cfg["forecast_settings"] = fsVal
			},
			expectErr: true,
		},
		{
			name: "advanced analysis forecast is true",
			mutate: func(cfg map[string]attr.Value) {
				advVal, diags := resource_report.NewAdvancedAnalysisValue(resource_report.AdvancedAnalysisValue{}.AttributeTypes(ctx), map[string]attr.Value{
					"forecast":      types.BoolValue(true),
					"not_trending":  types.BoolNull(),
					"trending_down": types.BoolNull(),
					"trending_up":   types.BoolNull(),
				})
				if diags.HasError() {
					t.Fatalf("NewAdvancedAnalysisValue: %v", diags)
				}
				cfg["advanced_analysis"] = advVal
			},
			expectErr: true,
		},
		{
			name: "advanced analysis forecast is false",
			mutate: func(cfg map[string]attr.Value) {
				advVal, diags := resource_report.NewAdvancedAnalysisValue(resource_report.AdvancedAnalysisValue{}.AttributeTypes(ctx), map[string]attr.Value{
					"forecast":      types.BoolValue(false),
					"not_trending":  types.BoolNull(),
					"trending_down": types.BoolNull(),
					"trending_up":   types.BoolNull(),
				})
				if diags.HasError() {
					t.Fatalf("NewAdvancedAnalysisValue: %v", diags)
				}
				cfg["advanced_analysis"] = advVal
			},
			expectErr: false,
		},
		{
			name: "unknown dimensions deferred",
			mutate: func(cfg map[string]attr.Value) {
				cfg["dimensions"] = types.ListUnknown(resource_report.DimensionsValue{}.Type(ctx))
			},
			expectErr: false,
		},
		{
			name: "unknown dimension element deferred",
			mutate: func(cfg map[string]attr.Value) {
				unknownDim := resource_report.NewDimensionsValueUnknown()
				dimsWithUnknown, d := types.ListValueFrom(ctx, resource_report.DimensionsValue{}.Type(ctx), []resource_report.DimensionsValue{dimYear, dimMonth, unknownDim})
				if d.HasError() {
					t.Fatalf("ListValueFrom dimsWithUnknown: %v", d)
				}
				cfg["dimensions"] = dimsWithUnknown
			},
			expectErr: false,
		},
		{
			name: "unknown aggregation deferred",
			mutate: func(cfg map[string]attr.Value) {
				cfg["aggregation"] = types.StringUnknown()
			},
			expectErr: false,
		},
		{
			name: "unknown secondary time range deferred",
			mutate: func(cfg map[string]attr.Value) {
				cfg["secondary_time_range"] = resource_report.NewSecondaryTimeRangeValueUnknown()
			},
			expectErr: false,
		},
		{
			name: "unknown display values deferred",
			mutate: func(cfg map[string]attr.Value) {
				cfg["display_values"] = types.StringUnknown()
			},
			expectErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfgMap := validCumulativeComparisonConfigMap(ctx, t)
			tt.mutate(cfgMap)
			config := buildReportConfigWithMap(ctx, t, cfgMap)

			// Test data source config validator
			dsv := reportCumulativeComparisonDataSourceValidator{}
			dsReq := datasource.ValidateConfigRequest{Config: config}
			dsResp := &datasource.ValidateConfigResponse{}
			dsv.ValidateDataSource(ctx, dsReq, dsResp)

			if tt.expectErr {
				if !dsResp.Diagnostics.HasError() {
					t.Fatalf("expected data source validator error but got none")
				}
			} else {
				if dsResp.Diagnostics.HasError() {
					t.Fatalf("expected no data source validator error but got: %v", dsResp.Diagnostics)
				}
			}

			// Test plan validation (used in ModifyPlan) shares identical behavior
			plan := tfsdk.Plan(config)
			var planDiags diag.Diagnostics
			validateReportCumulativeComparison(ctx, plan, &planDiags)
			if tt.expectErr {
				if !planDiags.HasError() {
					t.Fatalf("expected plan validation error but got none")
				}
			} else {
				if planDiags.HasError() {
					t.Fatalf("expected no plan validation error but got: %v", planDiags)
				}
			}
		})
	}
}

func TestReportResource_ModifyPlan_CumulativeComparisonInherited(t *testing.T) {
	ctx := t.Context()

	// Prior state has layout = "cumulative_comparison" and valid config
	validMap := validCumulativeComparisonConfigMap(ctx, t)
	stateCfg := buildReportConfigWithMap(ctx, t, validMap)
	state := tfsdk.State(stateCfg)

	// Proposed plan has layout omitted/unknown (inheriting cumulative_comparison from state)
	// but invalid dimensions (only 1 dimension)
	invalidMap := validCumulativeComparisonConfigMap(ctx, t)
	invalidMap["layout"] = types.StringUnknown()
	dim1 := resource_report.NewDimensionsValueMust(resource_report.DimensionsValue{}.AttributeTypes(ctx), map[string]attr.Value{
		"id":   types.StringValue("year"),
		"type": types.StringValue("datetime"),
	})
	dimsVal, diags := types.ListValueFrom(ctx, resource_report.DimensionsValue{}.Type(ctx), []resource_report.DimensionsValue{dim1})
	if diags.HasError() {
		t.Fatalf("ListValueFrom dimensions: %v", diags)
	}
	invalidMap["dimensions"] = dimsVal

	planCfg := buildReportConfigWithMap(ctx, t, invalidMap)
	plan := tfsdk.Plan(planCfg)

	// Config on update has layout omitted (null)
	omittedMap := validCumulativeComparisonConfigMap(ctx, t)
	omittedMap["layout"] = types.StringNull()
	configCfg := buildReportConfigWithMap(ctx, t, omittedMap)

	r := &reportResource{}
	req := resource.ModifyPlanRequest{
		State:  state,
		Config: configCfg,
		Plan:   plan,
	}
	resp := &resource.ModifyPlanResponse{}

	r.ModifyPlan(ctx, req, resp)

	if !resp.Diagnostics.HasError() {
		t.Fatal("expected ModifyPlan to fail on invalid dimensions when layout is cumulative_comparison in plan")
	}

	foundExpectedError := false
	for _, d := range resp.Diagnostics.Errors() {
		if d.Summary() == "Invalid Dimensions Configuration" {
			foundExpectedError = true
			break
		}
	}
	if !foundExpectedError {
		t.Fatalf("expected 'Invalid Dimensions Configuration' error, got: %v", resp.Diagnostics)
	}
}

// TestReportResource_ModifyPlan_ExplicitUnknownLayoutDeferred asserts that an explicitly
// unknown planned layout (e.g. from a dynamic expression in HCL) defers validation
// rather than substituting the prior state's layout, so transitions away from cumulative_comparison
// are not falsely rejected before the layout is known.
func TestReportResource_ModifyPlan_ExplicitUnknownLayoutDeferred(t *testing.T) {
	ctx := t.Context()

	// Prior state has layout = "cumulative_comparison" and valid config
	validMap := validCumulativeComparisonConfigMap(ctx, t)
	stateCfg := buildReportConfigWithMap(ctx, t, validMap)
	state := tfsdk.State(stateCfg)

	// In config and plan, layout is explicitly UNKNOWN (e.g. referencing an unknown dynamic output)
	// Dimensions only has 1 dimension (valid for other layouts, but invalid for cumulative_comparison)
	unknownLayoutMap := validCumulativeComparisonConfigMap(ctx, t)
	unknownLayoutMap["layout"] = types.StringUnknown()
	dim1 := resource_report.NewDimensionsValueMust(resource_report.DimensionsValue{}.AttributeTypes(ctx), map[string]attr.Value{
		"id":   types.StringValue("year"),
		"type": types.StringValue("datetime"),
	})
	dimsVal, diags := types.ListValueFrom(ctx, resource_report.DimensionsValue{}.Type(ctx), []resource_report.DimensionsValue{dim1})
	if diags.HasError() {
		t.Fatalf("ListValueFrom dimensions: %v", diags)
	}
	unknownLayoutMap["dimensions"] = dimsVal

	cfg := buildReportConfigWithMap(ctx, t, unknownLayoutMap)
	plan := tfsdk.Plan(cfg)

	r := &reportResource{}
	req := resource.ModifyPlanRequest{
		State:  state,
		Config: cfg,
		Plan:   plan,
	}
	resp := &resource.ModifyPlanResponse{}

	r.ModifyPlan(ctx, req, resp)

	// Since layout is unknown in config, validation must be deferred and NO errors should be produced.
	if resp.Diagnostics.HasError() {
		t.Fatalf("expected unknown layout to defer validation without errors, but got: %v", resp.Diagnostics)
	}
}

// TestReportCumulativeComparison_UnknownMetricsDeferred asserts that an unknown
// metrics list defers the one-metric check rather than rejecting it with
// "0 metrics were configured". The list resolves at apply time, so rejecting at
// plan time would fail a config that is in fact valid.
func TestReportCumulativeComparison_UnknownMetricsDeferred(t *testing.T) {
	ctx := t.Context()

	cfgMap := validCumulativeComparisonConfigMap(ctx, t)
	cfgMap["metrics"] = types.ListUnknown(resource_report.MetricsValue{}.Type(ctx))

	cfg := buildReportConfigWithMap(ctx, t, cfgMap)
	plan := tfsdk.Plan(cfg)

	var planDiags diag.Diagnostics
	validateReportCumulativeComparison(ctx, plan, &planDiags)

	if planDiags.HasError() {
		t.Fatalf("expected unknown metrics to defer validation without errors, but got: %v", planDiags)
	}
}

// TestReportCumulativeComparison_UnknownMetrics_StillValidatesOtherAttributes asserts
// that when metrics is unknown the validator defers only the metric check, and
// continues validating other known attributes (such as missing secondary_time_range).
func TestReportCumulativeComparison_UnknownMetrics_StillValidatesOtherAttributes(t *testing.T) {
	ctx := t.Context()

	cfgMap := validCumulativeComparisonConfigMap(ctx, t)
	cfgMap["metrics"] = types.ListUnknown(resource_report.MetricsValue{}.Type(ctx))

	// Missing secondary_time_range is an independent violation
	cfgMap["secondary_time_range"] = resource_report.NewSecondaryTimeRangeValueNull()

	cfg := buildReportConfigWithMap(ctx, t, cfgMap)
	plan := tfsdk.Plan(cfg)

	var planDiags diag.Diagnostics
	validateReportCumulativeComparison(ctx, plan, &planDiags)

	if !planDiags.HasError() {
		t.Fatal("expected validator to catch missing secondary_time_range even when metric is unknown, but got no errors")
	}

	foundExpected := false
	for _, d := range planDiags.Errors() {
		if d.Summary() == "Missing Secondary Time Range" {
			foundExpected = true
			break
		}
	}
	if !foundExpected {
		t.Fatalf("expected 'Missing Secondary Time Range' error, got: %v", planDiags)
	}
}

func TestReportCumulativeComparison_PartialUnknownStillReportsDefiniteErrors(t *testing.T) {
	ctx := t.Context()
	dimensionTypes := resource_report.DimensionsValue{}.AttributeTypes(ctx)
	dimension := func(id, dimensionType string) resource_report.DimensionsValue {
		return resource_report.NewDimensionsValueMust(dimensionTypes, map[string]attr.Value{
			"id":   types.StringValue(id),
			"type": types.StringValue(dimensionType),
		})
	}
	metricTypes := resource_report.MetricsValue{}.AttributeTypes(ctx)
	knownMetric := resource_report.NewMetricsValueMust(metricTypes, map[string]attr.Value{
		"type":  types.StringValue("basic"),
		"value": types.StringValue("cost"),
	})

	tests := []struct {
		name        string
		validate    func(*diag.Diagnostics)
		wantCount   int
		wantSummary string
		wantPath    path.Path
	}{
		{
			name: "four dimensions remain invalid with an unknown element",
			validate: func(diagnostics *diag.Diagnostics) {
				values := []resource_report.DimensionsValue{
					dimension("year", "datetime"),
					dimension("month", "datetime"),
					dimension("day", "datetime"),
					resource_report.NewDimensionsValueUnknown(),
				}
				list, listDiags := types.ListValueFrom(ctx, resource_report.DimensionsValue{}.Type(ctx), values)
				if listDiags.HasError() {
					t.Fatalf("build dimensions: %v", listDiags)
				}
				validateReportCumulativeDimensions(ctx, list, diagnostics)
			},
			wantCount:   1,
			wantSummary: "Invalid Dimensions Configuration",
			wantPath:    path.Root("config").AtName("dimensions"),
		},
		{
			name: "known wrong dimension remains invalid beside an unknown element",
			validate: func(diagnostics *diag.Diagnostics) {
				values := []resource_report.DimensionsValue{
					dimension("week", "datetime"),
					resource_report.NewDimensionsValueUnknown(),
					dimension("day", "datetime"),
				}
				list, listDiags := types.ListValueFrom(ctx, resource_report.DimensionsValue{}.Type(ctx), values)
				if listDiags.HasError() {
					t.Fatalf("build dimensions: %v", listDiags)
				}
				validateReportCumulativeDimensions(ctx, list, diagnostics)
			},
			wantCount:   1,
			wantSummary: "Invalid Dimensions Configuration",
			wantPath:    path.Root("config").AtName("dimensions"),
		},
		{
			name: "known wrong dimension type remains invalid beside an unknown field",
			validate: func(diagnostics *diag.Diagnostics) {
				unknownID := resource_report.NewDimensionsValueMust(dimensionTypes, map[string]attr.Value{
					"id":   types.StringUnknown(),
					"type": types.StringValue("datetime"),
				})
				values := []resource_report.DimensionsValue{
					dimension("year", "datetime"),
					unknownID,
					dimension("day", "wrong"),
				}
				list, listDiags := types.ListValueFrom(ctx, resource_report.DimensionsValue{}.Type(ctx), values)
				if listDiags.HasError() {
					t.Fatalf("build dimensions: %v", listDiags)
				}
				validateReportCumulativeDimensions(ctx, list, diagnostics)
			},
			wantCount:   1,
			wantSummary: "Invalid Dimensions Configuration",
			wantPath:    path.Root("config").AtName("dimensions"),
		},
		{
			name: "three dimensions defer an unknown element",
			validate: func(diagnostics *diag.Diagnostics) {
				values := []resource_report.DimensionsValue{
					dimension("year", "datetime"),
					resource_report.NewDimensionsValueUnknown(),
					dimension("day", "datetime"),
				}
				list, listDiags := types.ListValueFrom(ctx, resource_report.DimensionsValue{}.Type(ctx), values)
				if listDiags.HasError() {
					t.Fatalf("build dimensions: %v", listDiags)
				}
				validateReportCumulativeDimensions(ctx, list, diagnostics)
			},
			wantCount: 0,
		},
		{
			name: "null dimension element is invalid",
			validate: func(diagnostics *diag.Diagnostics) {
				values := []resource_report.DimensionsValue{
					dimension("year", "datetime"),
					resource_report.NewDimensionsValueNull(),
					dimension("day", "datetime"),
				}
				list, listDiags := types.ListValueFrom(ctx, resource_report.DimensionsValue{}.Type(ctx), values)
				if listDiags.HasError() {
					t.Fatalf("build dimensions: %v", listDiags)
				}
				validateReportCumulativeDimensions(ctx, list, diagnostics)
			},
			wantCount:   1,
			wantSummary: "Invalid Dimensions Configuration",
			wantPath:    path.Root("config").AtName("dimensions"),
		},
		{
			name: "two metrics remain invalid with an unknown element",
			validate: func(diagnostics *diag.Diagnostics) {
				values := []resource_report.MetricsValue{knownMetric, resource_report.NewMetricsValueUnknown()}
				list, listDiags := types.ListValueFrom(ctx, resource_report.MetricsValue{}.Type(ctx), values)
				if listDiags.HasError() {
					t.Fatalf("build metrics: %v", listDiags)
				}
				validateReportCumulativeMetrics(list, diagnostics)
			},
			wantCount:   1,
			wantSummary: "Invalid Metrics Configuration",
			wantPath:    path.Root("config").AtName("metrics"),
		},
		{
			name: "one unknown metric defers element validation",
			validate: func(diagnostics *diag.Diagnostics) {
				values := []resource_report.MetricsValue{resource_report.NewMetricsValueUnknown()}
				list, listDiags := types.ListValueFrom(ctx, resource_report.MetricsValue{}.Type(ctx), values)
				if listDiags.HasError() {
					t.Fatalf("build metrics: %v", listDiags)
				}
				validateReportCumulativeMetrics(list, diagnostics)
			},
			wantCount: 0,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var diagnostics diag.Diagnostics
			test.validate(&diagnostics)
			if len(diagnostics) != test.wantCount {
				t.Fatalf("diagnostic count = %d, want %d: %v", len(diagnostics), test.wantCount, diagnostics)
			}
			if test.wantCount == 0 {
				return
			}
			diagnostic := diagnostics[0]
			if diagnostic.Severity() != diag.SeverityError {
				t.Errorf("severity = %s, want error", diagnostic.Severity())
			}
			if diagnostic.Summary() != test.wantSummary {
				t.Errorf("summary = %q, want %q", diagnostic.Summary(), test.wantSummary)
			}
			withPath, ok := diagnostic.(diag.DiagnosticWithPath)
			if !ok {
				t.Fatalf("diagnostic %T has no path", diagnostic)
			}
			if got := withPath.Path(); !got.Equal(test.wantPath) {
				t.Errorf("path = %s, want %s", got, test.wantPath)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// TestToExternalConfig_ClearableNestedAttributes
// ---------------------------------------------------------------------------

// TestToExternalConfig_ClearableNestedAttributes verifies that toExternalConfig
// populates nullable.Nullable fields with SetNull() when count, custom_time_range,
// limit_by_change, and metric_filter are null on ConfigValue, and with Set(...)
// when they are populated.
func TestToExternalConfig_ClearableNestedAttributes(t *testing.T) {
	ctx := t.Context()

	t.Run("all null sends explicit JSON null", func(t *testing.T) {
		cfgMap := validCumulativeComparisonConfigMap(ctx, t)
		cfgVal, diags := resource_report.NewConfigValue(resource_report.ConfigValue{}.AttributeTypes(ctx), cfgMap)
		if diags.HasError() {
			t.Fatalf("NewConfigValue: %v", diags)
		}

		ext, extDiags := toExternalConfig(ctx, cfgVal)
		if extDiags.HasError() {
			t.Fatalf("toExternalConfig: %v", extDiags)
		}

		if !ext.Count.IsSpecified() || !ext.Count.IsNull() {
			t.Errorf("expected Count to be specified and null, got specified=%v null=%v", ext.Count.IsSpecified(), ext.Count.IsNull())
		}
		if !ext.CustomTimeRange.IsSpecified() || !ext.CustomTimeRange.IsNull() {
			t.Errorf("expected CustomTimeRange to be specified and null, got specified=%v null=%v", ext.CustomTimeRange.IsSpecified(), ext.CustomTimeRange.IsNull())
		}
		if !ext.LimitByChange.IsSpecified() || !ext.LimitByChange.IsNull() {
			t.Errorf("expected LimitByChange to be specified and null, got specified=%v null=%v", ext.LimitByChange.IsSpecified(), ext.LimitByChange.IsNull())
		}
		if !ext.MetricFilter.IsSpecified() || !ext.MetricFilter.IsNull() {
			t.Errorf("expected MetricFilter to be specified and null, got specified=%v null=%v", ext.MetricFilter.IsSpecified(), ext.MetricFilter.IsNull())
		}
	})

	t.Run("all populated sends non-null values", func(t *testing.T) {
		cfgMap := validCumulativeComparisonConfigMap(ctx, t)
		cfgMap["count"] = resource_report.NewCountValueMust(resource_report.CountValue{}.AttributeTypes(ctx), map[string]attr.Value{
			"id":   types.StringValue("service_description"),
			"type": types.StringValue("fixed"),
		})
		cfgMap["custom_time_range"] = resource_report.NewCustomTimeRangeValueMust(resource_report.CustomTimeRangeValue{}.AttributeTypes(ctx), map[string]attr.Value{
			"from": types.StringValue("2025-01-01T00:00:00Z"),
			"to":   types.StringValue("2025-02-01T00:00:00Z"),
		})
		metricObj := resource_report.NewMetricValueMust(resource_report.MetricValue{}.AttributeTypes(ctx), map[string]attr.Value{
			"type":  types.StringValue("basic"),
			"value": types.StringValue("cost"),
		})
		cfgMap["limit_by_change"] = resource_report.NewLimitByChangeValueMust(resource_report.LimitByChangeValue{}.AttributeTypes(ctx), map[string]attr.Value{
			"change_type":             types.StringValue("percentage"),
			"include_incomplete_data": types.BoolValue(false),
			"metric":                  metricObj,
			"operator":                types.StringValue("gte"),
			"values":                  types.ListValueMust(types.Float64Type, []attr.Value{types.Float64Value(50)}),
		})
		cfgMap["metric_filter"] = resource_report.NewMetricFilterValueMust(resource_report.MetricFilterValue{}.AttributeTypes(ctx), map[string]attr.Value{
			"metric":   metricObj,
			"operand":  types.StringNull(),
			"operator": types.StringValue("gt"),
			"values":   types.ListValueMust(types.Float64Type, []attr.Value{types.Float64Value(100)}),
		})

		cfgVal, diags := resource_report.NewConfigValue(resource_report.ConfigValue{}.AttributeTypes(ctx), cfgMap)
		if diags.HasError() {
			t.Fatalf("NewConfigValue: %v", diags)
		}

		ext, extDiags := toExternalConfig(ctx, cfgVal)
		if extDiags.HasError() {
			t.Fatalf("toExternalConfig: %v", extDiags)
		}

		if !ext.Count.IsSpecified() || ext.Count.IsNull() {
			t.Errorf("expected Count to be specified and non-null, got specified=%v null=%v", ext.Count.IsSpecified(), ext.Count.IsNull())
		}
		if !ext.CustomTimeRange.IsSpecified() || ext.CustomTimeRange.IsNull() {
			t.Errorf("expected CustomTimeRange to be specified and non-null, got specified=%v null=%v", ext.CustomTimeRange.IsSpecified(), ext.CustomTimeRange.IsNull())
		}
		if !ext.LimitByChange.IsSpecified() || ext.LimitByChange.IsNull() {
			t.Errorf("expected LimitByChange to be specified and non-null, got specified=%v null=%v", ext.LimitByChange.IsSpecified(), ext.LimitByChange.IsNull())
		}
		if !ext.MetricFilter.IsSpecified() || ext.MetricFilter.IsNull() {
			t.Errorf("expected MetricFilter to be specified and non-null, got specified=%v null=%v", ext.MetricFilter.IsSpecified(), ext.MetricFilter.IsNull())
		}
	})
}

// ---------------------------------------------------------------------------
// TestReportCustomTimeRangeModeValidator
// ---------------------------------------------------------------------------

func TestReportCustomTimeRangeModeValidator(t *testing.T) {
	ctx := t.Context()

	knownCTR := resource_report.NewCustomTimeRangeValueMust(resource_report.CustomTimeRangeValue{}.AttributeTypes(ctx), map[string]attr.Value{
		"from": types.StringValue("2025-01-01T00:00:00Z"),
		"to":   types.StringValue("2025-02-01T00:00:00Z"),
	})
	nullCTR := resource_report.NewCustomTimeRangeValueNull()
	unknownCTR := resource_report.NewCustomTimeRangeValueUnknown()

	makeTimeRange := func(mode types.String) resource_report.TimeRangeValue {
		return resource_report.NewTimeRangeValueMust(resource_report.TimeRangeValue{}.AttributeTypes(ctx), map[string]attr.Value{
			"amount":          types.Int64Value(1),
			"include_current": types.BoolValue(true),
			"mode":            mode,
			"unit":            types.StringValue("month"),
		})
	}
	trCustom := makeTimeRange(types.StringValue("custom"))
	trLast := makeTimeRange(types.StringValue("last"))
	trCurrent := makeTimeRange(types.StringValue("current"))
	trUnknownMode := makeTimeRange(types.StringUnknown())
	trNullMode := makeTimeRange(types.StringNull())
	nullTR := resource_report.NewTimeRangeValueNull()
	unknownTR := resource_report.NewTimeRangeValueUnknown()

	ctrPath := path.Root("config").AtName("custom_time_range")

	tests := []struct {
		name        string
		ctr         resource_report.CustomTimeRangeValue
		tr          resource_report.TimeRangeValue
		priorTR     *resource_report.TimeRangeValue
		wantCount   int
		wantSummary string
		wantPath    path.Path
	}{
		{
			name:      "both custom_time_range and time_range are null",
			ctr:       nullCTR,
			tr:        nullTR,
			wantCount: 0,
		},
		{
			name:      "valid custom_time_range with time_range.mode = custom",
			ctr:       knownCTR,
			tr:        trCustom,
			wantCount: 0,
		},
		{
			name:      "valid time_range.mode = last without custom_time_range",
			ctr:       nullCTR,
			tr:        trLast,
			wantCount: 0,
		},
		{
			name:        "invalid custom_time_range set when time_range is null",
			ctr:         knownCTR,
			tr:          nullTR,
			wantCount:   1,
			wantSummary: "Invalid Custom Time Range Configuration",
			wantPath:    ctrPath,
		},
		{
			name:        "invalid custom_time_range set when time_range.mode is last",
			ctr:         knownCTR,
			tr:          trLast,
			wantCount:   1,
			wantSummary: "Invalid Custom Time Range Configuration",
			wantPath:    ctrPath,
		},
		{
			name:        "invalid custom_time_range set when time_range.mode is current",
			ctr:         knownCTR,
			tr:          trCurrent,
			wantCount:   1,
			wantSummary: "Invalid Custom Time Range Configuration",
			wantPath:    ctrPath,
		},
		{
			name:        "invalid custom_time_range set when time_range.mode is null",
			ctr:         knownCTR,
			tr:          trNullMode,
			wantCount:   1,
			wantSummary: "Invalid Custom Time Range Configuration",
			wantPath:    ctrPath,
		},
		{
			name:        "invalid time_range.mode = custom when custom_time_range is null",
			ctr:         nullCTR,
			tr:          trCustom,
			wantCount:   1,
			wantSummary: "Missing Custom Time Range",
			wantPath:    ctrPath,
		},
		{
			name:      "unknown custom_time_range defers validation when mode is last",
			ctr:       unknownCTR,
			tr:        trLast,
			wantCount: 0,
		},
		{
			name:      "unknown custom_time_range defers validation when mode is custom",
			ctr:       unknownCTR,
			tr:        trCustom,
			wantCount: 0,
		},
		{
			name:      "unknown time_range defers validation when custom_time_range is set",
			ctr:       knownCTR,
			tr:        unknownTR,
			wantCount: 0,
		},
		{
			name:      "unknown time_range.mode defers validation when custom_time_range is set",
			ctr:       knownCTR,
			tr:        trUnknownMode,
			wantCount: 0,
		},
		{
			name:      "unknown time_range.mode defers validation when custom_time_range is null",
			ctr:       nullCTR,
			tr:        trUnknownMode,
			wantCount: 0,
		},
		{
			name:        "update inherits priorState time_range.mode = custom and rejects cleared custom_time_range",
			ctr:         nullCTR,
			tr:          nullTR,
			priorTR:     &trCustom,
			wantCount:   1,
			wantSummary: "Missing Custom Time Range",
			wantPath:    ctrPath,
		},
		{
			name:      "update inherits priorState time_range.mode = custom and allows kept custom_time_range",
			ctr:       knownCTR,
			tr:        nullTR,
			priorTR:   &trCustom,
			wantCount: 0,
		},
		{
			name:        "update inherits priorState time_range.mode = last and rejects added custom_time_range",
			ctr:         knownCTR,
			tr:          nullTR,
			priorTR:     &trLast,
			wantCount:   1,
			wantSummary: "Invalid Custom Time Range Configuration",
			wantPath:    ctrPath,
		},
		{
			name:      "update inherits priorState time_range.mode = last and allows null custom_time_range",
			ctr:       nullCTR,
			tr:        nullTR,
			priorTR:   &trLast,
			wantCount: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfgMap := validCumulativeComparisonConfigMap(ctx, t)
			cfgMap["layout"] = types.StringValue("table")
			cfgMap["custom_time_range"] = tt.ctr
			cfgMap["time_range"] = tt.tr
			config := buildReportConfigWithMap(ctx, t, cfgMap)

			var priorState attributeGetter
			if tt.priorTR != nil {
				priorMap := validCumulativeComparisonConfigMap(ctx, t)
				priorMap["layout"] = types.StringValue("table")
				priorMap["time_range"] = *tt.priorTR
				priorCfg := buildReportConfigWithMap(ctx, t, priorMap)
				priorState = tfsdk.State(priorCfg)
			}

			var diagnostics diag.Diagnostics
			validateReportCustomTimeRangeMode(ctx, config, priorState, &diagnostics)

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
		})
	}
}

// ---------------------------------------------------------------------------
// TestReportCountAggregationValidator_PriorStateInheritance
// ---------------------------------------------------------------------------

func TestReportCountAggregationValidator_PriorStateInheritance(t *testing.T) {
	ctx := t.Context()

	knownCount := resource_report.NewCountValueMust(resource_report.CountValue{}.AttributeTypes(ctx), map[string]attr.Value{
		"id":   types.StringValue("service_description"),
		"type": types.StringValue("fixed"),
	})
	nullCount := resource_report.NewCountValueNull()
	unknownCount := resource_report.NewCountValueUnknown()

	countPath := path.Root("config").AtName("count")

	tests := []struct {
		name        string
		count       resource_report.CountValue
		agg         types.String
		priorAgg    *types.String
		wantCount   int
		wantSummary string
		wantPath    path.Path
	}{
		{
			name:        "update with omitted aggregation inherits priorState aggregation=count and rejects cleared count",
			count:       nullCount,
			agg:         types.StringNull(),
			priorAgg:    new(types.StringValue("count")),
			wantCount:   1,
			wantSummary: "Missing Count Configuration",
			wantPath:    countPath,
		},
		{
			name:      "update with omitted aggregation inherits priorState aggregation=count and allows kept count",
			count:     knownCount,
			agg:       types.StringNull(),
			priorAgg:  new(types.StringValue("count")),
			wantCount: 0,
		},
		{
			name:        "update with omitted aggregation inherits priorState aggregation=total and rejects added count",
			count:       knownCount,
			agg:         types.StringNull(),
			priorAgg:    new(types.StringValue("total")),
			wantCount:   1,
			wantSummary: "Invalid Count Configuration",
			wantPath:    countPath,
		},
		{
			name:      "update with omitted aggregation inherits priorState aggregation=total and allows null count",
			count:     nullCount,
			agg:       types.StringNull(),
			priorAgg:  new(types.StringValue("total")),
			wantCount: 0,
		},
		{
			name:      "update with unknown aggregation defers validation even if priorState had aggregation=count",
			count:     nullCount,
			agg:       types.StringUnknown(),
			priorAgg:  new(types.StringValue("count")),
			wantCount: 0,
		},
		{
			name:      "update with unknown count defers validation",
			count:     unknownCount,
			agg:       types.StringNull(),
			priorAgg:  new(types.StringValue("count")),
			wantCount: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfgMap := validCumulativeComparisonConfigMap(ctx, t)
			cfgMap["layout"] = types.StringValue("table")
			cfgMap["count"] = tt.count
			cfgMap["aggregation"] = tt.agg
			config := buildReportConfigWithMap(ctx, t, cfgMap)

			var priorState attributeGetter
			if tt.priorAgg != nil {
				priorMap := validCumulativeComparisonConfigMap(ctx, t)
				priorMap["layout"] = types.StringValue("table")
				priorMap["aggregation"] = *tt.priorAgg
				priorCfg := buildReportConfigWithMap(ctx, t, priorMap)
				priorState = tfsdk.State(priorCfg)
			}

			var diagnostics diag.Diagnostics
			validateReportCountAggregation(ctx, config, priorState, &diagnostics)

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
		})
	}
}

func buildReportConfigWithConfigValue(ctx context.Context, t *testing.T, configVal resource_report.ConfigValue) tfsdk.Config {
	t.Helper()
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
	attrValues["name"] = tftypes.NewValue(tftypes.String, "test-report")

	configTFVal, err := configVal.ToTerraformValue(ctx)
	if err != nil {
		t.Fatalf("Config ToTerraformValue: %v", err)
	}
	attrValues["config"] = configTFVal

	return tfsdk.Config{
		Schema: schema,
		Raw:    tftypes.NewValue(schemaType, attrValues),
	}
}

// TestReportResource_ModifyPlan_NameOnlyConfigPreservesWrapper verifies that
// omitting the top-level `config` wrapper (or leaving it unknown) on an existing
// or imported report with count aggregation or custom time mode does not
// falsely trigger "Missing Count Configuration" or "Missing Custom Time Range".
// The top-level `config` wrapper is Category B (preserved on omission, not cleared).
func TestReportResource_ModifyPlan_NameOnlyConfigPreservesWrapper(t *testing.T) {
	ctx := t.Context()

	knownCount := resource_report.NewCountValueMust(resource_report.CountValue{}.AttributeTypes(ctx), map[string]attr.Value{
		"id":   types.StringValue("service_description"),
		"type": types.StringValue("fixed"),
	})
	knownCTR := resource_report.NewCustomTimeRangeValueMust(resource_report.CustomTimeRangeValue{}.AttributeTypes(ctx), map[string]attr.Value{
		"from": types.StringValue("2025-01-01T00:00:00Z"),
		"to":   types.StringValue("2025-02-01T00:00:00Z"),
	})
	trCustom := resource_report.NewTimeRangeValueMust(resource_report.TimeRangeValue{}.AttributeTypes(ctx), map[string]attr.Value{
		"amount":          types.Int64Null(),
		"include_current": types.BoolNull(),
		"mode":            types.StringValue("custom"),
		"unit":            types.StringNull(),
	})

	countStateMap := validCumulativeComparisonConfigMap(ctx, t)
	countStateMap["layout"] = types.StringValue("table")
	countStateMap["aggregation"] = types.StringValue("count")
	countStateMap["count"] = knownCount
	countStateMap["secondary_time_range"] = resource_report.NewSecondaryTimeRangeValueNull()

	customTimeStateMap := validCumulativeComparisonConfigMap(ctx, t)
	customTimeStateMap["layout"] = types.StringValue("table")
	customTimeStateMap["time_range"] = trCustom
	customTimeStateMap["custom_time_range"] = knownCTR
	customTimeStateMap["secondary_time_range"] = resource_report.NewSecondaryTimeRangeValueNull()

	tests := []struct {
		name      string
		stateMap  map[string]attr.Value
		configVal resource_report.ConfigValue
	}{
		{
			name:      "name-only config (null wrapper) with count aggregation in prior state",
			stateMap:  countStateMap,
			configVal: resource_report.NewConfigValueNull(),
		},
		{
			name:      "unknown config wrapper with count aggregation in prior state",
			stateMap:  countStateMap,
			configVal: resource_report.NewConfigValueUnknown(),
		},
		{
			name:      "name-only config (null wrapper) with custom time range mode in prior state",
			stateMap:  customTimeStateMap,
			configVal: resource_report.NewConfigValueNull(),
		},
		{
			name:      "unknown config wrapper with custom time range mode in prior state",
			stateMap:  customTimeStateMap,
			configVal: resource_report.NewConfigValueUnknown(),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stateCfg := buildReportConfigWithMap(ctx, t, tt.stateMap)
			configCfg := buildReportConfigWithConfigValue(ctx, t, tt.configVal)

			// ValidateResource (ConfigValidators phase)
			for _, v := range (&reportResource{}).ConfigValidators(ctx) {
				valResp := &resource.ValidateConfigResponse{}
				v.ValidateResource(ctx, resource.ValidateConfigRequest{Config: configCfg}, valResp)
				if valResp.Diagnostics.HasError() {
					t.Fatalf("ConfigValidator %T returned unexpected error: %v", v, valResp.Diagnostics)
				}
			}

			// ModifyPlan phase (plan preserves state config when wrapper is omitted)
			mpReq := resource.ModifyPlanRequest{
				State:  tfsdk.State(stateCfg),
				Config: configCfg,
				Plan:   tfsdk.Plan(stateCfg),
			}
			mpResp := &resource.ModifyPlanResponse{}
			(&reportResource{}).ModifyPlan(ctx, mpReq, mpResp)
			if mpResp.Diagnostics.HasError() {
				t.Fatalf("ModifyPlan returned unexpected error: %v", mpResp.Diagnostics)
			}
		})
	}
}

// TestReportCustomTimeRangeModeValidator_TwoPhaseCreateAndUpdate exercises both
// validation phases (ValidateResource followed by ModifyPlan) to verify that an
// existing custom-mode report that keeps custom_time_range while omitting
// time_range (or time_range.mode) is not rejected during ValidateResource before
// ModifyPlan can inherit the stored mode from prior state, while creates and
// invalid updates are still rejected in ModifyPlan.
func TestReportCustomTimeRangeModeValidator_TwoPhaseCreateAndUpdate(t *testing.T) {
	ctx := t.Context()

	knownCTR := resource_report.NewCustomTimeRangeValueMust(resource_report.CustomTimeRangeValue{}.AttributeTypes(ctx), map[string]attr.Value{
		"from": types.StringValue("2025-01-01T00:00:00Z"),
		"to":   types.StringValue("2025-02-01T00:00:00Z"),
	})
	nullCTR := resource_report.NewCustomTimeRangeValueNull()

	makeTimeRange := func(mode types.String) resource_report.TimeRangeValue {
		return resource_report.NewTimeRangeValueMust(resource_report.TimeRangeValue{}.AttributeTypes(ctx), map[string]attr.Value{
			"amount":          types.Int64Null(),
			"include_current": types.BoolNull(),
			"mode":            mode,
			"unit":            types.StringNull(),
		})
	}
	trCustom := makeTimeRange(types.StringValue("custom"))
	trLast := makeTimeRange(types.StringValue("last"))
	trNullMode := makeTimeRange(types.StringNull())
	nullTR := resource_report.NewTimeRangeValueNull()

	schema := resource_report.ReportResourceSchema(ctx)
	nullState := tfsdk.State{Schema: schema, Raw: tftypes.NewValue(schema.Type().TerraformType(ctx), nil)}

	runTwoPhase := func(t *testing.T, state tfsdk.State, cfgMap map[string]attr.Value) diag.Diagnostics {
		t.Helper()
		configCfg := buildReportConfigWithMap(ctx, t, cfgMap)

		valResp := &resource.ValidateConfigResponse{}
		reportCustomTimeRangeModeValidator{}.ValidateResource(ctx, resource.ValidateConfigRequest{Config: configCfg}, valResp)
		if valResp.Diagnostics.HasError() {
			return valResp.Diagnostics
		}

		mpReq := resource.ModifyPlanRequest{
			State:  state,
			Config: configCfg,
			Plan:   tfsdk.Plan(configCfg),
		}
		mpResp := &resource.ModifyPlanResponse{}
		(&reportResource{}).ModifyPlan(ctx, mpReq, mpResp)
		return mpResp.Diagnostics
	}

	t.Run("create with custom_time_range and omitted time_range fails in ModifyPlan", func(t *testing.T) {
		cfgMap := validCumulativeComparisonConfigMap(ctx, t)
		cfgMap["layout"] = types.StringValue("table")
		cfgMap["custom_time_range"] = knownCTR
		cfgMap["time_range"] = nullTR

		diags := runTwoPhase(t, nullState, cfgMap)
		if !diags.HasError() {
			t.Fatal("expected error on create when custom_time_range is set without time_range, got none")
		}
		if diags.Errors()[0].Summary() != "Invalid Custom Time Range Configuration" {
			t.Fatalf("unexpected error summary: %q", diags.Errors()[0].Summary())
		}
	})

	t.Run("create with custom_time_range and omitted time_range.mode fails in ModifyPlan", func(t *testing.T) {
		cfgMap := validCumulativeComparisonConfigMap(ctx, t)
		cfgMap["layout"] = types.StringValue("table")
		cfgMap["custom_time_range"] = knownCTR
		cfgMap["time_range"] = trNullMode

		diags := runTwoPhase(t, nullState, cfgMap)
		if !diags.HasError() {
			t.Fatal("expected error on create when custom_time_range is set with null time_range.mode, got none")
		}
		if diags.Errors()[0].Summary() != "Invalid Custom Time Range Configuration" {
			t.Fatalf("unexpected error summary: %q", diags.Errors()[0].Summary())
		}
	})

	t.Run("update keeping custom_time_range while omitting time_range succeeds via prior state", func(t *testing.T) {
		stateMap := validCumulativeComparisonConfigMap(ctx, t)
		stateMap["layout"] = types.StringValue("table")
		stateMap["custom_time_range"] = knownCTR
		stateMap["time_range"] = trCustom
		state := tfsdk.State(buildReportConfigWithMap(ctx, t, stateMap))

		cfgMap := validCumulativeComparisonConfigMap(ctx, t)
		cfgMap["layout"] = types.StringValue("table")
		cfgMap["custom_time_range"] = knownCTR
		cfgMap["time_range"] = nullTR

		diags := runTwoPhase(t, state, cfgMap)
		if diags.HasError() {
			t.Fatalf("expected no error when updating custom-mode report with omitted time_range, got: %v", diags)
		}
	})

	t.Run("update keeping custom_time_range while omitting time_range.mode succeeds via prior state", func(t *testing.T) {
		stateMap := validCumulativeComparisonConfigMap(ctx, t)
		stateMap["layout"] = types.StringValue("table")
		stateMap["custom_time_range"] = knownCTR
		stateMap["time_range"] = trCustom
		state := tfsdk.State(buildReportConfigWithMap(ctx, t, stateMap))

		cfgMap := validCumulativeComparisonConfigMap(ctx, t)
		cfgMap["layout"] = types.StringValue("table")
		cfgMap["custom_time_range"] = knownCTR
		cfgMap["time_range"] = trNullMode

		diags := runTwoPhase(t, state, cfgMap)
		if diags.HasError() {
			t.Fatalf("expected no error when updating custom-mode report with omitted time_range.mode, got: %v", diags)
		}
	})

	t.Run("update adding custom_time_range while omitting time_range on last-mode report fails in ModifyPlan", func(t *testing.T) {
		stateMap := validCumulativeComparisonConfigMap(ctx, t)
		stateMap["layout"] = types.StringValue("table")
		stateMap["custom_time_range"] = nullCTR
		stateMap["time_range"] = trLast
		state := tfsdk.State(buildReportConfigWithMap(ctx, t, stateMap))

		cfgMap := validCumulativeComparisonConfigMap(ctx, t)
		cfgMap["layout"] = types.StringValue("table")
		cfgMap["custom_time_range"] = knownCTR
		cfgMap["time_range"] = nullTR

		diags := runTwoPhase(t, state, cfgMap)
		if !diags.HasError() {
			t.Fatal("expected error when adding custom_time_range to last-mode report with omitted time_range, got none")
		}
		if diags.Errors()[0].Summary() != "Invalid Custom Time Range Configuration" {
			t.Fatalf("unexpected error summary: %q", diags.Errors()[0].Summary())
		}
	})

	t.Run("data source with custom_time_range and omitted time_range fails in ValidateConfig", func(t *testing.T) {
		cfgMap := validCumulativeComparisonConfigMap(ctx, t)
		cfgMap["layout"] = types.StringValue("table")
		cfgMap["custom_time_range"] = knownCTR
		cfgMap["time_range"] = nullTR
		configCfg := buildReportConfigWithMap(ctx, t, cfgMap)

		dsResp := &datasource.ValidateConfigResponse{}
		(&reportQueryDataSource{}).ValidateConfig(ctx, datasource.ValidateConfigRequest{Config: configCfg}, dsResp)
		if !dsResp.Diagnostics.HasError() {
			t.Fatal("expected data source ValidateConfig error when custom_time_range is set without time_range, got none")
		}
		if dsResp.Diagnostics.Errors()[0].Summary() != "Invalid Custom Time Range Configuration" {
			t.Fatalf("unexpected error summary: %q", dsResp.Diagnostics.Errors()[0].Summary())
		}
	})
}

// TestReportCountAggregationValidator_TwoPhaseCreateAndUpdate exercises both
// validation phases (ValidateResource followed by ModifyPlan) for count and
// aggregation so that an existing count-mode report that keeps count while
// omitting aggregation succeeds across both phases.
func TestReportCountAggregationValidator_TwoPhaseCreateAndUpdate(t *testing.T) {
	ctx := t.Context()

	knownCount := resource_report.NewCountValueMust(resource_report.CountValue{}.AttributeTypes(ctx), map[string]attr.Value{
		"id":   types.StringValue("service_description"),
		"type": types.StringValue("fixed"),
	})
	nullCount := resource_report.NewCountValueNull()

	schema := resource_report.ReportResourceSchema(ctx)
	nullState := tfsdk.State{Schema: schema, Raw: tftypes.NewValue(schema.Type().TerraformType(ctx), nil)}

	runTwoPhase := func(t *testing.T, state tfsdk.State, cfgMap map[string]attr.Value) diag.Diagnostics {
		t.Helper()
		configCfg := buildReportConfigWithMap(ctx, t, cfgMap)

		valResp := &resource.ValidateConfigResponse{}
		reportCountAggregationValidator{}.ValidateResource(ctx, resource.ValidateConfigRequest{Config: configCfg}, valResp)
		if valResp.Diagnostics.HasError() {
			return valResp.Diagnostics
		}

		mpReq := resource.ModifyPlanRequest{
			State:  state,
			Config: configCfg,
			Plan:   tfsdk.Plan(configCfg),
		}
		mpResp := &resource.ModifyPlanResponse{}
		(&reportResource{}).ModifyPlan(ctx, mpReq, mpResp)
		return mpResp.Diagnostics
	}

	t.Run("create with count and omitted aggregation fails in ModifyPlan", func(t *testing.T) {
		cfgMap := validCumulativeComparisonConfigMap(ctx, t)
		cfgMap["layout"] = types.StringValue("table")
		cfgMap["count"] = knownCount
		cfgMap["aggregation"] = types.StringNull()

		diags := runTwoPhase(t, nullState, cfgMap)
		if !diags.HasError() {
			t.Fatal("expected error on create when count is set without aggregation, got none")
		}
		if diags.Errors()[0].Summary() != "Invalid Count Configuration" {
			t.Fatalf("unexpected error summary: %q", diags.Errors()[0].Summary())
		}
	})

	t.Run("update keeping count while omitting aggregation succeeds via prior state", func(t *testing.T) {
		stateMap := validCumulativeComparisonConfigMap(ctx, t)
		stateMap["layout"] = types.StringValue("table")
		stateMap["count"] = knownCount
		stateMap["aggregation"] = types.StringValue("count")
		state := tfsdk.State(buildReportConfigWithMap(ctx, t, stateMap))

		cfgMap := validCumulativeComparisonConfigMap(ctx, t)
		cfgMap["layout"] = types.StringValue("table")
		cfgMap["count"] = knownCount
		cfgMap["aggregation"] = types.StringNull()

		diags := runTwoPhase(t, state, cfgMap)
		if diags.HasError() {
			t.Fatalf("expected no error when updating count-mode report with omitted aggregation, got: %v", diags)
		}
	})

	t.Run("update adding count while omitting aggregation on total-mode report fails in ModifyPlan", func(t *testing.T) {
		stateMap := validCumulativeComparisonConfigMap(ctx, t)
		stateMap["layout"] = types.StringValue("table")
		stateMap["count"] = nullCount
		stateMap["aggregation"] = types.StringValue("total")
		state := tfsdk.State(buildReportConfigWithMap(ctx, t, stateMap))

		cfgMap := validCumulativeComparisonConfigMap(ctx, t)
		cfgMap["layout"] = types.StringValue("table")
		cfgMap["count"] = knownCount
		cfgMap["aggregation"] = types.StringNull()

		diags := runTwoPhase(t, state, cfgMap)
		if !diags.HasError() {
			t.Fatal("expected error when adding count to total-mode report with omitted aggregation, got none")
		}
		if diags.Errors()[0].Summary() != "Invalid Count Configuration" {
			t.Fatalf("unexpected error summary: %q", diags.Errors()[0].Summary())
		}
	})

	t.Run("data source with count and omitted aggregation fails in ValidateConfig", func(t *testing.T) {
		cfgMap := validCumulativeComparisonConfigMap(ctx, t)
		cfgMap["layout"] = types.StringValue("table")
		cfgMap["count"] = knownCount
		cfgMap["aggregation"] = types.StringNull()
		configCfg := buildReportConfigWithMap(ctx, t, cfgMap)

		dsResp := &datasource.ValidateConfigResponse{}
		(&reportQueryDataSource{}).ValidateConfig(ctx, datasource.ValidateConfigRequest{Config: configCfg}, dsResp)
		if !dsResp.Diagnostics.HasError() {
			t.Fatal("expected data source ValidateConfig error when count is set without aggregation, got none")
		}
		if dsResp.Diagnostics.Errors()[0].Summary() != "Invalid Count Configuration" {
			t.Fatalf("unexpected error summary: %q", dsResp.Diagnostics.Errors()[0].Summary())
		}
	})
}

// TestUseNullModifiers_PreserveWhenConfigWrapperNull verifies that when the
// top-level `config` block is omitted (Null) or Unknown in req.Config, the
// Category A child plan modifiers (count, custom_time_range, limit_by_change,
// metric_filter) do not overwrite the preserved prior-state plan value with Null.
func TestUseNullModifiers_PreserveWhenConfigWrapperNull(t *testing.T) {
	ctx := t.Context()

	knownCount := types.ObjectValueMust(resource_report.CountValue{}.AttributeTypes(ctx), map[string]attr.Value{
		"id":   types.StringValue("service_description"),
		"type": types.StringValue("fixed"),
	})
	nullCount := types.ObjectNull(resource_report.CountValue{}.AttributeTypes(ctx))

	knownCTR := types.ObjectValueMust(resource_report.CustomTimeRangeValue{}.AttributeTypes(ctx), map[string]attr.Value{
		"from": types.StringValue("2025-01-01T00:00:00Z"),
		"to":   types.StringValue("2025-02-01T00:00:00Z"),
	})
	nullCTR := types.ObjectNull(resource_report.CustomTimeRangeValue{}.AttributeTypes(ctx))

	metricObj := resource_report.NewMetricValueMust(resource_report.MetricValue{}.AttributeTypes(ctx), map[string]attr.Value{
		"type":  types.StringValue("basic"),
		"value": types.StringValue("cost"),
	})
	knownLBC := types.ObjectValueMust(resource_report.LimitByChangeValue{}.AttributeTypes(ctx), map[string]attr.Value{
		"change_type":             types.StringValue("percentage"),
		"include_incomplete_data": types.BoolValue(false),
		"metric":                  metricObj,
		"operator":                types.StringValue("gte"),
		"values":                  types.ListValueMust(types.Float64Type, []attr.Value{types.Float64Value(50)}),
	})
	nullLBC := types.ObjectNull(resource_report.LimitByChangeValue{}.AttributeTypes(ctx))

	knownMF := types.ObjectValueMust(resource_report.MetricFilterValue{}.AttributeTypes(ctx), map[string]attr.Value{
		"metric":   metricObj,
		"operand":  types.StringNull(),
		"operator": types.StringValue("gt"),
		"values":   types.ListValueMust(types.Float64Type, []attr.Value{types.Float64Value(100)}),
	})
	nullMF := types.ObjectNull(resource_report.MetricFilterValue{}.AttributeTypes(ctx))

	modifiers := []struct {
		name     string
		mod      planmodifier.Object
		knownObj types.Object
		nullObj  types.Object
	}{
		{"count", useNullForCount(), knownCount, nullCount},
		{"custom_time_range", useNullForCustomTimeRange(), knownCTR, nullCTR},
		{"limit_by_change", useNullForLimitByChange(), knownLBC, nullLBC},
		{"metric_filter", useNullForMetricFilter(), knownMF, nullMF},
	}

	nullWrapperCfg := buildReportConfigWithConfigValue(ctx, t, resource_report.NewConfigValueNull())
	unknownWrapperCfg := buildReportConfigWithConfigValue(ctx, t, resource_report.NewConfigValueUnknown())
	presentWrapperCfg := buildReportConfigWithMap(ctx, t, validCumulativeComparisonConfigMap(ctx, t))

	for _, tc := range modifiers {
		t.Run(tc.name+"/preserves_when_config_wrapper_is_null", func(t *testing.T) {
			req := planmodifier.ObjectRequest{
				Config:      nullWrapperCfg,
				ConfigValue: tc.nullObj,
				StateValue:  tc.knownObj,
				PlanValue:   tc.knownObj,
			}
			resp := &planmodifier.ObjectResponse{PlanValue: tc.knownObj}
			tc.mod.PlanModifyObject(ctx, req, resp)
			if resp.PlanValue.IsNull() {
				t.Fatalf("expected %s plan modifier to preserve PlanValue when top-level config is null, but got null", tc.name)
			}
		})

		t.Run(tc.name+"/preserves_when_config_wrapper_is_unknown", func(t *testing.T) {
			req := planmodifier.ObjectRequest{
				Config:      unknownWrapperCfg,
				ConfigValue: tc.nullObj,
				StateValue:  tc.knownObj,
				PlanValue:   tc.knownObj,
			}
			resp := &planmodifier.ObjectResponse{PlanValue: tc.knownObj}
			tc.mod.PlanModifyObject(ctx, req, resp)
			if resp.PlanValue.IsNull() {
				t.Fatalf("expected %s plan modifier to preserve PlanValue when top-level config is unknown, but got null", tc.name)
			}
		})

		t.Run(tc.name+"/clears_when_config_wrapper_is_present_and_child_is_null", func(t *testing.T) {
			req := planmodifier.ObjectRequest{
				Config:      presentWrapperCfg,
				ConfigValue: tc.nullObj,
				StateValue:  tc.knownObj,
				PlanValue:   tc.knownObj,
			}
			resp := &planmodifier.ObjectResponse{PlanValue: tc.knownObj}
			tc.mod.PlanModifyObject(ctx, req, resp)
			if !resp.PlanValue.IsNull() {
				t.Fatalf("expected %s plan modifier to set PlanValue to null when config is present and child is omitted", tc.name)
			}
		})
	}
}
