package provider

import (
	"context"
	"time"

	"github.com/doitintl/terraform-provider-doit/internal/provider/datasource_custom_metrics"
	"github.com/doitintl/terraform-provider-doit/internal/provider/models"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// mapCustomMetricsToDataSource maps the fetched custom metrics into data.Items.
// An empty result is an empty list, never null.
func mapCustomMetricsToDataSource(ctx context.Context, items []models.CustomMetric, data *customMetricsDataSourceModel) diag.Diagnostics {
	var diags diag.Diagnostics

	values := make([]datasource_custom_metrics.ItemsValue, 0, len(items))
	for i := range items {
		item := &items[i]

		decimals, d := buildCustomMetricsDataSourceDecimals(ctx, nullableToPointer(item.Decimals))
		diags.Append(d...)
		variables, d := buildCustomMetricsDataSourceVariables(ctx, item.Variables)
		diags.Append(d...)
		if diags.HasError() {
			return diags
		}

		owner := types.StringNull()
		if item.Owner != nil {
			owner = types.StringValue(string(*item.Owner))
		}
		createTime := types.StringNull()
		if item.CreateTime != nil {
			createTime = types.StringValue(item.CreateTime.Format(time.RFC3339Nano))
		}
		updateTime := types.StringNull()
		if item.UpdateTime != nil {
			updateTime = types.StringValue(item.UpdateTime.Format(time.RFC3339Nano))
		}

		value, d := datasource_custom_metrics.NewItemsValue(
			datasource_custom_metrics.ItemsValue{}.AttributeTypes(ctx),
			map[string]attr.Value{
				"id":          types.StringPointerValue(item.Id),
				"etag":        types.StringPointerValue(item.Etag),
				"name":        types.StringValue(item.Name),
				"description": types.StringValue(item.Description),
				"owner":       owner,
				"formula":     types.StringValue(item.Formula),
				"unit_type":   types.StringValue(string(item.UnitType)),
				"decimals":    decimals,
				"variables":   variables,
				"create_time": createTime,
				"update_time": updateTime,
			},
		)
		diags.Append(d...)
		if d.HasError() {
			return diags
		}
		values = append(values, value)
	}

	list, d := types.ListValueFrom(ctx, datasource_custom_metrics.ItemsValue{}.Type(ctx), values)
	diags.Append(d...)
	data.Items = list
	return diags
}

// buildCustomMetricsDataSourceDecimals maps decimals: a metric without display
// precision has none, which is null.
func buildCustomMetricsDataSourceDecimals(ctx context.Context, decimals *models.CustomMetricDecimals) (datasource_custom_metrics.DecimalsValue, diag.Diagnostics) {
	if decimals == nil {
		return datasource_custom_metrics.NewDecimalsValueNull(), nil
	}
	return datasource_custom_metrics.NewDecimalsValue(
		datasource_custom_metrics.DecimalsValue{}.AttributeTypes(ctx),
		map[string]attr.Value{
			"min": types.Int64Value(int64(decimals.Min)),
			"max": types.Int64Value(int64(decimals.Max)),
		},
	)
}

// buildCustomMetricsDataSourceVariables maps the variables list, in letter order.
func buildCustomMetricsDataSourceVariables(ctx context.Context, variables []models.CustomMetricVariable) (types.List, diag.Diagnostics) {
	var diags diag.Diagnostics

	elemType := datasource_custom_metrics.VariablesValue{}.Type(ctx)
	values := make([]datasource_custom_metrics.VariablesValue, 0, len(variables))
	for _, v := range variables {
		metric, d := datasource_custom_metrics.NewMetricValue(
			datasource_custom_metrics.MetricValue{}.AttributeTypes(ctx),
			map[string]attr.Value{
				"type":  types.StringValue(string(v.Metric.Type)),
				"value": types.StringValue(v.Metric.Value),
			},
		)
		diags.Append(d...)
		if d.HasError() {
			return types.ListNull(elemType), diags
		}

		variable, d := datasource_custom_metrics.NewVariablesValue(
			datasource_custom_metrics.VariablesValue{}.AttributeTypes(ctx),
			map[string]attr.Value{
				"allocation_id": types.StringValue(v.AllocationId),
				"letter":        types.StringPointerValue(v.Letter),
				"metric":        metric,
			},
		)
		diags.Append(d...)
		if d.HasError() {
			return types.ListNull(elemType), diags
		}
		values = append(values, variable)
	}

	list, d := types.ListValueFrom(ctx, elemType, values)
	diags.Append(d...)
	return list, diags
}
