package provider

import (
	"context"
	"fmt"

	"github.com/doitintl/terraform-provider-doit/internal/provider/datasource_labels"
	"github.com/doitintl/terraform-provider-doit/internal/provider/models"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var _ datasource.DataSource = (*labelsDataSource)(nil)

func NewLabelsDataSource() datasource.DataSource {
	return &labelsDataSource{}
}

type labelsDataSource struct {
	client *models.ClientWithResponses
}

type labelsDataSourceModel struct {
	datasource_labels.LabelsModel
}

func (d *labelsDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_labels"
}

func (d *labelsDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = datasource_labels.LabelsDataSourceSchema(ctx)
}

func (d *labelsDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *labelsDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data labelsDataSourceModel

	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Build query parameters
	params := &models.ListLabelsParams{}
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
	if !data.SortBy.IsNull() && !data.SortBy.IsUnknown() {
		sortBy := models.ListLabelsParamsSortBy(data.SortBy.ValueString())
		params.SortBy = &sortBy
	}
	if !data.SortOrder.IsNull() && !data.SortOrder.IsUnknown() {
		sortOrder := models.ListLabelsParamsSortOrder(data.SortOrder.ValueString())
		params.SortOrder = &sortOrder
	}

	apiResp, err := d.client.ListLabelsWithResponse(ctx, params)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Reading Labels",
			fmt.Sprintf("Unable to read labels: %v", err),
		)
		return
	}

	if apiResp.StatusCode() != 200 || apiResp.JSON200 == nil {
		resp.Diagnostics.AddError(
			"Error Reading Labels",
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

	// Map labels list
	if result.Labels != nil && len(*result.Labels) > 0 {
		labelVals := make([]datasource_labels.LabelsValue, 0, len(*result.Labels))
		for _, label := range *result.Labels {
			// Handle time.Time to string conversion
			var createTimeVal, updateTimeVal types.String
			if label.CreateTime != nil {
				createTimeVal = types.StringValue(label.CreateTime.Format("2006-01-02T15:04:05Z"))
			} else {
				createTimeVal = types.StringNull()
			}
			if label.UpdateTime != nil {
				updateTimeVal = types.StringValue(label.UpdateTime.Format("2006-01-02T15:04:05Z"))
			} else {
				updateTimeVal = types.StringNull()
			}

			// Handle optional Type enum
			var typeVal types.String
			if label.Type != nil {
				typeVal = types.StringValue(string(*label.Type))
			} else {
				typeVal = types.StringNull()
			}

			labelVal, diags := datasource_labels.NewLabelsValue(
				datasource_labels.LabelsValue{}.AttributeTypes(ctx),
				map[string]attr.Value{
					"id":          types.StringValue(label.Id),
					"name":        types.StringValue(label.Name),
					"color":       types.StringValue(string(label.Color)),
					"type":        typeVal,
					"create_time": createTimeVal,
					"update_time": updateTimeVal,
				},
			)
			resp.Diagnostics.Append(diags...)
			labelVals = append(labelVals, labelVal)
		}

		labelList, diags := types.ListValueFrom(ctx, datasource_labels.LabelsValue{}.Type(ctx), labelVals)
		resp.Diagnostics.Append(diags...)
		data.Labels = labelList
	} else {
		data.Labels = types.ListNull(datasource_labels.LabelsValue{}.Type(ctx))
	}

	// Set optional filter params to null if they were unknown
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
