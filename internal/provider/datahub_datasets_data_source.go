package provider

import (
	"context"
	"fmt"

	"github.com/doitintl/terraform-provider-doit/internal/provider/datasource_datahub_datasets"
	"github.com/doitintl/terraform-provider-doit/internal/provider/models"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var _ datasource.DataSource = (*datahubDatasetsDataSource)(nil)
var _ datasource.DataSourceWithConfigure = (*datahubDatasetsDataSource)(nil)

func NewDatahubDatasetsDataSource() datasource.DataSource {
	return &datahubDatasetsDataSource{}
}

type datahubDatasetsDataSource struct {
	client *models.ClientWithResponses
}

func (d *datahubDatasetsDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_datahub_datasets"
}

func (d *datahubDatasetsDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	client, ok := req.ProviderData.(*models.ClientWithResponses)
	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Data Source Configure Type",
			fmt.Sprintf("Expected *models.ClientWithResponses, got: %T.", req.ProviderData),
		)
		return
	}
	d.client = client
}

func (d *datahubDatasetsDataSource) Schema(ctx context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = datasource_datahub_datasets.DatahubDatasetsDataSourceSchema(ctx)
}

func (d *datahubDatasetsDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data datasource_datahub_datasets.DatahubDatasetsModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// The list endpoint has no parameters — no pagination, no filters.
	apiResp, err := d.client.ListDatahubDatasetsWithResponse(ctx)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Reading DataHub Datasets",
			fmt.Sprintf("Unable to read DataHub datasets: %v", err),
		)
		return
	}

	if apiResp.StatusCode() != 200 || apiResp.JSON200 == nil {
		resp.Diagnostics.AddError(
			"Error Reading DataHub Datasets",
			fmt.Sprintf("API returned status %d: %s", apiResp.StatusCode(), string(apiResp.Body)),
		)
		return
	}

	result := apiResp.JSON200
	var allDatasets []models.ListDatahubDatasets200ResponseDatasetsItem
	if result.Datasets != nil {
		allDatasets = *result.Datasets
	}

	// Map datasets list
	if len(allDatasets) > 0 {
		datasetVals := make([]datasource_datahub_datasets.DatasetsValue, 0, len(allDatasets))
		for _, ds := range allDatasets {
			dsVal, diags := datasource_datahub_datasets.NewDatasetsValue(
				datasource_datahub_datasets.DatasetsValue{}.AttributeTypes(ctx),
				map[string]attr.Value{
					"name":         types.StringPointerValue(ds.Name),
					"description":  types.StringPointerValue(ds.Description),
					"records":      types.Int64PointerValue(ds.Records),
					"updated_by":   types.StringPointerValue(ds.UpdatedBy),
					"last_updated": types.StringPointerValue(ds.LastUpdated),
				},
			)
			resp.Diagnostics.Append(diags...)
			datasetVals = append(datasetVals, dsVal)
		}

		datasetList, diags := types.ListValueFrom(ctx, datasource_datahub_datasets.DatasetsValue{}.Type(ctx), datasetVals)
		resp.Diagnostics.Append(diags...)
		data.Datasets = datasetList
	} else {
		emptyList, diags := types.ListValueFrom(ctx, datasource_datahub_datasets.DatasetsValue{}.Type(ctx), []datasource_datahub_datasets.DatasetsValue{})
		resp.Diagnostics.Append(diags...)
		data.Datasets = emptyList
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
