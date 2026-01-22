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
			IncludeNull:      components[i].IncludeNull.ValueBoolPointer(),
			InverseSelection: components[i].InverseSelection.ValueBoolPointer(),
			Key:              components[i].Key.ValueString(),
			Mode:             models.AllocationComponentMode(components[i].Mode.ValueString()),
			Type:             models.DimensionsTypes(components[i].ComponentsType.ValueString()),
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

	// Populate Group Rules if present
	if !plan.Rules.IsNull() && !plan.Rules.IsUnknown() {
		var planRules []resource_allocation.RulesValue
		d := plan.Rules.ElementsAs(ctx, &planRules, false)
		diags.Append(d...)
		if diags.HasError() {
			return diags
		}
		rules := make([]models.GroupAllocationRule, len(planRules))
		for i := range planRules {
			rules[i] = models.GroupAllocationRule{
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
	return diags
}

// populateState fetches the allocation from the API and populates the Terraform state.
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
//   - allowNotFound=false (used by Create and Update):
//     404 is unexpected and indicates an error. After a successful Create or
//     Update API call, the resource MUST exist. A 404 here could indicate:
//
//   - A transient API issue (rare, but possible)
//
//   - An eventual consistency problem
//
//   - A bug in the provider or API
//     In these cases, we return an error so the user knows something went wrong
//     and can retry. This prevents silent resource orphaning.
//
// # Why This Matters
//
// Without this distinction, a transient 404 during Create would:
//  1. Create the resource successfully (API returns 200 with ID)
//  2. GET returns 404 (transient issue)
//  3. populateState sets state.Id = null (no error!)
//  4. Terraform "succeeds" but loses track of the resource
//  5. Resource is orphaned - exists in API but not in Terraform state
//
// With allowNotFound=false for Create/Update, step 3 returns an error,
// the user sees the failure, and can retry or investigate.
func (r *allocationResource) populateState(ctx context.Context, state *allocationResourceModel, allowNotFound bool) (diags diag.Diagnostics) {
	var resp *models.Allocation

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

	// Handle externally deleted resource - remove from state
	if httpResp.StatusCode() == 404 {
		state.Id = types.StringNull()
		return
	}

	resp = httpResp.JSON200
	if resp == nil {
		diags.AddError(
			"Error Reading DoiT Allocation",
			"Received empty response body for allocation ID "+state.Id.ValueString(),
		)
		return
	}

	state.Id = types.StringPointerValue(resp.Id)
	state.Type = types.StringPointerValue(resp.Type)
	// This is due to a bug in the API where the description is not returned for group allocations
	// Will be removed once the API is fixed
	if resp.Description != nil && *resp.Description != "" {
		state.Description = types.StringPointerValue(resp.Description)
	}
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
			var d diag.Diagnostics
			m["components"], d = toAllocationRuleComponentsListValue(ctx, resp.Rule.Components)
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
		// Map to store existing actions
		existingActionsByID := make(map[string]string)
		existingActionsByIndex := make([]string, 0)

		if !state.Rules.IsNull() && !state.Rules.IsUnknown() {
			var stateRules []resource_allocation.RulesValue
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

		rules := make([]attr.Value, len(*resp.Rules))
		for i, rule := range *resp.Rules {
			// Determine Action
			var action string
			if rule.Id != nil {
				if a, ok := existingActionsByID[*rule.Id]; ok {
					action = a
				}
			}
			if action == "" && i < len(existingActionsByIndex) {
				action = existingActionsByIndex[i]
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
			if components != nil {
				var d diag.Diagnostics
				m["components"], d = toAllocationRuleComponentsListValue(ctx, components)
				diags.Append(d...)
				if diags.HasError() {
					return
				}
			} else {
				m["components"] = types.ListNull(resource_allocation.ComponentsValue{}.Type(ctx))
			}
			var d diag.Diagnostics
			rules[i], d = resource_allocation.NewRulesValue(resource_allocation.RulesValue{}.AttributeTypes(ctx), m)
			diags.Append(d...)
			if diags.HasError() {
				return
			}
		}
		var d diag.Diagnostics
		state.Rules, d = types.ListValueFrom(ctx, resource_allocation.RulesValue{}.Type(ctx), rules)
		diags.Append(d...)
		if diags.HasError() {
			return
		}
	} else {
		state.Rules = types.ListNull(resource_allocation.RulesValue{}.Type(ctx))
	}
	return
}

func toAllocationRuleComponentsListValue(ctx context.Context, components []models.AllocationComponent) (res basetypes.ListValue, diags diag.Diagnostics) {
	stateComponents := make([]attr.Value, len(components))
	for i, component := range components {
		m := map[string]attr.Value{
			"include_null":      types.BoolPointerValue(component.IncludeNull),
			"inverse_selection": types.BoolPointerValue(component.InverseSelection),
			"key":               types.StringValue(component.Key),
			"mode":              types.StringValue(string(component.Mode)),
			"type":              types.StringValue(string(component.Type)),
		}
		values := make([]attr.Value, len(component.Values))
		for j := range component.Values {
			values[j] = types.StringValue(component.Values[j])
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
