package provider

import (
	"context"
	"fmt"

	"github.com/doitintl/terraform-provider-doit/internal/provider/datasource_budget"
	"github.com/doitintl/terraform-provider-doit/internal/provider/models"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var _ datasource.DataSource = (*budgetDataSource)(nil)
var _ datasource.DataSourceWithConfigure = (*budgetDataSource)(nil)

func NewBudgetDataSource() datasource.DataSource {
	return &budgetDataSource{}
}

type (
	budgetDataSource struct {
		client *models.ClientWithResponses
	}
	budgetDataSourceModel struct {
		datasource_budget.BudgetModel
	}
)

func (d *budgetDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_budget"
}

func (d *budgetDataSource) Schema(ctx context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = datasource_budget.BudgetDataSourceSchema(ctx)
}

func (d *budgetDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}

	client, ok := req.ProviderData.(*models.ClientWithResponses)
	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Data Source Configure Type",
			fmt.Sprintf("Expected *models.ClientWithResponses, got: %T. Please report this issue to the provider developers.", req.ProviderData),
		)
		return
	}

	d.client = client
}

func (d *budgetDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data budgetDataSourceModel

	// Read Terraform configuration data into the model
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Call API to get budget
	budgetResp, err := d.client.GetBudgetWithResponse(ctx, data.Id.ValueString())
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Reading Budget",
			"Could not read budget ID "+data.Id.ValueString()+": "+err.Error(),
		)
		return
	}

	if budgetResp.StatusCode() != 200 {
		resp.Diagnostics.AddError(
			"Error Reading Budget",
			fmt.Sprintf("Could not read budget ID %s, status: %d, body: %s", data.Id.ValueString(), budgetResp.StatusCode(), string(budgetResp.Body)),
		)
		return
	}

	if budgetResp.JSON200 == nil {
		resp.Diagnostics.AddError(
			"Error Reading Budget",
			"Received empty response body for budget ID "+data.Id.ValueString(),
		)
		return
	}

	budget := budgetResp.JSON200

	// Map API response to model
	resp.Diagnostics.Append(d.mapBudgetToModel(ctx, budget, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Save data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

// mapBudgetToModel maps the API BudgetAPI response to the data source model.
// This is similar to mapBudgetToModel in budget.go but uses data source types.
func (d *budgetDataSource) mapBudgetToModel(ctx context.Context, budget *models.BudgetAPI, data *budgetDataSourceModel) (diags diag.Diagnostics) {
	data.Id = types.StringPointerValue(budget.Id)
	data.Name = types.StringValue(budget.Name)
	data.Description = types.StringPointerValue(budget.Description)
	data.Amount = types.Float64PointerValue(budget.Amount)
	data.Currency = types.StringValue(string(budget.Currency))
	data.Type = types.StringValue(budget.Type)
	data.TimeInterval = types.StringValue(budget.TimeInterval)
	data.StartPeriod = types.Int64Value(budget.StartPeriod)
	data.Metric = types.StringPointerValue(budget.Metric)
	data.GrowthPerPeriod = types.Float64PointerValue(budget.GrowthPerPeriod)
	data.UsePrevSpend = types.BoolPointerValue(budget.UsePrevSpend)
	data.CreateTime = types.Int64PointerValue(budget.CreateTime)
	data.UpdateTime = types.Int64PointerValue(budget.UpdateTime)
	data.CurrentUtilization = types.Float64PointerValue(budget.CurrentUtilization)
	data.ForecastedUtilization = types.Float64PointerValue(budget.ForecastedUtilization)

	if budget.EndPeriod != nil && *budget.EndPeriod > 0 {
		data.EndPeriod = types.Int64PointerValue(budget.EndPeriod)
	} else {
		data.EndPeriod = types.Int64Null()
	}

	if budget.Public != nil && *budget.Public != "" {
		data.Public = types.StringValue(string(*budget.Public))
	} else {
		data.Public = types.StringNull()
	}

	// Map alerts
	if budget.Alerts != nil && len(*budget.Alerts) > 0 {
		alertsList := make([]datasource_budget.AlertsValue, len(*budget.Alerts))
		for i, alert := range *budget.Alerts {
			alertAttrs := map[string]attr.Value{
				"forecasted_date": types.Int64PointerValue(alert.ForecastedDate),
				"percentage":      types.Float64PointerValue(alert.Percentage),
				"triggered":       types.BoolPointerValue(alert.Triggered),
			}
			var d diag.Diagnostics
			alertsList[i], d = datasource_budget.NewAlertsValue(datasource_budget.AlertsValue{}.AttributeTypes(ctx), alertAttrs)
			diags.Append(d...)
		}
		alertsListValue, d := types.ListValueFrom(ctx, datasource_budget.AlertsValue{}.Type(ctx), alertsList)
		diags.Append(d...)
		data.Alerts = alertsListValue
	} else {
		data.Alerts = types.ListNull(datasource_budget.AlertsValue{}.Type(ctx))
	}

	// Map collaborators
	if budget.Collaborators != nil && len(*budget.Collaborators) > 0 {
		collaboratorsList := make([]datasource_budget.CollaboratorsValue, len(*budget.Collaborators))
		for i, collab := range *budget.Collaborators {
			collabAttrs := map[string]attr.Value{
				"email": types.StringPointerValue(collab.Email),
				"role":  types.StringPointerValue((*string)(collab.Role)),
			}
			var d diag.Diagnostics
			collaboratorsList[i], d = datasource_budget.NewCollaboratorsValue(datasource_budget.CollaboratorsValue{}.AttributeTypes(ctx), collabAttrs)
			diags.Append(d...)
		}
		collaboratorsListValue, d := types.ListValueFrom(ctx, datasource_budget.CollaboratorsValue{}.Type(ctx), collaboratorsList)
		diags.Append(d...)
		data.Collaborators = collaboratorsListValue
	} else {
		data.Collaborators = types.ListNull(datasource_budget.CollaboratorsValue{}.Type(ctx))
	}

	// Map recipients list
	if budget.Recipients != nil {
		recipientsList, listDiags := types.ListValueFrom(ctx, types.StringType, *budget.Recipients)
		diags.Append(listDiags...)
		data.Recipients = recipientsList
	} else {
		data.Recipients = types.ListNull(types.StringType)
	}

	// Map recipients_slack_channels
	if budget.RecipientsSlackChannels != nil && len(*budget.RecipientsSlackChannels) > 0 {
		slackChannelsList := make([]datasource_budget.RecipientsSlackChannelsValue, len(*budget.RecipientsSlackChannels))
		for i, slack := range *budget.RecipientsSlackChannels {
			slackAttrs := map[string]attr.Value{
				"customer_id": types.StringPointerValue(slack.CustomerId),
				"id":          types.StringPointerValue(slack.Id),
				"name":        types.StringPointerValue(slack.Name),
				"shared":      types.BoolPointerValue(slack.Shared),
				"type":        types.StringPointerValue(slack.Type),
				"workspace":   types.StringPointerValue(slack.Workspace),
			}
			var d diag.Diagnostics
			slackChannelsList[i], d = datasource_budget.NewRecipientsSlackChannelsValue(datasource_budget.RecipientsSlackChannelsValue{}.AttributeTypes(ctx), slackAttrs)
			diags.Append(d...)
		}
		slackChannelsListValue, d := types.ListValueFrom(ctx, datasource_budget.RecipientsSlackChannelsValue{}.Type(ctx), slackChannelsList)
		diags.Append(d...)
		data.RecipientsSlackChannels = slackChannelsListValue
	} else {
		data.RecipientsSlackChannels = types.ListNull(datasource_budget.RecipientsSlackChannelsValue{}.Type(ctx))
	}

	// Map scope list
	if budget.Scope != nil {
		scopeList, listDiags := types.ListValueFrom(ctx, types.StringType, *budget.Scope)
		diags.Append(listDiags...)
		data.Scope = scopeList
	} else {
		data.Scope = types.ListNull(types.StringType)
	}

	// Map scopes list
	if len(budget.Scopes) > 0 {
		scopesList := make([]datasource_budget.ScopesValue, len(budget.Scopes))
		for i, scope := range budget.Scopes {
			var valuesVal types.List
			if scope.Values != nil {
				var listDiags diag.Diagnostics
				valuesVal, listDiags = types.ListValueFrom(ctx, types.StringType, *scope.Values)
				diags.Append(listDiags...)
			} else {
				valuesVal = types.ListNull(types.StringType)
			}

			scopeAttrs := map[string]attr.Value{
				"id":      types.StringValue(scope.Id),
				"inverse": types.BoolPointerValue(scope.Inverse),
				"mode":    types.StringValue(string(scope.Mode)),
				"type":    types.StringValue(string(scope.Type)),
				"values":  valuesVal,
			}
			var d diag.Diagnostics
			scopesList[i], d = datasource_budget.NewScopesValue(datasource_budget.ScopesValue{}.AttributeTypes(ctx), scopeAttrs)
			diags.Append(d...)
		}
		scopesListValue, d := types.ListValueFrom(ctx, datasource_budget.ScopesValue{}.Type(ctx), scopesList)
		diags.Append(d...)
		data.Scopes = scopesListValue
	} else {
		data.Scopes = types.ListNull(datasource_budget.ScopesValue{}.Type(ctx))
	}

	// Map seasonal_amounts list
	if budget.SeasonalAmounts != nil {
		seasonalAmountsList, listDiags := types.ListValueFrom(ctx, types.Float64Type, *budget.SeasonalAmounts)
		diags.Append(listDiags...)
		data.SeasonalAmounts = seasonalAmountsList
	} else {
		data.SeasonalAmounts = types.ListNull(types.Float64Type)
	}

	return diags
}
