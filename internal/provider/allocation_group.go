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
	"terraform-provider-doit/internal/provider/resource_allocation_group"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func (plan *allocationGroupResourceModel) getActions(ctx context.Context) (actions map[string]string, d diag.Diagnostics) {
	actions = make(map[string]string)
	var diags diag.Diagnostics
	if !plan.Rules.IsNull() {
		planRules := []resource_allocation_group.RulesValue{}
		diags = plan.Rules.ElementsAs(ctx, &planRules, false)
		d.Append(diags...)
		if d.HasError() {
			return
		}
		for i := range planRules {
			if !planRules[i].Id.IsNull() {
				actions[planRules[i].Id.ValueString()] = planRules[i].Action.ValueString()
			}
		}
	}
	return
}

func (plan *allocationGroupResourceModel) toRequest(ctx context.Context) (groupAllocationRequest models.GroupAllocationRequest, d diag.Diagnostics) {
	var diags diag.Diagnostics
	groupAllocationRequest.UnallocatedCosts = plan.UnallocatedCosts.ValueStringPointer()
	groupAllocationRequest.Description = plan.Description.ValueStringPointer()
	groupAllocationRequest.Name = plan.Name.ValueString()
	if !plan.Rules.IsNull() {
		planRules := []resource_allocation_group.RulesValue{}
		diags = plan.Rules.ElementsAs(ctx, &planRules, false)
		d.Append(diags...)
		if d.HasError() {
			return
		}
		groupAllocationRequest.Rules = make([]models.GroupAllocationRule, len(planRules))
		for i := range planRules {
			fmt.Println("Action is nil", planRules[i].Action.IsNull(), planRules[i].Action.ValueString())
			groupAllocationRequest.Rules[i] = models.GroupAllocationRule{
				Name:        planRules[i].Name.ValueStringPointer(),
				Id:          planRules[i].Id.ValueStringPointer(),
				Action:      models.GroupAllocationRuleAction(planRules[i].Action.ValueString()),
				Description: planRules[i].Description.ValueStringPointer(),
				Formula:     planRules[i].Formula.ValueStringPointer(),
			}
			// Don't send components if selecting existing allocation
			if !planRules[i].Components.IsNull() && planRules[i].Action.ValueString() != "select" {
				ruleComponents := []resource_allocation_group.ComponentsValue{}
				diags = planRules[i].Components.ElementsAs(ctx, &ruleComponents, true)
				d.Append(diags...)
				if d.HasError() {
					return
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
					diags := ruleComponents[j].Values.ElementsAs(ctx, &createComponents[j].Values, true)
					d.Append(diags...)
					if d.HasError() {
						return
					}
				}
				groupAllocationRequest.Rules[i].Components = &createComponents
			}
		}
	}
	return groupAllocationRequest, d
}

func (state *allocationGroupResourceModel) populate(groupAllocation *models.GroupAllocation, client *ClientTest, actions map[string]string, ctx context.Context) (d diag.Diagnostics) {
	var diags diag.Diagnostics
	state.Description = types.StringPointerValue(groupAllocation.Description)
	state.Id = types.StringPointerValue(groupAllocation.Id)
	state.AllocationType = types.StringValue("group")
	state.Type = types.StringValue("managed")
	state.Name = types.StringPointerValue(groupAllocation.Name)
	state.Cloud = types.StringNull()
	state.UnallocatedCosts = types.StringPointerValue(groupAllocation.UnallocatedCosts)
	state.TimeCreated = types.Int64PointerValue(groupAllocation.TimeCreated)
	state.TimeModified = types.Int64PointerValue(groupAllocation.TimeModified)
	// Overwrite components with refreshed state
	if groupAllocation.Rules != nil {
		stateRules := make([]attr.Value, len(*groupAllocation.Rules))
		for i, rule := range *groupAllocation.Rules {
			stateRule := resource_allocation_group.RulesValue{
				CreateTime: types.Int64PointerValue(rule.CreateTime),
				Id:         types.StringPointerValue(rule.Id),
				Name:       types.StringPointerValue(rule.Name),
				Owner:      types.StringPointerValue(rule.Owner),
				RulesType:  types.StringPointerValue(rule.Type),
				UpdateTime: types.Int64PointerValue(rule.UpdateTime),
				UrlUi:      types.StringPointerValue(rule.UrlUI),
			}
			if rule.AllocationType != nil {
				stateRule.AllocationType = types.StringValue(string(*rule.AllocationType))
			}
			// if the ruleId is not in the map of actions, it has to be a new rule
			if rule.Id != nil {
				action, ok := actions[*rule.Id]
				if !ok {
					action = "create"
				}
				stateRule.Action = types.StringValue(action)
			}

			var singleAllocation *models.SingleAllocation
			singleAllocation, err := client.GetAllocation(stateRule.Id.ValueString())
			if err != nil {
				diags.AddError(
					"Error Reading Doit Console Allocation",
					"Could not read Doit Console Allocation ID "+state.Id.ValueString()+": "+err.Error(),
				)
				return
			}
			// This is an ugly hack because we have two different version of the ComponentsValue / ComponentsType and we need to convert between them
			// This would not be necessary if we could unify resource_allocation and resource_allocation_group but the generator cannot be used with the
			// "full" OpenAPI spec, so we have to split it
			singleAllocationState := allocationResourceModel{}
			diags = singleAllocationState.populate(singleAllocation, ctx)
			d.Append(diags...)
			if d.HasError() {
				return
			}
			stateRule.Description, diags = singleAllocationState.Description.ToStringValue(ctx)
			d.Append(diags...)
			if d.HasError() {
				return
			}
			stateRule.Formula, diags = singleAllocationState.Rule.Formula.ToStringValue(ctx)
			d.Append(diags...)
			if d.HasError() {
				return
			}
			singleComponents := []resource_allocation.ComponentsValue{}
			diags = singleAllocationState.Rule.Components.ElementsAs(ctx, &singleComponents, false)
			d.Append(diags...)
			if d.HasError() {
				return
			}
			groupComponents := make([]attr.Value, len(singleComponents))
			for i := range singleComponents {
				groupComponents[i], diags = resource_allocation_group.NewComponentsValue(singleComponents[i].AttributeTypes(ctx), map[string]attr.Value{
					"include_null":      singleComponents[i].IncludeNull,
					"inverse_selection": singleComponents[i].InverseSelection,
					"key":               singleComponents[i].Key,
					"mode":              singleComponents[i].Mode,
					"type":              singleComponents[i].ComponentsType,
					"values":            singleComponents[i].Values,
				})
				d.Append(diags...)
				if d.HasError() {
					return
				}
			}
			stateRule.Components, diags = types.ListValue(groupComponents[0].Type(ctx), groupComponents)
			d.Append(diags...)
			if d.HasError() {
				return
			}

			stateRules[i], diags = resource_allocation_group.NewRulesValue(stateRule.AttributeTypes(ctx), map[string]attr.Value{
				"action":          stateRule.Action,
				"allocation_type": stateRule.AllocationType,
				"components":      stateRule.Components,
				"create_time":     stateRule.CreateTime,
				"description":     stateRule.Description,
				"formula":         stateRule.Formula,
				"id":              stateRule.Id,
				"name":            stateRule.Name,
				"owner":           stateRule.Owner,
				"type":            stateRule.RulesType,
				"update_time":     stateRule.UpdateTime,
				"url_ui":          stateRule.UrlUi,
			})
			d.Append(diags...)
			if d.HasError() {
				return
			}
		}
		state.Rules, diags = types.ListValue(stateRules[0].Type(ctx), stateRules)
		d.Append(diags...)
		if d.HasError() {
			return
		}
	}
	return diags
}

// CreateAllocationGroup - Create new allocation
func (c *ClientTest) CreateAllocationGroup(allocation models.GroupAllocationRequest) (*models.GroupAllocation, error) {
	rb, err := json.Marshal(allocation)
	if err != nil {
		return nil, err
	}
	urlRequestBase := fmt.Sprintf("%s/analytics/v1/allocations", c.HostURL)
	urlRequestContext := addContextToURL(c.Auth.CustomerContext, urlRequestBase)
	req, err := http.NewRequest("POST", urlRequestContext, strings.NewReader(string(rb)))
	log.Println("URL----------------")
	log.Println(req.URL)
	if err != nil {
		return nil, err
	}

	body, err := c.doRequest(req)
	if err != nil {
		log.Println("ERROR REQUEST----------------")
		log.Println(err)
		log.Println(string(rb))
		return nil, err
	}

	allocationResponse := models.GroupAllocation{}
	err = json.Unmarshal(body, &allocationResponse)
	if err != nil {
		log.Println("ERROR UNMARSHALL----------------")
		log.Println(err)
		return nil, err
	}
	log.Println("AllocationGroup response----------------")
	log.Println(allocationResponse)
	return &allocationResponse, nil
}

// UpdateAllocationGroup - Updates an allocation
func (c *ClientTest) UpdateAllocationGroup(allocationID string, allocation models.GroupAllocationRequest) (*models.GroupAllocation, error) {
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
	log.Println("Update URL----------------")
	log.Println(req.URL)
	body, err := c.doRequest(req)
	if err != nil {
		fmt.Println("ERROR REQUEST----------------")
		fmt.Println(err)
		fmt.Println(string(rb))
		return nil, err
	}

	allocationResponse := models.GroupAllocation{}
	err = json.Unmarshal(body, &allocationResponse)
	if err != nil {
		return nil, err
	}
	log.Println("AllocationGroup response----------------")
	log.Println(allocationResponse)
	return &allocationResponse, nil
}

func (c *ClientTest) DeleteAllocationGroup(allocationID string) error {
	urlRequestBase := fmt.Sprintf("%s/analytics/v1/allocations/%s", c.HostURL, allocationID)
	urlRequestContext := addContextToURL(c.Auth.CustomerContext, urlRequestBase)
	req, err := http.NewRequest("DELETE", urlRequestContext, nil)
	if err != nil {
		return err
	}

	_, err = c.doRequest(req)
	if err != nil {
		return err
	}

	return nil
}

// GetAllocationGroup - Returns a specifc allocation
func (c *ClientTest) GetAllocationGroup(orderID string) (*models.GroupAllocation, error) {
	urlRequestBase := fmt.Sprintf("%s/analytics/v1/allocations/%s", c.HostURL, orderID)
	urlRequestContext := addContextToURL(c.Auth.CustomerContext, urlRequestBase)
	req, err := http.NewRequest("GET", urlRequestContext, nil)
	if err != nil {
		return nil, err
	}

	body, err := c.doRequest(req)
	if err != nil {
		return nil, err
	}

	allocation := models.GroupAllocation{}
	err = json.Unmarshal(body, &allocation)
	if err != nil {
		return nil, err
	}

	return &allocation, nil
}
