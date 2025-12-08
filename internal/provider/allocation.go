package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"

	"terraform-provider-doit/internal/provider/models"
	"terraform-provider-doit/internal/provider/resource_allocation"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
)

func (plan *allocationResourceModel) toRequest(ctx context.Context) (req models.Allocation, diags diag.Diagnostics) {
	allocationType := models.AllocationAllocationType(plan.AllocationType.ValueString())
	req.AllocationType = &allocationType
	req.AnomalyDetection = plan.AnomalyDetection.ValueBoolPointer()
	req.Description = plan.Description.ValueStringPointer()
	req.Name = plan.Name.ValueStringPointer()
	req.Type = plan.Type.ValueStringPointer()
	if !plan.Rule.IsNull() && !plan.Rule.IsUnknown() {
		req.Rule = &models.AllocationRule{
			Formula: plan.Rule.Formula.ValueString(),
		}
		if !plan.Rule.Components.IsNull() {
			planComponents := []resource_allocation.ComponentsValue{}
			diags = plan.Rule.Components.ElementsAs(ctx, &planComponents, false)
			diags.Append(diags...)
			if diags.HasError() {
				return req, diags
			}
			req.Rule.Components = make([]models.AllocationComponent, len(planComponents))
			for i := range planComponents {
				req.Rule.Components[i] = models.AllocationComponent{
					IncludeNull:      planComponents[i].IncludeNull.ValueBoolPointer(),
					InverseSelection: planComponents[i].InverseSelection.ValueBoolPointer(),
					Key:              planComponents[i].Key.ValueString(),
					Mode:             models.AllocationComponentMode(planComponents[i].Mode.ValueString()),
					Type:             models.DimensionsTypes(planComponents[i].ComponentsType.ValueString()),
				}
				diags = planComponents[i].Values.ElementsAs(ctx, &req.Rule.Components[i].Values, false)
				diags.Append(diags...)
				if diags.HasError() {
					return req, diags
				}
			}
		}
	}
	if !plan.Rules.IsNull() && !plan.Rules.IsUnknown() {
		var planRules []resource_allocation.RulesValue
		diags = plan.Rules.ElementsAs(ctx, &planRules, false)
		diags.Append(diags...)
		if diags.HasError() {
			return req, diags
		}
		rules := make([]models.GroupAllocationRule, len(planRules))
		for i := range planRules {
			rules[i] = models.GroupAllocationRule{
				Name:        planRules[i].Name.ValueStringPointer(),
				Id:          planRules[i].Id.ValueStringPointer(),
				Action:      models.GroupAllocationRuleAction(planRules[i].Action.ValueString()),
				Description: planRules[i].Description.ValueStringPointer(),
				Formula:     planRules[i].Formula.ValueStringPointer(),
			}
			// Don't send components if selecting existing allocation
			if !planRules[i].Components.IsNull() && planRules[i].Action.ValueString() != "select" {
				var ruleComponents []resource_allocation.ComponentsValue
				diags = planRules[i].Components.ElementsAs(ctx, &ruleComponents, true)
				diags.Append(diags...)
				if diags.HasError() {
					return req, diags
				}
				createComponents := make([]models.AllocationComponent, len(ruleComponents))
				for j := range ruleComponents {
					createComponents[j] = models.AllocationComponent{
						IncludeNull:      ruleComponents[j].IncludeNull.ValueBoolPointer(),
						InverseSelection: ruleComponents[j].InverseSelection.ValueBoolPointer(),
						Key:              ruleComponents[j].Key.ValueString(),
						Mode:             models.AllocationComponentMode(ruleComponents[j].Mode.ValueString()),
						Type:             models.DimensionsTypes(ruleComponents[j].ComponentsType.ValueString()),
					}
					diags = ruleComponents[j].Values.ElementsAs(ctx, &createComponents[j].Values, true)
					diags.Append(diags...)
					if diags.HasError() {
						return req, diags
					}
				}
				rules[i].Components = &createComponents
			}
		}
		req.Rules = &rules
	}
	return req, diags
}

func (r *allocationResource) populateState(ctx context.Context, state *allocationResourceModel) (diags diag.Diagnostics) {
	var d diag.Diagnostics

	// Get refreshed allocation value from DoiT using the ID from the state.
	httpResp, err := r.client.GetAllocationWithResponse(ctx, state.Id.ValueString())
	if err != nil {
		if strings.Contains(err.Error(), "404") {
			// The resource was deleted. This is an edge case for create,
			// but necessary for the read function.
			state.Id = types.StringNull()
			return
		}
		diags.AddError(
			"Error Reading Doit Console Allocation",
			"Could not read Doit Console Allocation ID "+state.Id.ValueString()+": "+err.Error(),
		)
		return
	}

	state.Id = types.StringPointerValue(resp.Id)
	state.Type = types.StringPointerValue(resp.Type)
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
			m["components"], d = toAllocationRuleComponentsListValue(ctx, resp.Rule.Components)
			diags.Append(d...)
			if diags.HasError() {
				return
			}
		}
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
				fullAlloc, err := r.client.GetAllocation(ctx, *rule.Id)
				if err == nil {
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
				m["components"], d = toAllocationRuleComponentsListValue(ctx, components)
				diags.Append(d...)
				if diags.HasError() {
					return
				}
			} else {
				m["components"] = types.ListNull(resource_allocation.ComponentsValue{}.Type(ctx))
			}
			rules[i], d = resource_allocation.NewRulesValue(resource_allocation.RulesValue{}.AttributeTypes(ctx), m)
			diags.Append(d...)
			if diags.HasError() {
				return
			}
		}
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

func (c *Client) CreateAllocation(ctx context.Context, allocation models.Allocation) (*models.Allocation, error) {
	rb, err := json.Marshal(allocation)
	if err != nil {
		return nil, err
	}
	urlRequestBase := fmt.Sprintf("%s/analytics/v1/allocations", c.HostURL)
	urlRequestContext := addContextToURL(c.Auth.CustomerContext, urlRequestBase)
	req, err := http.NewRequest("POST", urlRequestContext, strings.NewReader(string(rb)))
	if err != nil {
		return nil, err
	}

	body, err := c.doRequest(ctx, req)
	if err != nil {
		return nil, err
	}

	allocationResponse := models.Allocation{}
	err = json.Unmarshal(body, &allocationResponse)
	if err != nil {
		return nil, err
	}
	return &allocationResponse, nil
}

func (c *Client) UpdateAllocation(ctx context.Context, allocationID string, allocation models.Allocation) (*models.Allocation, error) {
	rb, err := json.Marshal(allocation)
	if err != nil {
		return nil, err
	}
	urlRequestBase := fmt.Sprintf("%s/analytics/v1/allocations/%s", c.HostURL, allocationID)
	urlRequestContext := addContextToURL(c.Auth.CustomerContext, urlRequestBase)
	req, err := http.NewRequest("PATCH", urlRequestContext, strings.NewReader(string(rb)))
	if err != nil {
		return nil, err
	}
	body, err := c.doRequest(ctx, req)
	if err != nil {
		return nil, err
	}

	allocationResponse := models.Allocation{}
	err = json.Unmarshal(body, &allocationResponse)
	if err != nil {
		return nil, err
	}
	return &allocationResponse, nil
}

func (c *Client) DeleteAllocation(ctx context.Context, allocationID string) error {
	urlRequestBase := fmt.Sprintf("%s/analytics/v1/allocations/%s", c.HostURL, allocationID)
	urlRequestContext := addContextToURL(c.Auth.CustomerContext, urlRequestBase)
	req, err := http.NewRequest("DELETE", urlRequestContext, nil)
	if err != nil {
		return err
	}

	_, err = c.doRequest(ctx, req)
	if err != nil {
		return err
	}

	return nil
}

func (c *Client) GetAllocation(ctx context.Context, id string) (*models.Allocation, error) {
	urlRequestBase := fmt.Sprintf("%s/analytics/v1/allocations/%s", c.HostURL, id)
	urlRequestContext := addContextToURL(c.Auth.CustomerContext, urlRequestBase)
	req, err := http.NewRequest("GET", urlRequestContext, nil)
	if err != nil {
		return nil, err
	}

	body, err := c.doRequest(ctx, req)
	if err != nil {
		return nil, err
	}

	allocation := models.Allocation{}
	err = json.Unmarshal(body, &allocation)
	if err != nil {
		return nil, err
	}
	return &allocation, nil
}

func toAllocationRuleComponentsListValue(ctx context.Context, components []models.AllocationComponent) (res basetypes.ListValue, diags diag.Diagnostics) {
	var d diag.Diagnostics
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
	res, diags = types.ListValueFrom(ctx, stateComponents[0].Type(ctx), stateComponents)
	d.Append(diags...)
	return
}
