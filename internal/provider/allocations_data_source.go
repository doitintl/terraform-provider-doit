package provider

import (
	"context"
	"fmt"

	"github.com/doitintl/terraform-provider-doit/internal/provider/datasource_allocations"
	"github.com/doitintl/terraform-provider-doit/internal/provider/models"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var _ datasource.DataSource = (*allocationsDataSource)(nil)

func NewAllocationsDataSource() datasource.DataSource {
	return &allocationsDataSource{}
}

type allocationsDataSource struct {
	client *models.ClientWithResponses
}

type allocationsDataSourceModel struct {
	datasource_allocations.AllocationsModel
}

func (d *allocationsDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_allocations"
}

func (d *allocationsDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = datasource_allocations.AllocationsDataSourceSchema(ctx)
}

func (d *allocationsDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *allocationsDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data allocationsDataSourceModel

	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Build query parameters
	params := &models.ListAllocationsParams{}
	if !data.Filter.IsNull() && !data.Filter.IsUnknown() {
		filter := data.Filter.ValueString()
		params.Filter = &filter
	}
	if !data.MaxResults.IsNull() && !data.MaxResults.IsUnknown() {
		maxResults := data.MaxResults.ValueString()
		params.MaxResults = &maxResults
	}
	if !data.PageToken.IsNull() && !data.PageToken.IsUnknown() {
		pageToken := data.PageToken.ValueString()
		params.PageToken = &pageToken
	}

	apiResp, err := d.client.ListAllocationsWithResponse(ctx, params)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Reading Allocations",
			fmt.Sprintf("Unable to read allocations: %v", err),
		)
		return
	}

	if apiResp.StatusCode() != 200 || apiResp.JSON200 == nil {
		resp.Diagnostics.AddError(
			"Error Reading Allocations",
			fmt.Sprintf("API returned status %d: %s", apiResp.StatusCode(), string(apiResp.Body)),
		)
		return
	}

	result := apiResp.JSON200

	// Map row count
	if result.RowCount != nil {
		data.RowCount = types.Int64Value(int64(*result.RowCount))
	} else {
		data.RowCount = types.Int64Null()
	}

	// Map page token
	if result.PageToken != nil {
		data.PageToken = types.StringValue(*result.PageToken)
	} else if data.PageToken.IsUnknown() {
		data.PageToken = types.StringNull()
	}

	// Map allocations list
	if result.Allocations != nil && len(*result.Allocations) > 0 {
		allocationVals := make([]datasource_allocations.AllocationsValue, 0, len(*result.Allocations))
		for _, alloc := range *result.Allocations {
			// Handle enum type
			var allocTypeVal types.String
			if alloc.AllocationType != nil {
				allocTypeVal = types.StringValue(string(*alloc.AllocationType))
			} else {
				allocTypeVal = types.StringNull()
			}

			allocVal, diags := datasource_allocations.NewAllocationsValue(
				datasource_allocations.AllocationsValue{}.AttributeTypes(ctx),
				map[string]attr.Value{
					"id":              types.StringPointerValue(alloc.Id),
					"name":            types.StringPointerValue(alloc.Name),
					"description":     types.StringPointerValue(alloc.Description),
					"owner":           types.StringPointerValue(alloc.Owner),
					"type":            types.StringPointerValue(alloc.Type),
					"allocation_type": allocTypeVal,
					"create_time":     types.Int64PointerValue(alloc.CreateTime),
					"update_time":     types.Int64PointerValue(alloc.UpdateTime),
					"url_ui":          types.StringPointerValue(alloc.UrlUI),
				},
			)
			resp.Diagnostics.Append(diags...)
			allocationVals = append(allocationVals, allocVal)
		}

		allocationList, diags := types.ListValueFrom(ctx, datasource_allocations.AllocationsValue{}.Type(ctx), allocationVals)
		resp.Diagnostics.Append(diags...)
		data.Allocations = allocationList
	} else {
		data.Allocations = types.ListNull(datasource_allocations.AllocationsValue{}.Type(ctx))
	}

	// Set optional filter params to null if they were unknown
	if data.Filter.IsUnknown() {
		data.Filter = types.StringNull()
	}
	if data.MaxResults.IsUnknown() {
		data.MaxResults = types.StringNull()
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
