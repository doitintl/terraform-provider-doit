package provider

import (
	"context"

	"github.com/doitintl/terraform-provider-doit/internal/provider/models"
	"github.com/doitintl/terraform-provider-doit/internal/provider/resource_alert"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// toAlertRequest converts the Terraform model to the API AlertRequest.
// This is used for create operations.
func (plan *alertResourceModel) toAlertRequest(ctx context.Context) (req models.AlertRequest, diags diag.Diagnostics) {
	req.Name = plan.Name.ValueString()

	// Convert recipients
	if !plan.Recipients.IsNull() && !plan.Recipients.IsUnknown() {
		var recipients []string
		diags.Append(plan.Recipients.ElementsAs(ctx, &recipients, false)...)
		if diags.HasError() {
			return req, diags
		}
		req.Recipients = &recipients
	}

	// Convert config
	req.Config, diags = plan.toAlertConfig(ctx)

	return req, diags
}

// toAlertUpdateRequest converts the Terraform model to the API AlertUpdateRequest.
// This is used for update operations.
func (plan *alertResourceModel) toAlertUpdateRequest(ctx context.Context) (req models.AlertUpdateRequest, diags diag.Diagnostics) {
	name := plan.Name.ValueString()
	req.Name = &name

	// Convert recipients
	if !plan.Recipients.IsNull() && !plan.Recipients.IsUnknown() {
		var recipients []string
		diags.Append(plan.Recipients.ElementsAs(ctx, &recipients, false)...)
		if diags.HasError() {
			return req, diags
		}
		req.Recipients = &recipients
	}

	// Convert config
	req.Config, diags = plan.toAlertConfig(ctx)

	return req, diags
}

// toAlertConfig converts the Terraform config object to the API AlertConfig.
func (plan *alertResourceModel) toAlertConfig(ctx context.Context) (config models.AlertConfig, diags diag.Diagnostics) {
	if plan.Config.IsNull() || plan.Config.IsUnknown() {
		diags.AddError("Config Required", "Alert config is required")
		return config, diags
	}

	// Extract config attributes
	configVal := plan.Config

	// Required fields
	config.Value = configVal.Value.ValueFloat64()
	config.TimeInterval = models.AlertConfigTimeInterval(configVal.TimeInterval.ValueString())

	// Metric (required nested object)
	if !configVal.Metric.IsNull() && !configVal.Metric.IsUnknown() {
		metricVal := configVal.Metric
		config.Metric = models.MetricConfig{
			Type:  metricVal.MetricType.ValueString(),
			Value: metricVal.Value.ValueString(),
		}
	}

	// Optional fields
	if !configVal.Condition.IsNull() && !configVal.Condition.IsUnknown() {
		condition := configVal.Condition.ValueString()
		config.Condition = &condition
	}

	if !configVal.Currency.IsNull() && !configVal.Currency.IsUnknown() {
		currency := models.Currency(configVal.Currency.ValueString())
		config.Currency = &currency
	}

	if !configVal.DataSource.IsNull() && !configVal.DataSource.IsUnknown() {
		dataSource := configVal.DataSource.ValueString()
		config.DataSource = &dataSource
	}

	if !configVal.EvaluateForEach.IsNull() && !configVal.EvaluateForEach.IsUnknown() {
		evaluateForEach := configVal.EvaluateForEach.ValueString()
		config.EvaluateForEach = &evaluateForEach
	}

	if !configVal.Operator.IsNull() && !configVal.Operator.IsUnknown() {
		operator := configVal.Operator.ValueString()
		config.Operator = &operator
	}

	// Attributions
	if !configVal.Attributions.IsNull() && !configVal.Attributions.IsUnknown() {
		var attributions []string
		diags.Append(configVal.Attributions.ElementsAs(ctx, &attributions, false)...)
		if diags.HasError() {
			return config, diags
		}
		config.Attributions = &attributions
	}

	// Scopes
	if !configVal.Scopes.IsNull() && !configVal.Scopes.IsUnknown() {
		var scopes []resource_alert.ScopesValue
		diags.Append(configVal.Scopes.ElementsAs(ctx, &scopes, false)...)
		if diags.HasError() {
			return config, diags
		}

		apiScopes := make([]models.ExternalConfigFilter, len(scopes))
		for i, scope := range scopes {
			filterType := models.DimensionsTypes(scope.ScopesType.ValueString())
			filterMode := models.ExternalConfigFilterMode(scope.Mode.ValueString())

			apiScopes[i] = models.ExternalConfigFilter{
				Id:      scope.Id.ValueString(),
				Inverse: scope.Inverse.ValueBoolPointer(),
				Mode:    filterMode,
				Type:    filterType,
			}
			if !scope.Values.IsNull() && !scope.Values.IsUnknown() {
				var values []string
				diags.Append(scope.Values.ElementsAs(ctx, &values, false)...)
				if diags.HasError() {
					return config, diags
				}
				apiScopes[i].Values = &values
			}
		}
		config.Scopes = &apiScopes
	}

	return config, diags
}

// populateState fetches the alert from the API and populates the state.
func (r *alertResource) populateState(ctx context.Context, state *alertResourceModel) (diags diag.Diagnostics) {
	// Get refreshed alert value from API
	alertResp, err := r.client.GetAlertWithResponse(ctx, state.Id.ValueString())
	if err != nil {
		diags.AddError(
			"Error Reading Alert",
			"Could not read alert ID "+state.Id.ValueString()+": "+err.Error(),
		)
		return
	}

	// Handle externally deleted resource - remove from state
	if alertResp.StatusCode() == 404 {
		state.Id = types.StringNull()
		return
	}

	resp := alertResp.JSON200
	if resp == nil {
		diags.AddError(
			"Error Reading DoiT Alert",
			"Received empty response body for alert ID "+state.Id.ValueString(),
		)
		return
	}

	return mapAlertToModel(ctx, resp, state)
}

// mapAlertToModel maps the API response to the Terraform model.
func mapAlertToModel(ctx context.Context, resp *models.Alert, state *alertResourceModel) (diags diag.Diagnostics) {
	if resp == nil {
		diags.AddError(
			"Error converting alert API response to Terraform model",
			"Received nil response body for alert",
		)
		return diags
	}

	state.Id = types.StringPointerValue(resp.Id)
	state.Name = types.StringValue(resp.Name)
	state.CreateTime = types.Int64PointerValue(resp.CreateTime)
	state.UpdateTime = types.Int64PointerValue(resp.UpdateTime)
	state.LastAlerted = types.Int64PointerValue(resp.LastAlerted)

	// Convert recipients list
	if resp.Recipients != nil {
		recipientsList, listDiags := types.ListValueFrom(ctx, types.StringType, *resp.Recipients)
		diags.Append(listDiags...)
		state.Recipients = recipientsList
	} else {
		state.Recipients = types.ListNull(types.StringType)
	}

	// Convert config
	if resp.Config != nil {
		configVal, configDiags := mapAlertConfigToModel(ctx, resp.Config)
		diags.Append(configDiags...)
		state.Config = configVal
	}

	return diags
}

// mapAlertConfigToModel maps the API AlertConfig to the Terraform ConfigValue.
func mapAlertConfigToModel(ctx context.Context, config *models.AlertConfig) (resource_alert.ConfigValue, diag.Diagnostics) {
	var diags diag.Diagnostics

	// Build attributions list
	var attributionsVal types.List
	if config.Attributions != nil {
		var listDiags diag.Diagnostics
		attributionsVal, listDiags = types.ListValueFrom(ctx, types.StringType, *config.Attributions)
		diags.Append(listDiags...)
	} else {
		attributionsVal = types.ListNull(types.StringType)
	}

	// Build scopes list
	var scopesVal types.List
	if config.Scopes != nil {
		scopesList := make([]resource_alert.ScopesValue, len(*config.Scopes))
		for i, scope := range *config.Scopes {
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
			scopesList[i], d = resource_alert.NewScopesValue(resource_alert.ScopesValue{}.AttributeTypes(ctx), scopeAttrs)
			diags.Append(d...)
		}
		var listDiags diag.Diagnostics
		scopesVal, listDiags = types.ListValueFrom(ctx, resource_alert.ScopesValue{}.Type(ctx), scopesList)
		diags.Append(listDiags...)
	} else {
		scopesVal = types.ListNull(resource_alert.ScopesValue{}.Type(ctx))
	}

	// Build metric value
	metricAttrs := map[string]attr.Value{
		"type":  types.StringValue(config.Metric.Type),
		"value": types.StringValue(config.Metric.Value),
	}
	metricVal, d := resource_alert.NewMetricValue(resource_alert.MetricValue{}.AttributeTypes(ctx), metricAttrs)
	diags.Append(d...)

	// Build config value
	configAttrs := map[string]attr.Value{
		"attributions":      attributionsVal,
		"condition":         types.StringPointerValue(config.Condition),
		"currency":          types.StringPointerValue((*string)(config.Currency)),
		"data_source":       types.StringPointerValue(config.DataSource),
		"evaluate_for_each": types.StringPointerValue(config.EvaluateForEach),
		"metric":            metricVal,
		"operator":          types.StringPointerValue(config.Operator),
		"scopes":            scopesVal,
		"time_interval":     types.StringValue(string(config.TimeInterval)),
		"value":             types.Float64Value(config.Value),
	}

	configVal, d := resource_alert.NewConfigValue(resource_alert.ConfigValue{}.AttributeTypes(ctx), configAttrs)
	diags.Append(d...)

	return configVal, diags
}
