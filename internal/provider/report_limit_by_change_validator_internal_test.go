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
	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

const (
	lbcCountSummary      = "Invalid Limit By Change Configuration"
	lbcMetricListedError = "Invalid Limit By Change Metric"
)

var (
	lbcPathCount  = path.Root("config").AtName("limit_by_change")
	lbcPathMetric = path.Root("config").AtName("limit_by_change").AtName("metric")

	lbcCost          = lbcMetricSpec{"basic", "cost"}
	lbcUsage         = lbcMetricSpec{"basic", "usage"}
	lbcSavings       = lbcMetricSpec{"basic", "savings"}
	lbcExtended      = lbcMetricSpec{"extended", "amortized_savings"}
	lbcCustomNumeric = lbcMetricSpec{"custom", "SIrmFwqPABo7JfOFGzTz"}
	lbcCustomPercent = lbcMetricSpec{"custom", "zJ0aYGHVSexVcon7tejj"}
)

type lbcMetricSpec struct {
	metricType string
	value      string
}

type lbcWantError struct {
	summary string
	path    path.Path
}

func lbcMetricsList(ctx context.Context, t *testing.T, specs ...lbcMetricSpec) types.List {
	t.Helper()
	values := make([]resource_report.MetricsValue, 0, len(specs))
	for _, s := range specs {
		values = append(values, resource_report.NewMetricsValueMust(resource_report.MetricsValue{}.AttributeTypes(ctx), map[string]attr.Value{
			"type":  types.StringValue(s.metricType),
			"value": types.StringValue(s.value),
		}))
	}
	list, diags := types.ListValueFrom(ctx, resource_report.MetricsValue{}.Type(ctx), values)
	if diags.HasError() {
		t.Fatalf("ListValueFrom metrics: %v", diags)
	}
	return list
}

func lbcMetricsListOf(ctx context.Context, t *testing.T, values ...resource_report.MetricsValue) types.List {
	t.Helper()
	list, diags := types.ListValueFrom(ctx, resource_report.MetricsValue{}.Type(ctx), values)
	if diags.HasError() {
		t.Fatalf("ListValueFrom metrics: %v", diags)
	}
	return list
}

func lbcKnownMetricsEntry(ctx context.Context, s lbcMetricSpec) resource_report.MetricsValue {
	return resource_report.NewMetricsValueMust(resource_report.MetricsValue{}.AttributeTypes(ctx), map[string]attr.Value{
		"type":  types.StringValue(s.metricType),
		"value": types.StringValue(s.value),
	})
}

func lbcPartialMetricsEntry(ctx context.Context, metricType, value attr.Value) resource_report.MetricsValue {
	return resource_report.NewMetricsValueMust(resource_report.MetricsValue{}.AttributeTypes(ctx), map[string]attr.Value{
		"type":  metricType,
		"value": value,
	})
}

func lbcDimensions(ctx context.Context, t *testing.T, dims ...[2]string) types.List {
	t.Helper()
	values := make([]resource_report.DimensionsValue, 0, len(dims))
	for _, d := range dims {
		values = append(values, resource_report.NewDimensionsValueMust(resource_report.DimensionsValue{}.AttributeTypes(ctx), map[string]attr.Value{
			"id":   types.StringValue(d[0]),
			"type": types.StringValue(d[1]),
		}))
	}
	list, diags := types.ListValueFrom(ctx, resource_report.DimensionsValue{}.Type(ctx), values)
	if diags.HasError() {
		t.Fatalf("ListValueFrom dimensions: %v", diags)
	}
	return list
}

func lbcValueFor(ctx context.Context, t *testing.T, metric resource_report.MetricValue) resource_report.LimitByChangeValue {
	t.Helper()
	values, diags := types.ListValueFrom(ctx, types.Float64Type, []float64{50})
	if diags.HasError() {
		t.Fatalf("ListValueFrom values: %v", diags)
	}
	return resource_report.NewLimitByChangeValueMust(resource_report.LimitByChangeValue{}.AttributeTypes(ctx), map[string]attr.Value{
		"change_type":             types.StringValue("percentage"),
		"include_incomplete_data": types.BoolValue(false),
		"metric":                  metric,
		"operator":                types.StringValue("gte"),
		"values":                  values,
	})
}

func lbcMetricValue(ctx context.Context, s lbcMetricSpec) resource_report.MetricValue {
	return resource_report.NewMetricValueMust(resource_report.MetricValue{}.AttributeTypes(ctx), map[string]attr.Value{
		"type":  types.StringValue(s.metricType),
		"value": types.StringValue(s.value),
	})
}

func lbcSecondaryRange(ctx context.Context) resource_report.SecondaryTimeRangeValue {
	return resource_report.NewSecondaryTimeRangeValueMust(resource_report.SecondaryTimeRangeValue{}.AttributeTypes(ctx), map[string]attr.Value{
		"amount":            types.Int64Value(1),
		"custom_time_range": resource_report.NewCustomTimeRangeValueNull(),
		"include_current":   types.BoolValue(false),
		"unit":              types.StringValue("month"),
	})
}

func lbcSecondaryCustomRange(ctx context.Context) resource_report.SecondaryTimeRangeValue {
	return resource_report.NewSecondaryTimeRangeValueMust(resource_report.SecondaryTimeRangeValue{}.AttributeTypes(ctx), map[string]attr.Value{
		"amount": types.Int64Null(),
		"custom_time_range": resource_report.NewCustomTimeRangeValueMust(resource_report.CustomTimeRangeValue{}.AttributeTypes(ctx), map[string]attr.Value{
			"from": types.StringValue("2026-08-01T00:00:00Z"),
			"to":   types.StringValue("2026-09-01T00:00:00Z"),
		}),
		"include_current": types.BoolNull(),
		"unit":            types.StringNull(),
	})
}

// lbcBaseConfigMap is a config that satisfies every limit_by_change rule: total
// aggregation, a secondary range, and a limit_by_change metric listed in metrics.
func lbcBaseConfigMap(ctx context.Context, t *testing.T) map[string]attr.Value {
	t.Helper()
	cfg := validCumulativeComparisonConfigMap(ctx, t)
	cfg["layout"] = types.StringNull()
	cfg["aggregation"] = types.StringValue("total")
	cfg["secondary_time_range"] = lbcSecondaryRange(ctx)
	cfg["metrics"] = lbcMetricsList(ctx, t, lbcCost)
	cfg["limit_by_change"] = lbcValueFor(ctx, t, lbcMetricValue(ctx, lbcCost))
	cfg["dimensions"] = lbcDimensions(ctx, t, [2]string{"year", "datetime"}, [2]string{"month", "datetime"})
	return cfg
}

// lbcWithMetric returns a map whose limit_by_change metric and metrics list both use m.
func lbcWithMetric(ctx context.Context, t *testing.T, cfg map[string]attr.Value, m lbcMetricSpec) {
	t.Helper()
	cfg["metrics"] = lbcMetricsList(ctx, t, m)
	cfg["limit_by_change"] = lbcValueFor(ctx, t, lbcMetricValue(ctx, m))
}

func lbcAssertDiags(t *testing.T, where string, diags diag.Diagnostics, want []lbcWantError) {
	t.Helper()
	if len(diags) != len(want) {
		t.Fatalf("%s: expected %d diagnostics, got %d: %v", where, len(want), len(diags), diags)
	}
	for i, d := range diags {
		if d.Severity() != diag.SeverityError {
			t.Errorf("%s: diagnostic %d severity = %v, want error", where, i, d.Severity())
		}
		if d.Summary() != want[i].summary {
			t.Errorf("%s: diagnostic %d summary = %q, want %q", where, i, d.Summary(), want[i].summary)
		}
		withPath, ok := d.(diag.DiagnosticWithPath)
		if !ok {
			t.Errorf("%s: diagnostic %d has no attribute path", where, i)
			continue
		}
		if !withPath.Path().Equal(want[i].path) {
			t.Errorf("%s: diagnostic %d path = %s, want %s", where, i, withPath.Path(), want[i].path)
		}
	}
}

// lbcRunAll runs the resource ConfigValidator, the data source ConfigValidator and
// the shared plan/config helper against the same config and requires identical results.
func lbcRunAll(ctx context.Context, t *testing.T, config tfsdk.Config, want []lbcWantError) {
	t.Helper()

	resResp := &resource.ValidateConfigResponse{}
	reportLimitByChangeValidator{}.ValidateResource(ctx, resource.ValidateConfigRequest{Config: config}, resResp)
	lbcAssertDiags(t, "resource validator", resResp.Diagnostics, want)

	dsResp := &datasource.ValidateConfigResponse{}
	reportLimitByChangeDataSourceValidator{}.ValidateDataSource(ctx, datasource.ValidateConfigRequest{Config: config}, dsResp)
	lbcAssertDiags(t, "data source validator", dsResp.Diagnostics, want)

	var planDiags diag.Diagnostics
	validateReportLimitByChange(ctx, tfsdk.Plan(config), &planDiags)
	lbcAssertDiags(t, "plan validation", planDiags, want)
}

func TestReportLimitByChangeValidator_ValidConfigs(t *testing.T) {
	ctx := t.Context()

	tests := []struct {
		name   string
		mutate func(cfg map[string]attr.Value)
	}{
		{
			name:   "baseline total aggregation with secondary range",
			mutate: func(_ map[string]attr.Value) {},
		},
		{
			name: "count aggregation with secondary range but no limit_by_change",
			mutate: func(cfg map[string]attr.Value) {
				cfg["aggregation"] = types.StringValue("count")
				cfg["limit_by_change"] = resource_report.NewLimitByChangeValueNull()
			},
		},
		{
			name: "count aggregation with limit_by_change and time dimensions but no secondary range",
			mutate: func(cfg map[string]attr.Value) {
				cfg["aggregation"] = types.StringValue("count")
				cfg["secondary_time_range"] = resource_report.NewSecondaryTimeRangeValueNull()
			},
		},
		{
			name: "percent_total with limit_by_change and secondary range",
			mutate: func(cfg map[string]attr.Value) {
				cfg["aggregation"] = types.StringValue("percent_total")
			},
		},
		{
			name: "percent_col with limit_by_change and secondary range",
			mutate: func(cfg map[string]attr.Value) {
				cfg["aggregation"] = types.StringValue("percent_col")
			},
		},
		{
			name: "percent_row with limit_by_change and secondary range",
			mutate: func(cfg map[string]attr.Value) {
				cfg["aggregation"] = types.StringValue("percent_row")
			},
		},
		{
			name: "total_over_total with limit_by_change and secondary range",
			mutate: func(cfg map[string]attr.Value) {
				cfg["aggregation"] = types.StringValue("total_over_total")
			},
		},
		{
			name: "secondary range set through custom_time_range",
			mutate: func(cfg map[string]attr.Value) {
				cfg["secondary_time_range"] = lbcSecondaryCustomRange(ctx)
			},
		},
		{
			name: "basic usage metric",
			mutate: func(cfg map[string]attr.Value) {
				lbcWithMetric(ctx, t, cfg, lbcUsage)
			},
		},
		{
			name: "basic savings metric",
			mutate: func(cfg map[string]attr.Value) {
				lbcWithMetric(ctx, t, cfg, lbcSavings)
			},
		},
		{
			name: "numeric custom metric without dimensions",
			mutate: func(cfg map[string]attr.Value) {
				lbcWithMetric(ctx, t, cfg, lbcCustomNumeric)
				cfg["dimensions"] = types.ListNull(resource_report.DimensionsValue{}.Type(ctx))
			},
		},
		{
			name: "numeric custom metric with a non-time dimension",
			mutate: func(cfg map[string]attr.Value) {
				lbcWithMetric(ctx, t, cfg, lbcCustomNumeric)
				cfg["dimensions"] = lbcDimensions(ctx, t, [2]string{"region", "fixed"})
			},
		},
		{
			name: "numeric custom metric with time dimensions",
			mutate: func(cfg map[string]attr.Value) {
				lbcWithMetric(ctx, t, cfg, lbcCustomNumeric)
			},
		},
		{
			name: "basic limit_by_change metric next to a percentage custom metric",
			mutate: func(cfg map[string]attr.Value) {
				cfg["metrics"] = lbcMetricsList(ctx, t, lbcCost, lbcCustomPercent)
			},
		},
		{
			name: "extended metric with time dimensions",
			mutate: func(cfg map[string]attr.Value) {
				lbcWithMetric(ctx, t, cfg, lbcExtended)
				cfg["data_source"] = types.StringValue("billing")
			},
		},
		{
			name: "extended metric without dimensions",
			mutate: func(cfg map[string]attr.Value) {
				lbcWithMetric(ctx, t, cfg, lbcExtended)
				cfg["data_source"] = types.StringValue("billing")
				cfg["dimensions"] = types.ListNull(resource_report.DimensionsValue{}.Type(ctx))
			},
		},
		{
			name: "extended metric next to other metrics",
			mutate: func(cfg map[string]attr.Value) {
				cfg["data_source"] = types.StringValue("billing")
				cfg["metrics"] = lbcMetricsList(ctx, t, lbcCost, lbcExtended, lbcUsage)
				cfg["limit_by_change"] = lbcValueFor(ctx, t, lbcMetricValue(ctx, lbcExtended))
			},
		},
		{
			name: "limit_by_change metric listed first",
			mutate: func(cfg map[string]attr.Value) {
				cfg["metrics"] = lbcMetricsList(ctx, t, lbcCost, lbcUsage)
			},
		},
		{
			name: "limit_by_change metric listed last",
			mutate: func(cfg map[string]attr.Value) {
				cfg["metrics"] = lbcMetricsList(ctx, t, lbcUsage, lbcSavings, lbcCost)
			},
		},
		{
			name: "count aggregation with limit_by_change but metric listed, secondary range null",
			mutate: func(cfg map[string]attr.Value) {
				cfg["aggregation"] = types.StringValue("count")
				cfg["secondary_time_range"] = resource_report.NewSecondaryTimeRangeValueNull()
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := lbcBaseConfigMap(ctx, t)
			tt.mutate(cfg)
			lbcRunAll(ctx, t, buildReportConfigWithMap(ctx, t, cfg), nil)
		})
	}
}

func TestReportLimitByChangeValidator_Deferral(t *testing.T) {
	ctx := t.Context()

	countWithSecondary := func(cfg map[string]attr.Value) {
		cfg["aggregation"] = types.StringValue("count")
	}
	metricNotListed := func(cfg map[string]attr.Value) {
		cfg["limit_by_change"] = lbcValueFor(ctx, t, lbcMetricValue(ctx, lbcUsage))
	}

	tests := []struct {
		name   string
		mutate func(cfg map[string]attr.Value)
	}{
		{
			name: "null limit_by_change",
			mutate: func(cfg map[string]attr.Value) {
				countWithSecondary(cfg)
				cfg["limit_by_change"] = resource_report.NewLimitByChangeValueNull()
			},
		},
		{
			name: "unknown limit_by_change",
			mutate: func(cfg map[string]attr.Value) {
				countWithSecondary(cfg)
				cfg["limit_by_change"] = resource_report.NewLimitByChangeValueUnknown()
			},
		},
		{
			name: "unknown secondary_time_range with count aggregation",
			mutate: func(cfg map[string]attr.Value) {
				countWithSecondary(cfg)
				cfg["secondary_time_range"] = resource_report.NewSecondaryTimeRangeValueUnknown()
			},
		},
		{
			name: "null secondary_time_range with count aggregation",
			mutate: func(cfg map[string]attr.Value) {
				countWithSecondary(cfg)
				cfg["secondary_time_range"] = resource_report.NewSecondaryTimeRangeValueNull()
			},
		},
		{
			name: "unknown aggregation",
			mutate: func(cfg map[string]attr.Value) {
				cfg["aggregation"] = types.StringUnknown()
			},
		},
		{
			name: "null aggregation",
			mutate: func(cfg map[string]attr.Value) {
				cfg["aggregation"] = types.StringNull()
			},
		},
		{
			name: "null metrics with unlisted limit_by_change metric",
			mutate: func(cfg map[string]attr.Value) {
				metricNotListed(cfg)
				cfg["metrics"] = types.ListNull(resource_report.MetricsValue{}.Type(ctx))
			},
		},
		{
			name: "unknown metrics with unlisted limit_by_change metric",
			mutate: func(cfg map[string]attr.Value) {
				metricNotListed(cfg)
				cfg["metrics"] = types.ListUnknown(resource_report.MetricsValue{}.Type(ctx))
			},
		},
		{
			name: "unknown limit_by_change metric",
			mutate: func(cfg map[string]attr.Value) {
				cfg["limit_by_change"] = lbcValueFor(ctx, t, resource_report.NewMetricValueUnknown())
			},
		},
		{
			name: "unknown limit_by_change metric value",
			mutate: func(cfg map[string]attr.Value) {
				cfg["limit_by_change"] = lbcValueFor(ctx, t, resource_report.NewMetricValueMust(resource_report.MetricValue{}.AttributeTypes(ctx), map[string]attr.Value{
					"type":  types.StringValue("basic"),
					"value": types.StringUnknown(),
				}))
			},
		},
		{
			name: "unknown limit_by_change metric type",
			mutate: func(cfg map[string]attr.Value) {
				cfg["limit_by_change"] = lbcValueFor(ctx, t, resource_report.NewMetricValueMust(resource_report.MetricValue{}.AttributeTypes(ctx), map[string]attr.Value{
					"type":  types.StringUnknown(),
					"value": types.StringValue("usage"),
				}))
			},
		},
		{
			name: "unknown metrics entry next to non-matching entry",
			mutate: func(cfg map[string]attr.Value) {
				metricNotListed(cfg)
				cfg["metrics"] = lbcMetricsListOf(ctx, t,
					lbcKnownMetricsEntry(ctx, lbcCost),
					resource_report.NewMetricsValueUnknown(),
				)
			},
		},
		{
			name: "unknown metrics entry before non-matching entry",
			mutate: func(cfg map[string]attr.Value) {
				metricNotListed(cfg)
				cfg["metrics"] = lbcMetricsListOf(ctx, t,
					resource_report.NewMetricsValueUnknown(),
					lbcKnownMetricsEntry(ctx, lbcCost),
				)
			},
		},
		{
			name: "metrics entry with unknown value",
			mutate: func(cfg map[string]attr.Value) {
				metricNotListed(cfg)
				cfg["metrics"] = lbcMetricsListOf(ctx, t,
					lbcKnownMetricsEntry(ctx, lbcCost),
					resource_report.NewMetricsValueMust(resource_report.MetricsValue{}.AttributeTypes(ctx), map[string]attr.Value{
						"type":  types.StringValue("basic"),
						"value": types.StringUnknown(),
					}),
				)
			},
		},
		{
			name: "entry with matching type and unknown value may still match",
			mutate: func(cfg map[string]attr.Value) {
				metricNotListed(cfg)
				cfg["metrics"] = lbcMetricsListOf(ctx, t,
					lbcPartialMetricsEntry(ctx, types.StringValue("basic"), types.StringUnknown()),
				)
			},
		},
		{
			name: "entry with unknown type and matching value may still match",
			mutate: func(cfg map[string]attr.Value) {
				metricNotListed(cfg)
				cfg["metrics"] = lbcMetricsListOf(ctx, t,
					lbcPartialMetricsEntry(ctx, types.StringUnknown(), types.StringValue("usage")),
				)
			},
		},
		{
			name: "entry with unknown type and unknown value may still match",
			mutate: func(cfg map[string]attr.Value) {
				metricNotListed(cfg)
				cfg["metrics"] = lbcMetricsListOf(ctx, t,
					lbcPartialMetricsEntry(ctx, types.StringUnknown(), types.StringUnknown()),
				)
			},
		},
		{
			name: "uncertain entry after a definite non-match still defers",
			mutate: func(cfg map[string]attr.Value) {
				metricNotListed(cfg)
				cfg["metrics"] = lbcMetricsListOf(ctx, t,
					lbcPartialMetricsEntry(ctx, types.StringValue("custom"), types.StringUnknown()),
					lbcPartialMetricsEntry(ctx, types.StringValue("basic"), types.StringUnknown()),
				)
			},
		},
		{
			name: "matching metrics entry next to unknown entry",
			mutate: func(cfg map[string]attr.Value) {
				cfg["metrics"] = lbcMetricsListOf(ctx, t,
					resource_report.NewMetricsValueUnknown(),
					lbcKnownMetricsEntry(ctx, lbcCost),
				)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := lbcBaseConfigMap(ctx, t)
			tt.mutate(cfg)
			lbcRunAll(ctx, t, buildReportConfigWithMap(ctx, t, cfg), nil)
		})
	}
}

func TestReportLimitByChangeValidator_InvalidConfigs(t *testing.T) {
	ctx := t.Context()

	countError := lbcWantError{summary: lbcCountSummary, path: lbcPathCount}
	metricError := lbcWantError{summary: lbcMetricListedError, path: lbcPathMetric}

	tests := []struct {
		name   string
		mutate func(cfg map[string]attr.Value)
		want   []lbcWantError
	}{
		{
			name: "count aggregation with basic metric and secondary range",
			mutate: func(cfg map[string]attr.Value) {
				cfg["aggregation"] = types.StringValue("count")
			},
			want: []lbcWantError{countError},
		},
		{
			name: "count aggregation with extended metric and secondary range",
			mutate: func(cfg map[string]attr.Value) {
				cfg["aggregation"] = types.StringValue("count")
				lbcWithMetric(ctx, t, cfg, lbcExtended)
			},
			want: []lbcWantError{countError},
		},
		{
			name: "count aggregation with custom metric and secondary range",
			mutate: func(cfg map[string]attr.Value) {
				cfg["aggregation"] = types.StringValue("count")
				lbcWithMetric(ctx, t, cfg, lbcCustomNumeric)
			},
			want: []lbcWantError{countError},
		},
		{
			name: "count aggregation with custom_time_range secondary range",
			mutate: func(cfg map[string]attr.Value) {
				cfg["aggregation"] = types.StringValue("count")
				cfg["secondary_time_range"] = lbcSecondaryCustomRange(ctx)
			},
			want: []lbcWantError{countError},
		},
		{
			name: "basic metric missing from metrics",
			mutate: func(cfg map[string]attr.Value) {
				cfg["limit_by_change"] = lbcValueFor(ctx, t, lbcMetricValue(ctx, lbcUsage))
			},
			want: []lbcWantError{metricError},
		},
		{
			name: "extended metric missing from metrics",
			mutate: func(cfg map[string]attr.Value) {
				cfg["limit_by_change"] = lbcValueFor(ctx, t, lbcMetricValue(ctx, lbcExtended))
			},
			want: []lbcWantError{metricError},
		},
		{
			name: "custom metric missing from metrics",
			mutate: func(cfg map[string]attr.Value) {
				cfg["limit_by_change"] = lbcValueFor(ctx, t, lbcMetricValue(ctx, lbcCustomNumeric))
			},
			want: []lbcWantError{metricError},
		},
		{
			name: "same metric type with a different value",
			mutate: func(cfg map[string]attr.Value) {
				cfg["metrics"] = lbcMetricsList(ctx, t, lbcUsage)
			},
			want: []lbcWantError{metricError},
		},
		{
			name: "same value with a different metric type",
			mutate: func(cfg map[string]attr.Value) {
				cfg["metrics"] = lbcMetricsList(ctx, t, lbcMetricSpec{"custom", "cost"})
			},
			want: []lbcWantError{metricError},
		},
		{
			name: "metric missing from metrics without a secondary range",
			mutate: func(cfg map[string]attr.Value) {
				cfg["secondary_time_range"] = resource_report.NewSecondaryTimeRangeValueNull()
				cfg["limit_by_change"] = lbcValueFor(ctx, t, lbcMetricValue(ctx, lbcUsage))
			},
			want: []lbcWantError{metricError},
		},
		{
			name: "metric missing from an empty metrics list",
			mutate: func(cfg map[string]attr.Value) {
				cfg["metrics"] = lbcMetricsList(ctx, t)
			},
			want: []lbcWantError{metricError},
		},
		{
			name: "metric missing from several known metrics",
			mutate: func(cfg map[string]attr.Value) {
				cfg["metrics"] = lbcMetricsList(ctx, t, lbcUsage, lbcSavings, lbcCustomPercent)
			},
			want: []lbcWantError{metricError},
		},
		{
			name: "entry with mismatched type and unknown value cannot match",
			mutate: func(cfg map[string]attr.Value) {
				cfg["limit_by_change"] = lbcValueFor(ctx, t, lbcMetricValue(ctx, lbcUsage))
				cfg["metrics"] = lbcMetricsListOf(ctx, t,
					lbcPartialMetricsEntry(ctx, types.StringValue("custom"), types.StringUnknown()),
				)
			},
			want: []lbcWantError{metricError},
		},
		{
			name: "entry with unknown type and mismatched value cannot match",
			mutate: func(cfg map[string]attr.Value) {
				cfg["limit_by_change"] = lbcValueFor(ctx, t, lbcMetricValue(ctx, lbcUsage))
				cfg["metrics"] = lbcMetricsListOf(ctx, t,
					lbcPartialMetricsEntry(ctx, types.StringUnknown(), types.StringValue("cost")),
				)
			},
			want: []lbcWantError{metricError},
		},
		{
			name: "known mismatching entries next to entries that cannot match",
			mutate: func(cfg map[string]attr.Value) {
				cfg["limit_by_change"] = lbcValueFor(ctx, t, lbcMetricValue(ctx, lbcUsage))
				cfg["metrics"] = lbcMetricsListOf(ctx, t,
					lbcKnownMetricsEntry(ctx, lbcCost),
					lbcPartialMetricsEntry(ctx, types.StringValue("custom"), types.StringUnknown()),
					lbcPartialMetricsEntry(ctx, types.StringUnknown(), types.StringValue("savings")),
				)
			},
			want: []lbcWantError{metricError},
		},
		{
			name: "entry with null type cannot match",
			mutate: func(cfg map[string]attr.Value) {
				cfg["limit_by_change"] = lbcValueFor(ctx, t, lbcMetricValue(ctx, lbcUsage))
				cfg["metrics"] = lbcMetricsListOf(ctx, t,
					lbcPartialMetricsEntry(ctx, types.StringNull(), types.StringValue("usage")),
				)
			},
			want: []lbcWantError{metricError},
		},
		{
			name: "count aggregation and unlisted metric report both errors",
			mutate: func(cfg map[string]attr.Value) {
				cfg["aggregation"] = types.StringValue("count")
				cfg["limit_by_change"] = lbcValueFor(ctx, t, lbcMetricValue(ctx, lbcUsage))
			},
			want: []lbcWantError{countError, metricError},
		},
		{
			name: "unknown aggregation does not suppress the unlisted metric error",
			mutate: func(cfg map[string]attr.Value) {
				cfg["aggregation"] = types.StringUnknown()
				cfg["limit_by_change"] = lbcValueFor(ctx, t, lbcMetricValue(ctx, lbcUsage))
			},
			want: []lbcWantError{metricError},
		},
		{
			name: "unknown metrics does not suppress the count error",
			mutate: func(cfg map[string]attr.Value) {
				cfg["aggregation"] = types.StringValue("count")
				cfg["metrics"] = types.ListUnknown(resource_report.MetricsValue{}.Type(ctx))
			},
			want: []lbcWantError{countError},
		},
		{
			name: "unknown secondary range does not suppress the unlisted metric error",
			mutate: func(cfg map[string]attr.Value) {
				cfg["secondary_time_range"] = resource_report.NewSecondaryTimeRangeValueUnknown()
				cfg["limit_by_change"] = lbcValueFor(ctx, t, lbcMetricValue(ctx, lbcUsage))
			},
			want: []lbcWantError{metricError},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := lbcBaseConfigMap(ctx, t)
			tt.mutate(cfg)
			lbcRunAll(ctx, t, buildReportConfigWithMap(ctx, t, cfg), tt.want)
		})
	}
}

// lbcModifyPlan runs ModifyPlan with the given prior state map (nil for create), planned map and config map.
func lbcModifyPlan(ctx context.Context, t *testing.T, state, plan, config map[string]attr.Value) *resource.ModifyPlanResponse {
	t.Helper()

	configCfg := buildReportConfigWithMap(ctx, t, config)
	req := resource.ModifyPlanRequest{
		Config: configCfg,
		Plan:   tfsdk.Plan(buildReportConfigWithMap(ctx, t, plan)),
	}
	if state == nil {
		schema := resource_report.ReportResourceSchema(ctx)
		req.State = tfsdk.State{Schema: schema, Raw: tftypes.NewValue(schema.Type().TerraformType(ctx), nil)}
	} else {
		req.State = tfsdk.State(buildReportConfigWithMap(ctx, t, state))
	}

	resp := &resource.ModifyPlanResponse{}
	(&reportResource{}).ModifyPlan(ctx, req, resp)
	return resp
}

func lbcCountErrors(diags diag.Diagnostics, summary string) int {
	n := 0
	for _, d := range diags.Errors() {
		if d.Summary() == summary {
			n++
		}
	}
	return n
}

func TestReportResource_ModifyPlan_LimitByChange(t *testing.T) {
	ctx := t.Context()

	t.Run("create validates the resolved config", func(t *testing.T) {
		cfg := lbcBaseConfigMap(ctx, t)
		cfg["limit_by_change"] = lbcValueFor(ctx, t, lbcMetricValue(ctx, lbcUsage))

		resp := lbcModifyPlan(ctx, t, nil, cfg, cfg)
		if got := lbcCountErrors(resp.Diagnostics, lbcMetricListedError); got != 1 {
			t.Fatalf("expected one %q error, got %d: %v", lbcMetricListedError, got, resp.Diagnostics)
		}
	})

	t.Run("create with a valid config produces no diagnostics", func(t *testing.T) {
		cfg := lbcBaseConfigMap(ctx, t)
		resp := lbcModifyPlan(ctx, t, nil, cfg, cfg)
		if len(resp.Diagnostics) != 0 {
			t.Fatalf("expected no diagnostics, got: %v", resp.Diagnostics)
		}
	})

	t.Run("update rejects a config whose metrics no longer list the limit_by_change metric", func(t *testing.T) {
		state := lbcBaseConfigMap(ctx, t)
		state["metrics"] = lbcMetricsList(ctx, t, lbcCost, lbcUsage)
		state["limit_by_change"] = lbcValueFor(ctx, t, lbcMetricValue(ctx, lbcUsage))

		cfg := lbcBaseConfigMap(ctx, t)
		cfg["limit_by_change"] = lbcValueFor(ctx, t, lbcMetricValue(ctx, lbcUsage))
		cfg["metrics"] = lbcMetricsList(ctx, t, lbcCost)

		resp := lbcModifyPlan(ctx, t, state, cfg, cfg)
		if got := lbcCountErrors(resp.Diagnostics, lbcMetricListedError); got != 1 {
			t.Fatalf("expected one %q error, got %d: %v", lbcMetricListedError, got, resp.Diagnostics)
		}
	})

	t.Run("update removing limit_by_change together with its metric is not rejected", func(t *testing.T) {
		// Prior state has limit_by_change on usage. The config drops both limit_by_change and the
		// usage metric; the plan still carries the inherited limit_by_change from state.
		state := lbcBaseConfigMap(ctx, t)
		state["metrics"] = lbcMetricsList(ctx, t, lbcCost, lbcUsage)
		state["limit_by_change"] = lbcValueFor(ctx, t, lbcMetricValue(ctx, lbcUsage))

		plan := lbcBaseConfigMap(ctx, t)
		plan["limit_by_change"] = lbcValueFor(ctx, t, lbcMetricValue(ctx, lbcUsage))
		plan["metrics"] = lbcMetricsList(ctx, t, lbcCost)

		cfg := lbcBaseConfigMap(ctx, t)
		cfg["limit_by_change"] = resource_report.NewLimitByChangeValueNull()
		cfg["metrics"] = lbcMetricsList(ctx, t, lbcCost)

		resp := lbcModifyPlan(ctx, t, state, plan, cfg)
		if resp.Diagnostics.HasError() {
			t.Fatalf("expected no errors, got: %v", resp.Diagnostics)
		}
	})
}

func lbcCountValue(ctx context.Context, t *testing.T) resource_report.CountValue {
	t.Helper()
	return resource_report.NewCountValueMust(resource_report.CountValue{}.AttributeTypes(ctx), map[string]attr.Value{
		"id":   types.StringValue("service_description"),
		"type": types.StringValue("fixed"),
	})
}

// lbcCountSecondaryStateMap is a valid prior state: count aggregation with a
// secondary range and no limit_by_change.
func lbcCountSecondaryStateMap(ctx context.Context, t *testing.T) map[string]attr.Value {
	t.Helper()
	cfg := lbcBaseConfigMap(ctx, t)
	cfg["aggregation"] = types.StringValue("count")
	cfg["count"] = lbcCountValue(ctx, t)
	cfg["limit_by_change"] = resource_report.NewLimitByChangeValueNull()
	return cfg
}

// An update that keeps aggregation = "count" and adds limit_by_change while omitting
// secondary_time_range inherits the secondary range from prior state (it is not
// clearable in place), so the API-effective config is the rejected combination.
func TestReportResource_ModifyPlan_LimitByChange_InheritedSecondaryRange(t *testing.T) {
	ctx := t.Context()

	state := lbcCountSecondaryStateMap(ctx, t)

	// The plan marks the omitted nested attribute as unknown rather than inheriting it.
	plan := lbcCountSecondaryStateMap(ctx, t)
	plan["limit_by_change"] = lbcValueFor(ctx, t, lbcMetricValue(ctx, lbcCost))
	plan["secondary_time_range"] = resource_report.NewSecondaryTimeRangeValueUnknown()

	cfg := lbcCountSecondaryStateMap(ctx, t)
	cfg["limit_by_change"] = lbcValueFor(ctx, t, lbcMetricValue(ctx, lbcCost))
	cfg["secondary_time_range"] = resource_report.NewSecondaryTimeRangeValueNull()

	resp := lbcModifyPlan(ctx, t, state, plan, cfg)
	if got := lbcCountErrors(resp.Diagnostics, lbcCountSummary); got != 1 {
		t.Fatalf("expected one %q error for the inherited secondary range, got %d: %v", lbcCountSummary, got, resp.Diagnostics)
	}
}

// Omitting aggregation and the count block forces a replacement (count is Category C),
// and the replacement is created from config alone, so the plan's inherited
// count/secondary values must not be treated as the effective configuration.
func TestReportResource_ModifyPlan_LimitByChange_OmittedCountForcesReplace(t *testing.T) {
	ctx := t.Context()

	state := lbcCountSecondaryStateMap(ctx, t)

	plan := lbcCountSecondaryStateMap(ctx, t)
	plan["limit_by_change"] = lbcValueFor(ctx, t, lbcMetricValue(ctx, lbcCost))
	plan["secondary_time_range"] = resource_report.NewSecondaryTimeRangeValueUnknown()

	cfg := lbcCountSecondaryStateMap(ctx, t)
	cfg["limit_by_change"] = lbcValueFor(ctx, t, lbcMetricValue(ctx, lbcCost))
	cfg["aggregation"] = types.StringNull()
	cfg["count"] = resource_report.NewCountValueNull()
	cfg["secondary_time_range"] = resource_report.NewSecondaryTimeRangeValueNull()

	resp := lbcModifyPlan(ctx, t, state, plan, cfg)
	if resp.Diagnostics.HasError() {
		t.Fatalf("expected no errors, got: %v", resp.Diagnostics)
	}
	if len(resp.RequiresReplace) == 0 {
		t.Fatalf("expected the removed count block to force replacement")
	}
}

// Without a secondary range in prior state there is nothing to inherit.
func TestReportResource_ModifyPlan_LimitByChange_NoInheritedSecondaryRange(t *testing.T) {
	ctx := t.Context()

	state := lbcCountSecondaryStateMap(ctx, t)
	state["secondary_time_range"] = resource_report.NewSecondaryTimeRangeValueNull()

	plan := lbcCountSecondaryStateMap(ctx, t)
	plan["secondary_time_range"] = resource_report.NewSecondaryTimeRangeValueUnknown()
	plan["limit_by_change"] = lbcValueFor(ctx, t, lbcMetricValue(ctx, lbcCost))

	cfg := lbcCountSecondaryStateMap(ctx, t)
	cfg["secondary_time_range"] = resource_report.NewSecondaryTimeRangeValueNull()
	cfg["limit_by_change"] = lbcValueFor(ctx, t, lbcMetricValue(ctx, lbcCost))

	resp := lbcModifyPlan(ctx, t, state, plan, cfg)
	if got := lbcCountErrors(resp.Diagnostics, lbcCountSummary); got != 0 {
		t.Fatalf("expected no %q error, got %d: %v", lbcCountSummary, got, resp.Diagnostics)
	}
}

// Switching to a non-count aggregation while a secondary range is kept is valid.
func TestReportResource_ModifyPlan_LimitByChange_InheritedSecondaryRangeNonCount(t *testing.T) {
	ctx := t.Context()

	state := lbcCountSecondaryStateMap(ctx, t)

	plan := lbcCountSecondaryStateMap(ctx, t)
	plan["aggregation"] = types.StringValue("total")
	plan["count"] = resource_report.NewCountValueNull()
	plan["secondary_time_range"] = resource_report.NewSecondaryTimeRangeValueUnknown()
	plan["limit_by_change"] = lbcValueFor(ctx, t, lbcMetricValue(ctx, lbcCost))

	cfg := lbcCountSecondaryStateMap(ctx, t)
	cfg["aggregation"] = types.StringValue("total")
	cfg["count"] = resource_report.NewCountValueNull()
	cfg["secondary_time_range"] = resource_report.NewSecondaryTimeRangeValueNull()
	cfg["limit_by_change"] = lbcValueFor(ctx, t, lbcMetricValue(ctx, lbcCost))

	resp := lbcModifyPlan(ctx, t, state, plan, cfg)
	if got := lbcCountErrors(resp.Diagnostics, lbcCountSummary); got != 0 {
		t.Fatalf("expected no %q error, got %d: %v", lbcCountSummary, got, resp.Diagnostics)
	}
}
