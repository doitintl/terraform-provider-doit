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
	if !data.SortBy.IsNull() && !data.SortBy.IsUnknown() {
		sortBy := models.ListAnnotationsParamsSortBy(data.SortBy.ValueString())
		params.SortBy = &sortBy
	}
	if !data.SortOrder.IsNull() && !data.SortOrder.IsUnknown() {
		sortOrder := models.ListAnnotationsParamsSortOrder(data.SortOrder.ValueString())
		params.SortOrder = &sortOrder
	}

	// Smart pagination: honor user-provided values, otherwise auto-paginate
	userControlsPagination := !data.MaxResults.IsNull() && !data.MaxResults.IsUnknown()

	var allAnnotations []models.AnnotationListItem

	if userControlsPagination {
		// Manual mode: single API call with user's params
		maxResultsVal := data.MaxResults.ValueString()
		params.MaxResults = &maxResultsVal
		if !data.PageToken.IsNull() && !data.PageToken.IsUnknown() {
			pageTokenVal := data.PageToken.ValueString()
			params.PageToken = &pageTokenVal
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
		if result.Annotations != nil {
			allAnnotations = *result.Annotations
		}

		// Preserve API's page_token for user to fetch next page
		data.PageToken = types.StringPointerValue(result.PageToken)
		if result.RowCount != nil {
			data.RowCount = types.Int64Value(int64(*result.RowCount))
		} else {
			data.RowCount = types.Int64Value(int64(len(allAnnotations)))
		}
		// max_results is already set by user, no change needed
	} else {
		// Auto mode: fetch all pages
		for {
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
			if result.Annotations != nil {
				allAnnotations = append(allAnnotations, *result.Annotations...)
			}

			if result.PageToken == nil || *result.PageToken == "" {
				break
			}
			params.PageToken = result.PageToken
		}

		// Auto mode: set counts based on what we fetched
		data.RowCount = types.Int64Value(int64(len(allAnnotations)))
		data.PageToken = types.StringNull()
		// max_results was not set; preserve null/unknown handling below
		if data.MaxResults.IsUnknown() {
			data.MaxResults = types.StringNull()
		}
	}

	// Map annotations list
	if len(allAnnotations) > 0 {
		annotationVals := make([]datasource_annotations.AnnotationsValue, 0, len(allAnnotations))
		for _, annotation := range allAnnotations {
			// Map nested labels
			labelsList := mapAnnotationLabels(ctx, annotation.Labels, &resp.Diagnostics)

			// Handle reports list
			var reportsList types.List
			if annotation.Reports != nil {
				reportVals := make([]attr.Value, 0, len(*annotation.Reports))
				for _, r := range *annotation.Reports {
					reportVals = append(reportVals, types.StringValue(r))
				}
				var diags diag.Diagnostics
				reportsList, diags = types.ListValue(types.StringType, reportVals)
				resp.Diagnostics.Append(diags...)
			} else {
				reportsList = types.ListNull(types.StringType)
			}

			// Handle time.Time to string conversion
			var createTimeVal, updateTimeVal types.String
			if annotation.CreateTime != nil {
				createTimeVal = types.StringValue(annotation.CreateTime.Format("2006-01-02T15:04:05Z07:00"))
			} else {
				createTimeVal = types.StringNull()
			}
			if annotation.UpdateTime != nil {
				updateTimeVal = types.StringValue(annotation.UpdateTime.Format("2006-01-02T15:04:05Z07:00"))
			} else {
				updateTimeVal = types.StringNull()
			}

			annotationVal, diags := datasource_annotations.NewAnnotationsValue(
				datasource_annotations.AnnotationsValue{}.AttributeTypes(ctx),
				map[string]attr.Value{
					"id":          types.StringValue(annotation.Id),
					"content":     types.StringValue(annotation.Content),
					"timestamp":   types.StringValue(annotation.Timestamp.Format("2006-01-02T15:04:05Z07:00")),
					"labels":      labelsList,
					"reports":     reportsList,
					"create_time": createTimeVal,
					"update_time": updateTimeVal,
				},
			)
			resp.Diagnostics.Append(diags...)
			annotationVals = append(annotationVals, annotationVal)
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
	if data.SortBy.IsUnknown() {
		data.SortBy = types.StringNull()
	}
	if data.SortOrder.IsUnknown() {
		data.SortOrder = types.StringNull()
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

// mapAnnotationLabels maps API LabelInfo slice to Terraform list.
func mapAnnotationLabels(ctx context.Context, labels *[]models.LabelInfo, diagnostics *diag.Diagnostics) types.List {
	if labels == nil || len(*labels) == 0 {
		return types.ListNull(datasource_annotations.LabelsValue{}.Type(ctx))
	}

	vals := make([]datasource_annotations.LabelsValue, 0, len(*labels))
	for _, label := range *labels {
		labelVal, diags := datasource_annotations.NewLabelsValue(
			datasource_annotations.LabelsValue{}.AttributeTypes(ctx),
			map[string]attr.Value{
				"id":   types.StringValue(label.Id),
				"name": types.StringValue(label.Name),
			},
		)
		diagnostics.Append(diags...)
		vals = append(vals, labelVal)
	}

	list, diags := types.ListValueFrom(ctx, datasource_annotations.LabelsValue{}.Type(ctx), vals)
	diagnostics.Append(diags...)
	return list
}
