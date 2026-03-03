package provider

import (
	"context"
	"fmt"

	"github.com/doitintl/terraform-provider-doit/internal/provider/datasource_asset"
	"github.com/doitintl/terraform-provider-doit/internal/provider/models"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var _ datasource.DataSource = (*assetDataSource)(nil)
var _ datasource.DataSourceWithConfigure = (*assetDataSource)(nil)

func NewAssetDataSource() datasource.DataSource {
	return &assetDataSource{}
}

type assetDataSource struct {
	client *models.ClientWithResponses
}

func (ds *assetDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_asset"
}

func (ds *assetDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (ds *assetDataSource) Schema(ctx context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = datasource_asset.AssetDataSourceSchema(ctx)
}

func (ds *assetDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var state datasource_asset.AssetModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// If ID is unknown (depends on a resource not yet created), return early
	if state.Id.IsUnknown() {
		resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
		return
	}

	id := state.Id.ValueString()
	assetResp, err := ds.client.GetAssetWithResponse(ctx, id)
	if err != nil {
		resp.Diagnostics.AddError("Error reading asset", err.Error())
		return
	}
	if assetResp.StatusCode() == 404 {
		resp.Diagnostics.AddError("Asset not found", fmt.Sprintf("Asset with ID %s not found", id))
		return
	}
	if assetResp.StatusCode() != 200 || assetResp.JSON200 == nil {
		resp.Diagnostics.AddError(
			"Error reading asset",
			fmt.Sprintf("Unexpected status: %d, body: %s", assetResp.StatusCode(), string(assetResp.Body)),
		)
		return
	}

	asset := assetResp.JSON200

	state.Id = types.StringPointerValue(asset.Id)
	state.Name = types.StringPointerValue(asset.Name)
	state.Type = types.StringPointerValue(asset.Type)
	state.Url = types.StringPointerValue(asset.Url)
	state.Quantity = types.Int64PointerValue(asset.Quantity)
	state.CreateTime = types.Int64PointerValue(asset.CreateTime)

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}
