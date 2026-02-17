package provider

import (
	"context"
	"fmt"

	"github.com/doitintl/terraform-provider-doit/internal/provider/datasource_budgets"
	"github.com/doitintl/terraform-provider-doit/internal/provider/models"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var _ datasource.DataSource = (*budgetsDataSource)(nil)

func NewBudgetsDataSource() datasource.DataSource {
	return &budgetsDataSource{}
}

type budgetsDataSource struct {
	client *models.ClientWithResponses
}

type budgetsDataSourceModel struct {
	datasource_budgets.BudgetsModel
}

func (d *budgetsDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_budgets"
}

func (d *budgetsDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = datasource_budgets.BudgetsDataSourceSchema(ctx)
}

func (d *budgetsDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *budgetsDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data budgetsDataSourceModel

	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Build query parameters
	params := &models.ListBudgetsParams{}
	if !data.Filter.IsNull() && !data.Filter.IsUnknown() {
		filter := data.Filter.ValueString()
		params.Filter = &filter
	}
	if !data.MinCreationTime.IsNull() && !data.MinCreationTime.IsUnknown() {
		minTime := data.MinCreationTime.ValueString()
		params.MinCreationTime = &minTime
	}
	if !data.MaxCreationTime.IsNull() && !data.MaxCreationTime.IsUnknown() {
		maxTime := data.MaxCreationTime.ValueString()
		params.MaxCreationTime = &maxTime
	}

	// Smart pagination: honor user-provided values, otherwise auto-paginate
	userControlsPagination := !data.MaxResults.IsNull() && !data.MaxResults.IsUnknown()

	var allBudgets []models.BudgetListItem

	if userControlsPagination {
		// Manual mode: single API call with user's params
		maxResultsVal := data.MaxResults.ValueString()
		params.MaxResults = &maxResultsVal
		if !data.PageToken.IsNull() && !data.PageToken.IsUnknown() {
			pageTokenVal := data.PageToken.ValueString()
			params.PageToken = &pageTokenVal
		}

		apiResp, err := d.client.ListBudgetsWithResponse(ctx, params)
		if err != nil {
			resp.Diagnostics.AddError(
				"Error Reading Budgets",
				fmt.Sprintf("Unable to read budgets: %v", err),
			)
			return
		}

		if apiResp.StatusCode() != 200 || apiResp.JSON200 == nil {
			resp.Diagnostics.AddError(
				"Error Reading Budgets",
				fmt.Sprintf("API returned status %d: %s", apiResp.StatusCode(), string(apiResp.Body)),
			)
			return
		}

		result := apiResp.JSON200
		if result.Budgets != nil {
			allBudgets = *result.Budgets
		}

		// Preserve API's page_token for user to fetch next page
		data.PageToken = types.StringPointerValue(result.PageToken)
		if result.RowCount != nil {
			data.RowCount = types.Int64Value(*result.RowCount)
		} else {
			data.RowCount = types.Int64Value(int64(len(allBudgets)))
		}
		// max_results is already set by user, no change needed
	} else {
		// Auto mode: fetch all pages, honoring user-provided page_token as starting point
		if !data.PageToken.IsNull() && !data.PageToken.IsUnknown() {
			pageTokenVal := data.PageToken.ValueString()
			params.PageToken = &pageTokenVal
		}
		for {
			apiResp, err := d.client.ListBudgetsWithResponse(ctx, params)
			if err != nil {
				resp.Diagnostics.AddError(
					"Error Reading Budgets",
					fmt.Sprintf("Unable to read budgets: %v", err),
				)
				return
			}

			if apiResp.StatusCode() != 200 || apiResp.JSON200 == nil {
				resp.Diagnostics.AddError(
					"Error Reading Budgets",
					fmt.Sprintf("API returned status %d: %s", apiResp.StatusCode(), string(apiResp.Body)),
				)
				return
			}

			result := apiResp.JSON200
			if result.Budgets != nil {
				allBudgets = append(allBudgets, *result.Budgets...)
			}

			if result.PageToken == nil || *result.PageToken == "" {
				break
			}
			params.PageToken = result.PageToken
		}

		// Auto mode: set counts based on what we fetched
		data.RowCount = types.Int64Value(int64(len(allBudgets)))
		data.PageToken = types.StringNull()
		// max_results was not set by user; normalize to null
		data.MaxResults = types.StringNull()
	}

	// Map budgets list
	if len(allBudgets) > 0 {
		budgetVals := make([]datasource_budgets.BudgetsValue, 0, len(allBudgets))
		for _, budget := range allBudgets {
			// Handle nested alert_thresholds
			alertThresholdsList, diags := mapAlertThresholds(ctx, budget.AlertThresholds)
			resp.Diagnostics.Append(diags...)

			// Handle nested scopes
			scopesList, diags := mapBudgetScopes(ctx, budget.Scopes)
			resp.Diagnostics.Append(diags...)

			// Handle deprecated scope list
			var scopeList types.List
			if budget.Scope != nil {
				scopeVals := make([]attr.Value, 0, len(*budget.Scope))
				for _, s := range *budget.Scope {
					scopeVals = append(scopeVals, types.StringValue(s))
				}
				scopeList, diags = types.ListValue(types.StringType, scopeVals)
				resp.Diagnostics.Append(diags...)
			} else {
				scopeList = types.ListNull(types.StringType)
			}

			budgetVal, diags := datasource_budgets.NewBudgetsValue(
				datasource_budgets.BudgetsValue{}.AttributeTypes(ctx),
				map[string]attr.Value{
					"id":                          types.StringPointerValue(budget.Id),
					"budget_name":                 types.StringPointerValue(budget.BudgetName),
					"owner":                       types.StringPointerValue(budget.Owner),
					"amount":                      types.Float64PointerValue(budget.Amount),
					"currency":                    types.StringPointerValue(budget.Currency),
					"current_utilization":         types.Float64PointerValue(budget.CurrentUtilization),
					"forecasted_utilization_date": types.Int64PointerValue(budget.ForecastedUtilizationDate),
					"create_time":                 types.Int64PointerValue(budget.CreateTime),
					"update_time":                 types.Int64PointerValue(budget.UpdateTime),
					"start_period":                types.Int64PointerValue(budget.StartPeriod),
					"end_period":                  types.Int64PointerValue(budget.EndPeriod),
					"time_interval":               types.StringPointerValue(budget.TimeInterval),
					"url":                         types.StringPointerValue(budget.Url),
					"alert_thresholds":            alertThresholdsList,
					"scopes":                      scopesList,
					"scope":                       scopeList,
				},
			)
			resp.Diagnostics.Append(diags...)
			budgetVals = append(budgetVals, budgetVal)
		}

		budgetList, diags := types.ListValueFrom(ctx, datasource_budgets.BudgetsValue{}.Type(ctx), budgetVals)
		resp.Diagnostics.Append(diags...)
		data.Budgets = budgetList
	} else {
		data.Budgets = types.ListNull(datasource_budgets.BudgetsValue{}.Type(ctx))
	}

	// Set optional filter params to null if they were unknown
	if data.Filter.IsUnknown() {
		data.Filter = types.StringNull()
	}
	if data.MinCreationTime.IsUnknown() {
		data.MinCreationTime = types.StringNull()
	}
	if data.MaxCreationTime.IsUnknown() {
		data.MaxCreationTime = types.StringNull()
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func mapAlertThresholds(ctx context.Context, thresholds *[]models.AlertThreshold) (types.List, diag.Diagnostics) {
	if thresholds == nil || len(*thresholds) == 0 {
		return types.ListNull(datasource_budgets.AlertThresholdsValue{}.Type(ctx)), nil
	}

	vals := make([]datasource_budgets.AlertThresholdsValue, 0, len(*thresholds))
	for _, t := range *thresholds {
		val, diags := datasource_budgets.NewAlertThresholdsValue(
			datasource_budgets.AlertThresholdsValue{}.AttributeTypes(ctx),
			map[string]attr.Value{
				"amount":     types.Float64PointerValue(t.Amount),
				"percentage": types.Float64PointerValue(t.Percentage),
			},
		)
		if diags.HasError() {
			return types.ListNull(datasource_budgets.AlertThresholdsValue{}.Type(ctx)), diags
		}
		vals = append(vals, val)
	}

	return types.ListValueFrom(ctx, datasource_budgets.AlertThresholdsValue{}.Type(ctx), vals)
}

func mapBudgetScopes(ctx context.Context, scopes *[]models.ExternalConfigFilter) (types.List, diag.Diagnostics) {
	if scopes == nil || len(*scopes) == 0 {
		return types.ListNull(datasource_budgets.ScopesValue{}.Type(ctx)), nil
	}

	vals := make([]datasource_budgets.ScopesValue, 0, len(*scopes))
	for _, s := range *scopes {
		// Handle values list
		var valuesList types.List
		if s.Values != nil {
			valStrs := make([]attr.Value, 0, len(*s.Values))
			for _, v := range *s.Values {
				valStrs = append(valStrs, types.StringValue(v))
			}
			var diags diag.Diagnostics
			valuesList, diags = types.ListValue(types.StringType, valStrs)
			if diags.HasError() {
				return types.ListNull(datasource_budgets.ScopesValue{}.Type(ctx)), diags
			}
		} else {
			valuesList = types.ListNull(types.StringType)
		}

		val, diags := datasource_budgets.NewScopesValue(
			datasource_budgets.ScopesValue{}.AttributeTypes(ctx),
			map[string]attr.Value{
				"id":      types.StringValue(s.Id),
				"inverse": types.BoolPointerValue(s.Inverse),
				"mode":    types.StringValue(string(s.Mode)),
				"type":    types.StringValue(string(s.Type)),
				"values":  valuesList,
			},
		)
		if diags.HasError() {
			return types.ListNull(datasource_budgets.ScopesValue{}.Type(ctx)), diags
		}
		vals = append(vals, val)
	}

	return types.ListValueFrom(ctx, datasource_budgets.ScopesValue{}.Type(ctx), vals)
}
