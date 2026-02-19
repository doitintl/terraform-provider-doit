package provider

import (
	"context"
	"fmt"
	"time"

	"github.com/doitintl/terraform-provider-doit/internal/provider/datasource_commitments"
	"github.com/doitintl/terraform-provider-doit/internal/provider/models"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
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

		// Set page_token from API response (null when no next page)
		data.PageToken = types.StringPointerValue(apiResp.JSON200.PageToken)
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

	// Map commitment items with proper diagnostics.
	commitmentsType := datasource_commitments.CommitmentsType{
		ObjectType: types.ObjectType{
			AttrTypes: datasource_commitments.CommitmentsValue{}.AttributeTypes(ctx),
		},
	}

	if len(allCommitments) == 0 {
		emptyList, d := types.ListValueFrom(ctx, commitmentsType, []datasource_commitments.CommitmentsValue{})
		resp.Diagnostics.Append(d...)
		data.Commitments = emptyList
	} else {
		commitmentValues := make([]datasource_commitments.CommitmentsValue, 0, len(allCommitments))
		for _, c := range allCommitments {
			cv, cvDiags := mapCommitmentListItem(ctx, c)
			resp.Diagnostics.Append(cvDiags...)
			commitmentValues = append(commitmentValues, cv)
		}
		commitmentList, d := types.ListValueFrom(ctx, commitmentsType, commitmentValues)
		resp.Diagnostics.Append(d...)
		data.Commitments = commitmentList
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func mapCommitmentListItem(ctx context.Context, c models.CommitmentExternalListItem) (datasource_commitments.CommitmentsValue, diag.Diagnostics) {
	var diags diag.Diagnostics

	var id types.String
	if c.Id != nil {
		id = types.StringValue(*c.Id)
	} else {
		id = types.StringNull()
	}

	var name types.String
	if c.Name != nil {
		name = types.StringValue(*c.Name)
	} else {
		name = types.StringNull()
	}

	var currency types.String
	if c.Currency != nil {
		currency = types.StringValue(*c.Currency)
	} else {
		currency = types.StringNull()
	}

	var cloudProvider types.String
	if c.CloudProvider != nil {
		cloudProvider = types.StringValue(string(*c.CloudProvider))
	} else {
		cloudProvider = types.StringNull()
	}

	var createTime types.Int64
	if c.CreateTime != nil {
		createTime = types.Int64Value(*c.CreateTime)
	} else {
		createTime = types.Int64Null()
	}

	var updateTime types.Int64
	if c.UpdateTime != nil {
		updateTime = types.Int64Value(*c.UpdateTime)
	} else {
		updateTime = types.Int64Null()
	}

	var totalCommitmentValue types.Float64
	if c.TotalCommitmentValue != nil {
		totalCommitmentValue = types.Float64Value(*c.TotalCommitmentValue)
	} else {
		totalCommitmentValue = types.Float64Null()
	}

	var totalCurrentAttainment types.Float64
	if c.TotalCurrentAttainment != nil {
		totalCurrentAttainment = types.Float64Value(*c.TotalCurrentAttainment)
	} else {
		totalCurrentAttainment = types.Float64Null()
	}

	var startDate types.String
	if c.StartDate != nil {
		startDate = types.StringValue(c.StartDate.UTC().Format(time.RFC3339))
	} else {
		startDate = types.StringNull()
	}

	var endDate types.String
	if c.EndDate != nil {
		endDate = types.StringValue(c.EndDate.UTC().Format(time.RFC3339))
	} else {
		endDate = types.StringNull()
	}

	// Map nested periods with proper diagnostics
	periods, periodsDiags := mapCommitmentsListPeriods(ctx, c.Periods)
	diags.Append(periodsDiags...)

	cv, cvDiags := datasource_commitments.NewCommitmentsValue(
		datasource_commitments.CommitmentsValue{}.AttributeTypes(ctx),
		map[string]attr.Value{
			"create_time":              createTime,
			"currency":                 currency,
			"end_date":                 endDate,
			"id":                       id,
			"name":                     name,
			"periods":                  periods,
			"cloud_provider":           cloudProvider,
			"start_date":               startDate,
			"total_commitment_value":   totalCommitmentValue,
			"total_current_attainment": totalCurrentAttainment,
			"update_time":              updateTime,
		},
	)
	diags.Append(cvDiags...)
	return cv, diags
}

func mapCommitmentsListPeriods(ctx context.Context, periods *[]models.CommitmentPeriod) (types.List, diag.Diagnostics) {
	var diags diag.Diagnostics

	periodsType := datasource_commitments.PeriodsType{
		ObjectType: types.ObjectType{
			AttrTypes: datasource_commitments.PeriodsValue{}.AttributeTypes(ctx),
		},
	}

	if periods == nil || len(*periods) == 0 {
		emptyList, d := types.ListValueFrom(ctx, periodsType, []datasource_commitments.PeriodsValue{})
		diags.Append(d...)
		return emptyList, diags
	}

	periodValues := make([]datasource_commitments.PeriodsValue, 0, len(*periods))
	for _, p := range *periods {
		var commitmentValue types.Float64
		if p.CommitmentValue != nil {
			commitmentValue = types.Float64Value(*p.CommitmentValue)
		} else {
			commitmentValue = types.Float64Null()
		}

		var marketplaceLimitPercentage types.Float64
		if p.MarketplaceLimitPercentage != nil {
			marketplaceLimitPercentage = types.Float64Value(*p.MarketplaceLimitPercentage)
		} else {
			marketplaceLimitPercentage = types.Float64Null()
		}

		var startDate types.String
		if p.StartDate != nil {
			startDate = types.StringValue(p.StartDate.UTC().Format(time.RFC3339))
		} else {
			startDate = types.StringNull()
		}

		var endDate types.String
		if p.EndDate != nil {
			endDate = types.StringValue(p.EndDate.UTC().Format(time.RFC3339))
		} else {
			endDate = types.StringNull()
		}

		pv, pvDiags := datasource_commitments.NewPeriodsValue(
			datasource_commitments.PeriodsValue{}.AttributeTypes(ctx),
			map[string]attr.Value{
				"commitment_value":             commitmentValue,
				"end_date":                     endDate,
				"marketplace_limit_percentage": marketplaceLimitPercentage,
				"start_date":                   startDate,
			},
		)
		diags.Append(pvDiags...)
		periodValues = append(periodValues, pv)
	}

	periodList, d := types.ListValueFrom(ctx, periodsType, periodValues)
	diags.Append(d...)
	return periodList, diags
}
