package provider

import (
	"context"
	"strings"

	"terraform-provider-doit/internal/provider/models"
	"terraform-provider-doit/internal/provider/resource_budget"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// toUpdateRequest converts the Terraform model to the API BudgetCreateUpdateRequest.
// This is used for both create and update operations since they use the same request type.
func (plan *budgetResourceModel) toUpdateRequest(ctx context.Context) (req models.BudgetCreateUpdateRequest, diags diag.Diagnostics) {
	// Convert alerts
	if !plan.Alerts.IsNull() && !plan.Alerts.IsUnknown() {
		var alerts []resource_budget.AlertsValue
		d := plan.Alerts.ElementsAs(ctx, &alerts, false)
		diags.Append(d...)
		if !diags.HasError() {
			apiAlerts := make([]models.BudgetCreateUpdateAlert, len(alerts))
			for i, alert := range alerts {
				apiAlerts[i] = models.BudgetCreateUpdateAlert{
					Percentage: alert.Percentage.ValueFloat64Pointer(),
				}
			}
			req.Alerts = &apiAlerts
		}
	}

	// Simple field conversions
	req.Amount = plan.Amount.ValueFloat64Pointer()

	// Convert collaborators
	if !plan.Collaborators.IsNull() && !plan.Collaborators.IsUnknown() {
		var collaborators []resource_budget.CollaboratorsValue
		d := plan.Collaborators.ElementsAs(ctx, &collaborators, false)
		diags.Append(d...)
		if !diags.HasError() {
			apiCollaborators := make([]models.Collaborator, len(collaborators))
			for i, collab := range collaborators {
				apiCollaborators[i] = models.Collaborator{
					Email: collab.Email.ValueStringPointer(),
					Role:  (*models.CollaboratorRole)(collab.Role.ValueStringPointer()),
				}
			}
			req.Collaborators = &apiCollaborators
		}
	}

	if !plan.Currency.IsNull() && !plan.Currency.IsUnknown() {
		currency := models.Currency(plan.Currency.ValueString())
		req.Currency = &currency
	}

	req.Description = plan.Description.ValueStringPointer()
	req.EndPeriod = plan.EndPeriod.ValueInt64Pointer()
	req.GrowthPerPeriod = plan.GrowthPerPeriod.ValueFloat64Pointer()
	req.Metric = plan.Metric.ValueStringPointer()
	req.Name = plan.Name.ValueStringPointer()

	if !plan.Public.IsNull() && !plan.Public.IsUnknown() {
		public := models.BudgetCreateUpdateRequestPublic(plan.Public.ValueString())
		req.Public = &public
	}

	// Convert recipients list
	if !plan.Recipients.IsNull() && !plan.Recipients.IsUnknown() {
		var recipients []string
		diags.Append(plan.Recipients.ElementsAs(ctx, &recipients, false)...)
		req.Recipients = &recipients
	}

	// Convert recipients_slack_channels
	if !plan.RecipientsSlackChannels.IsNull() && !plan.RecipientsSlackChannels.IsUnknown() {
		var slackChannels []resource_budget.RecipientsSlackChannelsValue
		d := plan.RecipientsSlackChannels.ElementsAs(ctx, &slackChannels, false)
		diags.Append(d...)
		if !diags.HasError() {
			apiSlackChannels := make([]models.SlackChannel, len(slackChannels))
			for i, slack := range slackChannels {
				apiSlackChannels[i] = models.SlackChannel{
					CustomerId: slack.CustomerId.ValueStringPointer(),
					Id:         slack.Id.ValueStringPointer(),
					Name:       slack.Name.ValueStringPointer(),
					Shared:     slack.Shared.ValueBoolPointer(),
					Type:       slack.RecipientsSlackChannelsType.ValueStringPointer(),
					Workspace:  slack.Workspace.ValueStringPointer(),
				}
			}
			req.RecipientsSlackChannels = &apiSlackChannels
		}
	}

	// Convert scope list
	if !plan.Scope.IsNull() && !plan.Scope.IsUnknown() {
		var scope []string
		diags.Append(plan.Scope.ElementsAs(ctx, &scope, false)...)
		req.Scope = &scope
	}

	// Convert seasonal_amounts list
	if !plan.SeasonalAmounts.IsNull() && !plan.SeasonalAmounts.IsUnknown() {
		var seasonalAmounts []float64
		diags.Append(plan.SeasonalAmounts.ElementsAs(ctx, &seasonalAmounts, false)...)
		req.SeasonalAmounts = &seasonalAmounts
	}

	req.StartPeriod = plan.StartPeriod.ValueInt64Pointer()
	req.TimeInterval = plan.TimeInterval.ValueStringPointer()
	req.Type = plan.Type.ValueStringPointer()
	req.UsePrevSpend = plan.UsePrevSpend.ValueBoolPointer()

	return req, diags
}

// populateState fetches the budget from the API and populates the state.
func (r *budgetResource) populateState(ctx context.Context, state *budgetResourceModel) (diags diag.Diagnostics) {
	// Get refreshed budget value from API
	budgetResp, err := r.client.GetBudgetWithResponse(ctx, state.Id.ValueString())
	if err != nil {
		if strings.Contains(err.Error(), "404") {
			// The resource was deleted
			state.Id = types.StringNull()
			return
		}
		diags.AddError(
			"Error Reading Budget",
			"Could not read budget ID "+state.Id.ValueString()+": "+err.Error(),
		)
		return
	}

	resp := budgetResp.JSON200
	if resp == nil {
		diags.AddError(
			"Error Reading DoiT Budget",
			"Received empty response body for budget ID "+state.Id.ValueString(),
		)
		return
	}

	state.Id = types.StringPointerValue(resp.Id)

	// Convert alerts with computed fields (forecasted_date, triggered)
	if resp.Alerts != nil && len(*resp.Alerts) > 0 {
		alertsList := make([]resource_budget.AlertsValue, len(*resp.Alerts))
		for i, alert := range *resp.Alerts {
			alertsList[i] = resource_budget.AlertsValue{
				ForecastedDate: types.Int64PointerValue(alert.ForecastedDate),
				Percentage:     types.Float64PointerValue(alert.Percentage),
				Triggered:      types.BoolPointerValue(alert.Triggered),
			}
		}
		alertsListValue, d := types.ListValueFrom(ctx, resource_budget.AlertsValue{}.Type(ctx), alertsList)
		diags.Append(d...)
		state.Alerts = alertsListValue
	} else {
		state.Alerts = types.ListNull(resource_budget.AlertsValue{}.Type(ctx))
	}

	state.Amount = types.Float64PointerValue(resp.Amount)

	// Convert collaborators
	if resp.Collaborators != nil && len(*resp.Collaborators) > 0 {
		collaboratorsList := make([]resource_budget.CollaboratorsValue, len(*resp.Collaborators))
		for i, collab := range *resp.Collaborators {
			collaboratorsList[i] = resource_budget.CollaboratorsValue{
				Email: types.StringPointerValue(collab.Email),
				Role:  types.StringPointerValue((*string)(collab.Role)),
			}
		}
		collaboratorsListValue, d := types.ListValueFrom(ctx, resource_budget.CollaboratorsValue{}.Type(ctx), collaboratorsList)
		diags.Append(d...)
		state.Collaborators = collaboratorsListValue
	} else {
		state.Collaborators = types.ListNull(resource_budget.CollaboratorsValue{}.Type(ctx))
	}

	state.Currency = types.StringValue(string(resp.Currency))
	state.Description = types.StringPointerValue(resp.Description)

	if resp.EndPeriod != nil && *resp.EndPeriod > 0 && *resp.EndPeriod != 2678400000 {
		state.EndPeriod = types.Int64PointerValue(resp.EndPeriod)
	} else {
		state.EndPeriod = types.Int64Null()
	}

	state.GrowthPerPeriod = types.Float64PointerValue(resp.GrowthPerPeriod)
	state.Metric = types.StringPointerValue(resp.Metric)
	state.Name = types.StringValue(resp.Name)

	if resp.Public != nil && *resp.Public != "" {
		state.Public = types.StringValue(string(*resp.Public))
	} else {
		state.Public = types.StringNull()
	}

	// Convert recipients list
	if resp.Recipients != nil {
		recipientsList, listDiags := types.ListValueFrom(ctx, types.StringType, *resp.Recipients)
		diags.Append(listDiags...)
		state.Recipients = recipientsList
	} else {
		state.Recipients = types.ListNull(types.StringType)
	}

	// Convert recipients_slack_channels
	if resp.RecipientsSlackChannels != nil && len(*resp.RecipientsSlackChannels) > 0 {
		slackChannelsList := make([]resource_budget.RecipientsSlackChannelsValue, len(*resp.RecipientsSlackChannels))
		for i, slack := range *resp.RecipientsSlackChannels {
			slackChannelsList[i] = resource_budget.RecipientsSlackChannelsValue{
				CustomerId:                  types.StringPointerValue(slack.CustomerId),
				Id:                          types.StringPointerValue(slack.Id),
				Name:                        types.StringPointerValue(slack.Name),
				Shared:                      types.BoolPointerValue(slack.Shared),
				RecipientsSlackChannelsType: types.StringPointerValue(slack.Type),
				Workspace:                   types.StringPointerValue(slack.Workspace),
			}
		}
		slackChannelsListValue, d := types.ListValueFrom(ctx, resource_budget.RecipientsSlackChannelsValue{}.Type(ctx), slackChannelsList)
		diags.Append(d...)
		state.RecipientsSlackChannels = slackChannelsListValue
	} else {
		state.RecipientsSlackChannels = types.ListNull(resource_budget.RecipientsSlackChannelsValue{}.Type(ctx))
	}

	// Convert scope list
	if resp.Scope != nil {
		scopeList, listDiags := types.ListValueFrom(ctx, types.StringType, *resp.Scope)
		diags.Append(listDiags...)
		state.Scope = scopeList
	} else {
		state.Scope = types.ListNull(types.StringType)
	}

	// Convert seasonal_amounts list
	if resp.SeasonalAmounts != nil {
		seasonalAmountsList, listDiags := types.ListValueFrom(ctx, types.Float64Type, *resp.SeasonalAmounts)
		diags.Append(listDiags...)
		state.SeasonalAmounts = seasonalAmountsList
	} else {
		state.SeasonalAmounts = types.ListNull(types.Float64Type)
	}

	state.StartPeriod = types.Int64Value(resp.StartPeriod)
	state.TimeInterval = types.StringValue(resp.TimeInterval)
	state.Type = types.StringValue(resp.Type)
	state.UsePrevSpend = types.BoolPointerValue(resp.UsePrevSpend)

	return diags
}
