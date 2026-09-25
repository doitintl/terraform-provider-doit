package provider

import (
	"context"
	"fmt"

	"github.com/doitintl/terraform-provider-doit/internal/provider/datasource_widgets"
	"github.com/doitintl/terraform-provider-doit/internal/provider/models"
	"github.com/hashicorp/terraform-plugin-framework-timeouts/datasource/timeouts"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var _ datasource.DataSource = (*widgetsDataSource)(nil)
var _ datasource.DataSourceWithConfigure = (*widgetsDataSource)(nil)

func NewWidgetsDataSource() datasource.DataSource {
	return &widgetsDataSource{}
}

type widgetsDataSource struct {
	client *models.ClientWithResponses
}

type widgetsDataSourceModel struct {
	datasource_widgets.WidgetsModel
	Timeouts timeouts.Value `tfsdk:"timeouts"`
}

func (d *widgetsDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_widgets"
}

func (d *widgetsDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	client, ok := req.ProviderData.(*models.ClientWithResponses)
	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Data Source Configure Type",
			fmt.Sprintf("Expected *models.ClientWithResponses, got: %T", req.ProviderData),
		)
		return
	}
	d.client = client
}

func (d *widgetsDataSource) Schema(ctx context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	s := datasource_widgets.WidgetsDataSourceSchema(ctx)
	s.Attributes["timeouts"] = timeouts.Attributes(ctx)
	resp.Schema = s
}

func (d *widgetsDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data widgetsDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	readTimeout, diags := data.Timeouts.Read(ctx, DefaultReadTimeout)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	ctx, cancel := context.WithTimeout(ctx, readTimeout)
	defer cancel()

	apiResp, err := d.client.ListWidgetsWithResponse(ctx, nil)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Reading Widgets",
			fmt.Sprintf("Unable to read widgets: %v", err),
		)
		return
	}
	if apiResp.StatusCode() != 200 || apiResp.JSON200 == nil {
		resp.Diagnostics.AddError(
			"Error Reading Widgets",
			fmt.Sprintf("API returned status %d: %s", apiResp.StatusCode(), string(apiResp.Body)),
		)
		return
	}

	resp.Diagnostics.Append(mapWidgetsToModel(ctx, apiResp.JSON200.Items, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if apiResp.JSON200.RowCount > 0 {
		data.RowCount = types.Int64Value(apiResp.JSON200.RowCount)
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func mapWidgetsToModel(ctx context.Context, items []models.WidgetItem, data *widgetsDataSourceModel) diag.Diagnostics {
	var diags diag.Diagnostics

	if len(items) == 0 {
		emptyList, d := types.ListValueFrom(ctx, datasource_widgets.ItemsValue{}.Type(ctx), []datasource_widgets.ItemsValue{})
		diags.Append(d...)
		data.Items = emptyList
		data.RowCount = types.Int64Value(0)
		return diags
	}

	itemVals := make([]datasource_widgets.ItemsValue, 0, len(items))
	for _, item := range items {
		val, d := datasource_widgets.NewItemsValue(
			datasource_widgets.ItemsValue{}.AttributeTypes(ctx),
			map[string]attr.Value{
				"alias":       types.StringValue(item.Alias),
				"description": types.StringValue(item.Description),
				"id":          types.StringValue(item.Id),
				"kind":        types.StringValue(string(item.Kind)),
				"name":        types.StringValue(item.Name),
				"type":        types.StringValue(string(item.Type)),
			},
		)
		diags.Append(d...)
		if diags.HasError() {
			return diags
		}
		itemVals = append(itemVals, val)
	}

	itemsList, d := types.ListValueFrom(ctx, datasource_widgets.ItemsValue{}.Type(ctx), itemVals)
	diags.Append(d...)
	data.Items = itemsList
	data.RowCount = types.Int64Value(int64(len(items)))

	return diags
}
