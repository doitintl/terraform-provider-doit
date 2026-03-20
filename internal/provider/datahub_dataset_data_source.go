package provider

import (
	"context"
	"fmt"

	"github.com/doitintl/terraform-provider-doit/internal/provider/datasource_datahub_dataset"
	"github.com/doitintl/terraform-provider-doit/internal/provider/models"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var _ datasource.DataSource = (*datahubDatasetDataSource)(nil)
var _ datasource.DataSourceWithConfigure = (*datahubDatasetDataSource)(nil)

func NewDatahubDatasetDataSource() datasource.DataSource {
	return &datahubDatasetDataSource{}
}

type datahubDatasetDataSource struct {
	client *models.ClientWithResponses
}

func (ds *datahubDatasetDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_datahub_dataset"
}

func (ds *datahubDatasetDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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
	ds.client = client
}

func (ds *datahubDatasetDataSource) Schema(ctx context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = datasource_datahub_dataset.DatahubDatasetDataSourceSchema(ctx)
	resp.Schema.Description = "Retrieves a specific DataHub dataset by name."
	resp.Schema.MarkdownDescription = resp.Schema.Description
}

func (ds *datahubDatasetDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var state datasource_datahub_dataset.DatahubDatasetModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// If name is unknown (depends on a resource not yet created), set all computed
	// attributes to unknown so consumers don't treat null as a real value during planning.
	if state.Name.IsUnknown() {
		state.Description = types.StringUnknown()
		state.Records = types.Int64Unknown()
		state.UpdatedBy = types.StringUnknown()
		state.LastUpdated = types.StringUnknown()
		resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
		return
	}

	name := state.Name.ValueString()
	datasetResp, err := ds.client.GetDatahubDatasetWithResponse(ctx, name)
	if err != nil {
		resp.Diagnostics.AddError("Error reading DataHub dataset", err.Error())
		return
	}
	if datasetResp.StatusCode() == 404 {
		resp.Diagnostics.AddError("DataHub dataset not found", fmt.Sprintf("Dataset with name %q not found", name))
		return
	}
	if datasetResp.StatusCode() != 200 || datasetResp.JSON200 == nil {
		resp.Diagnostics.AddError(
			"Error reading DataHub dataset",
			fmt.Sprintf("Unexpected status: %d, body: %s", datasetResp.StatusCode(), string(datasetResp.Body)),
		)
		return
	}

	dataset := datasetResp.JSON200

	state.Name = types.StringPointerValue(dataset.Name)
	state.Description = types.StringPointerValue(dataset.Description)
	state.Records = types.Int64PointerValue(dataset.Records)
	state.UpdatedBy = types.StringPointerValue(dataset.UpdatedBy)
	state.LastUpdated = types.StringPointerValue(dataset.LastUpdated)

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}
