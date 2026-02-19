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

	// If any filter/pagination input is unknown, return unknown list
	if data.Filter.IsUnknown() || data.MaxResults.IsUnknown() || data.PageToken.IsUnknown() {
		data.Assets = types.ListUnknown(datasource_assets.AssetsValue{}.Type(ctx))
		resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
		return
	}
	// Build query parameters
	params := &models.IdOfAssetsParams{}
	if !data.Filter.IsNull() {
		filter := data.Filter.ValueString()
		params.Filter = &filter
	}

	// Smart pagination: honor user-provided values, otherwise auto-paginate
	userControlsPagination := !data.MaxResults.IsNull()

	var allAssets []models.AssetItem

	if userControlsPagination {
		// Manual mode: single API call with user's params
		maxResultsVal := data.MaxResults.ValueInt64()
		params.MaxResults = &maxResultsVal
		if !data.PageToken.IsNull() {
			pageTokenVal := data.PageToken.ValueString()
			params.PageToken = &pageTokenVal
		}

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
		if result.Assets != nil {
			allAssets = *result.Assets
		}

		// Preserve API's page_token for user to fetch next page
		data.PageToken = types.StringPointerValue(result.PageToken)
		if result.RowCount != nil {
			data.RowCount = types.Int64Value(*result.RowCount)
		} else {
			data.RowCount = types.Int64Value(int64(len(allAssets)))
		}
		// max_results is already set by user, no change needed
	} else {
		// Auto mode: fetch all pages, honoring user-provided page_token as starting point
		if !data.PageToken.IsNull() {
			pageTokenVal := data.PageToken.ValueString()
			params.PageToken = &pageTokenVal
		}
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
			if result.Assets != nil {
				allAssets = append(allAssets, *result.Assets...)
			}

			if result.PageToken == nil || *result.PageToken == "" {
				break
			}
			params.PageToken = result.PageToken
		}

		// Auto mode: set counts based on what we fetched
		data.RowCount = types.Int64Value(int64(len(allAssets)))
		data.PageToken = types.StringNull()
		// max_results was not set; preserve null
	}

	// Map assets list
	if len(allAssets) > 0 {
		assetVals := make([]datasource_assets.AssetsValue, 0, len(allAssets))
		for _, asset := range allAssets {
			assetVal, diags := datasource_assets.NewAssetsValue(
				datasource_assets.AssetsValue{}.AttributeTypes(ctx),
				map[string]attr.Value{
					"id":          types.StringPointerValue(asset.Id),
					"name":        types.StringPointerValue(asset.Name),
					"type":        types.StringPointerValue(asset.Type),
					"url":         types.StringPointerValue(asset.Url),
					"quantity":    types.Int64PointerValue(asset.Quantity),
					"create_time": types.Int64PointerValue(asset.CreateTime),
				},
			)
			resp.Diagnostics.Append(diags...)
			assetVals = append(assetVals, assetVal)
		}

		assetList, diags := types.ListValueFrom(ctx, datasource_assets.AssetsValue{}.Type(ctx), assetVals)
		resp.Diagnostics.Append(diags...)
		data.Assets = assetList
	} else {
		emptyList, diags := types.ListValueFrom(ctx, datasource_assets.AssetsValue{}.Type(ctx), []datasource_assets.AssetsValue{})
		resp.Diagnostics.Append(diags...)
		data.Assets = emptyList
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
