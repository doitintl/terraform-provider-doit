package provider

import (
	"context"
	"fmt"
	"time"

	"github.com/doitintl/terraform-provider-doit/internal/provider/resource_report"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
)

// warnNASentinels appends a Warning diagnostic for every string inside valueLists
// that matches the legacy NullFallback sentinel pattern (e.g. "[Service N/A]").
//
// basePath is the path to the list attribute that holds the filter/scope blocks
// (e.g. path.Root("config").AtName("filters") for reports).
// Each element of valueLists corresponds to one filter/scope block at index i;
// the attribute path reported in the diagnostic is basePath[i].values.
//
// This is a path-agnostic helper shared by all resource validators.
//
// We use []basetypes.StringValue as the ElementsAs target instead of []string
// so that unknown and null elements (e.g. cross-resource references like
// doit_allocation.xxx.id during plan) are represented natively rather than
// causing a "Value Conversion Error" crash.
func warnNASentinels(ctx context.Context, basePath path.Path, valueLists []types.List, diags *diag.Diagnostics) {
	for i, vl := range valueLists {
		if vl.IsNull() || vl.IsUnknown() {
			continue
		}
		var vals []basetypes.StringValue
		if d := vl.ElementsAs(ctx, &vals, false); d.HasError() {
			diags.Append(d...)
			continue
		}
		for _, val := range vals {
			if val.IsUnknown() || val.IsNull() {
				continue
			}
			if isNAFallback(val.ValueString()) {
				diags.AddAttributeWarning(
					basePath.AtListIndex(i).AtName("values"),
					"Deprecated Value Syntax",
					fmt.Sprintf(
						"%q uses the legacy NullFallback sentinel syntax. "+
							"Use `include_null = true` on this block instead — it is semantically "+
							"equivalent and avoids unexpected behaviour when running `terraform import`.",
						val.ValueString(),
					),
				)
			}
		}
	}
}

// reportMetricsLengthValidator validates that metrics list has 1-4 items when specified.
// The API supports up to 4 metrics per report.
type reportMetricsLengthValidator struct{}

var _ resource.ConfigValidator = reportMetricsLengthValidator{}

func (v reportMetricsLengthValidator) Description(_ context.Context) string {
	return "Validates that metrics list has 1-4 items when specified"
}

func (v reportMetricsLengthValidator) MarkdownDescription(_ context.Context) string {
	return "Validates that `metrics` list has 1-4 items when specified"
}

func (v reportMetricsLengthValidator) ValidateResource(ctx context.Context, req resource.ValidateConfigRequest, resp *resource.ValidateConfigResponse) {
	var metrics types.List

	// Get config.metrics
	diags := req.Config.GetAttribute(ctx, path.Root("config").AtName("metrics"), &metrics)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Skip validation if null/unknown - let API handle default behavior
	if metrics.IsNull() || metrics.IsUnknown() {
		return
	}

	// Check for empty list
	// The API silently preserves existing metrics when an empty array is sent,
	// rather than clearing them. This causes a Terraform consistency error
	// because the planned state (empty) differs from the actual state (preserved).
	// To prevent this, we require at least one metric if the attribute is specified.
	if len(metrics.Elements()) == 0 {
		resp.Diagnostics.AddAttributeError(
			path.Root("config").AtName("metrics"),
			"Empty Metrics List Not Supported",
			"The 'metrics' attribute cannot be set to an empty list. "+
				"The API does not support clearing metrics with an empty array - it silently preserves existing values, "+
				"causing Terraform state inconsistency. Options:\n"+
				"  - Specify 1-4 metrics: metrics = [{type = \"basic\", value = \"cost\"}]\n"+
				"  - Omit the attribute entirely to let the API use its default metric",
		)
		return
	}

	// Check for max 4 metrics
	if len(metrics.Elements()) > 4 {
		resp.Diagnostics.AddAttributeError(
			path.Root("config").AtName("metrics"),
			"Invalid Metrics Configuration",
			fmt.Sprintf("Report can have up to 4 metrics. Found %d metrics.", len(metrics.Elements())),
		)
	}
}

// reportForecastConflictValidator rejects configurations where forecast_settings
// is set but forecast is explicitly false. The API automatically enables forecast
// when forecastSettings is provided, so sending forecast=false alongside it causes
// a 500 server error or perpetual drift.
type reportForecastConflictValidator struct{}

var _ resource.ConfigValidator = reportForecastConflictValidator{}

func (v reportForecastConflictValidator) Description(_ context.Context) string {
	return "Validates that forecast_settings is not set when forecast is explicitly disabled"
}

func (v reportForecastConflictValidator) MarkdownDescription(_ context.Context) string {
	return "Validates that `forecast_settings` is not set when `forecast` is explicitly disabled"
}

func (v reportForecastConflictValidator) ValidateResource(ctx context.Context, req resource.ValidateConfigRequest, resp *resource.ValidateConfigResponse) {
	var forecastSettings resource_report.ForecastSettingsValue
	diags := req.Config.GetAttribute(ctx, path.Root("config").AtName("forecast_settings"), &forecastSettings)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() || forecastSettings.IsNull() || forecastSettings.IsUnknown() {
		return
	}

	var forecast types.Bool
	diags = req.Config.GetAttribute(ctx, path.Root("config").AtName("advanced_analysis").AtName("forecast"), &forecast)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() || forecast.IsNull() || forecast.IsUnknown() {
		return
	}

	if !forecast.ValueBool() {
		resp.Diagnostics.AddAttributeError(
			path.Root("config").AtName("forecast_settings"),
			"Conflicting Forecast Configuration",
			"Cannot set forecast_settings when advanced_analysis.forecast is false. "+
				"The API automatically enables forecasting when forecast_settings is provided. "+
				"Either remove forecast_settings or set forecast = true.",
		)
	}
}

// reportTimestampValidator validates custom_time_range objects:
// 1. When set, both from and to must be specified.
// 2. Any provided from/to values must be valid RFC3339 timestamps.
//
// This is a ConfigValidator because attribute-level validators do not fire on
// attributes inside SingleNestedAttribute with CustomType (which the code
// generator adds to all nested objects).
type reportTimestampValidator struct{}

var _ resource.ConfigValidator = reportTimestampValidator{}

func (v reportTimestampValidator) Description(_ context.Context) string {
	return "Validates custom_time_range objects set both from and to and contain valid RFC3339 timestamps"
}

func (v reportTimestampValidator) MarkdownDescription(_ context.Context) string {
	return "Validates `custom_time_range` objects set both `from` and `to` and contain valid RFC3339 timestamps"
}

func (v reportTimestampValidator) ValidateResource(ctx context.Context, req resource.ValidateConfigRequest, resp *resource.ValidateConfigResponse) {
	validateReportTimestamps(ctx, req.Config, &resp.Diagnostics)
}

// reportTimestampDataSourceValidator is the data-source counterpart of
// reportTimestampValidator. The report_query data source reuses the report
// resource's config types, so the same validation applies — rejecting incomplete
// date ranges at plan time instead of letting them fail at the API.
type reportTimestampDataSourceValidator struct{}

var _ datasource.ConfigValidator = reportTimestampDataSourceValidator{}

func (v reportTimestampDataSourceValidator) Description(_ context.Context) string {
	return "Validates custom_time_range objects set both from and to and contain valid RFC3339 timestamps"
}

func (v reportTimestampDataSourceValidator) MarkdownDescription(_ context.Context) string {
	return "Validates `custom_time_range` objects set both `from` and `to` and contain valid RFC3339 timestamps"
}

func (v reportTimestampDataSourceValidator) ValidateDataSource(ctx context.Context, req datasource.ValidateConfigRequest, resp *datasource.ValidateConfigResponse) {
	validateReportTimestamps(ctx, req.Config, &resp.Diagnostics)
}

// validateReportTimestamps is the shared body used by both the resource and the
// report_query data source. It rejects custom time ranges missing a bound and
// empty forecast date ranges, and validates that any provided from/to values are
// RFC3339. Unknown values are deferred (not treated as empty) so dynamic
// references validate once resolved.
func validateReportTimestamps(ctx context.Context, config tfsdk.Config, diags *diag.Diagnostics) {
	// A configured custom_time_range needs both bounds: the API replaces the stored
	// range with the configured block, so an omitted bound is not kept from the
	// existing report.
	ctrPaths := []path.Path{
		path.Root("config").AtName("custom_time_range"),
		path.Root("config").AtName("secondary_time_range").AtName("custom_time_range"),
	}
	for _, p := range ctrPaths {
		var ctr resource_report.CustomTimeRangeValue
		d := config.GetAttribute(ctx, p, &ctr)
		diags.Append(d...)
		if d.HasError() || ctr.IsNull() || ctr.IsUnknown() {
			continue
		}

		if ctr.From.IsNull() {
			diags.AddAttributeError(
				p.AtName("from"),
				"Missing Custom Time Range Field",
				"`from` is required when `custom_time_range` is set.",
			)
		}
		if ctr.To.IsNull() {
			diags.AddAttributeError(
				p.AtName("to"),
				"Missing Custom Time Range Field",
				"`to` is required when `custom_time_range` is set.",
			)
		}
	}

	futureCtrPaths := []path.Path{
		path.Root("config").AtName("forecast_settings").AtName("future_custom_date_range"),
	}
	for _, p := range futureCtrPaths {
		var ctr resource_report.FutureCustomDateRangeValue
		d := config.GetAttribute(ctx, p, &ctr)
		diags.Append(d...)
		if d.HasError() || ctr.IsNull() || ctr.IsUnknown() {
			continue
		}
		if ctr.From.IsUnknown() || ctr.To.IsUnknown() {
			continue // defer validation until values are known
		}
		if ctr.From.IsNull() && ctr.To.IsNull() {
			diags.AddAttributeError(
				p,
				"Empty Future Custom Date Range",
				"future_custom_date_range requires at least one of `from` or `to` to be set.",
			)
		}
	}

	historicalCtrPaths := []path.Path{
		path.Root("config").AtName("forecast_settings").AtName("historical_custom_date_range"),
	}
	for _, p := range historicalCtrPaths {
		var ctr resource_report.HistoricalCustomDateRangeValue
		d := config.GetAttribute(ctx, p, &ctr)
		diags.Append(d...)
		if d.HasError() || ctr.IsNull() || ctr.IsUnknown() {
			continue
		}
		if ctr.From.IsUnknown() || ctr.To.IsUnknown() {
			continue // defer validation until values are known
		}
		if ctr.From.IsNull() && ctr.To.IsNull() {
			diags.AddAttributeError(
				p,
				"Empty Historical Custom Date Range",
				"historical_custom_date_range requires at least one of `from` or `to` to be set.",
			)
		}
	}

	// Validate individual timestamp formats.
	timestampPaths := []path.Path{
		path.Root("config").AtName("custom_time_range").AtName("from"),
		path.Root("config").AtName("custom_time_range").AtName("to"),
		path.Root("config").AtName("secondary_time_range").AtName("custom_time_range").AtName("from"),
		path.Root("config").AtName("secondary_time_range").AtName("custom_time_range").AtName("to"),
		path.Root("config").AtName("forecast_settings").AtName("future_custom_date_range").AtName("from"),
		path.Root("config").AtName("forecast_settings").AtName("future_custom_date_range").AtName("to"),
		path.Root("config").AtName("forecast_settings").AtName("historical_custom_date_range").AtName("from"),
		path.Root("config").AtName("forecast_settings").AtName("historical_custom_date_range").AtName("to"),
	}

	for _, p := range timestampPaths {
		var val types.String
		d := config.GetAttribute(ctx, p, &val)
		diags.Append(d...)
		if d.HasError() {
			continue
		}
		if val.IsNull() || val.IsUnknown() {
			continue
		}
		validateRFC3339(val.ValueString(), p, diags)
	}
}

// reportCountAggregationValidator enforces the two-way coupling between
// config.count and config.aggregation:
//   - count is only valid when aggregation is "count" (the count object selects
//     the field whose distinct values are counted; it is meaningless otherwise); and
//   - aggregation = "count" REQUIRES a count block (the API has no default counted
//     field).
//
// The API rejects either mismatch with a 400, so surface both at plan time.
//
// This is a ConfigValidator because attribute-level validators do not fire on
// attributes inside SingleNestedAttribute with CustomType.
type reportCountAggregationValidator struct{}

var _ resource.ConfigValidator = reportCountAggregationValidator{}

func (v reportCountAggregationValidator) Description(_ context.Context) string {
	return "Validates that count is set if and only if aggregation is \"count\""
}

func (v reportCountAggregationValidator) MarkdownDescription(_ context.Context) string {
	return "Validates that `count` is set if and only if `aggregation` is `\"count\"`"
}

func (v reportCountAggregationValidator) ValidateResource(ctx context.Context, req resource.ValidateConfigRequest, resp *resource.ValidateConfigResponse) {
	// Defer when aggregation is omitted in config so ModifyPlan can inherit the
	// stored aggregation on update (or reject on create).
	var aggregation types.String
	d := req.Config.GetAttribute(ctx, path.Root("config").AtName("aggregation"), &aggregation)
	resp.Diagnostics.Append(d...)
	if d.HasError() || aggregation.IsNull() {
		return
	}
	validateReportCountAggregation(ctx, req.Config, nil, &resp.Diagnostics)
}

// reportCustomTimeRangeModeValidator enforces the two-way coupling between
// config.custom_time_range and config.time_range.mode:
//   - custom_time_range is only valid when time_range.mode is "custom"; and
//   - time_range.mode = "custom" requires a custom_time_range block.
//
// The API rejects either mismatch with a 400, so surface both at plan time.
type reportCustomTimeRangeModeValidator struct{}

var _ resource.ConfigValidator = reportCustomTimeRangeModeValidator{}

func (v reportCustomTimeRangeModeValidator) Description(_ context.Context) string {
	return "Validates that custom_time_range is set if and only if time_range.mode is \"custom\""
}

func (v reportCustomTimeRangeModeValidator) MarkdownDescription(_ context.Context) string {
	return "Validates that `custom_time_range` is set if and only if `time_range.mode` is `\"custom\"`"
}

func (v reportCustomTimeRangeModeValidator) ValidateResource(ctx context.Context, req resource.ValidateConfigRequest, resp *resource.ValidateConfigResponse) {
	// Defer when time_range is omitted in config so ModifyPlan can check the stored
	// mode on update (or reject on create). An omitted time_range.mode is reported
	// by reportTimeRangeRequiredFieldsValidator.
	var timeRange resource_report.TimeRangeValue
	d := req.Config.GetAttribute(ctx, path.Root("config").AtName("time_range"), &timeRange)
	resp.Diagnostics.Append(d...)
	if d.HasError() || timeRange.IsNull() || timeRange.Mode.IsNull() {
		return
	}
	validateReportCustomTimeRangeMode(ctx, req.Config, nil, &resp.Diagnostics)
}

// reportCustomTimeRangeUnitValidator rejects time_range.unit alongside
// time_range.mode = "custom". A ConfigValidator rather than an attribute
// validator because attribute validators do not run on attributes inside a
// SingleNestedAttribute with a CustomType.
type reportCustomTimeRangeUnitValidator struct{}

var _ resource.ConfigValidator = reportCustomTimeRangeUnitValidator{}

func (v reportCustomTimeRangeUnitValidator) Description(_ context.Context) string {
	return "Validates that unit is not set when time_range.mode is \"custom\""
}

func (v reportCustomTimeRangeUnitValidator) MarkdownDescription(_ context.Context) string {
	return "Validates that `unit` is not set when `time_range.mode` is `\"custom\"`"
}

func (v reportCustomTimeRangeUnitValidator) ValidateResource(ctx context.Context, req resource.ValidateConfigRequest, resp *resource.ValidateConfigResponse) {
	validateReportCustomTimeRangeUnit(ctx, req.Config, &resp.Diagnostics)
}

// validateReportCustomTimeRangeUnit enforces that time_range.unit is absent when
// mode is "custom": a custom range takes its bounds from custom_time_range, and
// the API both rejects the combination and omits unit from custom-mode
// responses. Shared by the report resource and the report_query data source.
// Unknown values are deferred, since they may resolve to a valid combination.
func validateReportCustomTimeRangeUnit(ctx context.Context, config tfsdk.Config, diags *diag.Diagnostics) {
	var timeRange resource_report.TimeRangeValue
	d := config.GetAttribute(ctx, path.Root("config").AtName("time_range"), &timeRange)
	diags.Append(d...)

	if d.HasError() || timeRange.IsNull() || timeRange.IsUnknown() {
		return
	}

	if timeRange.Mode.IsUnknown() || timeRange.Unit.IsUnknown() {
		return
	}

	if timeRange.Mode.IsNull() || timeRange.Mode.ValueString() != "custom" {
		return
	}

	if timeRange.Unit.IsNull() {
		return
	}

	diags.AddAttributeError(
		path.Root("config").AtName("time_range").AtName("unit"),
		"Invalid Time Range Configuration",
		fmt.Sprintf("`unit` must not be set when `time_range.mode` is \"custom\", but it is %q. "+
			"A custom range takes its bounds from `custom_time_range`, so `unit` has no effect "+
			"and the API rejects the combination. Remove `unit` from the `time_range` block.",
			timeRange.Unit.ValueString()),
	)
}

// validateReportCountAggregation is the shared body used by both the report
// resource and the report_query data source, which build the same ExternalConfig
// from an identical schema. It enforces the count/aggregation coupling in both
// directions (the API rejects either mismatch with a 400; verified: an omitted
// aggregation does NOT default to "count").
//
// priorState, when non-nil, supplies aggregation when the config omits it on
// update (aggregation is Optional+Computed and not clearable in place, so the
// API retains the stored value). Unknown values (e.g. cross-resource references)
// are deferred, since they may resolve to a valid combination.
func validateReportCountAggregation(ctx context.Context, getter, priorState attributeGetter, diags *diag.Diagnostics) {
	var configWrapper resource_report.ConfigValue
	d := getter.GetAttribute(ctx, path.Root("config"), &configWrapper)
	diags.Append(d...)
	if d.HasError() || configWrapper.IsNull() || configWrapper.IsUnknown() {
		return
	}

	var count resource_report.CountValue
	d = getter.GetAttribute(ctx, path.Root("config").AtName("count"), &count)
	diags.Append(d...)
	if d.HasError() {
		return
	}

	var aggregation types.String
	d = getter.GetAttribute(ctx, path.Root("config").AtName("aggregation"), &aggregation)
	diags.Append(d...)
	if d.HasError() {
		return
	}
	if aggregation.IsNull() && priorState != nil {
		d = priorState.GetAttribute(ctx, path.Root("config").AtName("aggregation"), &aggregation)
		diags.Append(d...)
		if d.HasError() {
			return
		}
	}

	// Defer while either value is unknown — it may resolve to a valid combination.
	if count.IsUnknown() || aggregation.IsUnknown() {
		return
	}

	if !count.IsNull() {
		if aggregation.ValueString() == "count" {
			return
		}
		// count set but aggregation is not "count".
		detail := "`count` is only applicable when `aggregation = \"count\"`. " +
			"Either set aggregation = \"count\" or remove the count block."
		if !aggregation.IsNull() {
			detail = fmt.Sprintf("`count` is only applicable when `aggregation = \"count\"`, but aggregation is %q. "+
				"Either set aggregation = \"count\" or remove the count block.", aggregation.ValueString())
		}
		diags.AddAttributeError(
			path.Root("config").AtName("count"),
			"Invalid Count Configuration",
			detail,
		)
		return
	}

	if aggregation.ValueString() == "count" {
		// aggregation "count" requires a count block.
		diags.AddAttributeError(
			path.Root("config").AtName("count"),
			"Missing Count Configuration",
			"`aggregation = \"count\"` requires a `count` block specifying the field to count "+
				"(e.g. count = { id = \"service_description\", type = \"fixed\" }).",
		)
	}
}

// validateReportCustomTimeRangeMode is the shared body used by both the report
// resource and the report_query data source. It enforces the two-way coupling
// between config.custom_time_range and config.time_range.mode.
//
// priorState, when non-nil, supplies time_range when the config omits it on
// update (time_range is Optional+Computed and not clearable in place, so the
// API retains the stored value). Unknown values are deferred.
func validateReportCustomTimeRangeMode(ctx context.Context, getter, priorState attributeGetter, diags *diag.Diagnostics) {
	var configWrapper resource_report.ConfigValue
	d := getter.GetAttribute(ctx, path.Root("config"), &configWrapper)
	diags.Append(d...)
	if d.HasError() || configWrapper.IsNull() || configWrapper.IsUnknown() {
		return
	}

	var customTimeRange resource_report.CustomTimeRangeValue
	d = getter.GetAttribute(ctx, path.Root("config").AtName("custom_time_range"), &customTimeRange)
	diags.Append(d...)
	if d.HasError() {
		return
	}

	var timeRange resource_report.TimeRangeValue
	d = getter.GetAttribute(ctx, path.Root("config").AtName("time_range"), &timeRange)
	diags.Append(d...)
	if d.HasError() {
		return
	}
	if timeRange.IsNull() && priorState != nil {
		d = priorState.GetAttribute(ctx, path.Root("config").AtName("time_range"), &timeRange)
		diags.Append(d...)
		if d.HasError() {
			return
		}
	}

	if customTimeRange.IsUnknown() || timeRange.IsUnknown() {
		return
	}

	mode := timeRange.Mode
	if mode.IsNull() && priorState != nil {
		var priorTimeRange resource_report.TimeRangeValue
		d = priorState.GetAttribute(ctx, path.Root("config").AtName("time_range"), &priorTimeRange)
		diags.Append(d...)
		if d.HasError() {
			return
		}
		if priorTimeRange.IsUnknown() {
			return
		}
		if !priorTimeRange.IsNull() {
			mode = priorTimeRange.Mode
		}
	}
	if mode.IsUnknown() {
		return
	}

	if !customTimeRange.IsNull() {
		if mode.ValueString() == "custom" {
			return
		}
		detail := "`custom_time_range` is only applicable when `time_range.mode = \"custom\"`. " +
			"Either set `time_range.mode = \"custom\"` or remove the `custom_time_range` block."
		if !mode.IsNull() {
			detail = fmt.Sprintf("`custom_time_range` is only applicable when `time_range.mode = \"custom\"`, but `time_range.mode` is %q. "+
				"Either set `time_range.mode = \"custom\"` or remove the `custom_time_range` block.", mode.ValueString())
		}
		diags.AddAttributeError(
			path.Root("config").AtName("custom_time_range"),
			"Invalid Custom Time Range Configuration",
			detail,
		)
		return
	}

	if mode.ValueString() == "custom" {
		diags.AddAttributeError(
			path.Root("config").AtName("custom_time_range"),
			"Missing Custom Time Range",
			"`time_range.mode = \"custom\"` requires a `custom_time_range` block specifying `from` and `to`.",
		)
	}
}

// reportFilterNAValidator warns when legacy NullFallback sentinel values such as
// "[Service N/A]" are found in config.filters[*].values. Users should use
// include_null = true on the filter block instead, which is semantically equivalent
// and avoids unexpected behaviour during `terraform import`.
type reportFilterNAValidator struct{}

var _ resource.ConfigValidator = reportFilterNAValidator{}

func (v reportFilterNAValidator) Description(_ context.Context) string {
	return "Warns when legacy NullFallback sentinel values (e.g. [Service N/A]) are used in filter values"
}

func (v reportFilterNAValidator) MarkdownDescription(_ context.Context) string {
	return "Warns when legacy NullFallback sentinel values (e.g. `[Service N/A]`) are used in " +
		"`config.filters[*].values`. Use `include_null = true` on the filter block instead."
}

func (v reportFilterNAValidator) ValidateResource(ctx context.Context, req resource.ValidateConfigRequest, resp *resource.ValidateConfigResponse) {
	const filtersPath = "filters"
	basePath := path.Root("config").AtName(filtersPath)

	var filters types.List
	diags := req.Config.GetAttribute(ctx, basePath, &filters)
	resp.Diagnostics.Append(diags...)
	if diags.HasError() || filters.IsNull() || filters.IsUnknown() {
		return
	}

	var filterVals []resource_report.FiltersValue
	diags = filters.ElementsAs(ctx, &filterVals, false)
	resp.Diagnostics.Append(diags...)
	if diags.HasError() {
		return
	}

	warnNAFilterValues(ctx, filterVals, &resp.Diagnostics)
}

// warnNAFilterValues appends a Warning diagnostic for every filter value that matches
// the legacy NullFallback sentinel pattern (e.g. "[Service N/A]").
// It is a package-level function so it can be unit-tested independently of tfsdk.Config.
func warnNAFilterValues(ctx context.Context, filterVals []resource_report.FiltersValue, diags *diag.Diagnostics) {
	valueLists := make([]types.List, len(filterVals))
	for i, f := range filterVals {
		valueLists[i] = f.Values
	}
	warnNASentinels(ctx, path.Root("config").AtName("filters"), valueLists, diags)
}

// reportCumulativeComparisonDataSourceValidator validates that layout = "cumulative_comparison"
// satisfies required dimensions, aggregation, metric, secondary time range, and forecast/comparative constraints
// on the doit_report_query data source:
//   - daily datetime dimensions (year, month, day in order, all with type "datetime")
//   - total aggregation (aggregation = "total")
//   - one metric (metrics has 1 item, or single metric block)
//   - a secondary time range (secondary_time_range is configured)
//   - no comparative (display_values is null or "actuals_only")
//   - no forecast (forecast_settings is not set, advanced_analysis.forecast is not true)
//
// Resource-level validation is evaluated in ModifyPlan on the effective plan so that
// Category B unclearable attributes (like secondary_time_range) can be preserved from prior state on update.
type reportCumulativeComparisonDataSourceValidator struct{}

var _ datasource.ConfigValidator = reportCumulativeComparisonDataSourceValidator{}

func (v reportCumulativeComparisonDataSourceValidator) Description(_ context.Context) string {
	return "Validates that layout = \"cumulative_comparison\" satisfies required dimensions, aggregation, metric, secondary time range, and forecast/comparative constraints"
}

func (v reportCumulativeComparisonDataSourceValidator) MarkdownDescription(_ context.Context) string {
	return "Validates that `layout = \"cumulative_comparison\"` satisfies required dimensions, aggregation, metric, secondary time range, and forecast/comparative constraints"
}

func (v reportCumulativeComparisonDataSourceValidator) ValidateDataSource(ctx context.Context, req datasource.ValidateConfigRequest, resp *datasource.ValidateConfigResponse) {
	validateReportCumulativeComparison(ctx, req.Config, &resp.Diagnostics)
}

// attributeGetter abstracts GetAttribute across tfsdk.Config and tfsdk.Plan,
// allowing shared validation logic to run both in ConfigValidators and ModifyPlan.
type attributeGetter interface {
	GetAttribute(ctx context.Context, path path.Path, target any) diag.Diagnostics
}

// validateReportCumulativeComparison is the shared validation logic for both the
// report resource and the report_query data source when layout = "cumulative_comparison".
// Unknown values are deferred so references validate once resolved during plan.
func validateReportCumulativeComparison(ctx context.Context, getter attributeGetter, diags *diag.Diagnostics) {
	var layout types.String
	d := getter.GetAttribute(ctx, path.Root("config").AtName("layout"), &layout)
	diags.Append(d...)
	if d.HasError() {
		return
	}
	validateReportCumulativeComparisonWithLayout(ctx, getter, layout, diags)
}

// validateReportCumulativeComparisonWithLayout validates the proposed config/plan attributes
// against the rules for layout = "cumulative_comparison". It accepts an explicitly resolved layout,
// enabling ModifyPlan to validate plans that inherit cumulative_comparison from prior state when
// layout is omitted in config on update.
func validateReportCumulativeComparisonWithLayout(ctx context.Context, getter attributeGetter, layout types.String, diags *diag.Diagnostics) {
	if layout.IsNull() || layout.IsUnknown() {
		return
	}
	if layout.ValueString() != "cumulative_comparison" {
		return
	}

	// 1. Daily datetime dimensions (year, month, day)
	var dimensions types.List
	d := getter.GetAttribute(ctx, path.Root("config").AtName("dimensions"), &dimensions)
	diags.Append(d...)
	if !d.HasError() {
		validateReportCumulativeDimensions(ctx, dimensions, diags)
	}

	// 2. Total aggregation
	var aggregation types.String
	d = getter.GetAttribute(ctx, path.Root("config").AtName("aggregation"), &aggregation)
	diags.Append(d...)
	if !d.HasError() && !aggregation.IsUnknown() {
		if aggregation.IsNull() || aggregation.ValueString() != "total" {
			detail := "`layout = \"cumulative_comparison\"` requires `aggregation = \"total\"`."
			if !aggregation.IsNull() {
				detail = fmt.Sprintf("`layout = \"cumulative_comparison\"` requires `aggregation = \"total\"`, but got %q.", aggregation.ValueString())
			}
			diags.AddAttributeError(
				path.Root("config").AtName("aggregation"),
				"Invalid Aggregation Configuration",
				detail,
			)
		}
	}

	// 3. One metric
	var metrics types.List
	d = getter.GetAttribute(ctx, path.Root("config").AtName("metrics"), &metrics)
	diags.Append(d...)

	if !d.HasError() {
		validateReportCumulativeMetrics(metrics, diags)
	}

	// 4. Secondary time range
	var secondaryTimeRange resource_report.SecondaryTimeRangeValue
	d = getter.GetAttribute(ctx, path.Root("config").AtName("secondary_time_range"), &secondaryTimeRange)
	diags.Append(d...)
	if !d.HasError() && !secondaryTimeRange.IsUnknown() {
		if secondaryTimeRange.IsNull() {
			diags.AddAttributeError(
				path.Root("config").AtName("secondary_time_range"),
				"Missing Secondary Time Range",
				"`layout = \"cumulative_comparison\"` requires `secondary_time_range` to be configured.",
			)
		}
	}

	// 5. No comparative
	var displayValues types.String
	d = getter.GetAttribute(ctx, path.Root("config").AtName("display_values"), &displayValues)
	diags.Append(d...)
	if !d.HasError() && !displayValues.IsUnknown() && !displayValues.IsNull() {
		if displayValues.ValueString() != "actuals_only" {
			diags.AddAttributeError(
				path.Root("config").AtName("display_values"),
				"Conflicting Comparative Configuration",
				fmt.Sprintf("`layout = \"cumulative_comparison\"` does not support comparative reports (`display_values` is %q). Omit `display_values` or set it to \"actuals_only\".", displayValues.ValueString()),
			)
		}
	}

	// 6. No forecast
	var forecastSettings resource_report.ForecastSettingsValue
	d = getter.GetAttribute(ctx, path.Root("config").AtName("forecast_settings"), &forecastSettings)
	diags.Append(d...)
	if !d.HasError() && !forecastSettings.IsUnknown() && !forecastSettings.IsNull() {
		diags.AddAttributeError(
			path.Root("config").AtName("forecast_settings"),
			"Conflicting Forecast Configuration",
			"`layout = \"cumulative_comparison\"` does not support forecasting. Remove `forecast_settings`.",
		)
	}

	var forecast types.Bool
	d = getter.GetAttribute(ctx, path.Root("config").AtName("advanced_analysis").AtName("forecast"), &forecast)
	diags.Append(d...)
	if !d.HasError() && !forecast.IsUnknown() && !forecast.IsNull() && forecast.ValueBool() {
		diags.AddAttributeError(
			path.Root("config").AtName("advanced_analysis").AtName("forecast"),
			"Conflicting Forecast Configuration",
			"`layout = \"cumulative_comparison\"` does not support forecasting. Set `advanced_analysis.forecast = false` or omit it.",
		)
	}
}

func validateReportCumulativeDimensions(ctx context.Context, dimensions types.List, diags *diag.Diagnostics) {
	if dimensions.IsUnknown() {
		return
	}
	if dimensions.IsNull() {
		diags.AddAttributeError(
			path.Root("config").AtName("dimensions"),
			"Invalid Dimensions Configuration",
			"`layout = \"cumulative_comparison\"` requires daily datetime dimensions (`year`, `month`, `day`), but `dimensions` is not set.",
		)
		return
	}

	var values []resource_report.DimensionsValue
	d := dimensions.ElementsAs(ctx, &values, false)
	diags.Append(d...)
	if d.HasError() {
		return
	}

	expected := []string{"year", "month", "day"}
	valid := len(values) == len(expected)
	for index, value := range values {
		if !valid {
			continue
		}
		if value.IsUnknown() {
			continue
		}
		if knownStringDiffers(value.Id, expected[index]) || knownStringDiffers(value.DimensionsType, "datetime") {
			valid = false
		}
	}
	if !valid {
		diags.AddAttributeError(
			path.Root("config").AtName("dimensions"),
			"Invalid Dimensions Configuration",
			"`layout = \"cumulative_comparison\"` requires daily datetime dimensions (`year`, `month`, `day`) in order with `type = \"datetime\"`.",
		)
	}
}

func knownStringDiffers(value types.String, expected string) bool {
	if value.IsUnknown() {
		return false
	}
	if value.IsNull() {
		return true
	}
	return value.ValueString() != expected
}

func validateReportCumulativeMetrics(metrics types.List, diags *diag.Diagnostics) {
	if metrics.IsUnknown() {
		return
	}

	numMetrics := 0
	if !metrics.IsNull() {
		numMetrics = len(metrics.Elements())
	}
	if numMetrics != 1 {
		diags.AddAttributeError(
			path.Root("config").AtName("metrics"),
			"Invalid Metrics Configuration",
			fmt.Sprintf("`layout = \"cumulative_comparison\"` requires exactly one metric, but %d metrics were configured.", numMetrics),
		)
	}
}

// reportLimitByChangeValidator validates config.limit_by_change against the rest
// of the report config:
//   - with secondary_time_range set, aggregation must not be "count" (the API
//     rejects it with LIMIT_BY_CHANGE_METRIC_NOT_ADDITIVE_WITH_SECONDARY), and
//   - limit_by_change.metric must also be listed in config.metrics.
//
// A percentage-typed custom metric is also rejected with a secondary range, but
// the unit type lives in the custom metric document rather than the config, so
// that case is left to the API.
//
// A ConfigValidator rather than an attribute validator because attribute
// validators do not run on attributes inside a SingleNestedAttribute with a
// CustomType.
type reportLimitByChangeValidator struct{}

var _ resource.ConfigValidator = reportLimitByChangeValidator{}

func (v reportLimitByChangeValidator) Description(_ context.Context) string {
	return "Validates that limit_by_change is not combined with count aggregation and a secondary time range, and that its metric is listed in metrics"
}

func (v reportLimitByChangeValidator) MarkdownDescription(_ context.Context) string {
	return "Validates that `limit_by_change` is not combined with `count` aggregation and a secondary time range, and that its metric is listed in `metrics`"
}

func (v reportLimitByChangeValidator) ValidateResource(ctx context.Context, req resource.ValidateConfigRequest, resp *resource.ValidateConfigResponse) {
	validateReportLimitByChange(ctx, req.Config, &resp.Diagnostics)
}

// reportLimitByChangeDataSourceValidator applies the same rules to the
// report_query data source, whose config reuses the report resource types.
type reportLimitByChangeDataSourceValidator struct{}

var _ datasource.ConfigValidator = reportLimitByChangeDataSourceValidator{}

func (v reportLimitByChangeDataSourceValidator) Description(ctx context.Context) string {
	return reportLimitByChangeValidator{}.Description(ctx)
}

func (v reportLimitByChangeDataSourceValidator) MarkdownDescription(ctx context.Context) string {
	return reportLimitByChangeValidator{}.MarkdownDescription(ctx)
}

func (v reportLimitByChangeDataSourceValidator) ValidateDataSource(ctx context.Context, req datasource.ValidateConfigRequest, resp *datasource.ValidateConfigResponse) {
	validateReportLimitByChange(ctx, req.Config, &resp.Diagnostics)
}

// validateReportLimitByChange is shared by the report resource (ConfigValidators
// and ModifyPlan) and the report_query data source. The two checks are
// independent: each defers on its own unknown inputs so that one unresolved
// attribute never hides the other's definite error.
func validateReportLimitByChange(ctx context.Context, getter attributeGetter, diags *diag.Diagnostics) {
	validateReportLimitByChangeCountSecondary(ctx, getter, nil, diags)
	validateReportLimitByChangeMetricListed(ctx, getter, nil, diags)
}

// validateReportLimitByChangeCountSecondary rejects aggregation = "count" when
// both limit_by_change and secondary_time_range are set.
//
// priorState, when non-nil, supplies the value of aggregation and
// secondary_time_range for attributes the config omits. Both are
// Optional+Computed and the API keeps the stored value when an update omits them,
// so the stored value is what the update would run with. (The plan cannot be used
// for this: it marks an omitted nested attribute as unknown.)
func validateReportLimitByChangeCountSecondary(ctx context.Context, getter, priorState attributeGetter, diags *diag.Diagnostics) {
	var limitByChange resource_report.LimitByChangeValue
	d := getter.GetAttribute(ctx, path.Root("config").AtName("limit_by_change"), &limitByChange)
	diags.Append(d...)
	if d.HasError() {
		return
	}

	var secondary resource_report.SecondaryTimeRangeValue
	d = getter.GetAttribute(ctx, path.Root("config").AtName("secondary_time_range"), &secondary)
	diags.Append(d...)
	if d.HasError() {
		return
	}
	if secondary.IsNull() && priorState != nil {
		d = priorState.GetAttribute(ctx, path.Root("config").AtName("secondary_time_range"), &secondary)
		diags.Append(d...)
		if d.HasError() {
			return
		}
	}

	var aggregation types.String
	d = getter.GetAttribute(ctx, path.Root("config").AtName("aggregation"), &aggregation)
	diags.Append(d...)
	if d.HasError() {
		return
	}
	if aggregation.IsNull() && priorState != nil {
		d = priorState.GetAttribute(ctx, path.Root("config").AtName("aggregation"), &aggregation)
		diags.Append(d...)
		if d.HasError() {
			return
		}
	}

	if limitByChange.IsNull() || limitByChange.IsUnknown() {
		return
	}
	if secondary.IsNull() || secondary.IsUnknown() {
		return
	}
	if aggregation.IsNull() || aggregation.IsUnknown() {
		return
	}

	if aggregation.ValueString() != "count" {
		return
	}
	diags.AddAttributeError(
		path.Root("config").AtName("limit_by_change"),
		"Invalid Limit By Change Configuration",
		"`limit_by_change` cannot be combined with `aggregation = \"count\"` when `secondary_time_range` is set: "+
			"a change between two ranges needs one summable number per range, which count aggregation does not provide "+
			"(the API rejects this with LIMIT_BY_CHANGE_METRIC_NOT_ADDITIVE_WITH_SECONDARY). "+
			"Use a different aggregation, or remove `secondary_time_range` or `limit_by_change`. "+
			"If `secondary_time_range` is not in your configuration, it is kept from the existing report.",
	)
}

// validateReportLimitByChangeMetricListed rejects a limit_by_change.metric that
// does not match (type and value) any entry of config.metrics.
//
// priorState, when non-nil, supplies metrics when the config omits it: the API keeps
// the stored list on update, so that is the list limit_by_change is checked against.
// Without it (create, or a forced replacement), or when the stored list is empty, an
// omitted list is deferred, because the API may default it.
func validateReportLimitByChangeMetricListed(ctx context.Context, getter, priorState attributeGetter, diags *diag.Diagnostics) {
	var limitByChange resource_report.LimitByChangeValue
	d := getter.GetAttribute(ctx, path.Root("config").AtName("limit_by_change"), &limitByChange)
	diags.Append(d...)
	if d.HasError() {
		return
	}

	var metrics types.List
	d = getter.GetAttribute(ctx, path.Root("config").AtName("metrics"), &metrics)
	diags.Append(d...)
	if d.HasError() {
		return
	}
	if metrics.IsNull() && priorState != nil {
		d = priorState.GetAttribute(ctx, path.Root("config").AtName("metrics"), &metrics)
		diags.Append(d...)
		if d.HasError() {
			return
		}
		if metrics.IsNull() || metrics.IsUnknown() {
			return
		}
		// The provider keeps an empty list in state when metrics was never configured
		// (see useEmptyForUnconfiguredMetricsMirror), so an empty stored list says nothing
		// about the metrics the API holds.
		if len(metrics.Elements()) == 0 {
			return
		}
	}

	if limitByChange.IsNull() || limitByChange.IsUnknown() {
		return
	}
	// A null metrics list may be defaulted by the API, so only a configured or stored list is checked.
	if metrics.IsNull() || metrics.IsUnknown() {
		return
	}

	metric := limitByChange.Metric
	if metric.IsNull() || metric.IsUnknown() {
		return
	}
	if metric.MetricType.IsNull() || metric.MetricType.IsUnknown() {
		return
	}
	if metric.Value.IsNull() || metric.Value.IsUnknown() {
		return
	}

	var configured []resource_report.MetricsValue
	d = metrics.ElementsAs(ctx, &configured, false)
	diags.Append(d...)
	if d.HasError() {
		return
	}

	hasUnknown := false
	for _, entry := range configured {
		if entry.IsUnknown() {
			hasUnknown = true
			continue
		}
		// A known component that differs is a definite non-match, whatever the other component resolves to.
		if knownStringDiffers(entry.MetricsType, metric.MetricType.ValueString()) {
			continue
		}
		if knownStringDiffers(entry.Value, metric.Value.ValueString()) {
			continue
		}
		if entry.MetricsType.IsUnknown() || entry.Value.IsUnknown() {
			hasUnknown = true
			continue
		}
		return
	}

	// An entry whose unresolved components could still match defers the error.
	if hasUnknown {
		return
	}

	diags.AddAttributeError(
		path.Root("config").AtName("limit_by_change").AtName("metric"),
		"Invalid Limit By Change Metric",
		fmt.Sprintf("`limit_by_change.metric` (type %q, value %q) must also be listed in `metrics`; the API rejects the report otherwise. "+
			"Add it to `metrics` or use one of the configured metrics in `limit_by_change.metric`.",
			metric.MetricType.ValueString(), metric.Value.ValueString()),
	)
}

// reportSecondaryTimeRangeValidator rejects secondary_time_range settings that the
// API refuses because the secondary range would overlap the primary range. The
// API compares whole UTC days on create and on update (against the merged config).
//
// Only the cases that can be decided from the configuration are checked here: a
// custom primary range against a custom secondary range or a one-year shift, a
// relative secondary range on a "last" primary range longer than one year, and a
// non-positive amount. Overlaps of a relative primary range with a custom
// secondary range depend on the current date and are left to the API.
//
// A ConfigValidator rather than an attribute validator because attribute
// validators do not run on attributes inside a SingleNestedAttribute with a
// CustomType.
type reportSecondaryTimeRangeValidator struct{}

var _ resource.ConfigValidator = reportSecondaryTimeRangeValidator{}

func (v reportSecondaryTimeRangeValidator) Description(_ context.Context) string {
	return "Validates that secondary_time_range does not overlap the primary time range"
}

func (v reportSecondaryTimeRangeValidator) MarkdownDescription(_ context.Context) string {
	return "Validates that `secondary_time_range` does not overlap the primary time range"
}

func (v reportSecondaryTimeRangeValidator) ValidateResource(ctx context.Context, req resource.ValidateConfigRequest, resp *resource.ValidateConfigResponse) {
	validateReportSecondaryTimeRange(ctx, req.Config, nil, &resp.Diagnostics)
}

// reportSecondaryTimeRangeDataSourceValidator applies the same rules to the
// report_query data source, whose config reuses the report resource types.
type reportSecondaryTimeRangeDataSourceValidator struct{}

var _ datasource.ConfigValidator = reportSecondaryTimeRangeDataSourceValidator{}

func (v reportSecondaryTimeRangeDataSourceValidator) Description(ctx context.Context) string {
	return reportSecondaryTimeRangeValidator{}.Description(ctx)
}

func (v reportSecondaryTimeRangeDataSourceValidator) MarkdownDescription(ctx context.Context) string {
	return reportSecondaryTimeRangeValidator{}.MarkdownDescription(ctx)
}

func (v reportSecondaryTimeRangeDataSourceValidator) ValidateDataSource(ctx context.Context, req datasource.ValidateConfigRequest, resp *datasource.ValidateConfigResponse) {
	validateReportSecondaryTimeRange(ctx, req.Config, nil, &resp.Diagnostics)
}

// validateReportSecondaryTimeRange is shared by the report resource
// (ConfigValidators and ModifyPlan) and the report_query data source.
//
// priorState, when non-nil, supplies secondary_time_range and time_range when the
// config omits them: both are Optional+Computed and the API keeps the stored value
// on update, then validates the merged config. custom_time_range
// is clearable, so an omitted one is cleared and has no fallback.
func validateReportSecondaryTimeRange(ctx context.Context, getter, priorState attributeGetter, diags *diag.Diagnostics) {
	var configWrapper resource_report.ConfigValue
	d := getter.GetAttribute(ctx, path.Root("config"), &configWrapper)
	diags.Append(d...)
	if d.HasError() || configWrapper.IsNull() || configWrapper.IsUnknown() {
		return
	}

	var secondary resource_report.SecondaryTimeRangeValue
	d = getter.GetAttribute(ctx, path.Root("config").AtName("secondary_time_range"), &secondary)
	diags.Append(d...)
	if d.HasError() {
		return
	}
	inherited := false
	if secondary.IsNull() && priorState != nil {
		d = priorState.GetAttribute(ctx, path.Root("config").AtName("secondary_time_range"), &secondary)
		diags.Append(d...)
		if d.HasError() {
			return
		}
		inherited = true
	}

	var timeRange resource_report.TimeRangeValue
	d = getter.GetAttribute(ctx, path.Root("config").AtName("time_range"), &timeRange)
	diags.Append(d...)
	if d.HasError() {
		return
	}
	if timeRange.IsNull() && priorState != nil {
		d = priorState.GetAttribute(ctx, path.Root("config").AtName("time_range"), &timeRange)
		diags.Append(d...)
		if d.HasError() {
			return
		}
		inherited = true
	}

	var customTimeRange resource_report.CustomTimeRangeValue
	d = getter.GetAttribute(ctx, path.Root("config").AtName("custom_time_range"), &customTimeRange)
	diags.Append(d...)
	if d.HasError() {
		return
	}

	if secondary.IsNull() || secondary.IsUnknown() {
		return
	}
	// The amount check does not depend on the primary range.
	validateReportSecondaryAmount(secondary, diags)
	if timeRange.IsUnknown() {
		return
	}

	// An omitted mode is not inherited: the API replaces the stored time range with
	// the configured block. validateReportTimeRangeRequiredFields reports it.
	mode := types.StringNull()
	if !timeRange.IsNull() {
		mode = timeRange.Mode
	}

	validateReportSecondaryOnLongPrimary(timeRange, mode, secondary, inherited, diags)
	validateReportSecondaryCustomOverlap(mode, customTimeRange, secondary, inherited, diags)
	validateReportSecondaryYearShiftOverlap(mode, customTimeRange, secondary, inherited, diags)
}

// validateReportSecondaryAmount rejects a relative secondary range whose amount is
// zero or negative. Zero shifts the secondary range onto the primary range.
func validateReportSecondaryAmount(secondary resource_report.SecondaryTimeRangeValue, diags *diag.Diagnostics) {
	if secondary.IsNull() || secondary.IsUnknown() {
		return
	}
	// A custom secondary range ignores amount.
	if !secondary.CustomTimeRange.IsNull() {
		return
	}
	if secondary.Amount.IsNull() || secondary.Amount.IsUnknown() {
		return
	}

	amount := secondary.Amount.ValueInt64()
	if amount > 0 {
		return
	}
	diags.AddAttributeError(
		path.Root("config").AtName("secondary_time_range").AtName("amount"),
		"Invalid Secondary Time Range Amount",
		fmt.Sprintf("`secondary_time_range.amount` must be greater than zero, got %d. "+
			"The API rejects a negative amount, and an amount of 0 makes the secondary range overlap the primary range.", amount),
	)
}

// validateReportSecondaryOnLongPrimary rejects a relative secondary range on a
// "last" primary range longer than one year: the API has no shifted range for such
// a primary range that would not overlap it.
//
// The secondary range counts as relative only when its amount and unit are set.
// A relative range missing either is reported by
// validateReportTimeRangeRequiredFields instead.
func validateReportSecondaryOnLongPrimary(timeRange resource_report.TimeRangeValue, mode types.String, secondary resource_report.SecondaryTimeRangeValue, inherited bool, diags *diag.Diagnostics) {
	if timeRange.IsNull() || timeRange.IsUnknown() {
		return
	}
	if secondary.IsNull() || secondary.IsUnknown() {
		return
	}
	if mode.IsNull() || mode.IsUnknown() {
		return
	}
	if timeRange.Unit.IsNull() || timeRange.Unit.IsUnknown() {
		return
	}
	if timeRange.Amount.IsNull() || timeRange.Amount.IsUnknown() {
		return
	}
	// A custom secondary range is allowed on any primary range.
	if secondary.CustomTimeRange.IsUnknown() {
		return
	}
	if !secondary.CustomTimeRange.IsNull() {
		return
	}
	if secondary.Amount.IsNull() || secondary.Amount.IsUnknown() {
		return
	}
	if secondary.Unit.IsNull() || secondary.Unit.IsUnknown() {
		return
	}

	if mode.ValueString() != "last" {
		return
	}
	unit := timeRange.Unit.ValueString()
	amount := timeRange.Amount.ValueInt64()
	if !reportPrimaryExceedsOneYear(unit, amount) {
		return
	}
	diags.AddAttributeError(
		path.Root("config").AtName("secondary_time_range"),
		"Overlapping Secondary Time Range",
		fmt.Sprintf("A relative `secondary_time_range` (`amount` and `unit`) cannot be combined with a primary `time_range` of the last %d %s(s): "+
			"the API has no shifted secondary range for a primary range longer than one year, because it would overlap the primary range. "+
			"Set `secondary_time_range.custom_time_range` to a range that ends before the primary range starts, or shorten `time_range`.%s",
			amount, unit, reportSecondaryInheritedNote(inherited)),
	)
}

// reportPrimaryExceedsOneYear reports whether a "last" primary range of amount
// units is longer than one year, using the API's per-unit limits.
func reportPrimaryExceedsOneYear(unit string, amount int64) bool {
	switch unit {
	case "day":
		return amount > 365
	case "week":
		return amount > 52
	case "month":
		return amount > 12
	case "quarter":
		return amount > 4
	case "year":
		return amount > 1
	default:
		return false
	}
}

// validateReportSecondaryCustomOverlap rejects a custom secondary range that shares
// a UTC day with the custom primary range.
func validateReportSecondaryCustomOverlap(mode types.String, customTimeRange resource_report.CustomTimeRangeValue, secondary resource_report.SecondaryTimeRangeValue, inherited bool, diags *diag.Diagnostics) {
	if mode.IsNull() || mode.IsUnknown() {
		return
	}
	if secondary.IsNull() || secondary.IsUnknown() {
		return
	}
	primaryFrom, primaryTo, ok := reportCustomRangeDays(customTimeRange)
	if !ok {
		return
	}
	secondaryFrom, secondaryTo, ok := reportCustomRangeDays(secondary.CustomTimeRange)
	if !ok {
		return
	}

	if mode.ValueString() != "custom" {
		return
	}
	if !reportDayRangesOverlap(primaryFrom, primaryTo, secondaryFrom, secondaryTo) {
		return
	}
	diags.AddAttributeError(
		path.Root("config").AtName("secondary_time_range").AtName("custom_time_range"),
		"Overlapping Secondary Time Range",
		fmt.Sprintf("`secondary_time_range.custom_time_range` (%s to %s) overlaps the primary `custom_time_range` (%s to %s). "+
			"The API compares whole days in UTC and rejects a secondary range that shares a day with the primary range. "+
			"End the secondary range on %s or earlier, or start it on %s or later.%s",
			reportFormatDay(secondaryFrom), reportFormatDay(secondaryTo),
			reportFormatDay(primaryFrom), reportFormatDay(primaryTo),
			reportFormatDay(primaryFrom.AddDate(0, 0, -1)), reportFormatDay(primaryTo.AddDate(0, 0, 1)),
			reportSecondaryInheritedNote(inherited)),
	)
}

// validateReportSecondaryYearShiftOverlap rejects the one-year relative secondary
// range (the only relative option the API accepts for a custom primary range)
// when the custom primary range spans a year or more, so that shifting it back
// one year still reaches the primary range's first day.
func validateReportSecondaryYearShiftOverlap(mode types.String, customTimeRange resource_report.CustomTimeRangeValue, secondary resource_report.SecondaryTimeRangeValue, inherited bool, diags *diag.Diagnostics) {
	if mode.IsNull() || mode.IsUnknown() {
		return
	}
	if secondary.IsNull() || secondary.IsUnknown() {
		return
	}
	if secondary.CustomTimeRange.IsUnknown() {
		return
	}
	if !secondary.CustomTimeRange.IsNull() {
		return
	}
	if secondary.Amount.IsNull() || secondary.Amount.IsUnknown() {
		return
	}
	if secondary.Unit.IsNull() || secondary.Unit.IsUnknown() {
		return
	}
	if secondary.IncludeCurrent.IsNull() || secondary.IncludeCurrent.IsUnknown() {
		return
	}
	primaryFrom, primaryTo, ok := reportCustomRangeDays(customTimeRange)
	if !ok {
		return
	}

	if mode.ValueString() != "custom" {
		return
	}
	// The API rejects every other relative option for a custom primary range on its own.
	if secondary.Amount.ValueInt64() != 1 || secondary.Unit.ValueString() != "year" || secondary.IncludeCurrent.ValueBool() {
		return
	}
	shiftedFrom := primaryFrom.AddDate(-1, 0, 0)
	shiftedTo := primaryTo.AddDate(-1, 0, 0)
	if !reportDayRangesOverlap(primaryFrom, primaryTo, shiftedFrom, shiftedTo) {
		return
	}
	diags.AddAttributeError(
		path.Root("config").AtName("secondary_time_range"),
		"Overlapping Secondary Time Range",
		fmt.Sprintf("Shifting the primary `custom_time_range` (%s to %s) back one year gives %s to %s, which overlaps the primary range: "+
			"the primary range spans a year or more. The API compares whole days in UTC and rejects an overlapping secondary range. "+
			"Shorten the primary range, or set `secondary_time_range.custom_time_range` to a range that ends on %s or earlier.%s",
			reportFormatDay(primaryFrom), reportFormatDay(primaryTo),
			reportFormatDay(shiftedFrom), reportFormatDay(shiftedTo),
			reportFormatDay(primaryFrom.AddDate(0, 0, -1)),
			reportSecondaryInheritedNote(inherited)),
	)
}

// reportCustomRangeDays returns the UTC days of a custom range's from and to, the
// granularity at which the API compares ranges. ok is false when the range or
// either bound is null or unknown, a bound is not RFC3339 (reported by
// validateReportTimestamps), or to is before from (rejected by the API on its own).
func reportCustomRangeDays(ctr resource_report.CustomTimeRangeValue) (time.Time, time.Time, bool) {
	if ctr.IsNull() || ctr.IsUnknown() {
		return time.Time{}, time.Time{}, false
	}
	if ctr.From.IsNull() || ctr.From.IsUnknown() {
		return time.Time{}, time.Time{}, false
	}
	if ctr.To.IsNull() || ctr.To.IsUnknown() {
		return time.Time{}, time.Time{}, false
	}

	from, err := time.Parse(time.RFC3339, ctr.From.ValueString())
	if err != nil {
		return time.Time{}, time.Time{}, false
	}
	to, err := time.Parse(time.RFC3339, ctr.To.ValueString())
	if err != nil {
		return time.Time{}, time.Time{}, false
	}
	if to.Before(from) {
		return time.Time{}, time.Time{}, false
	}
	return from.UTC().Truncate(24 * time.Hour), to.UTC().Truncate(24 * time.Hour), true
}

// reportDayRangesOverlap reports whether two inclusive day ranges share a day.
func reportDayRangesOverlap(primaryFrom, primaryTo, secondaryFrom, secondaryTo time.Time) bool {
	return !secondaryFrom.After(primaryTo) && !secondaryTo.Before(primaryFrom)
}

func reportFormatDay(day time.Time) string {
	return day.Format(time.DateOnly)
}

// reportSecondaryInheritedNote explains that part of the checked configuration
// comes from the existing report rather than from the configuration.
func reportSecondaryInheritedNote(inherited bool) string {
	if !inherited {
		return ""
	}
	return " If `secondary_time_range` or `time_range` is not in your configuration, it is kept from the existing report."
}

// reportTimeRangeRequiredFieldsValidator rejects time range blocks that omit a
// field the API requires. A configured time_range or secondary_time_range replaces
// the stored object as a whole, and the API has no per-field defaults: omitted
// fields are not kept from the existing report.
type reportTimeRangeRequiredFieldsValidator struct{}

var _ resource.ConfigValidator = reportTimeRangeRequiredFieldsValidator{}

func (v reportTimeRangeRequiredFieldsValidator) Description(_ context.Context) string {
	return "Validates that time_range and secondary_time_range set every field the API requires"
}

func (v reportTimeRangeRequiredFieldsValidator) MarkdownDescription(_ context.Context) string {
	return "Validates that `time_range` and `secondary_time_range` set every field the API requires"
}

func (v reportTimeRangeRequiredFieldsValidator) ValidateResource(ctx context.Context, req resource.ValidateConfigRequest, resp *resource.ValidateConfigResponse) {
	validateReportTimeRangeRequiredFields(ctx, req.Config, &resp.Diagnostics)
}

// reportTimeRangeRequiredFieldsDataSourceValidator is the data-source counterpart
// of reportTimeRangeRequiredFieldsValidator.
type reportTimeRangeRequiredFieldsDataSourceValidator struct{}

var _ datasource.ConfigValidator = reportTimeRangeRequiredFieldsDataSourceValidator{}

func (v reportTimeRangeRequiredFieldsDataSourceValidator) Description(ctx context.Context) string {
	return reportTimeRangeRequiredFieldsValidator{}.Description(ctx)
}

func (v reportTimeRangeRequiredFieldsDataSourceValidator) MarkdownDescription(ctx context.Context) string {
	return reportTimeRangeRequiredFieldsValidator{}.MarkdownDescription(ctx)
}

func (v reportTimeRangeRequiredFieldsDataSourceValidator) ValidateDataSource(ctx context.Context, req datasource.ValidateConfigRequest, resp *datasource.ValidateConfigResponse) {
	validateReportTimeRangeRequiredFields(ctx, req.Config, &resp.Diagnostics)
}

// validateReportTimeRangeRequiredFields is the shared body used by the report
// resource and the report_query data source. Missing custom_time_range bounds are
// reported by validateReportTimestamps.
func validateReportTimeRangeRequiredFields(ctx context.Context, getter attributeGetter, diags *diag.Diagnostics) {
	var configWrapper resource_report.ConfigValue
	d := getter.GetAttribute(ctx, path.Root("config"), &configWrapper)
	diags.Append(d...)
	if d.HasError() || configWrapper.IsNull() || configWrapper.IsUnknown() {
		return
	}

	var timeRange resource_report.TimeRangeValue
	d = getter.GetAttribute(ctx, path.Root("config").AtName("time_range"), &timeRange)
	diags.Append(d...)
	if d.HasError() {
		return
	}

	var secondary resource_report.SecondaryTimeRangeValue
	d = getter.GetAttribute(ctx, path.Root("config").AtName("secondary_time_range"), &secondary)
	diags.Append(d...)
	if d.HasError() {
		return
	}

	validateReportPrimaryTimeRangeFields(timeRange, diags)
	validateReportSecondaryTimeRangeFields(secondary, diags)
	validateReportSecondaryRequiresTimeRange(timeRange, secondary, diags)
}

// validateReportPrimaryTimeRangeFields requires time_range.mode, and
// time_range.unit unless the mode is "custom". An omitted amount or
// include_current is valid: the API stores 0 and false.
func validateReportPrimaryTimeRangeFields(timeRange resource_report.TimeRangeValue, diags *diag.Diagnostics) {
	if timeRange.IsNull() || timeRange.IsUnknown() {
		return
	}
	timeRangePath := path.Root("config").AtName("time_range")

	if timeRange.Mode.IsUnknown() {
		return
	}
	if timeRange.Mode.IsNull() {
		diags.AddAttributeError(
			timeRangePath.AtName("mode"),
			"Missing Time Range Field",
			"`mode` is required when `time_range` is set.",
		)
		return
	}
	if timeRange.Mode.ValueString() == "custom" {
		return
	}

	if timeRange.Unit.IsNull() {
		diags.AddAttributeError(
			timeRangePath.AtName("unit"),
			"Missing Time Range Field",
			fmt.Sprintf("`unit` is required when `time_range.mode` is %q.", timeRange.Mode.ValueString()),
		)
	}
}

// validateReportSecondaryTimeRangeFields requires amount, unit, and
// include_current on a relative secondary range (one without custom_time_range).
func validateReportSecondaryTimeRangeFields(secondary resource_report.SecondaryTimeRangeValue, diags *diag.Diagnostics) {
	if secondary.IsNull() || secondary.IsUnknown() {
		return
	}
	if secondary.CustomTimeRange.IsUnknown() {
		return
	}
	if !secondary.CustomTimeRange.IsNull() {
		return
	}
	secondaryPath := path.Root("config").AtName("secondary_time_range")
	const detail = "`%s` is required when `secondary_time_range` does not set `custom_time_range`."

	if secondary.Amount.IsNull() {
		diags.AddAttributeError(secondaryPath.AtName("amount"), "Missing Secondary Time Range Field", fmt.Sprintf(detail, "amount"))
	}
	if secondary.Unit.IsNull() {
		diags.AddAttributeError(secondaryPath.AtName("unit"), "Missing Secondary Time Range Field", fmt.Sprintf(detail, "unit"))
	}
	if secondary.IncludeCurrent.IsNull() {
		diags.AddAttributeError(secondaryPath.AtName("include_current"), "Missing Secondary Time Range Field", fmt.Sprintf(detail, "include_current"))
	}
}

// validateReportSecondaryRequiresTimeRange rejects secondary_time_range without
// time_range. The API rejects a request that sets the secondary range without the
// primary range, and the provider sends secondary_time_range on every update while
// an omitted time_range is not sent.
func validateReportSecondaryRequiresTimeRange(timeRange resource_report.TimeRangeValue, secondary resource_report.SecondaryTimeRangeValue, diags *diag.Diagnostics) {
	if secondary.IsNull() || secondary.IsUnknown() {
		return
	}
	if !timeRange.IsNull() {
		return
	}

	diags.AddAttributeError(
		path.Root("config").AtName("time_range"),
		"Missing Time Range",
		"`time_range` is required when `secondary_time_range` is set.",
	)
}

// reportAttributionQueryDataSourceValidator validates DoiT Attribute attribution
// queries on the doit_report_query data source:
//   - data_source = "attribution" requires at least one attribution tier
//     dimension in group, dimensions or filters;
//   - attribution tier dimensions require data_source = "attribution" (the API
//     default, billing or billing-datahub, rejects them); and
//   - every tier dimension must belong to the same attribution (same id).
//
// The generated schema of the doit_report resource leaves out both the data
// source and the tier types (see attribution_schema.go), so this only applies
// to queries.
type reportAttributionQueryDataSourceValidator struct{}

var _ datasource.ConfigValidator = reportAttributionQueryDataSourceValidator{}

func (v reportAttributionQueryDataSourceValidator) Description(_ context.Context) string {
	return "Validates that data_source = \"attribution\" and the attribution tier dimensions are used together, for a single attribution"
}

func (v reportAttributionQueryDataSourceValidator) MarkdownDescription(_ context.Context) string {
	return "Validates that `data_source = \"attribution\"` and the attribution tier dimensions are used together, for a single attribution"
}

func (v reportAttributionQueryDataSourceValidator) ValidateDataSource(ctx context.Context, req datasource.ValidateConfigRequest, resp *datasource.ValidateConfigResponse) {
	validateReportAttributionQuery(ctx, req.Config, &resp.Diagnostics)
}

const invalidAttributionQuerySummary = "Invalid Attribution Query Configuration"

// attributionTierRef is one attribution tier dimension found in a report config.
type attributionTierRef struct {
	path path.Path // path of the group/dimensions/filters element
	id   types.String
}

func validateReportAttributionQuery(ctx context.Context, getter attributeGetter, diags *diag.Diagnostics) {
	configPath := path.Root("config")
	var refs []attributionTierRef
	hasUnknown := false

	var groups types.List
	d := getter.GetAttribute(ctx, configPath.AtName("group"), &groups)
	diags.Append(d...)
	if !d.HasError() && collectAttributionTierRefs(ctx, groups, configPath.AtName("group"), func(value resource_report.GroupValue) (types.String, types.String) {
		return value.GroupType, value.Id
	}, &refs, diags) {
		hasUnknown = true
	}

	var dimensions types.List
	d = getter.GetAttribute(ctx, configPath.AtName("dimensions"), &dimensions)
	diags.Append(d...)
	if !d.HasError() && collectAttributionTierRefs(ctx, dimensions, configPath.AtName("dimensions"), func(value resource_report.DimensionsValue) (types.String, types.String) {
		return value.DimensionsType, value.Id
	}, &refs, diags) {
		hasUnknown = true
	}

	var filters types.List
	d = getter.GetAttribute(ctx, configPath.AtName("filters"), &filters)
	diags.Append(d...)
	if !d.HasError() && collectAttributionTierRefs(ctx, filters, configPath.AtName("filters"), func(value resource_report.FiltersValue) (types.String, types.String) {
		return value.FiltersType, value.Id
	}, &refs, diags) {
		hasUnknown = true
	}

	// Independent of data_source: all tier dimensions must share one attribution.
	validateAttributionTierRefsSameID(refs, diags)

	var dataSource types.String
	d = getter.GetAttribute(ctx, configPath.AtName("data_source"), &dataSource)
	diags.Append(d...)
	if d.HasError() {
		return
	}
	if dataSource.IsUnknown() {
		return
	}

	if dataSource.IsNull() || dataSource.ValueString() != attributionReportDataSource {
		for _, ref := range refs {
			diags.AddAttributeError(
				ref.path.AtName("type"),
				invalidAttributionQuerySummary,
				fmt.Sprintf("Attribution tier dimensions select a DoiT Attribute attribution and require `data_source = %q`. "+
					"Without it the API uses the billing data source and rejects the query. See %s",
					attributionReportDataSource, attributeAttributionsGuideURL),
			)
		}
		return
	}

	if len(refs) == 0 && !hasUnknown {
		diags.AddAttributeError(
			configPath.AtName("data_source"),
			invalidAttributionQuerySummary,
			fmt.Sprintf("`data_source = %q` requires at least one `attribution_tier_1`, `attribution_tier_2` or "+
				"`attribution_tier_3` dimension in `group`, `dimensions` or `filters`. Its `id` is the attribution "+
				"set ID with `-` replaced by `_`. See %s",
				attributionReportDataSource, attributeAttributionsGuideURL),
		)
	}
}

// collectAttributionTierRefs appends the attribution tier dimensions of list to
// refs and reports whether the list, an element, or an element type is unknown.
// fields returns the element's type and id.
func collectAttributionTierRefs[T attr.Value](
	ctx context.Context,
	list types.List,
	listPath path.Path,
	fields func(T) (types.String, types.String),
	refs *[]attributionTierRef,
	diags *diag.Diagnostics,
) bool {
	if list.IsUnknown() {
		return true
	}
	if list.IsNull() {
		return false
	}

	var elements []T
	d := list.ElementsAs(ctx, &elements, false)
	diags.Append(d...)
	if d.HasError() {
		return false
	}

	hasUnknown := false
	for index, element := range elements {
		if element.IsUnknown() {
			hasUnknown = true
			continue
		}
		if element.IsNull() {
			continue
		}
		dimensionType, id := fields(element)
		if dimensionType.IsUnknown() {
			hasUnknown = true
			continue
		}
		if dimensionType.IsNull() || !isAttributionTierType(dimensionType.ValueString()) {
			continue
		}
		*refs = append(*refs, attributionTierRef{path: listPath.AtListIndex(index), id: id})
	}
	return hasUnknown
}

// validateAttributionTierRefsSameID reports every tier dimension whose known id
// differs from the first known id. Unknown and null ids are skipped.
func validateAttributionTierRefsSameID(refs []attributionTierRef, diags *diag.Diagnostics) {
	first := ""
	for _, ref := range refs {
		if ref.id.IsUnknown() {
			continue
		}
		if ref.id.IsNull() {
			continue
		}
		if first == "" {
			first = ref.id.ValueString()
			continue
		}
		if ref.id.ValueString() == first {
			continue
		}
		diags.AddAttributeError(
			ref.path.AtName("id"),
			invalidAttributionQuerySummary,
			fmt.Sprintf("Every attribution tier dimension in a query must belong to the same attribution, "+
				"but %q differs from %q. Query each attribution separately.", ref.id.ValueString(), first),
		)
	}
}
