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

// overlayBudgetComputedFields preserves all user-configured values from the Terraform
// plan and selectively overlays only server-assigned Computed fields from the API response.
//
// Computed-only fields (always from API):
//   - id, create_time, update_time, current_utilization, forecasted_utilization
//   - alerts[].forecasted_date, alerts[].triggered
//
// Optional+Computed fields: only resolved from the API when IsUnknown() (user omitted them).
// Known values are never touched — the user's plan is the source of truth.
//
// This prevents "Provider produced inconsistent result" errors caused by the API
// normalizing user-provided values (e.g. stripping [Service N/A] sentinels,
// renaming alias types like allocation_rule → attribution).
func overlayBudgetComputedFields(ctx context.Context, apiResp *models.BudgetAPI, plan *budgetResourceModel) diag.Diagnostics {
	var diags diag.Diagnostics

	// ── Computed-only fields: ALWAYS set from API response ──
	plan.Id = types.StringPointerValue(apiResp.Id)
	plan.CreateTime = types.Int64PointerValue(apiResp.CreateTime)
	plan.UpdateTime = types.Int64PointerValue(apiResp.UpdateTime)
	plan.CurrentUtilization = types.Float64PointerValue(apiResp.CurrentUtilization)
	plan.ForecastedUtilization = types.Float64PointerValue(apiResp.ForecastedUtilization)

	// ── Alerts: mixed Computed + user-configurable ──
	// The user sets percentage; the API computes forecasted_date and triggered.
	// We preserve the user's percentages and overlay the computed sub-fields.
	if !plan.Alerts.IsNull() && !plan.Alerts.IsUnknown() {
		var planAlerts []resource_budget.AlertsValue
		alertsDiags := plan.Alerts.ElementsAs(ctx, &planAlerts, false)
		diags.Append(alertsDiags...)
		if !alertsDiags.HasError() && len(planAlerts) > 0 {
			changed := false
			for i := range planAlerts {
				// Overlay computed sub-fields from the API response.
				var apiAlert *models.ExternalBudgetAlert
				if apiResp.Alerts != nil && i < len(*apiResp.Alerts) {
					apiAlert = &(*apiResp.Alerts)[i]
				}

				// forecasted_date and triggered are Computed-only: always overlay.
				forecastedDate := types.Int64Null()
				triggered := types.BoolNull()
				if apiAlert != nil {
					forecastedDate = types.Int64PointerValue(apiAlert.ForecastedDate)
					triggered = types.BoolPointerValue(apiAlert.Triggered)
				}

				// percentage is Optional+Computed: resolve only when unknown.
				percentage := planAlerts[i].Percentage
				if percentage.IsUnknown() {
					if apiAlert != nil {
						percentage = types.Float64PointerValue(apiAlert.Percentage)
					} else {
						percentage = types.Float64Null()
					}
				}

				// Rebuild the alert value with preserved percentage + overlaid computed fields.
				alertAttrs := map[string]attr.Value{
					"forecasted_date": forecastedDate,
					"percentage":      percentage,
					"triggered":       triggered,
				}
				var d diag.Diagnostics
				planAlerts[i], d = resource_budget.NewAlertsValue(resource_budget.AlertsValue{}.AttributeTypes(ctx), alertAttrs)
				diags.Append(d...)
				changed = true
			}
			if changed {
				alertsListValue, d := types.ListValueFrom(ctx, resource_budget.AlertsValue{}.Type(ctx), planAlerts)
				diags.Append(d...)
				plan.Alerts = alertsListValue
			}
		}
	} else if plan.Alerts.IsUnknown() {
		// User omitted alerts entirely — resolve from API response since the
		// API provides default alerts.
		if apiResp.Alerts != nil && len(*apiResp.Alerts) > 0 {
			alertsList := make([]resource_budget.AlertsValue, len(*apiResp.Alerts))
			for i, alert := range *apiResp.Alerts {
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
			plan.Alerts = alertsListValue
		} else {
			emptyAlerts, d := types.ListValueFrom(ctx, resource_budget.AlertsValue{}.Type(ctx), []resource_budget.AlertsValue{})
			diags.Append(d...)
			plan.Alerts = emptyAlerts
		}
	}

	// ── Optional+Computed scalar fields: resolve only when unknown ──

	if plan.Amount.IsUnknown() {
		plan.Amount = types.Float64PointerValue(apiResp.Amount)
	}
	if plan.Currency.IsUnknown() {
		plan.Currency = types.StringValue(string(apiResp.Currency))
	}
	if plan.Description.IsUnknown() {
		plan.Description = types.StringPointerValue(apiResp.Description)
	}
	if plan.EndPeriod.IsUnknown() {
		if apiResp.EndPeriod != nil && *apiResp.EndPeriod > 0 {
			plan.EndPeriod = types.Int64PointerValue(apiResp.EndPeriod)
		} else {
			plan.EndPeriod = types.Int64Null()
		}
	}
	if plan.GrowthPerPeriod.IsUnknown() {
		plan.GrowthPerPeriod = types.Float64PointerValue(apiResp.GrowthPerPeriod)
	}
	if plan.Metric.IsUnknown() {
		plan.Metric = types.StringPointerValue(apiResp.Metric)
	}
	if plan.Name.IsUnknown() {
		plan.Name = types.StringValue(apiResp.Name)
	}
	if plan.Public.IsUnknown() {
		if apiResp.Public != nil && *apiResp.Public != "" {
			plan.Public = types.StringValue(string(*apiResp.Public))
		} else {
			plan.Public = types.StringNull()
		}
	}
	if plan.StartPeriod.IsUnknown() {
		plan.StartPeriod = types.Int64Value(apiResp.StartPeriod)
	}
	if plan.TimeInterval.IsUnknown() {
		plan.TimeInterval = types.StringValue(apiResp.TimeInterval)
	}
	if plan.Type.IsUnknown() {
		plan.Type = types.StringValue(apiResp.Type)
	}
	if plan.UsePrevSpend.IsUnknown() {
		plan.UsePrevSpend = types.BoolPointerValue(apiResp.UsePrevSpend)
	}

	// ── Optional+Computed list fields: resolve only when unknown ──
	// Known lists (including []) are never touched.
	// Lists where the API auto-populates defaults (e.g. collaborators adds creator
	// as owner, recipients may be auto-set) must resolve from the API response
	// to capture those defaults. Lists where the API returns null/empty when
	// omitted can safely resolve to null.

	if plan.Collaborators.IsUnknown() {
		// API auto-adds creator as owner — resolve from API response.
		if apiResp.Collaborators != nil && len(*apiResp.Collaborators) > 0 {
			collabsList := make([]resource_budget.CollaboratorsValue, len(*apiResp.Collaborators))
			for i, collab := range *apiResp.Collaborators {
				collabAttrs := map[string]attr.Value{
					"email": types.StringPointerValue(collab.Email),
					"role":  types.StringPointerValue((*string)(collab.Role)),
				}
				var d diag.Diagnostics
				collabsList[i], d = resource_budget.NewCollaboratorsValue(resource_budget.CollaboratorsValue{}.AttributeTypes(ctx), collabAttrs)
				diags.Append(d...)
			}
			collabsListValue, d := types.ListValueFrom(ctx, resource_budget.CollaboratorsValue{}.Type(ctx), collabsList)
			diags.Append(d...)
			plan.Collaborators = collabsListValue
		} else {
			emptyCollabs, d := types.ListValueFrom(ctx, resource_budget.CollaboratorsValue{}.Type(ctx), []resource_budget.CollaboratorsValue{})
			diags.Append(d...)
			plan.Collaborators = emptyCollabs
		}
	}
	if plan.Recipients.IsUnknown() {
		// API may auto-populate recipients — resolve from API response.
		if apiResp.Recipients != nil {
			recipientsList, d := types.ListValueFrom(ctx, types.StringType, *apiResp.Recipients)
			diags.Append(d...)
			plan.Recipients = recipientsList
		} else {
			var emptyDiags diag.Diagnostics
			plan.Recipients, emptyDiags = types.ListValue(types.StringType, []attr.Value{})
			diags.Append(emptyDiags...)
		}
	}
	if plan.RecipientsSlackChannels.IsUnknown() {
		// API may auto-populate Slack channels — resolve from API response when present.
		if apiResp.RecipientsSlackChannels != nil && len(*apiResp.RecipientsSlackChannels) > 0 {
			channelsList := make([]resource_budget.RecipientsSlackChannelsValue, len(*apiResp.RecipientsSlackChannels))
			for i, channel := range *apiResp.RecipientsSlackChannels {
				channelAttrs := map[string]attr.Value{
					"customer_id": types.StringPointerValue(channel.CustomerId),
					"id":          types.StringPointerValue(channel.Id),
					"name":        types.StringPointerValue(channel.Name),
					"shared":      types.BoolPointerValue(channel.Shared),
					"type":        types.StringPointerValue(channel.Type),
					"workspace":   types.StringPointerValue(channel.Workspace),
				}
				var d diag.Diagnostics
				channelsList[i], d = resource_budget.NewRecipientsSlackChannelsValue(
					resource_budget.RecipientsSlackChannelsValue{}.AttributeTypes(ctx), channelAttrs)
				diags.Append(d...)
			}
			channelsListValue, d := types.ListValueFrom(ctx, resource_budget.RecipientsSlackChannelsValue{}.Type(ctx), channelsList)
			diags.Append(d...)
			plan.RecipientsSlackChannels = channelsListValue
		} else {
			// Resolve to empty list (not null) to match mapBudgetToModel Read path.
			emptyChannels, d := types.ListValueFrom(ctx, resource_budget.RecipientsSlackChannelsValue{}.Type(ctx), []resource_budget.RecipientsSlackChannelsValue{})
			diags.Append(d...)
			plan.RecipientsSlackChannels = emptyChannels
		}
	}
	if plan.Scope.IsUnknown() {
		// Resolve to empty list (not null) to match mapBudgetToModel Read path.
		var emptyDiags diag.Diagnostics
		plan.Scope, emptyDiags = types.ListValue(types.StringType, []attr.Value{})
		diags.Append(emptyDiags...)
	}
	if plan.Scopes.IsUnknown() {
		// Resolve to empty list (not null) to match mapBudgetToModel Read path.
		emptyScopes, d := types.ListValueFrom(ctx, resource_budget.ScopesValue{}.Type(ctx), []resource_budget.ScopesValue{})
		diags.Append(d...)
		plan.Scopes = emptyScopes
	}
	if plan.SeasonalAmounts.IsUnknown() {
		// Resolve to empty list (not null) to match mapBudgetToModel Read path.
		var emptyDiags diag.Diagnostics
		plan.SeasonalAmounts, emptyDiags = types.ListValue(types.Float64Type, []attr.Value{})
		diags.Append(emptyDiags...)
	}

	// ── Resolve unknowns inside scopes[] elements ──
	// Scopes have Optional+Computed boolean fields (inverse, include_null, case_insensitive)
	// and an Optional+Computed list field (values) that arrive as Unknown when the user
	// omits them. Resolve from API response to match mapBudgetToModel Read path.
	var apiScopes []models.ExternalConfigFilter
	if len(apiResp.Scopes) > 0 {
		apiScopes = apiResp.Scopes
	}
	if !plan.Scopes.IsNull() && !plan.Scopes.IsUnknown() {
		var planScopes []resource_budget.ScopesValue
		scopesDiags := plan.Scopes.ElementsAs(ctx, &planScopes, false)
		diags.Append(scopesDiags...)
		if !scopesDiags.HasError() {
			changed := false
			for i := range planScopes {
				if planScopes[i].Inverse.IsUnknown() {
					if i < len(apiScopes) {
						planScopes[i].Inverse = types.BoolPointerValue(apiScopes[i].Inverse)
					} else {
						planScopes[i].Inverse = types.BoolValue(false)
					}
					changed = true
				}
				if planScopes[i].IncludeNull.IsUnknown() {
					if i < len(apiScopes) {
						planScopes[i].IncludeNull = types.BoolPointerValue(apiScopes[i].IncludeNull)
					} else {
						planScopes[i].IncludeNull = types.BoolValue(false)
					}
					changed = true
				}
				if planScopes[i].CaseInsensitive.IsUnknown() {
					if i < len(apiScopes) {
						planScopes[i].CaseInsensitive = types.BoolPointerValue(apiScopes[i].CaseInsensitive)
					} else {
						planScopes[i].CaseInsensitive = types.BoolValue(false)
					}
					changed = true
				}
				if planScopes[i].Values.IsUnknown() {
					var emptyDiags diag.Diagnostics
					planScopes[i].Values, emptyDiags = types.ListValue(types.StringType, []attr.Value{})
					diags.Append(emptyDiags...)
					changed = true
				}
				if planScopes[i].Id.IsUnknown() {
					planScopes[i].Id = types.StringNull()
					changed = true
				}
				if planScopes[i].Mode.IsUnknown() {
					planScopes[i].Mode = types.StringNull()
					changed = true
				}
				if planScopes[i].ScopesType.IsUnknown() {
					planScopes[i].ScopesType = types.StringNull()
					changed = true
				}
			}
			if changed {
				// Rebuild the scopes list — framework treats list elements as immutable.
				scopesListValue, d := types.ListValueFrom(ctx, resource_budget.ScopesValue{}.Type(ctx), planScopes)
				diags.Append(d...)
				plan.Scopes = scopesListValue
			}
		}
	}

	// ── Resolve unknowns inside collaborators[] elements ──
	if !plan.Collaborators.IsNull() && !plan.Collaborators.IsUnknown() {
		var planCollabs []resource_budget.CollaboratorsValue
		collabsDiags := plan.Collaborators.ElementsAs(ctx, &planCollabs, false)
		diags.Append(collabsDiags...)
		if !collabsDiags.HasError() {
			changed := false
			for i := range planCollabs {
				if planCollabs[i].Email.IsUnknown() {
					planCollabs[i].Email = types.StringNull()
					changed = true
				}
				if planCollabs[i].Role.IsUnknown() {
					planCollabs[i].Role = types.StringNull()
					changed = true
				}
			}
			if changed {
				collabsListValue, d := types.ListValueFrom(ctx, resource_budget.CollaboratorsValue{}.Type(ctx), planCollabs)
				diags.Append(d...)
				plan.Collaborators = collabsListValue
			}
		}
	}

	// ── Resolve unknowns inside recipients_slack_channels[] elements ──
	// Resolve computed subfields from API response by index matching.
	var apiChannels []models.SlackChannel
	if apiResp.RecipientsSlackChannels != nil {
		apiChannels = *apiResp.RecipientsSlackChannels
	}
	if !plan.RecipientsSlackChannels.IsNull() && !plan.RecipientsSlackChannels.IsUnknown() {
		var planChannels []resource_budget.RecipientsSlackChannelsValue
		channelsDiags := plan.RecipientsSlackChannels.ElementsAs(ctx, &planChannels, false)
		diags.Append(channelsDiags...)
		if !channelsDiags.HasError() {
			changed := false
			for i := range planChannels {
				// Find matching API channel by index (order is preserved by API).
				var apiCh *models.SlackChannel
				if i < len(apiChannels) {
					apiCh = &apiChannels[i]
				}

				if planChannels[i].CustomerId.IsUnknown() {
					if apiCh != nil {
						planChannels[i].CustomerId = types.StringPointerValue(apiCh.CustomerId)
					} else {
						planChannels[i].CustomerId = types.StringNull()
					}
					changed = true
				}
				if planChannels[i].Id.IsUnknown() {
					if apiCh != nil {
						planChannels[i].Id = types.StringPointerValue(apiCh.Id)
					} else {
						planChannels[i].Id = types.StringNull()
					}
					changed = true
				}
				if planChannels[i].Name.IsUnknown() {
					if apiCh != nil {
						planChannels[i].Name = types.StringPointerValue(apiCh.Name)
					} else {
						planChannels[i].Name = types.StringNull()
					}
					changed = true
				}
				if planChannels[i].Shared.IsUnknown() {
					if apiCh != nil {
						planChannels[i].Shared = types.BoolPointerValue(apiCh.Shared)
					} else {
						planChannels[i].Shared = types.BoolValue(false)
					}
					changed = true
				}
				if planChannels[i].RecipientsSlackChannelsType.IsUnknown() {
					if apiCh != nil {
						planChannels[i].RecipientsSlackChannelsType = types.StringPointerValue(apiCh.Type)
					} else {
						planChannels[i].RecipientsSlackChannelsType = types.StringNull()
					}
					changed = true
				}
				if planChannels[i].Workspace.IsUnknown() {
					if apiCh != nil {
						planChannels[i].Workspace = types.StringPointerValue(apiCh.Workspace)
					} else {
						planChannels[i].Workspace = types.StringNull()
					}
					changed = true
				}
			}
			if changed {
				channelsListValue, d := types.ListValueFrom(ctx, resource_budget.RecipientsSlackChannelsValue{}.Type(ctx), planChannels)
				diags.Append(d...)
				plan.RecipientsSlackChannels = channelsListValue
			}
		}
	}

	return diags
}

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
				CaseInsensitive: scope.CaseInsensitive.ValueBoolPointer(),
				Id:              scope.Id.ValueString(),
				IncludeNull:     scope.IncludeNull.ValueBoolPointer(),
				Inverse:         scope.Inverse.ValueBoolPointer(),
				Mode:            filterMode,
				Type:            filterType,
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

// mapBudgetToModel maps the full API response to the Terraform model.
// This is used ONLY by Read and ImportState — Create/Update use overlayBudgetComputedFields instead.
// This function contains sentinel restoration and alias normalization for the Read path.
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
		// Extract existing scope types, IDs, and includeNull from state for alias normalization
		// and preserving user-configured values the API does not echo back.
		// When called from Create/Update, state already contains the user's plan values
		// (e.g. "allocation_rule"), while the API returns canonical names ("attribution").
		var existingScopeTypes []string
		var existingScopeIDs []string
		var existingScopeIncludeNull []*bool
		var existingScopeCaseInsensitive []*bool
		var existingScopeValues []types.List
		if !state.Scopes.IsNull() && !state.Scopes.IsUnknown() {
			var existingScopes []resource_budget.ScopesValue
			if d := state.Scopes.ElementsAs(ctx, &existingScopes, false); !d.HasError() {
				for _, es := range existingScopes {
					existingScopeTypes = append(existingScopeTypes, es.ScopesType.ValueString())
					existingScopeIDs = append(existingScopeIDs, es.Id.ValueString())
					existingScopeIncludeNull = append(existingScopeIncludeNull, es.IncludeNull.ValueBoolPointer())
					existingScopeCaseInsensitive = append(existingScopeCaseInsensitive, es.CaseInsensitive.ValueBoolPointer())
					existingScopeValues = append(existingScopeValues, es.Values)
				}
			}
		}

		scopesList := make([]resource_budget.ScopesValue, len(resp.Scopes))
		for i, scope := range resp.Scopes {
			// Sentinel-restoration logic (mirrors report.go):
			// The API strips legacy "[... N/A]" NullFallback sentinels from scope values
			// and converts them to includeNull=true. We restore any stripped sentinels
			// by comparing the API response against the prior state.
			// See: https://doitintl.atlassian.net/browse/CMP-38116
			apiIncludeNull := scope.IncludeNull != nil && *scope.IncludeNull
			var apiValues []string
			if scope.Values != nil {
				apiValues = *scope.Values
			}
			mergedValues := apiValues
			if i < len(existingScopeValues) && !existingScopeValues[i].IsNull() && !existingScopeValues[i].IsUnknown() {
				var stateVals []string
				if d := existingScopeValues[i].ElementsAs(ctx, &stateVals, false); !d.HasError() {
					mergedValues = mergeSentinelValues(apiValues, stateVals, apiIncludeNull)
				}
			}
			var valuesVal types.List
			if len(mergedValues) > 0 {
				var listDiags diag.Diagnostics
				valuesVal, listDiags = types.ListValueFrom(ctx, types.StringType, mergedValues)
				diags.Append(listDiags...)
			} else {
				var emptyDiags diag.Diagnostics
				valuesVal, emptyDiags = types.ListValue(types.StringType, []attr.Value{})
				diags.Append(emptyDiags...)
			}

			scopeType := string(scope.Type)
			scopeID := scope.Id
			// Normalize alias types and IDs to preserve user's configured value.
			// E.g. user configures "allocation_rule", API returns "attribution" — preserve user's value.
			if i < len(existingScopeTypes) {
				scopeType = normalizeDimensionsType(scopeType, existingScopeTypes[i])
			}
			if i < len(existingScopeIDs) {
				scopeID = normalizeDimensionsType(scopeID, existingScopeIDs[i])
			}

			// The API does not reliably echo includeNull — it returns false as a default
			// regardless of the value sent. Always prefer the plan/state value when available.
			// The API response is only used as a fallback (e.g., during ImportState when there
			// is no prior plan/state).
			includeNullVal := types.BoolValue(false)
			if i < len(existingScopeIncludeNull) && existingScopeIncludeNull[i] != nil {
				includeNullVal = types.BoolValue(*existingScopeIncludeNull[i])
			} else if scope.IncludeNull != nil {
				includeNullVal = types.BoolValue(*scope.IncludeNull)
			}

			// The API may not reliably echo caseInsensitive — always prefer the plan/state
			// value when available.
			caseInsensitiveVal := types.BoolValue(false)
			if i < len(existingScopeCaseInsensitive) && existingScopeCaseInsensitive[i] != nil {
				caseInsensitiveVal = types.BoolValue(*existingScopeCaseInsensitive[i])
			} else if scope.CaseInsensitive != nil {
				caseInsensitiveVal = types.BoolValue(*scope.CaseInsensitive)
			}

			scopeAttrs := map[string]attr.Value{
				"case_insensitive": caseInsensitiveVal,
				"id":               types.StringValue(scopeID),
				"include_null":     includeNullVal,
				"inverse":          types.BoolPointerValue(scope.Inverse),
				"mode":             types.StringValue(string(scope.Mode)),
				"type":             types.StringValue(scopeType),
				"values":           valuesVal,
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
