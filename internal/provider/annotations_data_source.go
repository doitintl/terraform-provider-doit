package provider

import (
	"context"
	"fmt"

	"github.com/doitintl/terraform-provider-doit/internal/provider/datasource_annotations"
	"github.com/doitintl/terraform-provider-doit/internal/provider/models"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var _ datasource.DataSource = (*annotationsDataSource)(nil)

func NewAnnotationsDataSource() datasource.DataSource {
	return &annotationsDataSource{}
}

type annotationsDataSource struct {
	client *models.ClientWithResponses
}

type annotationsDataSourceModel struct {
	datasource_annotations.AnnotationsModel
}

func (d *annotationsDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_annotations"
}

func (d *annotationsDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = datasource_annotations.AnnotationsDataSourceSchema(ctx)
}

func (d *annotationsDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *annotationsDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data annotationsDataSourceModel

	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Build query parameters
	params := &models.ListAnnotationsParams{}
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
		sortBy := models.ListAnnotationsParamsSortBy(data.SortBy.ValueString())
		params.SortBy = &sortBy
	}
	if !data.SortOrder.IsNull() && !data.SortOrder.IsUnknown() {
		sortOrder := models.ListAnnotationsParamsSortOrder(data.SortOrder.ValueString())
		params.SortOrder = &sortOrder
	}

	apiResp, err := d.client.ListAnnotationsWithResponse(ctx, params)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Reading Annotations",
			fmt.Sprintf("Unable to read annotations: %v", err),
		)
		return
	}

	if apiResp.StatusCode() != 200 || apiResp.JSON200 == nil {
		resp.Diagnostics.AddError(
			"Error Reading Annotations",
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

	// Map annotations list
	if result.Annotations != nil && len(*result.Annotations) > 0 {
		annotationVals := make([]datasource_annotations.AnnotationsValue, 0, len(*result.Annotations))
		for _, ann := range *result.Annotations {
			// Handle time.Time to string conversion
			var createTimeVal, updateTimeVal types.String
			if ann.CreateTime != nil {
				createTimeVal = types.StringValue(ann.CreateTime.Format("2006-01-02T15:04:05Z"))
			} else {
				createTimeVal = types.StringNull()
			}
			if ann.UpdateTime != nil {
				updateTimeVal = types.StringValue(ann.UpdateTime.Format("2006-01-02T15:04:05Z"))
			} else {
				updateTimeVal = types.StringNull()
			}

			// Handle labels list
			labelsList, diags := mapAnnotationLabels(ctx, ann.Labels)
			resp.Diagnostics.Append(diags...)

			// Handle reports list ([]string)
			var reportsList types.List
			if ann.Reports != nil {
				reportVals := make([]attr.Value, 0, len(*ann.Reports))
				for _, r := range *ann.Reports {
					reportVals = append(reportVals, types.StringValue(r))
				}
				reportsList, diags = types.ListValue(types.StringType, reportVals)
				resp.Diagnostics.Append(diags...)
			} else {
				reportsList = types.ListNull(types.StringType)
			}

			annVal, diags := datasource_annotations.NewAnnotationsValue(
				datasource_annotations.AnnotationsValue{}.AttributeTypes(ctx),
				map[string]attr.Value{
					"id":          types.StringValue(ann.Id),
					"content":     types.StringValue(ann.Content),
					"timestamp":   types.StringValue(ann.Timestamp.Format("2006-01-02T15:04:05Z")),
					"create_time": createTimeVal,
					"update_time": updateTimeVal,
					"labels":      labelsList,
					"reports":     reportsList,
				},
			)
			resp.Diagnostics.Append(diags...)
			annotationVals = append(annotationVals, annVal)
		}

		annotationList, diags := types.ListValueFrom(ctx, datasource_annotations.AnnotationsValue{}.Type(ctx), annotationVals)
		resp.Diagnostics.Append(diags...)
		data.Annotations = annotationList
	} else {
		data.Annotations = types.ListNull(datasource_annotations.AnnotationsValue{}.Type(ctx))
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

func mapAnnotationLabels(ctx context.Context, labels *[]models.LabelInfo) (types.List, diag.Diagnostics) {
	if labels == nil || len(*labels) == 0 {
		return types.ListNull(datasource_annotations.LabelsValue{}.Type(ctx)), nil
	}

	vals := make([]datasource_annotations.LabelsValue, 0, len(*labels))
	for _, l := range *labels {
		val, diags := datasource_annotations.NewLabelsValue(
			datasource_annotations.LabelsValue{}.AttributeTypes(ctx),
			map[string]attr.Value{
				"id":   types.StringValue(l.Id),
				"name": types.StringValue(l.Name),
			},
		)
		if diags.HasError() {
			return types.ListNull(datasource_annotations.LabelsValue{}.Type(ctx)), diags
		}
		vals = append(vals, val)
	}

	return types.ListValueFrom(ctx, datasource_annotations.LabelsValue{}.Type(ctx), vals)
}
