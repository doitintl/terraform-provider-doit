package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"terraform-provider-doit/internal/provider/models"
	"terraform-provider-doit/internal/provider/resource_allocation_group"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
)

// GroupAllocationsModelOverride overrides the model generated from the OpenAPI spec, as it is currently wrong.
// Issue has been created to the appropriate team owning Allocations.
type GroupAllocationsModelOverride struct {
	models.GroupAllocation
	CreateTime *int64 `json:"createTime,omitempty"`
	UpdateTime *int64 `json:"updateTime,omitempty"`
}

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
	var d diag.Diagnostics

	respAlg, err := r.client.GetAllocationGroup(ctx, state.Id.ValueString())
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

	if plan.Description.ValueString() != "" {
		state.Description = plan.Description // WARN: not persisted at all by the API
	}
	state.Id = types.StringPointerValue(respAlg.Id)
	if respAlg.AllocationType != nil {
		state.AllocationType = types.StringValue(string(*respAlg.AllocationType))
	}
	state.Type = types.StringPointerValue(respAlg.Type)
	state.Name = types.StringPointerValue(respAlg.Name)
	state.Cloud = types.StringPointerValue(respAlg.Cloud)
	state.UnallocatedCosts = types.StringPointerValue(respAlg.UnallocatedCosts)
	state.TimeCreated = types.Int64PointerValue(respAlg.CreateTime)  // WARN, overridden
	state.TimeModified = types.Int64PointerValue(respAlg.UpdateTime) // WARN, overridden

	if respAlg.Rules != nil && len(*respAlg.Rules) > 0 {

		var planRules []resource_allocation_group.RulesValue

		planRules = make([]resource_allocation_group.RulesValue, len(plan.Rules.Elements()))
		diags := plan.Rules.ElementsAs(ctx, &planRules, false)
		d.Append(diags...)
		if diags.HasError() {
			return diags
		}

		stateRules := make([]attr.Value, len(*respAlg.Rules))
		for i, respAlgRule := range *respAlg.Rules {

			if respAlgRule.Id == nil {
				d.AddError("Allocation Rule present in the Allocation Group response had no ID", fmt.Sprintf("rule response: %+v", respAlgRule))
				return d
			}

			respAl, err := r.client.GetAllocation(ctx, *respAlgRule.Id)
			if err != nil {
				d.AddError(
					"Error Reading Doit Console Allocation",
					"Could not read Doit Console Allocation ID `"+*respAlgRule.Id+"`: "+err.Error(),
				)
				return d
			}

			if respAl.Rule == nil {
				d.AddError("Allocation had no Rule", fmt.Sprintf("rule response: %+v", respAl))
				return d
			}

			stateRule := resource_allocation_group.RulesValue{
				// The GetAllocationGroup API method only returns an incomplete representation of the actual rule
				// therefore we merge data from the AllocationGroup and Allocation GET API responses.

				// Allocation Group GET data
				Id:         types.StringPointerValue(respAlgRule.Id),
				Name:       types.StringPointerValue(respAlgRule.Name),
				Owner:      types.StringPointerValue(respAlgRule.Owner),
				RulesType:  types.StringPointerValue(respAlgRule.Type),
				CreateTime: types.Int64PointerValue(respAlgRule.CreateTime),
				UpdateTime: types.Int64PointerValue(respAlgRule.UpdateTime),
				UrlUi:      types.StringPointerValue(respAlgRule.UrlUI),

				// Allocation GET data
				Formula:     types.StringPointerValue(respAl.Rule.Formula),
				Description: types.StringPointerValue(respAl.Description),
			}

			respAlComponents, diags := toAllocationRuleComponentsListValue[resource_allocation_group.ComponentsType](ctx, respAl.Rule.Components)
			d.Append(diags...)
			if d.HasError() {
				return d
			}

			if respAl.AllocationType != nil {
				stateRule.AllocationType = types.StringValue(string(*respAl.AllocationType))
			}

			// Finally, we merge plan data that is never returned by the API responses to prevent a perma-diff
			for _, pr := range planRules {
				switch pr.Action.ValueString() {
				case string(models.Select):
					if pr.Id.Equal(stateRule.Id) {
						stateRule.Action = pr.Action
						// Only include 'select' plan rule components  to avoid a perma-diff
						if pr.Components.IsUnknown() {
							stateRule.Components = basetypes.NewListNull(respAlComponents.ElementType(ctx))
						} else {
							stateRule.Components = pr.Components
						}
					}
				case string(models.Update):
					if pr.Id.Equal(stateRule.Id) {
						stateRule.Action = pr.Action
						stateRule.Components = respAlComponents
					}
				case string(models.Create):
					if pr.Name.Equal(stateRule.Name) {
						stateRule.Action = pr.Action
						stateRule.Components = respAlComponents
					}
				}
			}

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

func (c *Client) CreateAllocationGroup(ctx context.Context, groupAllocation models.GroupAllocationRequest) (*GroupAllocationsModelOverride, error) {
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

	allocationResponse := new(GroupAllocationsModelOverride)
	err = json.Unmarshal(body, allocationResponse)
	if err != nil {
		return nil, err
	}

	return allocationResponse, nil
}

func (c *Client) UpdateAllocationGroup(ctx context.Context, id string, allocationReq models.GroupAllocationRequest) (*GroupAllocationsModelOverride, error) {
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

	groupAllocation := new(GroupAllocationsModelOverride)
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

func (c *Client) GetAllocationGroup(ctx context.Context, id string) (*GroupAllocationsModelOverride, error) {
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

	allocation := new(GroupAllocationsModelOverride)
	err = json.Unmarshal(body, allocation)
	if err != nil {
		return nil, err
	}

	return allocation, nil
}
