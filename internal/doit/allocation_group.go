package doit

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"slices"
	"strings"

	"terraform-provider-doit/internal/doit/models"
	"terraform-provider-doit/internal/doit/resource_allocation_group"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func (plan *allocationGroupResourceModel) getActions(ctx context.Context) (map[string]string, diag.Diagnostics) {
	actions := make(map[string]string)
	var diags diag.Diagnostics
	if !plan.Rules.IsNull() {
		var planRules []resource_allocation_group.RulesValue
		diags = plan.Rules.ElementsAs(ctx, &planRules, false)
		diags.Append(diags...)
		if diags.HasError() {
			return nil, diags
		}
		for i := range planRules {
			if !planRules[i].Id.IsNull() {
				actions[planRules[i].Id.ValueString()] = planRules[i].Action.ValueString()
			}
		}
	}
	return actions, diags
}

func (plan *allocationGroupResourceModel) toRequest(ctx context.Context) (models.GroupAllocationRequest, diag.Diagnostics) {
	var (
		req   models.GroupAllocationRequest
		diags diag.Diagnostics
	)

	req.UnallocatedCosts = plan.UnallocatedCosts.ValueStringPointer()
	req.Description = plan.Description.ValueStringPointer()
	req.Name = plan.Name.ValueString()
	if !plan.Rules.IsNull() {
		var planRules []resource_allocation_group.RulesValue
		diags = plan.Rules.ElementsAs(ctx, &planRules, false)
		diags.Append(diags...)
		if diags.HasError() {
			return req, diags
		}
		req.Rules = make([]models.GroupAllocationRule, len(planRules))
		for i := range planRules {
			req.Rules[i] = models.GroupAllocationRule{
				Name:        planRules[i].Name.ValueStringPointer(),
				Id:          planRules[i].Id.ValueStringPointer(),
				Action:      models.GroupAllocationRuleAction(planRules[i].Action.ValueString()),
				Description: planRules[i].Description.ValueStringPointer(),
				Formula:     planRules[i].Formula.ValueStringPointer(),
			}
			// Don't send components if selecting existing allocation
			if !planRules[i].Components.IsNull() && planRules[i].Action.ValueString() != "select" {
				var ruleComponents []resource_allocation_group.ComponentsValue
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
				req.Rules[i].Components = &createComponents
			}
		}
	}
	return req, diags
}

func (r *allocationGroupResource) populateState(ctx context.Context, plan, state *allocationGroupResourceModel) diag.Diagnostics {
	var d, diags diag.Diagnostics

	resp, err := r.client.GetAllocationGroup(ctx, state.Id.ValueString())
	if err != nil {
		if strings.Contains(err.Error(), "404") {
			// The resource was deleted. This is an edge case for create,
			// but necessary for the read function.
			state.Id = types.StringNull()
			return d
		}
		d.AddError(
			"Error Reading Doit Console Allocation Group",
			"Could not read Doit Console Allocation Group ID "+state.Id.ValueString()+": "+err.Error(),
		)
		return d
	}

	state.Description = types.StringPointerValue(resp.Description)
	state.Id = types.StringPointerValue(resp.Id)
	if resp.AllocationType != nil {
		state.AllocationType = types.StringValue(string(*resp.AllocationType))
	}
	state.Type = types.StringPointerValue(resp.Type)
	state.Name = types.StringPointerValue(resp.Name)
	state.Cloud = types.StringPointerValue(resp.Cloud)
	state.UnallocatedCosts = types.StringPointerValue(resp.UnallocatedCosts)
	state.TimeCreated = types.Int64PointerValue(resp.TimeCreated)
	state.TimeModified = types.Int64PointerValue(resp.TimeModified)

	if resp.Rules != nil && len(*resp.Rules) > 0 {

		var planRules []resource_allocation_group.RulesValue

		if plan != nil {
			// Merge user-provided data with incomplete data returned from the server
			planRules = make([]resource_allocation_group.RulesValue, len(plan.Rules.Elements()))
			diags = plan.Rules.ElementsAs(ctx, &planRules, false)
			d.Append(diags...)
			if diags.HasError() {
				return diags
			}
		}

		stateRules := make([]attr.Value, len(*resp.Rules))
		for i, rule := range *resp.Rules {
			stateRule := resource_allocation_group.RulesValue{
				Id:         types.StringPointerValue(rule.Id),
				Name:       types.StringPointerValue(rule.Name),
				Owner:      types.StringPointerValue(rule.Owner),
				RulesType:  types.StringPointerValue(rule.Type),
				CreateTime: types.Int64PointerValue(rule.CreateTime),
				UpdateTime: types.Int64PointerValue(rule.UpdateTime),
				UrlUi:      types.StringPointerValue(rule.UrlUI),
			}

			if rule.AllocationType != nil {
				stateRule.AllocationType = types.StringValue(string(*rule.AllocationType))
			}

			for _, pr := range planRules {
				if slices.Contains([]string{"select", "update"}, strings.ToLower(pr.Action.ValueString())) && pr.Id.ValueString() == stateRule.Id.ValueString() {
					stateRule.Components = pr.Components
					stateRule.Action = pr.Action
					stateRule.Description = pr.Description
					stateRule.Formula = pr.Formula
					continue
				}
				if strings.ToLower(pr.Action.ValueString()) == "create" && pr.Name.ValueString() == stateRule.Name.ValueString() {
					stateRule.Components = pr.Components
					stateRule.Action = pr.Action
					stateRule.Description = pr.Description
					stateRule.Formula = pr.Formula
					continue
				}
			}

			// TODO: apparently missing things
			stateRules[i], diags = resource_allocation_group.NewRulesValue(stateRule.AttributeTypes(ctx), map[string]attr.Value{
				"id":              stateRule.Id,
				"name":            stateRule.Name,
				"owner":           stateRule.Owner,
				"type":            stateRule.RulesType,
				"create_time":     stateRule.CreateTime,
				"update_time":     stateRule.UpdateTime,
				"url_ui":          stateRule.UrlUi,
				"components":      stateRule.Components,
				"action":          stateRule.Action,
				"description":     stateRule.Description,
				"formula":         stateRule.Formula,
				"allocation_type": stateRule.AllocationType,
			})
			d.Append(diags...)
			if d.HasError() {
				return d
			}
		}

		state.Rules, diags = types.ListValue(stateRules[0].Type(ctx), stateRules)
		d.Append(diags...)
		if d.HasError() {
			return d
		}
	}

	return d
}

func (c *Client) CreateAllocationGroup(ctx context.Context, groupAllocation models.GroupAllocationRequest) (*models.GroupAllocation, error) {
	rb, err := json.Marshal(groupAllocation)
	if err != nil {
		return nil, err
	}

	urlRequestBase := fmt.Sprintf("%s/analytics/v1/allocations", c.HostURL)
	urlRequestContext := addContextToURL(c.Auth.CustomerContext, urlRequestBase)

	req, err := http.NewRequest(http.MethodPost, urlRequestContext, strings.NewReader(string(rb)))
	if err != nil {
		return nil, err
	}

	body, err := c.doRequest(ctx, req)
	if err != nil {
		return nil, err
	}

	allocationResponse := new(models.GroupAllocation)
	err = json.Unmarshal(body, allocationResponse)
	if err != nil {
		return nil, err
	}

	return allocationResponse, nil
}

func (c *Client) UpdateAllocationGroup(ctx context.Context, id string, allocationReq models.GroupAllocationRequest) (*models.GroupAllocation, error) {
	rb, err := json.Marshal(allocationReq)
	if err != nil {
		return nil, err
	}
	urlRequestBase := fmt.Sprintf("%s/analytics/v1/allocations/%s", c.HostURL, id)
	urlRequestContext := addContextToURL(c.Auth.CustomerContext, urlRequestBase)
	req, err := http.NewRequest(http.MethodPatch, urlRequestContext, strings.NewReader(string(rb)))
	if err != nil {
		return nil, err
	}
	body, err := c.doRequest(ctx, req)
	if err != nil {
		return nil, err
	}

	groupAllocation := new(models.GroupAllocation)
	err = json.Unmarshal(body, groupAllocation)
	if err != nil {
		return nil, err
	}
	return groupAllocation, nil
}

func (c *Client) DeleteAllocationGroup(ctx context.Context, id string) error {
	urlRequestBase := fmt.Sprintf("%s/analytics/v1/allocations/%s", c.HostURL, id)
	urlRequestContext := addContextToURL(c.Auth.CustomerContext, urlRequestBase)
	req, err := http.NewRequest(http.MethodDelete, urlRequestContext, nil)
	if err != nil {
		return err
	}

	_, err = c.doRequest(ctx, req)
	if err != nil {
		return err
	}

	return nil
}

func (c *Client) GetAllocationGroup(ctx context.Context, id string) (*models.GroupAllocation, error) {
	urlRequestBase := fmt.Sprintf("%s/analytics/v1/allocations/%s", c.HostURL, id)
	urlRequestContext := addContextToURL(c.Auth.CustomerContext, urlRequestBase)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, urlRequestContext, nil)
	if err != nil {
		return nil, err
	}

	body, err := c.doRequest(ctx, req)
	if err != nil {
		return nil, err
	}

	allocation := new(models.GroupAllocation)
	err = json.Unmarshal(body, allocation)
	if err != nil {
		return nil, err
	}

	return allocation, nil
}
