package provider

import (
	"context"
	"fmt"

	"github.com/doitintl/terraform-provider-doit/internal/provider/datasource_assets"
	"github.com/doitintl/terraform-provider-doit/internal/provider/models"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var _ datasource.DataSource = (*assetsDataSource)(nil)

func NewAssetsDataSource() datasource.DataSource {
	return &assetsDataSource{}
}

type assetsDataSource struct {
	client *models.ClientWithResponses
}

type assetsDataSourceModel struct {
	datasource_assets.AssetsModel
}

func (d *assetsDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_assets"
}

func (d *assetsDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = datasource_assets.AssetsDataSourceSchema(ctx)
}

func (d *assetsDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *assetsDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data assetsDataSourceModel

	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Build query parameters
	params := &models.IdOfAssetsParams{}
	if !data.Filter.IsNull() && !data.Filter.IsUnknown() {
		filter := data.Filter.ValueString()
		params.Filter = &filter
	}

	// Auto-paginate: fetch all pages
	var allAssets []models.AssetItem

	for {
		apiResp, err := d.client.IdOfAssetsWithResponse(ctx, params)
		if err != nil {
			resp.Diagnostics.AddError(
				"Error Reading Assets",
				fmt.Sprintf("Unable to read assets: %v", err),
			)
			return
		}

		if apiResp.StatusCode() != 200 || apiResp.JSON200 == nil {
			resp.Diagnostics.AddError(
				"Error Reading Assets",
				fmt.Sprintf("API returned status %d: %s", apiResp.StatusCode(), string(apiResp.Body)),
			)
			return
		}

		result := apiResp.JSON200

		// Accumulate assets
		if result.Assets != nil {
			allAssets = append(allAssets, *result.Assets...)
		}

		// Check for next page
		if result.PageToken == nil || *result.PageToken == "" {
			break
		}
		params.PageToken = result.PageToken
	}

	// Set row count to total accumulated
	data.RowCount = types.Int64Value(int64(len(allAssets)))

	// Page token is always null after auto-pagination
	data.PageToken = types.StringNull()

	// Ignore max_results input
	data.MaxResults = types.Int64Null()

	// Map assets list
	if len(allAssets) > 0 {
		assetVals := make([]datasource_assets.AssetsValue, 0, len(allAssets))
		for _, asset := range allAssets {
			assetVal, diags := datasource_assets.NewAssetsValue(
				datasource_assets.AssetsValue{}.AttributeTypes(ctx),
				map[string]attr.Value{
					"id":            types.StringPointerValue(asset.Id),
					"name":          types.StringPointerValue(asset.Name),
					"type":          types.StringPointerValue(asset.Type),
					"url":           types.StringPointerValue(asset.Url),
					"quantity":      types.Int64PointerValue(asset.Quantity),
					"create_time":   types.Int64PointerValue(asset.CreateTime),
					"used_licenses": types.Int64PointerValue(asset.UsedLicenses),
				},
			)
			resp.Diagnostics.Append(diags...)
			assetVals = append(assetVals, assetVal)
		}

		assetList, diags := types.ListValueFrom(ctx, datasource_assets.AssetsValue{}.Type(ctx), assetVals)
		resp.Diagnostics.Append(diags...)
		data.Assets = assetList
	} else {
		data.Assets = types.ListNull(datasource_assets.AssetsValue{}.Type(ctx))
	}

	// Set optional filter param to null if it was unknown
	if data.Filter.IsUnknown() {
		data.Filter = types.StringNull()
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
