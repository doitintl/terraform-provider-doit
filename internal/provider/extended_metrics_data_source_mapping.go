package provider

import (
	"context"

	"github.com/doitintl/terraform-provider-doit/internal/provider/datasource_extended_metrics"
	"github.com/doitintl/terraform-provider-doit/internal/provider/models"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// mapExtendedMetricsToDataSource maps the list response items to the items
// attribute. items is [] rather than null when there are none.
func mapExtendedMetricsToDataSource(ctx context.Context, items []models.ExtendedMetric, data *extendedMetricsDataSourceModel) diag.Diagnostics {
	var diags diag.Diagnostics

	values := make([]datasource_extended_metrics.ItemsValue, 0, len(items))
	for i := range items {
		item := &items[i]

		value, d := datasource_extended_metrics.NewItemsValue(
			datasource_extended_metrics.ItemsValue{}.AttributeTypes(ctx),
			map[string]attr.Value{
				"key":         types.StringValue(item.Key),
				"label":       types.StringValue(item.Label),
				"data_source": types.StringValue(string(item.DataSource)),
			},
		)
		diags.Append(d...)
		if diags.HasError() {
			return diags
		}
		values = append(values, value)
	}

	list, d := types.ListValueFrom(ctx, datasource_extended_metrics.ItemsValue{}.Type(ctx), values)
	diags.Append(d...)
	data.Items = list
	return diags
}
