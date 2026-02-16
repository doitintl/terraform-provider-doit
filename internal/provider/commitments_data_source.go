package provider

import (
	"context"
	"fmt"

	"github.com/doitintl/terraform-provider-doit/internal/provider/datasource_commitments"
	"github.com/doitintl/terraform-provider-doit/internal/provider/models"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
)

var _ datasource.DataSource = &commitmentsDataSource{}

func NewCommitmentsDataSource() datasource.DataSource {
	return &commitmentsDataSource{}
}

type commitmentsDataSource struct {
	client *models.ClientWithResponses
}

func (d *commitmentsDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_commitments"
}

func (d *commitmentsDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	client, ok := req.ProviderData.(*models.ClientWithResponses)
	if !ok {
		resp.Diagnostics.AddError("Unexpected Data Source Configure Type",
			fmt.Sprintf("Expected *models.ClientWithResponses, got: %T", req.ProviderData))
		return
	}
	d.client = client
}

func (d *commitmentsDataSource) Schema(ctx context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = datasource_commitments.CommitmentsDataSourceSchema(ctx)
	resp.Schema.Description = "Lists commitment contracts with optional filtering and pagination."
	resp.Schema.MarkdownDescription = resp.Schema.Description
}

func (d *commitmentsDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data datasource_commitments.CommitmentsModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Build API params
	params := &models.ListCommitmentsParams{}

	// Optional filter
	if !data.Filter.IsNull() && !data.Filter.IsUnknown() {
		filterVal := data.Filter.ValueString()
		params.Filter = &filterVal
	}

	// Optional sort_by
	if !data.SortBy.IsNull() && !data.SortBy.IsUnknown() {
		sortByVal := models.ListCommitmentsParamsSortBy(data.SortBy.ValueString())
		params.SortBy = &sortByVal
	}

	// Optional sort_order
	if !data.SortOrder.IsNull() && !data.SortOrder.IsUnknown() {
		sortOrderVal := models.ListCommitmentsParamsSortOrder(data.SortOrder.ValueString())
		params.SortOrder = &sortOrderVal
	}

	// Smart pagination: honor user-provided values, otherwise auto-paginate
	userControlsPagination := !data.MaxResults.IsNull() && !data.MaxResults.IsUnknown()

	var allCommitments []models.CommitmentExternalListItem

	if userControlsPagination {
		// Manual mode: single API call with user's params
		maxResultsVal := data.MaxResults.ValueString()
		params.MaxResults = &maxResultsVal

		if !data.PageToken.IsNull() && !data.PageToken.IsUnknown() {
			pageTokenVal := data.PageToken.ValueString()
			params.PageToken = &pageTokenVal
		}

		apiResp, err := d.client.ListCommitmentsWithResponse(ctx, params)
		if err != nil {
			resp.Diagnostics.AddError("Error listing commitments", err.Error())
			return
		}
		if apiResp.JSON200 == nil {
			resp.Diagnostics.AddError("Unexpected API response",
				fmt.Sprintf("Status: %d, Body: %s", apiResp.StatusCode(), string(apiResp.Body)))
			return
		}

		if apiResp.JSON200.Commitments != nil {
			allCommitments = *apiResp.JSON200.Commitments
		}

		// Preserve user-provided pagination values to prevent drift
		// max_results stays as user set it
		if apiResp.JSON200.PageToken != nil && *apiResp.JSON200.PageToken != "" {
			data.PageToken = types.StringValue(*apiResp.JSON200.PageToken)
		} else {
			// If no next page token and user didn't provide one, keep their value
			if data.PageToken.IsNull() || data.PageToken.IsUnknown() {
				data.PageToken = types.StringNull()
			}
		}
		if apiResp.JSON200.RowCount != nil {
			data.RowCount = types.Int64Value(*apiResp.JSON200.RowCount)
		} else {
			data.RowCount = types.Int64Value(int64(len(allCommitments)))
		}
	} else {
		// Auto mode: fetch all pages
		for {
			apiResp, err := d.client.ListCommitmentsWithResponse(ctx, params)
			if err != nil {
				resp.Diagnostics.AddError("Error listing commitments", err.Error())
				return
			}
			if apiResp.JSON200 == nil {
				resp.Diagnostics.AddError("Unexpected API response",
					fmt.Sprintf("Status: %d, Body: %s", apiResp.StatusCode(), string(apiResp.Body)))
				return
			}

			if apiResp.JSON200.Commitments != nil {
				allCommitments = append(allCommitments, *apiResp.JSON200.Commitments...)
			}

			// Check if there are more pages
			if apiResp.JSON200.PageToken == nil || *apiResp.JSON200.PageToken == "" {
				break
			}
			params.PageToken = apiResp.JSON200.PageToken
		}

		data.RowCount = types.Int64Value(int64(len(allCommitments)))
		data.PageToken = types.StringNull()
		data.MaxResults = types.StringNull()
	}

	// Set optional params to null if not user-provided (prevent drift)
	if data.Filter.IsUnknown() {
		data.Filter = types.StringNull()
	}
	if data.SortBy.IsUnknown() {
		data.SortBy = types.StringNull()
	}
	if data.SortOrder.IsUnknown() {
		data.SortOrder = types.StringNull()
	}

	// Map commitment items
	commitmentsType := datasource_commitments.CommitmentsType{
		ObjectType: types.ObjectType{
			AttrTypes: datasource_commitments.CommitmentsValue{}.AttributeTypes(ctx),
		},
	}

	if len(allCommitments) == 0 {
		data.Commitments = types.ListValueMust(commitmentsType, []attr.Value{})
	} else {
		commitmentValues := make([]attr.Value, 0, len(allCommitments))
		for _, c := range allCommitments {
			cv := mapCommitmentListItem(ctx, c)
			commitmentValues = append(commitmentValues, cv)
		}
		data.Commitments = types.ListValueMust(commitmentsType, commitmentValues)
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func mapCommitmentListItem(ctx context.Context, c models.CommitmentExternalListItem) datasource_commitments.CommitmentsValue {
	var name basetypes.StringValue
	if c.Name != nil {
		name = types.StringValue(*c.Name)
	} else {
		name = types.StringNull()
	}

	var currency basetypes.StringValue
	if c.Currency != nil {
		currency = types.StringValue(*c.Currency)
	} else {
		currency = types.StringNull()
	}

	var provider basetypes.StringValue
	if c.Provider != nil {
		provider = types.StringValue(string(*c.Provider))
	} else {
		provider = types.StringNull()
	}

	var createTime basetypes.Int64Value
	if c.CreateTime != nil {
		createTime = types.Int64Value(*c.CreateTime)
	} else {
		createTime = types.Int64Null()
	}

	var updateTime basetypes.Int64Value
	if c.UpdateTime != nil {
		updateTime = types.Int64Value(*c.UpdateTime)
	} else {
		updateTime = types.Int64Null()
	}

	var totalCommitmentValue basetypes.Float64Value
	if c.TotalCommitmentValue != nil {
		totalCommitmentValue = types.Float64Value(*c.TotalCommitmentValue)
	} else {
		totalCommitmentValue = types.Float64Null()
	}

	var totalCurrentAttainment basetypes.Float64Value
	if c.TotalCurrentAttainment != nil {
		totalCurrentAttainment = types.Float64Value(*c.TotalCurrentAttainment)
	} else {
		totalCurrentAttainment = types.Float64Null()
	}

	var startDate basetypes.StringValue
	if c.StartDate != nil {
		startDate = types.StringValue(c.StartDate.Format("2006-01-02"))
	} else {
		startDate = types.StringNull()
	}

	var endDate basetypes.StringValue
	if c.EndDate != nil {
		endDate = types.StringValue(c.EndDate.Format("2006-01-02"))
	} else {
		endDate = types.StringNull()
	}

	// Map nested periods
	periods := mapCommitmentsListPeriods(ctx, c.Periods)

	return datasource_commitments.NewCommitmentsValueMust(
		datasource_commitments.CommitmentsValue{}.AttributeTypes(ctx),
		map[string]attr.Value{
			"create_time":              createTime,
			"currency":                 currency,
			"end_date":                 endDate,
			"name":                     name,
			"periods":                  periods,
			"provider":                 provider,
			"start_date":               startDate,
			"total_commitment_value":   totalCommitmentValue,
			"total_current_attainment": totalCurrentAttainment,
			"update_time":              updateTime,
		},
	)
}

func mapCommitmentsListPeriods(ctx context.Context, periods *[]models.CommitmentPeriod) basetypes.ListValue {
	periodsType := datasource_commitments.PeriodsType{
		ObjectType: types.ObjectType{
			AttrTypes: datasource_commitments.PeriodsValue{}.AttributeTypes(ctx),
		},
	}

	if periods == nil || len(*periods) == 0 {
		return types.ListValueMust(periodsType, []attr.Value{})
	}

	periodValues := make([]attr.Value, 0, len(*periods))
	for _, p := range *periods {
		var commitmentValue basetypes.Float64Value
		if p.CommitmentValue != nil {
			commitmentValue = types.Float64Value(*p.CommitmentValue)
		} else {
			commitmentValue = types.Float64Null()
		}

		var marketplaceLimitPercentage basetypes.Float64Value
		if p.MarketplaceLimitPercentage != nil {
			marketplaceLimitPercentage = types.Float64Value(*p.MarketplaceLimitPercentage)
		} else {
			marketplaceLimitPercentage = types.Float64Null()
		}

		var startDate basetypes.StringValue
		if p.StartDate != nil {
			startDate = types.StringValue(p.StartDate.Format("2006-01-02"))
		} else {
			startDate = types.StringNull()
		}

		var endDate basetypes.StringValue
		if p.EndDate != nil {
			endDate = types.StringValue(p.EndDate.Format("2006-01-02"))
		} else {
			endDate = types.StringNull()
		}

		pv := datasource_commitments.NewPeriodsValueMust(
			datasource_commitments.PeriodsValue{}.AttributeTypes(ctx),
			map[string]attr.Value{
				"commitment_value":             commitmentValue,
				"end_date":                     endDate,
				"marketplace_limit_percentage": marketplaceLimitPercentage,
				"start_date":                   startDate,
			},
		)
		periodValues = append(periodValues, pv)
	}

	return types.ListValueMust(periodsType, periodValues)
}
