package provider

import (
	"context"
	"fmt"
	"time"

	"github.com/doitintl/terraform-provider-doit/internal/provider/datasource_insights"
	"github.com/doitintl/terraform-provider-doit/internal/provider/models"
	"github.com/hashicorp/terraform-plugin-framework-timeouts/datasource/timeouts"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var _ datasource.DataSource = (*insightsDataSource)(nil)
var _ datasource.DataSourceWithConfigure = (*insightsDataSource)(nil)

func NewInsightsDataSource() datasource.DataSource {
	return &insightsDataSource{}
}

type insightsDataSource struct {
	client *models.ClientWithResponses
}

type insightsDataSourceModel struct {
	datasource_insights.InsightsModel
	Timeouts timeouts.Value `tfsdk:"timeouts"`
}

func (ds *insightsDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_insights"
}

func (ds *insightsDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (ds *insightsDataSource) Schema(ctx context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	s := datasource_insights.InsightsDataSourceSchema(ctx)
	s.Attributes["timeouts"] = timeouts.Attributes(ctx)
	resp.Schema = s
}

func (ds *insightsDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data insightsDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	readTimeout, diags := data.Timeouts.Read(ctx, 2*time.Minute)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	ctx, cancel := context.WithTimeout(ctx, readTimeout)
	defer cancel()

	// Composite inputs (lists) require IsFullyKnown to catch unknown elements.
	if !req.Config.Raw.IsFullyKnown() {
		data.Results = types.ListUnknown(datasource_insights.ResultsValue{}.Type(ctx))
		data.Pagination = datasource_insights.NewPaginationValueUnknown()
		resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
		return
	}

	// Build query parameters from user-provided filter fields
	params := &models.GetInsightResultsParams{}

	if !data.SearchTerm.IsNull() {
		params.SearchTerm = new(data.SearchTerm.ValueString())
	}
	if !data.Category.IsNull() {
		params.Category = new(models.GetInsightResultsParamsCategory(data.Category.ValueString()))
	}
	if !data.CloudProvider.IsNull() {
		params.CloudProvider = new(data.CloudProvider.ValueString())
	}
	if !data.EasyWin.IsNull() {
		params.EasyWin = new(data.EasyWin.ValueBool())
	}
	if !data.CloudFlows.IsNull() {
		params.CloudFlows = new(data.CloudFlows.ValueBool())
	}

	// List filters: display_status, priority, source, tag
	if !data.DisplayStatus.IsNull() {
		var statuses []string
		resp.Diagnostics.Append(data.DisplayStatus.ElementsAs(ctx, &statuses, false)...)
		if resp.Diagnostics.HasError() {
			return
		}
		dsStatuses := make([]models.GetInsightResultsParamsDisplayStatus, len(statuses))
		for i, s := range statuses {
			dsStatuses[i] = models.GetInsightResultsParamsDisplayStatus(s)
		}
		params.DisplayStatus = &dsStatuses
	}
	if !data.Priority.IsNull() {
		var priorities []string
		resp.Diagnostics.Append(data.Priority.ElementsAs(ctx, &priorities, false)...)
		if resp.Diagnostics.HasError() {
			return
		}
		pPriorities := make([]models.GetInsightResultsParamsPriority, len(priorities))
		for i, p := range priorities {
			pPriorities[i] = models.GetInsightResultsParamsPriority(p)
		}
		params.Priority = &pPriorities
	}
	if !data.Source.IsNull() {
		var sources []string
		resp.Diagnostics.Append(data.Source.ElementsAs(ctx, &sources, false)...)
		if resp.Diagnostics.HasError() {
			return
		}
		params.Source = &sources
	}
	if !data.Tag.IsNull() {
		var tags []string
		resp.Diagnostics.Append(data.Tag.ElementsAs(ctx, &tags, false)...)
		if resp.Diagnostics.HasError() {
			return
		}
		params.Tag = &tags
	}

	// Smart pagination: honor user-provided values, otherwise auto-paginate
	userControlsPagination := !data.MaxResults.IsNull()

	var allInsights []models.InsightResponse

	if userControlsPagination {
		// Manual mode: single API call with user's params
		params.MaxResults = new(int(data.MaxResults.ValueInt64()))
		if !data.PageToken.IsNull() {
			params.PageToken = new(data.PageToken.ValueString())
		}

		apiResp, err := ds.client.GetInsightResultsWithResponse(ctx, params)
		if err != nil {
			resp.Diagnostics.AddError(
				"Error Reading Insights",
				fmt.Sprintf("Unable to read insights: %v", err),
			)
			return
		}
		if apiResp.StatusCode() != 200 || apiResp.JSON200 == nil {
			resp.Diagnostics.AddError(
				"Error Reading Insights",
				fmt.Sprintf("API returned status %d: %s", apiResp.StatusCode(), string(apiResp.Body)),
			)
			return
		}

		result := apiResp.JSON200
		if result.Results != nil {
			allInsights = *result.Results
		}

		// Populate pagination from response
		if result.Pagination != nil {
			paginationVal, pDiags := datasource_insights.NewPaginationValue(
				datasource_insights.PaginationValue{}.AttributeTypes(ctx),
				map[string]attr.Value{
					"page_token": types.StringPointerValue(result.Pagination.PageToken),
					"row_count":  types.Int64Value(int64(result.Pagination.RowCount)),
				},
			)
			resp.Diagnostics.Append(pDiags...)
			data.Pagination = paginationVal
		} else {
			data.Pagination = datasource_insights.NewPaginationValueNull()
		}
	} else {
		// Auto mode: fetch all pages
		if !data.PageToken.IsNull() {
			params.PageToken = new(data.PageToken.ValueString())
		}
		for {
			apiResp, err := ds.client.GetInsightResultsWithResponse(ctx, params)
			if err != nil {
				resp.Diagnostics.AddError(
					"Error Reading Insights",
					fmt.Sprintf("Unable to read insights: %v", err),
				)
				return
			}
			if apiResp.StatusCode() != 200 || apiResp.JSON200 == nil {
				resp.Diagnostics.AddError(
					"Error Reading Insights",
					fmt.Sprintf("API returned status %d: %s", apiResp.StatusCode(), string(apiResp.Body)),
				)
				return
			}

			result := apiResp.JSON200
			if result.Results != nil {
				allInsights = append(allInsights, *result.Results...)
			}

			if result.Pagination == nil || result.Pagination.PageToken == nil || *result.Pagination.PageToken == "" {
				break
			}
			params.PageToken = result.Pagination.PageToken
		}

		// Auto mode: set pagination with total count, no next token
		paginationVal, pDiags := datasource_insights.NewPaginationValue(
			datasource_insights.PaginationValue{}.AttributeTypes(ctx),
			map[string]attr.Value{
				"page_token": types.StringNull(),
				"row_count":  types.Int64Value(int64(len(allInsights))),
			},
		)
		resp.Diagnostics.Append(pDiags...)
		data.Pagination = paginationVal
	}

	// Map insights to Terraform model
	resp.Diagnostics.Append(mapInsightsListToModel(ctx, allInsights, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

// mapInsightsListToModel maps a slice of InsightResponse to the list model's Results field.
func mapInsightsListToModel(ctx context.Context, insights []models.InsightResponse, data *insightsDataSourceModel) (diags diag.Diagnostics) {
	if len(insights) == 0 {
		emptyList, d := types.ListValueFrom(ctx, datasource_insights.ResultsValue{}.Type(ctx), []datasource_insights.ResultsValue{})
		diags.Append(d...)
		data.Results = emptyList
		return diags
	}

	resultVals := make([]datasource_insights.ResultsValue, 0, len(insights))
	for _, insight := range insights {
		val, d := mapInsightToResultsValue(ctx, &insight)
		diags.Append(d...)
		if diags.HasError() {
			return diags
		}
		resultVals = append(resultVals, val)
	}

	resultsList, d := types.ListValueFrom(ctx, datasource_insights.ResultsValue{}.Type(ctx), resultVals)
	diags.Append(d...)
	data.Results = resultsList
	return diags
}

// mapInsightToResultsValue maps a single InsightResponse to the generated ResultsValue type.
func mapInsightToResultsValue(ctx context.Context, insight *models.InsightResponse) (datasource_insights.ResultsValue, diag.Diagnostics) {
	var diags diag.Diagnostics

	// Categories
	var categoriesVal types.List
	if insight.Categories != nil {
		catStrings := make([]string, len(*insight.Categories))
		for i, c := range *insight.Categories {
			catStrings[i] = string(c)
		}
		var d diag.Diagnostics
		categoriesVal, d = types.ListValueFrom(ctx, types.StringType, catStrings)
		diags.Append(d...)
	} else {
		var d diag.Diagnostics
		categoriesVal, d = types.ListValueFrom(ctx, types.StringType, []string{})
		diags.Append(d...)
	}

	// Tags
	var tagsVal types.List
	if insight.Tags != nil {
		var d diag.Diagnostics
		tagsVal, d = types.ListValueFrom(ctx, types.StringType, *insight.Tags)
		diags.Append(d...)
	} else {
		var d diag.Diagnostics
		tagsVal, d = types.ListValueFrom(ctx, types.StringType, []string{})
		diags.Append(d...)
	}

	// CloudProvider
	var cloudProviderVal types.String
	if insight.CloudProvider != nil {
		cloudProviderVal = types.StringValue(*insight.CloudProvider)
	} else {
		cloudProviderVal = types.StringNull()
	}

	// DisplayStatus
	var displayStatusVal types.String
	if insight.DisplayStatus != nil {
		displayStatusVal = types.StringValue(string(*insight.DisplayStatus))
	} else {
		displayStatusVal = types.StringNull()
	}

	// LastUpdated
	var lastUpdatedVal types.String
	if insight.LastUpdated != nil {
		lastUpdatedVal = types.StringValue(insight.LastUpdated.UTC().Format(time.RFC3339))
	} else {
		lastUpdatedVal = types.StringNull()
	}

	// Source
	var sourceVal types.String
	if insight.Source != nil {
		sourceVal = types.StringValue(*insight.Source)
	} else {
		sourceVal = types.StringNull()
	}

	// Summary
	var summaryVal datasource_insights.SummaryValue
	if insight.Summary != nil {
		var d diag.Diagnostics
		summaryVal, d = datasource_insights.NewSummaryValue(
			datasource_insights.SummaryValue{}.AttributeTypes(ctx),
			map[string]attr.Value{
				"operational_risks":       types.Float64PointerValue(insight.Summary.OperationalRisks),
				"performance_risks":       types.Float64PointerValue(insight.Summary.PerformanceRisks),
				"potential_daily_savings": types.Float64PointerValue(insight.Summary.PotentialDailySavings),
				"reliability_risks":       types.Float64PointerValue(insight.Summary.ReliabilityRisks),
				"security_risks":          types.Float64PointerValue(insight.Summary.SecurityRisks),
				"sustainability_risks":    types.Float64PointerValue(insight.Summary.SustainabilityRisks),
			},
		)
		diags.Append(d...)
	} else {
		summaryVal = datasource_insights.NewSummaryValueNull()
	}

	// LastStatusChange
	var lscVal datasource_insights.LastStatusChangeValue
	if insight.LastStatusChange != nil {
		var d diag.Diagnostics
		lscVal, d = datasource_insights.NewLastStatusChangeValue(
			datasource_insights.LastStatusChangeValue{}.AttributeTypes(ctx),
			map[string]attr.Value{
				"last_changed_at": types.StringValue(insight.LastStatusChange.LastChangedAt.UTC().Format(time.RFC3339)),
				"user_id":         types.StringValue(insight.LastStatusChange.UserId),
			},
		)
		diags.Append(d...)
	} else {
		lscVal = datasource_insights.NewLastStatusChangeValueNull()
	}

	// DismissalDetails
	var ddVal datasource_insights.DismissalDetailsValue
	if insight.DismissalDetails != nil {
		reasonVal := types.StringNull()
		if insight.DismissalDetails.Reason != nil {
			reasonVal = types.StringValue(string(*insight.DismissalDetails.Reason))
		}
		commentVal := types.StringNull()
		if insight.DismissalDetails.Comment != nil {
			commentVal = types.StringValue(*insight.DismissalDetails.Comment)
		}
		var d diag.Diagnostics
		ddVal, d = datasource_insights.NewDismissalDetailsValue(
			datasource_insights.DismissalDetailsValue{}.AttributeTypes(ctx),
			map[string]attr.Value{
				"reason":  reasonVal,
				"comment": commentVal,
			},
		)
		diags.Append(d...)
	} else {
		ddVal = datasource_insights.NewDismissalDetailsValueNull()
	}

	resultVal, d := datasource_insights.NewResultsValue(
		datasource_insights.ResultsValue{}.AttributeTypes(ctx),
		map[string]attr.Value{
			"categories":               categoriesVal,
			"cloud_flow_template_id":   stringPtrOrNull(insight.CloudFlowTemplateId),
			"cloud_provider":           cloudProviderVal,
			"detailed_description_mdx": stringPtrOrNull(insight.DetailedDescriptionMdx),
			"dismissal_details":        ddVal,
			"display_status":           displayStatusVal,
			"easy_win_description":     stringPtrOrNull(insight.EasyWinDescription),
			"key":                      types.StringPointerValue(insight.Key),
			"last_status_change":       lscVal,
			"last_updated":             lastUpdatedVal,
			"report_url":               stringPtrOrNull(insight.ReportUrl),
			"short_description":        types.StringPointerValue(insight.ShortDescription),
			"source":                   sourceVal,
			"summary":                  summaryVal,
			"tags":                     tagsVal,
			"title":                    types.StringPointerValue(insight.Title),
		},
	)
	diags.Append(d...)

	return resultVal, diags
}
