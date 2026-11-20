package provider

import (
	"context"
	"time"

	"github.com/doitintl/terraform-provider-doit/internal/provider/datasource_custom_metric"
	"github.com/doitintl/terraform-provider-doit/internal/provider/models"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// mapCustomMetricToDataSource maps the API response into the data source model.
func mapCustomMetricToDataSource(ctx context.Context, resp *models.CustomMetric, data *customMetricDataSourceModel) diag.Diagnostics {
	var diags diag.Diagnostics

	data.Id = types.StringPointerValue(resp.Id)
	data.Etag = types.StringPointerValue(resp.Etag)
	data.Name = types.StringValue(resp.Name)
	data.Description = types.StringValue(resp.Description)
	data.Formula = types.StringValue(resp.Formula)
	data.UnitType = types.StringValue(string(resp.UnitType))

	data.Owner = types.StringNull()
	if resp.Owner != nil {
		data.Owner = types.StringValue(string(*resp.Owner))
	}
	data.CreateTime = types.StringNull()
	if resp.CreateTime != nil {
		data.CreateTime = types.StringValue(resp.CreateTime.Format(time.RFC3339Nano))
	}
	data.UpdateTime = types.StringNull()
	if resp.UpdateTime != nil {
		data.UpdateTime = types.StringValue(resp.UpdateTime.Format(time.RFC3339Nano))
	}

	decimals, d := buildCustomMetricDataSourceDecimals(ctx, nullableToPointer(resp.Decimals))
	diags.Append(d...)
	data.Decimals = decimals

	variables, d := buildCustomMetricDataSourceVariables(ctx, resp.Variables)
	diags.Append(d...)
	data.Variables = variables

	return diags
}

// buildCustomMetricDataSourceDecimals maps decimals: a metric without display
// precision has none, which is null.
func buildCustomMetricDataSourceDecimals(ctx context.Context, decimals *models.CustomMetricDecimals) (datasource_custom_metric.DecimalsValue, diag.Diagnostics) {
	if decimals == nil {
		return datasource_custom_metric.NewDecimalsValueNull(), nil
	}
	return datasource_custom_metric.NewDecimalsValue(
		datasource_custom_metric.DecimalsValue{}.AttributeTypes(ctx),
		map[string]attr.Value{
			"min": types.Int64Value(int64(decimals.Min)),
			"max": types.Int64Value(int64(decimals.Max)),
		},
	)
}

// buildCustomMetricDataSourceVariables maps the variables list, in letter order.
func buildCustomMetricDataSourceVariables(ctx context.Context, variables []models.CustomMetricVariable) (types.List, diag.Diagnostics) {
	var diags diag.Diagnostics

	elemType := datasource_custom_metric.VariablesValue{}.Type(ctx)
	values := make([]datasource_custom_metric.VariablesValue, 0, len(variables))
	for _, v := range variables {
		metric, d := datasource_custom_metric.NewMetricValue(
			datasource_custom_metric.MetricValue{}.AttributeTypes(ctx),
			map[string]attr.Value{
				"type":  types.StringValue(string(v.Metric.Type)),
				"value": types.StringValue(v.Metric.Value),
			},
		)
		diags.Append(d...)
		if d.HasError() {
			return types.ListNull(elemType), diags
		}

		variable, d := datasource_custom_metric.NewVariablesValue(
			datasource_custom_metric.VariablesValue{}.AttributeTypes(ctx),
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
