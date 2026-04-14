// Package provider implements the DoiT Terraform provider.
package provider

import (
	"context"
	"fmt"
	"log"

	"github.com/doitintl/terraform-provider-doit/internal/provider/models"
	"github.com/doitintl/terraform-provider-doit/internal/provider/resource_allocation"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
)

// overlayComputedFields sets only the Computed-only fields from the API response
// onto the plan model. This implements the plan-first state pattern recommended
// by HashiCorp (https://developer.hashicorp.com/terraform/plugin/framework/resources/create):
//
// All user-configured values (name, description, rule, rules, unallocated_costs)
// stay exactly as the user wrote them in the plan. Only server-assigned values
// that the user cannot configure are overlaid from the API response:
//
//   - id: assigned by the API on creation
//   - allocation_type: computed from whether rule or rules was provided
//   - anomaly_detection: server-managed flag
//   - create_time: set by the API on creation
//   - update_time: set by the API on every modification
//   - type: computed by the API (preset vs custom)
//
// This prevents "Provider produced inconsistent result" errors caused by the API
// normalizing user-provided values (e.g. stripping [Service N/A] sentinels,
// renaming "Amazon Elastic Container Service for Kubernetes (EKS)" to
// "Amazon Elastic Container Service for Kubernetes").
func overlayComputedFields(ctx context.Context, apiResp *models.Allocation, plan *allocationResourceModel) diag.Diagnostics {
	var diags diag.Diagnostics

	// Computed-only fields: always set from API response.
	plan.Id = types.StringPointerValue(apiResp.Id)
	plan.CreateTime = types.Int64PointerValue(apiResp.CreateTime)
	plan.UpdateTime = types.Int64PointerValue(apiResp.UpdateTime)
	plan.AnomalyDetection = types.BoolPointerValue(apiResp.AnomalyDetection)
	plan.Type = types.StringPointerValue(apiResp.Type)

	if apiResp.AllocationType != nil {
		plan.AllocationType = types.StringValue(string(*apiResp.AllocationType))
	} else {
		plan.AllocationType = types.StringNull()
	}

	// Optional+Computed fields at top level: resolve unknowns using the API
	// response where available, falling back to null.
	if plan.Rules.IsUnknown() {
		plan.Rules = types.ListNull(resource_allocation.RulesValue{}.Type(ctx))
	}
	if plan.Rule.IsUnknown() {
		plan.Rule = resource_allocation.NewRuleValueNull()
	}
	if plan.UnallocatedCosts.IsUnknown() {
		plan.UnallocatedCosts = types.StringPointerValue(apiResp.UnallocatedCosts)
	}

	// Resolve unknowns inside rules[] elements.
	// Fields like 'description', 'id' inside each rule are Optional+Computed.
	// When the user doesn't set them, they arrive as unknown.
	if !plan.Rules.IsNull() && !plan.Rules.IsUnknown() {
		var planRules []resource_allocation.RulesValue
		elementsDiags := plan.Rules.ElementsAs(ctx, &planRules, false)
		diags.Append(elementsDiags...)
		if !elementsDiags.HasError() {
			changed := false
			for i := range planRules {
				// For each unknown Optional+Computed field, overlay the API
				// response value (which may contain a generated default like
				// formula="A") instead of null. This prevents perpetual drift
				// where Read returns the API default but state has null.
				apiRule := safeGetGroupRule(apiResp, i)
				if planRules[i].Description.IsUnknown() {
					if apiRule != nil {
						planRules[i].Description = types.StringPointerValue(apiRule.Description)
					} else {
						planRules[i].Description = types.StringNull()
					}
					changed = true
				}
				if planRules[i].Id.IsUnknown() {
					if apiRule != nil {
						planRules[i].Id = types.StringPointerValue(apiRule.Id)
					} else {
						planRules[i].Id = types.StringNull()
					}
					changed = true
				}
				if planRules[i].Formula.IsUnknown() {
					if apiRule != nil {
						planRules[i].Formula = types.StringPointerValue(apiRule.Formula)
					} else {
						planRules[i].Formula = types.StringNull()
					}
					changed = true
				}
				if planRules[i].Name.IsUnknown() {
					if apiRule != nil {
						planRules[i].Name = types.StringPointerValue(apiRule.Name)
					} else {
						planRules[i].Name = types.StringNull()
					}
					changed = true
				}
				if planRules[i].Components.IsUnknown() {
					planRules[i].Components = types.ListNull(resource_allocation.ComponentsValue{}.Type(ctx))
					changed = true
				}
				// Also resolve unknowns inside components.
				if !planRules[i].Components.IsNull() && !planRules[i].Components.IsUnknown() {
					resolved, compDiags := resolveComponentUnknowns(ctx, &planRules[i].Components)
					diags.Append(compDiags...)
					if resolved {
						changed = true
					}
				}
			}
			if changed {
				var rulesDiags diag.Diagnostics
				plan.Rules, rulesDiags = types.ListValueFrom(ctx, resource_allocation.RulesValue{}.Type(ctx), planRules)
				diags.Append(rulesDiags...)
			}
		}
	}

	// Resolve unknowns inside single rule (formula and components).
	if !plan.Rule.IsNull() && !plan.Rule.IsUnknown() {
		changed := false
		formula := plan.Rule.Formula
		if formula.IsUnknown() {
			if apiResp.Rule != nil {
				formula = types.StringValue(apiResp.Rule.Formula)
			} else {
				formula = types.StringNull()
			}
			changed = true
		}

		components := plan.Rule.Components
		if !components.IsNull() && !components.IsUnknown() {
			resolved, compDiags := resolveComponentUnknowns(ctx, &components)
			diags.Append(compDiags...)
			if resolved {
				changed = true
			}
		}

		if changed {
			// Rebuild the Rule value with updated components and formula.
			m := map[string]attr.Value{
				"formula":    formula,
				"components": components,
			}
			var ruleDiags diag.Diagnostics
			plan.Rule, ruleDiags = resource_allocation.NewRuleValue(resource_allocation.RuleValue{}.AttributeTypes(ctx), m)
			diags.Append(ruleDiags...)
		}
	}

	return diags
}

// safeGetGroupRule returns the API response's group rule at index i, or nil if
// the index is out of bounds or the rules slice is nil.
func safeGetGroupRule(apiResp *models.Allocation, i int) *models.GroupAllocationRule {
	if apiResp.Rules == nil || i >= len(*apiResp.Rules) {
		return nil
	}
	return (*apiResp.Rules)[i]
}

// resolveComponentUnknowns resolves unknown Optional+Computed fields inside
// component elements (case_insensitive, include_null, inverse, inverse_selection).
// Returns true if any fields were changed.
func resolveComponentUnknowns(ctx context.Context, components *basetypes.ListValue) (bool, diag.Diagnostics) {
	var comps []resource_allocation.ComponentsValue
	if d := components.ElementsAs(ctx, &comps, false); d.HasError() {
		return false, d
	}
	changed := false
	for i := range comps {
		if comps[i].CaseInsensitive.IsUnknown() {
			comps[i].CaseInsensitive = types.BoolValue(false)
			changed = true
		}
		if comps[i].IncludeNull.IsUnknown() {
			comps[i].IncludeNull = types.BoolValue(false)
			changed = true
		}
		if comps[i].Inverse.IsUnknown() {
			comps[i].Inverse = types.BoolValue(false)
			changed = true
		}
		if comps[i].InverseSelection.IsUnknown() {
			comps[i].InverseSelection = types.BoolValue(false)
			changed = true
		}
	}
	if changed {
		var listDiags diag.Diagnostics
		*components, listDiags = types.ListValueFrom(ctx, resource_allocation.ComponentsValue{}.Type(ctx), comps)
		return changed, listDiags
	}
	return changed, nil
}

func (plan *allocationResourceModel) toCreateRequest(ctx context.Context) (req models.CreateAllocationRequest, diags diag.Diagnostics) {
	// Create request uses value types for Description/Name, Update uses pointers.
	// We use the common helper to generate the complex Rule/Rules structures (which are shared types)
	// and then map the simple fields.

	common := models.UpdateAllocationRequest{}
	// Note: We deliberately use fillAllocationCommon to populate Rule and Rules,
	// so that the logic is shared between Create and Update.
	diags = plan.fillAllocationCommon(ctx, &common)
	if diags.HasError() {
		return req, diags
	}

	req.Description = ""
	if common.Description != nil {
		req.Description = *common.Description
	}
	req.Name = ""
	if common.Name != nil {
		req.Name = *common.Name
	}
	req.UnallocatedCosts = common.UnallocatedCosts
	req.Rule = common.Rule
	req.Rules = common.Rules

	return req, diags
}

func (plan *allocationResourceModel) toUpdateRequest(ctx context.Context) (req models.UpdateAllocationRequest, diags diag.Diagnostics) {
	// Update request is structurally identical to the common request helper
	diags = plan.fillAllocationCommon(ctx, &req)
	return req, diags
}

// Helper to convert a slice of ComponentsValue to a slice of AllocationComponent models.
func convertComponentsToModels(ctx context.Context, components []resource_allocation.ComponentsValue) (result []models.AllocationComponent, diags diag.Diagnostics) {
	result = make([]models.AllocationComponent, len(components))
	for i := range components {
		result[i] = models.AllocationComponent{
			CaseInsensitive:  components[i].CaseInsensitive.ValueBoolPointer(),
			IncludeNull:      components[i].IncludeNull.ValueBoolPointer(),
			Inverse:          components[i].Inverse.ValueBoolPointer(),
			InverseSelection: components[i].InverseSelection.ValueBoolPointer(),
			Key:              components[i].Key.ValueString(),
			Mode:             models.AllocationComponentMode(components[i].Mode.ValueString()),
			Type:             models.AllocationDimensionsTypes(components[i].ComponentsType.ValueString()),
		}
		d := components[i].Values.ElementsAs(ctx, &result[i].Values, true)
		diags.Append(d...)
		if diags.HasError() {
			return
		}
	}
	return
}

// Helper to fill common fields into UpdateAllocationRequest model (which uses pointers).
func (plan *allocationResourceModel) fillAllocationCommon(ctx context.Context, req *models.UpdateAllocationRequest) (diags diag.Diagnostics) {
	req.Description = plan.Description.ValueStringPointer()
	req.Name = plan.Name.ValueStringPointer()
	// UnallocatedCosts is only sent if not empty because it is invalid for "single" allocations.
	if v := plan.UnallocatedCosts.ValueString(); v != "" {
		req.UnallocatedCosts = &v
	}

	// Populate single Rule if present
	if !plan.Rule.IsNull() && !plan.Rule.IsUnknown() {
		req.Rule = &models.AllocationRule{
			Formula: plan.Rule.Formula.ValueString(),
		}
		if !plan.Rule.Components.IsNull() {
			planComponents := []resource_allocation.ComponentsValue{}
			d := plan.Rule.Components.ElementsAs(ctx, &planComponents, false)
			diags.Append(d...)
			if diags.HasError() {
				return diags
			}
			req.Rule.Components, diags = convertComponentsToModels(ctx, planComponents)
			if diags.HasError() {
				return diags
			}
		}
	}

	// Populate Group Rules if present.
	// Only send rules when the list is non-null, non-unknown, AND non-empty.
	// For single-rule allocations, Rules should be null (omitted from request).
	// Sending "rules": [] causes a 500 error from the API.
	if !plan.Rules.IsNull() && !plan.Rules.IsUnknown() {
		var planRules []resource_allocation.RulesValue
		d := plan.Rules.ElementsAs(ctx, &planRules, false)
		diags.Append(d...)
		if diags.HasError() {
			return diags
		}
		if len(planRules) > 0 {
			rules := make([]*models.GroupAllocationRule, len(planRules))
			for i := range planRules {
				rules[i] = &models.GroupAllocationRule{
					Name:        planRules[i].Name.ValueStringPointer(),
					Formula:     planRules[i].Formula.ValueStringPointer(),
					Action:      models.GroupAllocationRuleAction(planRules[i].Action.ValueString()),
					Id:          planRules[i].Id.ValueStringPointer(),
					Description: planRules[i].Description.ValueStringPointer(),
				}

				// Don't send components if selecting existing allocation (action "select")
				// But for "create" or "update" action, components are required/allowed.
				if !planRules[i].Components.IsNull() && planRules[i].Action.ValueString() != "select" {
					var ruleComponents []resource_allocation.ComponentsValue
					d := planRules[i].Components.ElementsAs(ctx, &ruleComponents, true)
					diags.Append(d...)
					if diags.HasError() {
						return diags
					}
					createComponents, d := convertComponentsToModels(ctx, ruleComponents)
					diags.Append(d...)
					if diags.HasError() {
						return diags
					}
					rules[i].Components = &createComponents
				}
			}
			req.Rules = &rules
		}
	}
	return diags
}

// populateState fetches the allocation from the API and populates the Terraform state.
// This is used by Read and ImportState. Create and Update use mapAllocationToModel
// directly with the API response instead.
//
// # 404 Handling Strategy
//
// The allowNotFound parameter controls how 404 responses are handled:
//
//   - allowNotFound=true (used by Read):
//     404 means the resource was deleted externally (outside Terraform).
//     We set state.Id to null, which signals Terraform to remove the resource
//     from state. On next plan, Terraform will propose recreating it.
//     This is the standard Terraform pattern for "externally deleted" resources.
//
//   - allowNotFound=false:
//     404 is unexpected and indicates an error. This is kept as a safety measure
//     but is not currently used since Create/Update no longer call populateState.
func (r *allocationResource) populateState(ctx context.Context, state *allocationResourceModel, allowNotFound bool) (diags diag.Diagnostics) {
	// Get refreshed allocation value from DoiT using the ID from the state.
	httpResp, err := r.client.GetAllocationWithResponse(ctx, state.Id.ValueString())
	if err != nil {
		diags.AddError(
			"Error Reading Doit Console Allocation",
			"Could not read Doit Console Allocation ID "+state.Id.ValueString()+": "+err.Error(),
		)
		return
	}

	// Handle 404 based on context
	if httpResp.StatusCode() == 404 {
		if allowNotFound {
			// Read context: Resource was deleted externally, mark for removal from state
			state.Id = types.StringNull()
			return
		}
		// Create/Update context: Resource should exist, 404 is an error
		diags.AddError(
			"Resource not found after operation",
			"The allocation was successfully created/updated but could not be read back (404). "+
				"This may indicate a transient API issue. Please retry the operation. "+
				"If the problem persists, the resource may need to be imported manually. "+
				"Allocation ID: "+state.Id.ValueString(),
		)
		return
	}

	// Check for successful response
	if httpResp.StatusCode() != 200 {
		diags.AddError(
			"Error Reading Allocation",
			fmt.Sprintf("Unexpected status code %d for allocation ID %s: %s",
				httpResp.StatusCode(), state.Id.ValueString(), string(httpResp.Body)),
		)
		return
	}

	if httpResp.JSON200 == nil {
		diags.AddError(
			"Error Reading DoiT Allocation",
			"Received empty response body for allocation ID "+state.Id.ValueString(),
		)
		return
	}

	return r.mapAllocationToModel(ctx, httpResp.JSON200, state)
}

// mapAllocationToModel maps an Allocation API response to the Terraform resource model.
// This is used by both populateState (for Read) and directly by Create/Update
// when the API returns the full object in the response.
func (r *allocationResource) mapAllocationToModel(ctx context.Context, resp *models.Allocation, state *allocationResourceModel) (diags diag.Diagnostics) {
	state.Id = types.StringPointerValue(resp.Id)
	state.Type = types.StringPointerValue(resp.Type)
	state.Description = types.StringPointerValue(resp.Description)
	state.AnomalyDetection = types.BoolPointerValue(resp.AnomalyDetection)
	state.CreateTime = types.Int64PointerValue(resp.CreateTime)
	state.UpdateTime = types.Int64PointerValue(resp.UpdateTime)
	state.Name = types.StringPointerValue(resp.Name)
	state.UnallocatedCosts = types.StringPointerValue(resp.UnallocatedCosts)

	if resp.AllocationType != nil {
		state.AllocationType = types.StringValue(string(*resp.AllocationType))
	} else {
		state.AllocationType = types.StringNull()
	}

	if resp.Rule != nil {
		m := map[string]attr.Value{
			"formula": types.StringValue(resp.Rule.Formula),
		}
		if resp.Rule.Components != nil {
			// Preserve existing component values from state for alias-type normalization.
			// Note: includeNull, inverse, and caseInsensitive ARE reliably echoed by the
			// API. inverseSelection is a deprecated field superseded by inverse — the API
			// accepts it on write but never returns it (the value is visible via inverse
			// instead). We preserve all four from state as a defensive baseline, which
			// also handles ImportState gracefully (falls back to API value).
			var existingComponents []resource_allocation.ComponentsValue
			if !state.Rule.IsNull() && !state.Rule.IsUnknown() &&
				!state.Rule.Components.IsNull() && !state.Rule.Components.IsUnknown() {
				if d := state.Rule.Components.ElementsAs(ctx, &existingComponents, false); d.HasError() {
					diags.Append(d...)
					return
				}
			}
			var d diag.Diagnostics
			m["components"], d = toAllocationRuleComponentsListValue(ctx, resp.Rule.Components, existingComponents)
			diags.Append(d...)
			if diags.HasError() {
				return
			}
		}
		var d diag.Diagnostics
		state.Rule, d = resource_allocation.NewRuleValue(resource_allocation.RuleValue{}.AttributeTypes(ctx), m)
		diags.Append(d...)
		if diags.HasError() {
			return
		}
	} else {
		state.Rule = resource_allocation.NewRuleValueNull()
	}

	if resp.Rules != nil && len(*resp.Rules) > 0 {
		// Parse state rules once for reuse (actions, component types, include_null, inverse_selection)
		var stateRules []resource_allocation.RulesValue
		existingActionsByID := make(map[string]string)
		existingActionsByIndex := make([]string, 0)

		if !state.Rules.IsNull() && !state.Rules.IsUnknown() {
			// We try to extract existing rules to preserve the "action" field which is not returned by the API.
			// If this fails, we proceed without existing actions.
			if d := state.Rules.ElementsAs(ctx, &stateRules, false); !d.HasError() {
				for _, rule := range stateRules {
					action := rule.Action.ValueString()
					existingActionsByIndex = append(existingActionsByIndex, action)
					if !rule.Id.IsNull() && !rule.Id.IsUnknown() {
						existingActionsByID[rule.Id.ValueString()] = action
					}
				}
			}
		}

		rules := make([]attr.Value, 0, len(*resp.Rules))
		ruleIndex := 0
		for _, rulePtr := range *resp.Rules {
			if rulePtr == nil {
				ruleIndex++
				continue
			}
			rule := *rulePtr
			// Determine Action
			var action string
			if rule.Id != nil {
				if a, ok := existingActionsByID[*rule.Id]; ok {
					action = a
				}
			}
			if action == "" && ruleIndex < len(existingActionsByIndex) {
				action = existingActionsByIndex[ruleIndex]
			}
			if action == "" {
				// Default to "select" if we can't determine the action (e.g. import)
				action = "select"
			}

			// Fetch details if missing (API response for group allocation rules often lacks formula/components)
			var formula string
			var components []models.AllocationComponent

			if rule.Formula != nil {
				formula = *rule.Formula
			}
			if rule.Components != nil {
				components = *rule.Components
			}

			if (formula == "" || components == nil) && rule.Id != nil && action != "select" {
				// Fetch full allocation to get formula and components
				respHTTPFullAlloc, err := r.client.GetAllocationWithResponse(ctx, *rule.Id)
				if err == nil && respHTTPFullAlloc.JSON200 != nil {
					fullAlloc := respHTTPFullAlloc.JSON200
					if fullAlloc.Rule != nil {
						formula = fullAlloc.Rule.Formula
						if fullAlloc.Rule.Components != nil {
							components = fullAlloc.Rule.Components
						}
					}
				} else {
					log.Printf("[WARN] Failed to fetch allocation details for rule %s: %v", *rule.Id, err)
				}
			}

			m := map[string]attr.Value{
				"action":      types.StringValue(action),
				"description": types.StringPointerValue(rule.Description),
				"formula":     types.StringValue(formula),
				"id":          types.StringPointerValue(rule.Id),
				"name":        types.StringPointerValue(rule.Name),
			}
			if len(components) > 0 {
				// Get existing component values from state for alias normalization and state preservation.
				// We reuse stateRules (parsed once before the loop) instead of re-parsing state.Rules on each iteration.
				var existingComponents []resource_allocation.ComponentsValue
				if ruleIndex < len(stateRules) {
					sr := stateRules[ruleIndex]
					if !sr.Components.IsNull() && !sr.Components.IsUnknown() {
						if d := sr.Components.ElementsAs(ctx, &existingComponents, false); d.HasError() {
							diags.Append(d...)
							return
						}
					}
				}
				var d diag.Diagnostics
				m["components"], d = toAllocationRuleComponentsListValue(ctx, components, existingComponents)
				diags.Append(d...)
				if diags.HasError() {
					return
				}
			} else {
				var d diag.Diagnostics
				m["components"], d = types.ListValueFrom(ctx, resource_allocation.ComponentsValue{}.Type(ctx), []resource_allocation.ComponentsValue{})
				diags.Append(d...)
			}
			ruleVal, d := resource_allocation.NewRulesValue(resource_allocation.RulesValue{}.AttributeTypes(ctx), m)
			diags.Append(d...)
			if diags.HasError() {
				return
			}
			rules = append(rules, ruleVal)
			ruleIndex++
		}
		var d diag.Diagnostics
		state.Rules, d = types.ListValueFrom(ctx, resource_allocation.RulesValue{}.Type(ctx), rules)
		diags.Append(d...)
		if diags.HasError() {
			return
		}
	} else if resp.Rule != nil {
		// Single-rule allocation: the API returns rules=nil because this isn't a
		// group allocation. Keep state.Rules null so it's omitted from subsequent
		// update requests. Sending "rules": [] to the API causes a 500 error.
		state.Rules = types.ListNull(resource_allocation.RulesValue{}.Type(ctx))
	} else {
		// API returned nil or empty slice for a group allocation (no rules left) -
		// return empty list to avoid inconsistent result if user sets [].
		// Pattern B: Normalize to empty list for user-configurable attributes.
		emptyRules, d := types.ListValueFrom(ctx, resource_allocation.RulesValue{}.Type(ctx), []resource_allocation.RulesValue{})
		diags.Append(d...)
		state.Rules = emptyRules
	}
	return
}

func toAllocationRuleComponentsListValue(ctx context.Context, components []models.AllocationComponent, existingComponents []resource_allocation.ComponentsValue) (res basetypes.ListValue, diags diag.Diagnostics) {
	// Handle empty slice: return an empty list without indexing stateComponents[0].
	if len(components) == 0 {
		res, diags = types.ListValueFrom(ctx, resource_allocation.ComponentsValue{}.Type(ctx), []resource_allocation.ComponentsValue{})
		return
	}
	stateComponents := make([]attr.Value, len(components))
	for i, component := range components {
		// Normalize alias types to preserve user's configured value
		compType := string(component.Type)
		if i < len(existingComponents) {
			compType = normalizeDimensionsType(compType, existingComponents[i].ComponentsType.ValueString())
		}

		// Note on field echo behavior (verified via API probe):
		// - includeNull, inverse, caseInsensitive: echoed correctly by the API.
		// - inverseSelection: DEPRECATED (superseded by inverse). The API accepts it
		//   on write but never echoes it back; the equivalent value is visible in
		//   inverse instead. We preserve all four from state so that:
		//   a) deprecated inverseSelection round-trips without drift for existing configs, and
		//   b) ImportState (no prior state) falls back cleanly to the API value.
		caseInsensitiveVal := types.BoolValue(false)
		if i < len(existingComponents) {
			caseInsensitiveVal = types.BoolValue(existingComponents[i].CaseInsensitive.ValueBool())
		} else if component.CaseInsensitive != nil {
			caseInsensitiveVal = types.BoolValue(*component.CaseInsensitive)
		}

		includeNullVal := types.BoolValue(false)
		if i < len(existingComponents) {
			includeNullVal = types.BoolValue(existingComponents[i].IncludeNull.ValueBool())
		} else if component.IncludeNull != nil {
			includeNullVal = types.BoolValue(*component.IncludeNull)
		}

		inverseSelectionVal := types.BoolValue(false)
		if i < len(existingComponents) {
			inverseSelectionVal = types.BoolValue(existingComponents[i].InverseSelection.ValueBool())
		} else if component.InverseSelection != nil {
			inverseSelectionVal = types.BoolValue(*component.InverseSelection)
		}

		inverseVal := types.BoolValue(false)
		if i < len(existingComponents) {
			inverseVal = types.BoolValue(existingComponents[i].Inverse.ValueBool())
		} else if component.Inverse != nil {
			inverseVal = types.BoolValue(*component.Inverse)
		}

		m := map[string]attr.Value{
			"case_insensitive":  caseInsensitiveVal,
			"include_null":      includeNullVal,
			"inverse":           inverseVal,
			"inverse_selection": inverseSelectionVal,
			"key":               types.StringValue(component.Key),
			"mode":              types.StringValue(string(component.Mode)),
			"type":              types.StringValue(compType),
		}
		// Merge API values with prior state to restore any NullFallback sentinels
		// that the API strips during normalization. This is needed for the Read path
		// to prevent perpetual plan drift when the user's config contains sentinels.
		// The Create/Update path uses plan-first state and doesn't need this.
		apiIncludeNull := component.IncludeNull != nil && *component.IncludeNull
		apiValues := component.Values
		if i < len(existingComponents) {
			var stateVals []string
			stateValsDiags := existingComponents[i].Values.ElementsAs(ctx, &stateVals, false)
			diags.Append(stateValsDiags...)
			if diags.HasError() {
				return
			}
			apiValues = mergeSentinelValues(apiValues, stateVals, apiIncludeNull)
		}
		values := make([]attr.Value, len(apiValues))
		for j := range apiValues {
			values[j] = types.StringValue(apiValues[j])
		}
		var d diag.Diagnostics
		m["values"], d = types.ListValue(types.StringType, values)
		diags.Append(d...)
		if diags.HasError() {
			return
		}
		stateComponents[i], d = resource_allocation.NewComponentsValue(resource_allocation.ComponentsValue{}.AttributeTypes(ctx), m)
		diags.Append(d...)
		if diags.HasError() {
			return
		}
	}
	var d diag.Diagnostics
	res, d = types.ListValueFrom(ctx, stateComponents[0].Type(ctx), stateComponents)
	diags.Append(d...)
	return
}
