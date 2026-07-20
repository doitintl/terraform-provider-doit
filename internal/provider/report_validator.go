package provider

import (
	"context"
	"fmt"

	"github.com/doitintl/terraform-provider-doit/internal/provider/resource_report"
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
	// Reject empty custom_time_range objects (set but both from and to are null).
	ctrPaths := []path.Path{
		path.Root("config").AtName("custom_time_range"),
		path.Root("config").AtName("secondary_time_range").AtName("custom_time_range"),
	}
	for _, p := range ctrPaths {
		var ctr resource_report.CustomTimeRangeValue
		diags := req.Config.GetAttribute(ctx, p, &ctr)
		resp.Diagnostics.Append(diags...)
		if diags.HasError() || ctr.IsNull() || ctr.IsUnknown() {
			continue
		}
		fromEmpty := ctr.From.IsNull() || ctr.From.IsUnknown()
		toEmpty := ctr.To.IsNull() || ctr.To.IsUnknown()
		if fromEmpty && toEmpty {
			resp.Diagnostics.AddAttributeError(
				p,
				"Empty Custom Time Range",
				"custom_time_range requires at least one of `from` or `to` to be set.",
			)
		}
	}

	// Validate individual timestamp formats.
	timestampPaths := []path.Path{
		path.Root("config").AtName("custom_time_range").AtName("from"),
		path.Root("config").AtName("custom_time_range").AtName("to"),
		path.Root("config").AtName("secondary_time_range").AtName("custom_time_range").AtName("from"),
		path.Root("config").AtName("secondary_time_range").AtName("custom_time_range").AtName("to"),
	}

	for _, p := range timestampPaths {
		var val types.String
		diags := req.Config.GetAttribute(ctx, p, &val)
		resp.Diagnostics.Append(diags...)
		if diags.HasError() {
			continue
		}
		if val.IsNull() || val.IsUnknown() {
			continue
		}
		validateRFC3339(val.ValueString(), p, &resp.Diagnostics)
	}
}

// reportMetricFieldsValidator validates that every configured metric object
// specifies both `type` and `value`. The DoiT API requires both fields on every
// ExternalMetric (in `metric`, `metric_filter.metric`, `limit_by_change.metric`,
// `group[*].limit.metric` and `metrics[*]`) and rejects requests that omit them
// with a cryptic "Field validation for 'Type'/'Value' failed on the 'required'
// tag" error at apply time. The generated schema marks these leaves
// Optional+Computed (the upstream OpenAPI ExternalMetric lists no required
// fields), so this validator surfaces the requirement as a clear plan-time error.
//
// This is a ConfigValidator because attribute-level validators do not fire on
// attributes inside SingleNestedAttribute with CustomType.
type reportMetricFieldsValidator struct{}

var _ resource.ConfigValidator = reportMetricFieldsValidator{}

func (v reportMetricFieldsValidator) Description(_ context.Context) string {
	return "Validates that every configured metric object specifies both type and value"
}

func (v reportMetricFieldsValidator) MarkdownDescription(_ context.Context) string {
	return "Validates that every configured metric object specifies both `type` and `value`"
}

// validateMetricFields reports an error for each of type/value that is explicitly
// null (omitted) on a configured metric. Unknown values (e.g. a custom-metric ID
// resolved after apply) are left to the API.
func validateMetricFields(metricType, value basetypes.StringValue, p path.Path, diags *diag.Diagnostics) {
	if metricType.IsNull() {
		diags.AddAttributeError(
			p.AtName("type"),
			"Missing Required Metric Field",
			"`type` is required on every metric object. The DoiT API rejects metrics without a `type` "+
				"(e.g. type = \"basic\").",
		)
	}
	if value.IsNull() {
		diags.AddAttributeError(
			p.AtName("value"),
			"Missing Required Metric Field",
			"`value` is required on every metric object. The DoiT API rejects metrics without a `value` "+
				"(e.g. value = \"cost\").",
		)
	}
}

func (v reportMetricFieldsValidator) ValidateResource(ctx context.Context, req resource.ValidateConfigRequest, resp *resource.ValidateConfigResponse) {
	validateReportMetricFieldsConfig(ctx, req.Config, &resp.Diagnostics)
}

// validateReportMetricFieldsConfig walks every metric object in a report config
// and requires both type and value on each. It is shared by the report resource
// (reportMetricFieldsValidator) and the report_query data source, both of which
// build the same ExternalConfig from an identical schema, so the requirement is
// enforced consistently across both consumers.
func validateReportMetricFieldsConfig(ctx context.Context, config tfsdk.Config, diags *diag.Diagnostics) {
	// Singular MetricValue objects.
	singular := map[string]resource_report.MetricValue{}
	singularPaths := map[string]path.Path{
		"metric":          path.Root("config").AtName("metric"),
		"metric_filter":   path.Root("config").AtName("metric_filter").AtName("metric"),
		"limit_by_change": path.Root("config").AtName("limit_by_change").AtName("metric"),
	}

	var metric resource_report.MetricValue
	if d := config.GetAttribute(ctx, path.Root("config").AtName("metric"), &metric); !d.HasError() {
		singular["metric"] = metric
	} else {
		diags.Append(d...)
	}

	var metricFilter resource_report.MetricFilterValue
	if d := config.GetAttribute(ctx, path.Root("config").AtName("metric_filter"), &metricFilter); !d.HasError() {
		if !metricFilter.IsNull() && !metricFilter.IsUnknown() {
			singular["metric_filter"] = metricFilter.Metric
		}
	} else {
		diags.Append(d...)
	}

	var limitByChange resource_report.LimitByChangeValue
	if d := config.GetAttribute(ctx, path.Root("config").AtName("limit_by_change"), &limitByChange); !d.HasError() {
		if !limitByChange.IsNull() && !limitByChange.IsUnknown() {
			singular["limit_by_change"] = limitByChange.Metric
		}
	} else {
		diags.Append(d...)
	}

	for key, m := range singular {
		if m.IsNull() || m.IsUnknown() {
			continue
		}
		validateMetricFields(m.MetricType, m.Value, singularPaths[key], diags)
	}

	// group[*].limit.metric
	var groups types.List
	groupDiags := config.GetAttribute(ctx, path.Root("config").AtName("group"), &groups)
	diags.Append(groupDiags...)
	if !groupDiags.HasError() && !groups.IsNull() && !groups.IsUnknown() {
		var groupVals []resource_report.GroupValue
		elemDiags := groups.ElementsAs(ctx, &groupVals, false)
		diags.Append(elemDiags...)
		if !elemDiags.HasError() {
			for i, g := range groupVals {
				if g.Limit.IsNull() || g.Limit.IsUnknown() {
					continue
				}
				if g.Limit.Metric.IsNull() || g.Limit.Metric.IsUnknown() {
					continue
				}
				validateMetricFields(
					g.Limit.Metric.MetricType, g.Limit.Metric.Value,
					path.Root("config").AtName("group").AtListIndex(i).AtName("limit").AtName("metric"),
					diags,
				)
			}
		}
	}

	// metrics[*]
	var metrics types.List
	metricsDiags := config.GetAttribute(ctx, path.Root("config").AtName("metrics"), &metrics)
	diags.Append(metricsDiags...)
	if !metricsDiags.HasError() && !metrics.IsNull() && !metrics.IsUnknown() {
		var metricsVals []resource_report.MetricsValue
		elemDiags := metrics.ElementsAs(ctx, &metricsVals, false)
		diags.Append(elemDiags...)
		if !elemDiags.HasError() {
			for i, m := range metricsVals {
				if m.IsNull() || m.IsUnknown() {
					continue
				}
				validateMetricFields(
					m.MetricsType, m.Value,
					path.Root("config").AtName("metrics").AtListIndex(i),
					diags,
				)
			}
		}
	}
}

// reportLimitByChangeFieldsValidator validates that when config.limit_by_change is
// set, its API-required fields (change_type, operator, values, include_incomplete_data)
// are provided. These are relaxed from Required to Optional+Computed in the schema so
// the object does not cause a permadiff when omitted (see report_resource.go), so the
// API requirement is enforced here at plan time instead. (metric.type/value are covered
// by reportMetricFieldsValidator.)
type reportLimitByChangeFieldsValidator struct{}

var _ resource.ConfigValidator = reportLimitByChangeFieldsValidator{}

func (v reportLimitByChangeFieldsValidator) Description(_ context.Context) string {
	return "Validates that a configured limit_by_change provides all API-required fields"
}

func (v reportLimitByChangeFieldsValidator) MarkdownDescription(_ context.Context) string {
	return "Validates that a configured `limit_by_change` provides all API-required fields"
}

func (v reportLimitByChangeFieldsValidator) ValidateResource(ctx context.Context, req resource.ValidateConfigRequest, resp *resource.ValidateConfigResponse) {
	validateLimitByChangeFieldsConfig(ctx, req.Config, &resp.Diagnostics)
}

// validateLimitByChangeFieldsConfig requires change_type, operator, values and
// include_incomplete_data when limit_by_change is set. Shared by the report resource
// and the report_query data source. Unknown values (resolved after apply) are left
// to the API.
func validateLimitByChangeFieldsConfig(ctx context.Context, config tfsdk.Config, diags *diag.Diagnostics) {
	var lbc resource_report.LimitByChangeValue
	if d := config.GetAttribute(ctx, path.Root("config").AtName("limit_by_change"), &lbc); d.HasError() {
		diags.Append(d...)
		return
	}
	if lbc.IsNull() || lbc.IsUnknown() {
		return
	}

	base := path.Root("config").AtName("limit_by_change")
	required := []struct {
		name string
		null bool
	}{
		{"change_type", lbc.ChangeType.IsNull()},
		{"operator", lbc.Operator.IsNull()},
		{"values", lbc.Values.IsNull()},
		{"include_incomplete_data", lbc.IncludeIncompleteData.IsNull()},
	}
	for _, r := range required {
		if r.null {
			diags.AddAttributeError(
				base.AtName(r.name),
				"Missing Required limit_by_change Field",
				fmt.Sprintf("`%s` is required when `limit_by_change` is set. The DoiT API rejects a "+
					"limit_by_change without it.", r.name),
			)
		}
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
