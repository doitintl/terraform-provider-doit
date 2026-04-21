package provider

import (
	"context"

	"github.com/doitintl/terraform-provider-doit/internal/provider/models"
	"github.com/doitintl/terraform-provider-doit/internal/provider/resource_alert"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// overlayAlertComputedFields uses the two-phase overlay pattern to reconcile
// the Terraform plan with the API response after Create/Update.
//
// Phase 1 (Resolve): Build a fully-resolved state from the API response using
// mapAlertToModel — the same mapping function used by Read/ImportState. This
// guarantees consistency between Create/Update and Read paths.
//
// Phase 2 (Overlay): Walk the plan field-by-field. Known (user-configured)
// values are preserved as-is. Unknown (user-omitted) values are replaced with
// the resolved counterpart. Computed-only fields always come from resolved.
//
// Used by: Create, Update
// NOT used by: Read, ImportState (which use populateState / mapAlertToModel directly).
func overlayAlertComputedFields(ctx context.Context, apiResp *models.Alert, plan *alertResourceModel) diag.Diagnostics {
	var diags diag.Diagnostics

	// Phase 1: Build fully-resolved state from API response.
	resolved := *plan
	diags.Append(mapAlertToModel(ctx, apiResp, &resolved)...)
	if diags.HasError() {
		return diags
	}

	// Phase 2: Overlay known plan values on top of resolved state.

	// ── Computed-only fields: always from resolved ──
	plan.Id = resolved.Id
	plan.CreateTime = resolved.CreateTime
	plan.UpdateTime = resolved.UpdateTime
	plan.LastAlerted = resolved.LastAlerted

	// ── Name: Required — never touch ──

	// ── Recipients: Optional+Computed list ──
	if plan.Recipients.IsUnknown() {
		plan.Recipients = resolved.Recipients
	}

	// ── Config: Required nested object — overlay subfields individually ──
	if plan.Config.IsUnknown() {
		plan.Config = resolved.Config
	} else if !plan.Config.IsNull() {
		diags.Append(overlayAlertConfig(ctx, &resolved.Config, &plan.Config)...)
	}

	return diags
}

// overlayAlertConfig overlays Known config subfields from the plan,
// replacing only Unknown values with the resolved counterpart.
func overlayAlertConfig(ctx context.Context, resolved, plan *resource_alert.ConfigValue) diag.Diagnostics {
	var diags diag.Diagnostics

	// ── Optional+Computed scalar fields: only when Unknown ──
	if plan.Attributions.IsUnknown() {
		plan.Attributions = resolved.Attributions
	}
	if plan.Condition.IsUnknown() {
		plan.Condition = resolved.Condition
	}
	if plan.Currency.IsUnknown() {
		plan.Currency = resolved.Currency
	}
	if plan.DataSource.IsUnknown() {
		plan.DataSource = resolved.DataSource
	}
	if plan.EvaluateForEach.IsUnknown() {
		plan.EvaluateForEach = resolved.EvaluateForEach
	}
	if plan.Operator.IsUnknown() {
		plan.Operator = resolved.Operator
	}
	if plan.TimeInterval.IsUnknown() {
		plan.TimeInterval = resolved.TimeInterval
	}

	// ── Metric: Required nested — subfields are both Required, never Unknown ──
	// Defensive: overlay if Unknown.
	if plan.Metric.IsUnknown() {
		plan.Metric = resolved.Metric
	}

	// ── Value: Required float64 — never Unknown ──

	// ── Scopes: Optional+Computed list ──
	if plan.Scopes.IsUnknown() {
		plan.Scopes = resolved.Scopes
	} else if !plan.Scopes.IsNull() {
		diags.Append(overlayListElements(ctx, &resolved.Scopes, &plan.Scopes, overlayAlertScope)...)
	}

	return diags
}

// overlayAlertScope resolves Unknown subfields in alert scope elements.
func overlayAlertScope(_ context.Context, resolved, plan *resource_alert.ScopesValue) diag.Diagnostics {
	if plan.CaseInsensitive.IsUnknown() {
		plan.CaseInsensitive = resolved.CaseInsensitive
	}
	if plan.Id.IsUnknown() {
		plan.Id = resolved.Id
	}
	if plan.IncludeNull.IsUnknown() {
		plan.IncludeNull = resolved.IncludeNull
	}
	if plan.Inverse.IsUnknown() {
		plan.Inverse = resolved.Inverse
	}
	if plan.Mode.IsUnknown() {
		plan.Mode = resolved.Mode
	}
	if plan.ScopesType.IsUnknown() {
		plan.ScopesType = resolved.ScopesType
	}
	if plan.Values.IsUnknown() {
		plan.Values = resolved.Values
	}
	return nil
}

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
		operator := models.MetricFilterText(configVal.Operator.ValueString())
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
// If the resource is not found (404), state.Id is set to null to trigger removal.
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
		// Use empty list instead of null to match user config if they set recipients = []
		var listDiags diag.Diagnostics
		state.Recipients, listDiags = types.ListValue(types.StringType, []attr.Value{})
		diags.Append(listDiags...)
	}

	// Convert config
	if resp.Config != nil {
		// Extract existing scope types/IDs/includeNull from state for alias normalization
		// and preserving user-configured values the API does not echo back.
		// When called from Create/Update, state already contains the user's plan values
		// (e.g. "allocation_rule"), while the API returns canonical names ("attribution").
		var existingScopeTypes, existingScopeIDs []string
		var existingScopeIncludeNull []*bool
		var existingScopeCaseInsensitive []*bool
		var existingScopeValues []types.List
		if !state.Config.IsNull() && !state.Config.IsUnknown() &&
			!state.Config.Scopes.IsNull() && !state.Config.Scopes.IsUnknown() {
			var existingScopes []resource_alert.ScopesValue
			if d := state.Config.Scopes.ElementsAs(ctx, &existingScopes, false); !d.HasError() {
				for _, es := range existingScopes {
					existingScopeTypes = append(existingScopeTypes, es.ScopesType.ValueString())
					existingScopeIDs = append(existingScopeIDs, es.Id.ValueString())
					existingScopeIncludeNull = append(existingScopeIncludeNull, es.IncludeNull.ValueBoolPointer())
					existingScopeCaseInsensitive = append(existingScopeCaseInsensitive, es.CaseInsensitive.ValueBoolPointer())
					existingScopeValues = append(existingScopeValues, es.Values)
				}
			}
		}
		configVal, configDiags := mapAlertConfigToModel(ctx, resp.Config, existingScopeTypes, existingScopeIDs, existingScopeIncludeNull, existingScopeCaseInsensitive, existingScopeValues)
		diags.Append(configDiags...)
		state.Config = configVal
	}

	return diags
}

// mapAlertConfigToModel maps the API AlertConfig to the Terraform ConfigValue.
func mapAlertConfigToModel(ctx context.Context, config *models.AlertConfig, existingScopeTypes, existingScopeIDs []string, existingScopeIncludeNull []*bool, existingScopeCaseInsensitive []*bool, existingScopeValues []types.List) (resource_alert.ConfigValue, diag.Diagnostics) {
	var diags diag.Diagnostics

	// Build attributions list
	var attributionsVal types.List
	if config.Attributions != nil {
		var listDiags diag.Diagnostics
		attributionsVal, listDiags = types.ListValueFrom(ctx, types.StringType, *config.Attributions)
		diags.Append(listDiags...)
	} else {
		// Return empty list for nil to avoid inconsistent result if user sets []
		var emptyDiags diag.Diagnostics
		attributionsVal, emptyDiags = types.ListValue(types.StringType, []attr.Value{})
		diags.Append(emptyDiags...)
	}

	// Build scopes list
	var scopesVal types.List
	if config.Scopes != nil {
		scopesList := make([]resource_alert.ScopesValue, len(*config.Scopes))
		for i, scope := range *config.Scopes {
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
			scopesList[i], d = resource_alert.NewScopesValue(resource_alert.ScopesValue{}.AttributeTypes(ctx), scopeAttrs)
			diags.Append(d...)
		}
		var listDiags diag.Diagnostics
		scopesVal, listDiags = types.ListValueFrom(ctx, resource_alert.ScopesValue{}.Type(ctx), scopesList)
		diags.Append(listDiags...)
	} else {
		// Return empty list for nil to avoid inconsistent result if user sets []
		emptyScopes, emptyDiags := types.ListValueFrom(ctx, resource_alert.ScopesValue{}.Type(ctx), []resource_alert.ScopesValue{})
		diags.Append(emptyDiags...)
		scopesVal = emptyScopes
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
		"operator":          types.StringPointerValue((*string)(config.Operator)),
		"scopes":            scopesVal,
		"time_interval":     types.StringValue(string(config.TimeInterval)),
		"value":             types.Float64Value(config.Value),
	}

	configVal, d := resource_alert.NewConfigValue(resource_alert.ConfigValue{}.AttributeTypes(ctx), configAttrs)
	diags.Append(d...)

	return configVal, diags
}
