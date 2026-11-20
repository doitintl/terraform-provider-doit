package provider

import (
	"context"
	"fmt"

	"github.com/doitintl/terraform-provider-doit/internal/provider/datasource_custom_metric"
	"github.com/doitintl/terraform-provider-doit/internal/provider/models"
	"github.com/hashicorp/terraform-plugin-framework-timeouts/datasource/timeouts"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var _ datasource.DataSource = (*customMetricDataSource)(nil)
var _ datasource.DataSourceWithConfigure = (*customMetricDataSource)(nil)

func NewCustomMetricDataSource() datasource.DataSource {
	return &customMetricDataSource{}
}

type customMetricDataSource struct{ client *models.ClientWithResponses }

type customMetricDataSourceModel struct {
	datasource_custom_metric.CustomMetricModel
	Timeouts timeouts.Value `tfsdk:"timeouts"`
}

func (d *customMetricDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_custom_metric"
}

func (d *customMetricDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	client, ok := req.ProviderData.(*models.ClientWithResponses)
	if !ok {
		resp.Diagnostics.AddError("Unexpected Data Source Configure Type", fmt.Sprintf("Expected *models.ClientWithResponses, got: %T", req.ProviderData))
		return
	}
	d.client = client
}

func (d *customMetricDataSource) Schema(ctx context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	s := datasource_custom_metric.CustomMetricDataSourceSchema(ctx)

	s.Description = "A custom metric: a formula over the metrics of up to five single allocations. `decimals` is null when the metric has no display precision, and the `letter` of each variable (`A` to `E`) follows its position. The ID of a preset metric is not found. Requires the calculated metrics entitlement."
	s.MarkdownDescription = s.Description

	s.Attributes["timeouts"] = timeouts.Attributes(ctx)
	resp.Schema = s
}

func (d *customMetricDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data customMetricDataSourceModel
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

	if data.Id.IsUnknown() {
		data.Etag = types.StringUnknown()
		data.Name = types.StringUnknown()
		data.Description = types.StringUnknown()
		data.Owner = types.StringUnknown()
		data.Formula = types.StringUnknown()
		data.UnitType = types.StringUnknown()
		data.CreateTime = types.StringUnknown()
		data.UpdateTime = types.StringUnknown()
		data.Decimals = datasource_custom_metric.NewDecimalsValueUnknown()
		data.Variables = types.ListUnknown(datasource_custom_metric.VariablesValue{}.Type(ctx))
		resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
		return
	}

	id := data.Id.ValueString()

	apiResp, err := d.client.GetCustomMetricWithResponse(ctx, id, &models.GetCustomMetricParams{})
	if err != nil {
		resp.Diagnostics.AddError("Error Reading Custom Metric", fmt.Sprintf("Could not read custom metric %s: %v", id, err))
		return
	}
	if apiResp.StatusCode() != 200 || apiResp.JSON200 == nil {
		resp.Diagnostics.AddError("Error Reading Custom Metric", fmt.Sprintf("Custom metric %s: status %d, body: %s", id, apiResp.StatusCode(), string(apiResp.Body)))
		return
	}

	if got := apiResp.JSON200.Id; got == nil || *got != id {
		returned := ""
		if got != nil {
			returned = *got
		}
		resp.Diagnostics.AddError("Invalid Custom Metric Response", fmt.Sprintf("Requested custom metric %s but API returned ID %q", id, returned))
		return
	}

	resp.Diagnostics.Append(mapCustomMetricToDataSource(ctx, apiResp.JSON200, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
