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

	// Auto-paginate: fetch all pages
	var allAllocations []models.AllocationListItem

	for {
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

		// Accumulate allocations
		if result.Allocations != nil {
			allAllocations = append(allAllocations, *result.Allocations...)
		}

		// Check for next page
		if result.PageToken == nil || *result.PageToken == "" {
			break
		}
		params.PageToken = result.PageToken
	}

	// Set row count to total accumulated
	data.RowCount = types.Int64Value(int64(len(allAllocations)))

	// Page token is always null after auto-pagination
	data.PageToken = types.StringNull()

	// Ignore max_results input
	data.MaxResults = types.StringNull()

	// Map allocations list
	if len(allAllocations) > 0 {
		allocationVals := make([]datasource_allocations.AllocationsValue, 0, len(allAllocations))
		for _, alloc := range allAllocations {
			// Handle optional AllocationType enum
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

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
