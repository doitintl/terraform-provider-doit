package provider

import (
	"context"
	"fmt"

	"github.com/doitintl/terraform-provider-doit/internal/provider/resource_report"
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
// 1. When set, at least one of from/to must be specified (rejects empty `{}`).
// 2. Any provided from/to values must be valid RFC3339 timestamps.
//
// This is a ConfigValidator because attribute-level validators do not fire on
// attributes inside SingleNestedAttribute with CustomType (which the code
// generator adds to all nested objects).
type reportTimestampValidator struct{}

var _ resource.ConfigValidator = reportTimestampValidator{}

func (v reportTimestampValidator) Description(_ context.Context) string {
	return "Validates custom_time_range objects are non-empty and contain valid RFC3339 timestamps"
}

func (v reportTimestampValidator) MarkdownDescription(_ context.Context) string {
	return "Validates `custom_time_range` objects are non-empty and contain valid RFC3339 timestamps"
}

func (v reportTimestampValidator) ValidateResource(ctx context.Context, req resource.ValidateConfigRequest, resp *resource.ValidateConfigResponse) {
	validateReportTimestamps(ctx, req.Config, &resp.Diagnostics)
}

// reportTimestampDataSourceValidator is the data-source counterpart of
// reportTimestampValidator. The report_query data source reuses the report
// resource's config types, so the same validation applies — rejecting empty date
// ranges at plan time instead of letting them fail at the API.
type reportTimestampDataSourceValidator struct{}

var _ datasource.ConfigValidator = reportTimestampDataSourceValidator{}

func (v reportTimestampDataSourceValidator) Description(_ context.Context) string {
	return "Validates custom_time_range objects are non-empty and contain valid RFC3339 timestamps"
}

func (v reportTimestampDataSourceValidator) MarkdownDescription(_ context.Context) string {
	return "Validates `custom_time_range` objects are non-empty and contain valid RFC3339 timestamps"
}

func (v reportTimestampDataSourceValidator) ValidateDataSource(ctx context.Context, req datasource.ValidateConfigRequest, resp *datasource.ValidateConfigResponse) {
	validateReportTimestamps(ctx, req.Config, &resp.Diagnostics)
}

// validateReportTimestamps is the shared body used by both the resource and the
// report_query data source. It rejects empty custom/forecast date ranges and
// validates that any provided from/to values are RFC3339. Unknown values are
// deferred (not treated as empty) so dynamic references validate once resolved.
func validateReportTimestamps(ctx context.Context, config tfsdk.Config, diags *diag.Diagnostics) {
	// Reject empty custom_time_range objects (set but both from and to are null).
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
		fromEmpty := ctr.From.IsNull() || ctr.From.IsUnknown()
		toEmpty := ctr.To.IsNull() || ctr.To.IsUnknown()
		if fromEmpty && toEmpty {
			diags.AddAttributeError(
				p,
				"Empty Custom Time Range",
				"custom_time_range requires at least one of `from` or `to` to be set.",
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
	validateReportCountAggregation(ctx, req.Config, &resp.Diagnostics)
}

// validateReportCountAggregation is the shared body used by both the report
// resource and the report_query data source, which build the same ExternalConfig
// from an identical schema. It enforces the count/aggregation coupling in both
// directions (the API rejects either mismatch with a 400; verified: an omitted
// aggregation does NOT default to "count"). Unknown values (e.g. cross-resource
// references) are deferred, since they may resolve to a valid combination.
func validateReportCountAggregation(ctx context.Context, config tfsdk.Config, diags *diag.Diagnostics) {
	var count resource_report.CountValue
	d := config.GetAttribute(ctx, path.Root("config").AtName("count"), &count)
	diags.Append(d...)
	if d.HasError() {
		return
	}

	var aggregation types.String
	d = config.GetAttribute(ctx, path.Root("config").AtName("aggregation"), &aggregation)
	diags.Append(d...)
	if d.HasError() {
		return
	}

	// Defer while either value is unknown — it may resolve to a valid combination.
	if count.IsUnknown() || aggregation.IsUnknown() {
		return
	}

	aggIsCount := !aggregation.IsNull() && aggregation.ValueString() == "count"

	switch {
	case !count.IsNull() && !aggIsCount:
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
	case count.IsNull() && aggIsCount:
		// aggregation "count" requires a count block.
		diags.AddAttributeError(
			path.Root("config").AtName("count"),
			"Missing Count Configuration",
			"`aggregation = \"count\"` requires a `count` block specifying the field to count "+
				"(e.g. count = { id = \"service_description\", type = \"fixed\" }).",
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
