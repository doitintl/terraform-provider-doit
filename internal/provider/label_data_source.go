package provider

import (
	"context"
	"fmt"

	"github.com/doitintl/terraform-provider-doit/internal/provider/datasource_label"
	"github.com/doitintl/terraform-provider-doit/internal/provider/models"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var _ datasource.DataSource = (*labelDataSource)(nil)
var _ datasource.DataSourceWithConfigure = (*labelDataSource)(nil)

func NewLabelDataSource() datasource.DataSource {
	return &labelDataSource{}
}

type (
	labelDataSource struct {
		client *models.ClientWithResponses
	}
	labelDataSourceModel struct {
		datasource_label.LabelModel
	}
)

func (d *labelDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_label"
}

func (d *labelDataSource) Schema(ctx context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = datasource_label.LabelDataSourceSchema(ctx)
}

func (d *labelDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}

	client, ok := req.ProviderData.(*models.ClientWithResponses)
	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Data Source Configure Type",
			fmt.Sprintf("Expected *models.ClientWithResponses, got: %T. Please report this issue to the provider developers.", req.ProviderData),
		)
		return
	}

	d.client = client
}

func (d *labelDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data labelDataSourceModel

	// Read Terraform configuration data into the model
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// If ID is unknown (depends on a resource not yet created), return early
	if data.Id.IsUnknown() {
		resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
		return
	}

	// Call API to get label
	labelResp, err := d.client.GetLabelWithResponse(ctx, data.Id.ValueString())
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Reading Label",
			"Could not read label ID "+data.Id.ValueString()+": "+err.Error(),
		)
		return
	}

	if labelResp.StatusCode() != 200 {
		resp.Diagnostics.AddError(
			"Error Reading Label",
			fmt.Sprintf("Could not read label ID %s, status: %d, body: %s", data.Id.ValueString(), labelResp.StatusCode(), string(labelResp.Body)),
		)
		return
	}

	if labelResp.JSON200 == nil {
		resp.Diagnostics.AddError(
			"Error Reading Label",
			"Received empty response body for label ID "+data.Id.ValueString(),
		)
		return
	}

	label := labelResp.JSON200

	// Map API response to model
	data.Id = types.StringValue(label.Id)
	data.Name = types.StringValue(label.Name)
	data.Color = types.StringValue(string(label.Color))

	if label.Type != nil {
		data.Type = types.StringValue(string(*label.Type))
	} else {
		data.Type = types.StringNull()
	}

	if label.CreateTime != nil {
		data.CreateTime = types.StringValue(label.CreateTime.UTC().Format("2006-01-02T15:04:05Z"))
	} else {
		data.CreateTime = types.StringNull()
	}

	if label.UpdateTime != nil {
		data.UpdateTime = types.StringValue(label.UpdateTime.UTC().Format("2006-01-02T15:04:05Z"))
	} else {
		data.UpdateTime = types.StringNull()
	}

	// Save data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
