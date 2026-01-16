package provider

import (
	"context"
	"fmt"

	"github.com/doitintl/terraform-provider-doit/internal/provider/datasource_dimensions"
	"github.com/doitintl/terraform-provider-doit/internal/provider/models"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var _ datasource.DataSource = (*dimensionsDataSource)(nil)

func NewDimensionsDataSource() datasource.DataSource {
	return &dimensionsDataSource{}
}

type dimensionsDataSource struct {
	client *models.ClientWithResponses
}

type dimensionsDataSourceModel struct {
	datasource_dimensions.DimensionsModel
}

func (d *dimensionsDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_dimensions"
}

func (d *dimensionsDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = datasource_dimensions.DimensionsDataSourceSchema(ctx)
}

func (d *dimensionsDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *dimensionsDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data dimensionsDataSourceModel

	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Build query parameters from optional inputs
	params := &models.ListDimensionsParams{}
	if !data.Filter.IsNull() && !data.Filter.IsUnknown() {
		filterVal := data.Filter.ValueString()
		params.Filter = &filterVal
	}
	if !data.MaxResults.IsNull() && !data.MaxResults.IsUnknown() {
		maxResultsVal := data.MaxResults.ValueString()
		params.MaxResults = &maxResultsVal
	}
	if !data.PageToken.IsNull() && !data.PageToken.IsUnknown() {
		pageTokenVal := data.PageToken.ValueString()
		params.PageToken = &pageTokenVal
	}
	if !data.SortBy.IsNull() && !data.SortBy.IsUnknown() {
		sortByVal := models.ListDimensionsParamsSortBy(data.SortBy.ValueString())
		params.SortBy = &sortByVal
	}
	if !data.SortOrder.IsNull() && !data.SortOrder.IsUnknown() {
		sortOrderVal := models.ListDimensionsParamsSortOrder(data.SortOrder.ValueString())
		params.SortOrder = &sortOrderVal
	}

	// Call the API to list dimensions
	apiResp, err := d.client.ListDimensionsWithResponse(ctx, params)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Reading Dimensions",
			fmt.Sprintf("Unable to read dimensions: %v", err),
		)
		return
	}

	if apiResp.StatusCode() != 200 || apiResp.JSON200 == nil {
		resp.Diagnostics.AddError(
			"Error Reading Dimensions",
			fmt.Sprintf("API returned status %d: %s", apiResp.StatusCode(), string(apiResp.Body)),
		)
		return
	}

	// Map API response to data source model
	result := apiResp.JSON200

	// Map dimensions list
	if result.Dimensions != nil && len(*result.Dimensions) > 0 {
		dimVals := make([]datasource_dimensions.DimensionsValue, 0, len(*result.Dimensions))
		for _, dim := range *result.Dimensions {
			dimVal, diags := datasource_dimensions.NewDimensionsValue(
				datasource_dimensions.DimensionsValue{}.AttributeTypes(ctx),
				map[string]attr.Value{
					"get_id": types.StringPointerValue(dim.GetID),
				},
			)
			resp.Diagnostics.Append(diags...)
			dimVals = append(dimVals, dimVal)
		}

		dimsList, diags := types.ListValueFrom(ctx, datasource_dimensions.DimensionsValue{}.Type(ctx), dimVals)
		resp.Diagnostics.Append(diags...)
		data.Dimensions = dimsList
	} else {
		data.Dimensions = types.ListNull(datasource_dimensions.DimensionsValue{}.Type(ctx))
	}

	// Map pagination fields
	data.PageToken = types.StringPointerValue(result.PageToken)
	if result.RowCount != nil {
		data.RowCount = types.Int64Value(*result.RowCount)
	} else {
		data.RowCount = types.Int64Null()
	}

	// Keep filter params as-is (they're input values)
	// Only set them if they were null/unknown before
	if data.Filter.IsUnknown() {
		data.Filter = types.StringNull()
	}
	if data.MaxResults.IsUnknown() {
		data.MaxResults = types.StringNull()
	}
	if data.SortBy.IsUnknown() {
		data.SortBy = types.StringNull()
	}
	if data.SortOrder.IsUnknown() {
		data.SortOrder = types.StringNull()
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
