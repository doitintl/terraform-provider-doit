package provider

import (
	"context"
	"fmt"
	"math/big"

	"github.com/doitintl/terraform-provider-doit/internal/provider/datasource_insights"
	"github.com/doitintl/terraform-provider-doit/internal/provider/models"

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
}

func (d *insightsDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_insights"
}

func (d *insightsDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = datasource_insights.InsightsDataSourceSchema(ctx)
}

func (d *insightsDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *insightsDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data insightsDataSourceModel

	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Build query parameters
	params := &models.GetInsightResultsParams{}

	if !data.SearchTerm.IsNull() && !data.SearchTerm.IsUnknown() {
		v := data.SearchTerm.ValueString()
		params.SearchTerm = &v
	}

	if !data.DisplayStatus.IsNull() && !data.DisplayStatus.IsUnknown() {
		var statuses []string
		resp.Diagnostics.Append(data.DisplayStatus.ElementsAs(ctx, &statuses, false)...)
		if resp.Diagnostics.HasError() {
			return
		}
		dsVals := make([]models.GetInsightResultsParamsDisplayStatus, len(statuses))
		for i, s := range statuses {
			dsVals[i] = models.GetInsightResultsParamsDisplayStatus(s)
		}
		params.DisplayStatus = &dsVals
	}

	if !data.Category.IsNull() && !data.Category.IsUnknown() {
		var categories []string
		resp.Diagnostics.Append(data.Category.ElementsAs(ctx, &categories, false)...)
		if resp.Diagnostics.HasError() {
			return
		}
		catVals := make([]models.GetInsightResultsParamsCategory, len(categories))
		for i, c := range categories {
			catVals[i] = models.GetInsightResultsParamsCategory(c)
		}
		params.Category = &catVals
	}

	if !data.Provider.IsNull() && !data.Provider.IsUnknown() {
		p := data.Provider.ValueString()
		params.Provider = &p
	}

	if !data.Source.IsNull() && !data.Source.IsUnknown() {
		var sources []string
		resp.Diagnostics.Append(data.Source.ElementsAs(ctx, &sources, false)...)
		if resp.Diagnostics.HasError() {
			return
		}
		srcVals := make([]models.Source, len(sources))
		copy(srcVals, sources)
		params.Source = &srcVals
	}

	if !data.Priority.IsNull() && !data.Priority.IsUnknown() {
		var priorities []string
		resp.Diagnostics.Append(data.Priority.ElementsAs(ctx, &priorities, false)...)
		if resp.Diagnostics.HasError() {
			return
		}
		priVals := make([]models.GetInsightResultsParamsPriority, len(priorities))
		for i, p := range priorities {
			priVals[i] = models.GetInsightResultsParamsPriority(p)
		}
		params.Priority = &priVals
	}

	if !data.Tag.IsNull() && !data.Tag.IsUnknown() {
		var tags []string
		resp.Diagnostics.Append(data.Tag.ElementsAs(ctx, &tags, false)...)
		if resp.Diagnostics.HasError() {
			return
		}
		params.Tag = &tags
	}

	if !data.EasyWin.IsNull() && !data.EasyWin.IsUnknown() {
		v := data.EasyWin.ValueBool()
		params.EasyWin = &v
	}

	if !data.CloudFlows.IsNull() && !data.CloudFlows.IsUnknown() {
		v := data.CloudFlows.ValueBool()
		params.CloudFlows = &v
	}

	if !data.Page.IsNull() && !data.Page.IsUnknown() {
		v := int(data.Page.ValueInt64())
		params.Page = &v
	}

	if !data.PageSize.IsNull() && !data.PageSize.IsUnknown() {
		v := int(data.PageSize.ValueInt64())
		params.PageSize = &v
	}

	// Make API call
	apiResp, err := d.client.GetInsightResultsWithResponse(ctx, params)
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

	// Map pagination
	if result.Pagination != nil {
		paginationVal, diags := datasource_insights.NewPaginationValue(
			datasource_insights.PaginationValue{}.AttributeTypes(ctx),
			map[string]attr.Value{
				"has_next_page": types.BoolValue(result.Pagination.HasNextPage),
			},
		)
		resp.Diagnostics.Append(diags...)
		data.Pagination = paginationVal
	} else {
		data.Pagination = datasource_insights.NewPaginationValueNull()
	}

	// Map results list
	if result.Results != nil && len(*result.Results) > 0 {
		resultVals := make([]datasource_insights.ResultsValue, 0, len(*result.Results))
		for _, insight := range *result.Results {
			resultVal := mapInsightToResultsValue(ctx, insight, &resp.Diagnostics)
			resultVals = append(resultVals, resultVal)
		}

		resultsList, diags := types.ListValueFrom(ctx, datasource_insights.ResultsValue{}.Type(ctx), resultVals)
		resp.Diagnostics.Append(diags...)
		data.Results = resultsList
	} else {
		emptyList, diags := types.ListValueFrom(ctx, datasource_insights.ResultsValue{}.Type(ctx), []datasource_insights.ResultsValue{})
		resp.Diagnostics.Append(diags...)
		data.Results = emptyList
	}

	// Preserve user-provided filter values in state; set computed-only values
	// Filter params that are Optional+Computed: if user didn't set them, keep them null
	if data.SearchTerm.IsUnknown() {
		data.SearchTerm = types.StringNull()
	}
	if data.DisplayStatus.IsUnknown() {
		data.DisplayStatus = types.ListNull(types.StringType)
	}
	if data.Category.IsUnknown() {
		data.Category = types.ListNull(types.StringType)
	}
	if data.Provider.IsUnknown() {
		data.Provider = types.StringNull()
	}
	if data.Source.IsUnknown() {
		data.Source = types.ListNull(types.StringType)
	}
	if data.Priority.IsUnknown() {
		data.Priority = types.ListNull(types.StringType)
	}
	if data.Tag.IsUnknown() {
		data.Tag = types.ListNull(types.StringType)
	}
	if data.EasyWin.IsUnknown() {
		data.EasyWin = types.BoolNull()
	}
	if data.CloudFlows.IsUnknown() {
		data.CloudFlows = types.BoolNull()
	}
	if data.Page.IsUnknown() {
		data.Page = types.Int64Null()
	}
	if data.PageSize.IsUnknown() {
		data.PageSize = types.Int64Null()
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

// mapInsightToResultsValue maps a single InsightResponse to a ResultsValue.
func mapInsightToResultsValue(ctx context.Context, insight models.InsightResponse, diagnostics *diag.Diagnostics) datasource_insights.ResultsValue {
	// Map categories list
	categoriesList := mapStringPointerSliceToList(ctx, func() *[]string {
		if insight.Categories == nil {
			return nil
		}
		cats := make([]string, len(*insight.Categories))
		for i, c := range *insight.Categories {
			cats[i] = string(c)
		}
		return &cats
	}(), diagnostics)

	// Map tags list
	tagsList := mapStringPointerSliceToList(ctx, insight.Tags, diagnostics)

	// Map summary nested object
	var summaryVal datasource_insights.SummaryValue
	if insight.Summary != nil {
		var diags diag.Diagnostics
		summaryVal, diags = datasource_insights.NewSummaryValue(
			datasource_insights.SummaryValue{}.AttributeTypes(ctx),
			map[string]attr.Value{
				"operational_risks":       float32PtrToNumberValue(insight.Summary.OperationalRisks),
				"performance_risks":       float32PtrToNumberValue(insight.Summary.PerformanceRisks),
				"potential_daily_savings": float32PtrToNumberValue(insight.Summary.PotentialDailySavings),
				"reliability_risks":       float32PtrToNumberValue(insight.Summary.ReliabilityRisks),
				"security_risks":          float32PtrToNumberValue(insight.Summary.SecurityRisks),
				"sustainability_risks":    float32PtrToNumberValue(insight.Summary.SustainabilityRisks),
			},
		)
		diagnostics.Append(diags...)
	} else {
		summaryVal = datasource_insights.NewSummaryValueNull()
	}

	// Map last status change nested object
	var lastStatusChangeVal datasource_insights.LastStatusChangeValue
	if insight.LastStatusChange != nil {
		var diags diag.Diagnostics
		lastStatusChangeVal, diags = datasource_insights.NewLastStatusChangeValue(
			datasource_insights.LastStatusChangeValue{}.AttributeTypes(ctx),
			map[string]attr.Value{
				"last_changed_at": types.StringValue(insight.LastStatusChange.LastChangedAt.String()),
				"user_id":         types.StringValue(insight.LastStatusChange.UserId),
			},
		)
		diagnostics.Append(diags...)
	} else {
		lastStatusChangeVal = datasource_insights.NewLastStatusChangeValueNull()
	}

	// Map display_status enum
	var displayStatusVal types.String
	if insight.DisplayStatus != nil {
		displayStatusVal = types.StringValue(string(*insight.DisplayStatus))
	} else {
		displayStatusVal = types.StringNull()
	}

	// Map provider
	var providerVal types.String
	if insight.Provider != nil {
		providerVal = types.StringValue(*insight.Provider)
	} else {
		providerVal = types.StringNull()
	}

	// Map source
	var sourceVal types.String
	if insight.Source != nil {
		sourceVal = types.StringValue(*insight.Source)
	} else {
		sourceVal = types.StringNull()
	}

	// Map last_updated
	var lastUpdatedVal types.String
	if insight.LastUpdated != nil {
		lastUpdatedVal = types.StringValue(insight.LastUpdated.String())
	} else {
		lastUpdatedVal = types.StringNull()
	}

	resultVal, diags := datasource_insights.NewResultsValue(
		datasource_insights.ResultsValue{}.AttributeTypes(ctx),
		map[string]attr.Value{
			"categories":               categoriesList,
			"cloud_flow_template_id":   types.StringPointerValue(insight.CloudFlowTemplateId),
			"detailed_description_mdx": types.StringPointerValue(insight.DetailedDescriptionMdx),
			"display_status":           displayStatusVal,
			"easy_win_description":     types.StringPointerValue(insight.EasyWinDescription),
			"key":                      types.StringPointerValue(insight.Key),
			"last_status_change":       lastStatusChangeVal,
			"last_updated":             lastUpdatedVal,
			"provider":                 providerVal,
			"report_url":               types.StringPointerValue(insight.ReportUrl),
			"short_description":        types.StringPointerValue(insight.ShortDescription),
			"source":                   sourceVal,
			"summary":                  summaryVal,
			"tags":                     tagsList,
			"title":                    types.StringPointerValue(insight.Title),
		},
	)
	diagnostics.Append(diags...)
	return resultVal
}

// mapStringPointerSliceToList maps a *[]string to a types.List of strings.
func mapStringPointerSliceToList(ctx context.Context, items *[]string, diagnostics *diag.Diagnostics) types.List {
	if items == nil || len(*items) == 0 {
		emptyList, d := types.ListValueFrom(ctx, types.StringType, []string{})
		diagnostics.Append(d...)
		return emptyList
	}

	list, d := types.ListValueFrom(ctx, types.StringType, *items)
	diagnostics.Append(d...)
	return list
}

// float32PtrToNumberValue converts a *float32 to a types.Number value.
func float32PtrToNumberValue(v *float32) types.Number {
	if v == nil {
		return types.NumberNull()
	}
	return types.NumberValue(big.NewFloat(float64(*v)))
}
