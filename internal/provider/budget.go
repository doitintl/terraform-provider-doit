package provider

import (
	"context"
	"fmt"

	"github.com/doitintl/terraform-provider-doit/internal/provider/models"
	"github.com/doitintl/terraform-provider-doit/internal/provider/resource_budget"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// toUpdateRequest converts the Terraform model to the API BudgetCreateUpdateRequest.
// This is used for both create and update operations since they use the same request type.
func (plan *budgetResourceModel) toUpdateRequest(ctx context.Context) (req models.BudgetCreateUpdateRequest, diags diag.Diagnostics) {
	// Convert alerts
	if !plan.Alerts.IsNull() && !plan.Alerts.IsUnknown() {
		var alerts []resource_budget.AlertsValue
		diags.Append(plan.Alerts.ElementsAs(ctx, &alerts, false)...)
		if diags.HasError() {
			return req, diags
		}

		reqAlerts := make([]models.BudgetCreateUpdateAlert, len(alerts))
		for i, alert := range alerts {
			reqAlerts[i] = models.BudgetCreateUpdateAlert{
				Percentage: alert.Percentage.ValueFloat64Pointer(),
			}
		}
		req.Alerts = &reqAlerts
	}

	// Convert collaborators
	if !plan.Collaborators.IsNull() && !plan.Collaborators.IsUnknown() {
		var collaborators []resource_budget.CollaboratorsValue
		diags.Append(plan.Collaborators.ElementsAs(ctx, &collaborators, false)...)
		if diags.HasError() {
			return req, diags
		}

		reqCollaborators := make([]models.Collaborator, len(collaborators))
		for i, collaborator := range collaborators {
			role := models.CollaboratorRole(collaborator.Role.ValueString())
			reqCollaborators[i] = models.Collaborator{
				Email: collaborator.Email.ValueStringPointer(),
				Role:  &role,
			}
		}
		req.Collaborators = &reqCollaborators
	}

	// Simple fields
	req.Amount = plan.Amount.ValueFloat64Pointer()
	if !plan.Currency.IsNull() && !plan.Currency.IsUnknown() {
		currency := models.Currency(plan.Currency.ValueString())
		req.Currency = &currency
	}
	req.Description = plan.Description.ValueStringPointer()

	// Only set EndPeriod if it's not null/unknown
	if !plan.EndPeriod.IsNull() && !plan.EndPeriod.IsUnknown() {
		req.EndPeriod = plan.EndPeriod.ValueInt64Pointer()
	}

	req.GrowthPerPeriod = plan.GrowthPerPeriod.ValueFloat64Pointer()
	if !plan.Metric.IsNull() && !plan.Metric.IsUnknown() {
		metric := models.BudgetCreateUpdateRequestMetric(plan.Metric.ValueString())
		req.Metric = &metric
	}
	req.Name = plan.Name.ValueStringPointer()

	if !plan.Public.IsNull() && !plan.Public.IsUnknown() {
		public := models.BudgetCreateUpdateRequestPublic(plan.Public.ValueString())
		req.Public = &public
	}

	if !plan.Recipients.IsNull() && !plan.Recipients.IsUnknown() {
		var recipients []string
		diags.Append(plan.Recipients.ElementsAs(ctx, &recipients, false)...)
		if diags.HasError() {
			return req, diags
		}
		req.Recipients = &recipients
	}

	// RecipientsSlackChannels
	if !plan.RecipientsSlackChannels.IsNull() && !plan.RecipientsSlackChannels.IsUnknown() {
		var slackChannels []resource_budget.RecipientsSlackChannelsValue
		diags.Append(plan.RecipientsSlackChannels.ElementsAs(ctx, &slackChannels, false)...)
		if diags.HasError() {
			return req, diags
		}

		reqSlackChannels := make([]models.SlackChannel, len(slackChannels))
		for i, channel := range slackChannels {
			reqSlackChannels[i] = models.SlackChannel{
				CustomerId: channel.CustomerId.ValueStringPointer(),
				Id:         channel.Id.ValueStringPointer(),
				Name:       channel.Name.ValueStringPointer(),
				Shared:     channel.Shared.ValueBoolPointer(),
				Workspace:  channel.Workspace.ValueStringPointer(),
				Type:       channel.RecipientsSlackChannelsType.ValueStringPointer(),
			}
		}
		req.RecipientsSlackChannels = &reqSlackChannels
	}

	if !plan.Scope.IsNull() && !plan.Scope.IsUnknown() {
		var scope []string
		diags.Append(plan.Scope.ElementsAs(ctx, &scope, false)...)
		if diags.HasError() {
			return req, diags
		}
		req.Scope = &scope
	}

	if !plan.Scopes.IsNull() && !plan.Scopes.IsUnknown() {
		var scopes []resource_budget.ScopesValue
		diags.Append(plan.Scopes.ElementsAs(ctx, &scopes, false)...)
		if diags.HasError() {
			return req, diags
		}

		reqScopes := make([]models.ExternalConfigFilter, len(scopes))
		for i, scope := range scopes {
			filterType := models.DimensionsTypes(scope.ScopesType.ValueString())
			filterMode := models.ExternalConfigFilterMode(scope.Mode.ValueString())

			reqScopes[i] = models.ExternalConfigFilter{
				Id:      scope.Id.ValueString(),
				Inverse: scope.Inverse.ValueBoolPointer(),
				Mode:    filterMode,
				Type:    filterType,
			}
			if !scope.Values.IsNull() && !scope.Values.IsUnknown() {
				var values []string
				diags.Append(scope.Values.ElementsAs(ctx, &values, false)...)
				if diags.HasError() {
					return req, diags
				}
				reqScopes[i].Values = &values
			}
		}
		req.Scopes = &reqScopes // Note: models.Scopes is now []ExternalConfigFilter, not a pointer to slice
	}

	if !plan.SeasonalAmounts.IsNull() && !plan.SeasonalAmounts.IsUnknown() {
		var seasonalAmounts []float64
		diags.Append(plan.SeasonalAmounts.ElementsAs(ctx, &seasonalAmounts, false)...)
		if diags.HasError() {
			return req, diags
		}
		req.SeasonalAmounts = &seasonalAmounts
	}

	// Only set StartPeriod if it's not null/unknown
	if !plan.StartPeriod.IsNull() && !plan.StartPeriod.IsUnknown() {
		req.StartPeriod = plan.StartPeriod.ValueInt64Pointer()
	}

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
		diags.AddError(
			"Error Reading Budget",
			"Could not read budget ID "+state.Id.ValueString()+": "+err.Error(),
		)
		return
	}

	// Handle externally deleted resource - remove from state
	if budgetResp.StatusCode() == 404 {
		state.Id = types.StringNull()
		return
	}

	// Check for successful response
	if budgetResp.StatusCode() != 200 {
		diags.AddError(
			"Error Reading Budget",
			fmt.Sprintf("Unexpected status code %d for budget ID %s: %s",
				budgetResp.StatusCode(), state.Id.ValueString(), string(budgetResp.Body)),
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

	return mapBudgetToModel(ctx, resp, state)
}

func mapBudgetToModel(ctx context.Context, resp *models.BudgetAPI, state *budgetResourceModel) (diags diag.Diagnostics) {
	if resp == nil {
		diags.AddError(
			"Error converting budget API response to Terraform model",
			"Received nil response body for budget",
		)
		return diags
	}
	state.Id = types.StringPointerValue(resp.Id)

	// Convert alerts with computed fields (forecasted_date, triggered)
	if resp.Alerts != nil && len(*resp.Alerts) > 0 {
		alertsList := make([]resource_budget.AlertsValue, len(*resp.Alerts))
		for i, alert := range *resp.Alerts {
			alertAttrs := map[string]attr.Value{
				"forecasted_date": types.Int64PointerValue(alert.ForecastedDate),
				"percentage":      types.Float64PointerValue(alert.Percentage),
				"triggered":       types.BoolPointerValue(alert.Triggered),
			}
			var d diag.Diagnostics
			alertsList[i], d = resource_budget.NewAlertsValue(resource_budget.AlertsValue{}.AttributeTypes(ctx), alertAttrs)
			diags.Append(d...)
		}
		alertsListValue, d := types.ListValueFrom(ctx, resource_budget.AlertsValue{}.Type(ctx), alertsList)
		diags.Append(d...)
		state.Alerts = alertsListValue
	} else {
		// API returned nil or empty slice - return empty list to avoid inconsistent result if user sets [].
		// Pattern B: Normalize to empty list for user-configurable attributes.
		emptyAlerts, d := types.ListValueFrom(ctx, resource_budget.AlertsValue{}.Type(ctx), []resource_budget.AlertsValue{})
		diags.Append(d...)
		state.Alerts = emptyAlerts
	}

	state.Amount = types.Float64PointerValue(resp.Amount)

	// Convert collaborators
	if resp.Collaborators != nil && len(*resp.Collaborators) > 0 {
		collaboratorsList := make([]resource_budget.CollaboratorsValue, len(*resp.Collaborators))
		for i, collab := range *resp.Collaborators {
			collabAttrs := map[string]attr.Value{
				"email": types.StringPointerValue(collab.Email),
				"role":  types.StringPointerValue((*string)(collab.Role)),
			}
			var d diag.Diagnostics
			collaboratorsList[i], d = resource_budget.NewCollaboratorsValue(resource_budget.CollaboratorsValue{}.AttributeTypes(ctx), collabAttrs)
			diags.Append(d...)
		}
		collaboratorsListValue, d := types.ListValueFrom(ctx, resource_budget.CollaboratorsValue{}.Type(ctx), collaboratorsList)
		diags.Append(d...)
		state.Collaborators = collaboratorsListValue
	} else {
		// Return empty list for nil to avoid inconsistent result if user sets []
		emptyCollabs, d := types.ListValueFrom(ctx, resource_budget.CollaboratorsValue{}.Type(ctx), []resource_budget.CollaboratorsValue{})
		diags.Append(d...)
		state.Collaborators = emptyCollabs
	}

	state.Currency = types.StringValue(string(resp.Currency))
	state.Description = types.StringPointerValue(resp.Description)

	if resp.EndPeriod != nil && *resp.EndPeriod > 0 {
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
		// Return empty list for nil to avoid inconsistent result if user sets []
		var emptyDiags diag.Diagnostics
		state.Recipients, emptyDiags = types.ListValue(types.StringType, []attr.Value{})
		diags.Append(emptyDiags...)
	}

	// Convert recipients_slack_channels
	if resp.RecipientsSlackChannels != nil && len(*resp.RecipientsSlackChannels) > 0 {
		slackChannelsList := make([]resource_budget.RecipientsSlackChannelsValue, len(*resp.RecipientsSlackChannels))
		for i, slack := range *resp.RecipientsSlackChannels {
			slackAttrs := map[string]attr.Value{
				"customer_id": types.StringPointerValue(slack.CustomerId),
				"id":          types.StringPointerValue(slack.Id),
				"name":        types.StringPointerValue(slack.Name),
				"shared":      types.BoolPointerValue(slack.Shared),
				"type":        types.StringPointerValue(slack.Type),
				"workspace":   types.StringPointerValue(slack.Workspace),
			}
			var d diag.Diagnostics
			slackChannelsList[i], d = resource_budget.NewRecipientsSlackChannelsValue(resource_budget.RecipientsSlackChannelsValue{}.AttributeTypes(ctx), slackAttrs)
			diags.Append(d...)
		}
		slackChannelsListValue, d := types.ListValueFrom(ctx, resource_budget.RecipientsSlackChannelsValue{}.Type(ctx), slackChannelsList)
		diags.Append(d...)
		state.RecipientsSlackChannels = slackChannelsListValue
	} else {
		// Return empty list for nil to avoid inconsistent result if user sets []
		emptyChannels, d := types.ListValueFrom(ctx, resource_budget.RecipientsSlackChannelsValue{}.Type(ctx), []resource_budget.RecipientsSlackChannelsValue{})
		diags.Append(d...)
		state.RecipientsSlackChannels = emptyChannels
	}

	// Convert scope list
	if resp.Scope != nil {
		scopeList, listDiags := types.ListValueFrom(ctx, types.StringType, *resp.Scope)
		diags.Append(listDiags...)
		state.Scope = scopeList
	} else {
		// Return empty list for nil to avoid inconsistent result if user sets []
		var emptyDiags diag.Diagnostics
		state.Scope, emptyDiags = types.ListValue(types.StringType, []attr.Value{})
		diags.Append(emptyDiags...)
	}

	// Convert scopes list
	if len(resp.Scopes) > 0 {
		scopesList := make([]resource_budget.ScopesValue, len(resp.Scopes))
		for i, scope := range resp.Scopes {
			var valuesVal types.List
			if scope.Values != nil {
				var listDiags diag.Diagnostics
				valuesVal, listDiags = types.ListValueFrom(ctx, types.StringType, *scope.Values)
				diags.Append(listDiags...)
			} else {
				// Return empty list for nil to avoid inconsistent result if user sets []
				var emptyDiags diag.Diagnostics
				valuesVal, emptyDiags = types.ListValue(types.StringType, []attr.Value{})
				diags.Append(emptyDiags...)
			}

			scopeAttrs := map[string]attr.Value{
				"id":      types.StringValue(scope.Id),
				"inverse": types.BoolPointerValue(scope.Inverse),
				"mode":    types.StringValue(string(scope.Mode)),
				"type":    types.StringValue(string(scope.Type)),
				"values":  valuesVal,
			}
			var d diag.Diagnostics
			scopesList[i], d = resource_budget.NewScopesValue(resource_budget.ScopesValue{}.AttributeTypes(ctx), scopeAttrs)
			diags.Append(d...)
		}
		scopesListValue, d := types.ListValueFrom(ctx, resource_budget.ScopesValue{}.Type(ctx), scopesList)
		diags.Append(d...)
		state.Scopes = scopesListValue
	} else {
		// Return empty list for nil to avoid inconsistent result if user sets []
		emptyScopes, d := types.ListValueFrom(ctx, resource_budget.ScopesValue{}.Type(ctx), []resource_budget.ScopesValue{})
		diags.Append(d...)
		state.Scopes = emptyScopes
	}

	// Convert seasonal_amounts list
	if resp.SeasonalAmounts != nil {
		seasonalAmountsList, listDiags := types.ListValueFrom(ctx, types.Float64Type, *resp.SeasonalAmounts)
		diags.Append(listDiags...)
		state.SeasonalAmounts = seasonalAmountsList
	} else {
		// Return empty list for nil to avoid inconsistent result if user sets []
		var emptyDiags diag.Diagnostics
		state.SeasonalAmounts, emptyDiags = types.ListValue(types.Float64Type, []attr.Value{})
		diags.Append(emptyDiags...)
	}

	state.StartPeriod = types.Int64Value(resp.StartPeriod)
	state.TimeInterval = types.StringValue(resp.TimeInterval)
	state.Type = types.StringValue(resp.Type)
	state.UsePrevSpend = types.BoolPointerValue(resp.UsePrevSpend)

	// Populate read-only fields
	state.CreateTime = types.Int64PointerValue(resp.CreateTime)
	state.UpdateTime = types.Int64PointerValue(resp.UpdateTime)
	state.CurrentUtilization = types.Float64PointerValue(resp.CurrentUtilization)
	state.ForecastedUtilization = types.Float64PointerValue(resp.ForecastedUtilization)

	return diags
}
