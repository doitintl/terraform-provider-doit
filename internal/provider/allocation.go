package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"terraform-provider-doit/internal/provider/models"
	"terraform-provider-doit/internal/provider/resource_allocation"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
)

func (plan *allocationResourceModel) toRequest(ctx context.Context) (models.Allocation, diag.Diagnostics) {
	var (
		req   models.Allocation
		diags diag.Diagnostics
	)

	allocationType := models.AllocationAllocationType(plan.AllocationType.ValueString())
	req.AllocationType = &allocationType
	req.AnomalyDetection = plan.AnomalyDetection.ValueBoolPointer()
	req.Description = plan.Description.ValueStringPointer()
	req.Name = plan.Name.ValueStringPointer()
	req.Type = plan.Type.ValueStringPointer()
	if !plan.Rule.IsNull() {
		req.Rule = &models.AllocationRule{
			Formula: plan.Rule.Formula.ValueStringPointer(),
		}
		if !plan.Rule.Components.IsNull() {
			planComponents := []resource_allocation.ComponentsValue{}
			diags = plan.Rule.Components.ElementsAs(ctx, &planComponents, false)
			diags.Append(diags...)
			if diags.HasError() {
				return req, diags
			}
			createComponents := make([]models.AllocationComponent, len(planComponents))
			req.Rule.Components = &createComponents
			for i := range planComponents {
				createComponent := models.AllocationComponent{
					IncludeNull:      planComponents[i].IncludeNull.ValueBoolPointer(),
					InverseSelection: planComponents[i].InverseSelection.ValueBoolPointer(),
					Key:              planComponents[i].Key.ValueString(),
					Mode:             models.AllocationComponentMode(planComponents[i].Mode.ValueString()),
					Type:             models.DimensionsTypes(planComponents[i].ComponentsType.ValueString()),
				}
				diags = planComponents[i].Values.ElementsAs(ctx, &createComponent.Values, false)
				diags.Append(diags...)
				if diags.HasError() {
					return req, diags
				}
				createComponents[i] = createComponent
			}
			req.Rule.Components = &createComponents
		}
	}
	if !plan.Rules.IsNull() {
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

func (r *allocationResource) populateState(ctx context.Context, state *allocationResourceModel) diag.Diagnostics {
	var d diag.Diagnostics

	// Get refreshed allocation value from DoiT using the ID from the state.
	resp, err := r.client.GetAllocation(ctx, state.Id.ValueString())
	if err != nil {
		if strings.Contains(err.Error(), "404") {
			// The resource was deleted. This is an edge case for create,
			// but necessary for the read function.
			state.Id = types.StringNull()
			return d
		}
		d.AddError(
			"Error Reading Doit Console Allocation",
			"Could not read Doit Console Allocation ID "+state.Id.ValueString()+": "+err.Error(),
		)
		return d
	}

	state.Id = types.StringPointerValue(resp.Id)
	state.Type = types.StringPointerValue(resp.Type)
	state.Description = types.StringPointerValue(resp.Description)
	state.AnomalyDetection = types.BoolPointerValue(resp.AnomalyDetection)
	state.CreateTime = types.Int64PointerValue(resp.CreateTime)
	state.UpdateTime = types.Int64PointerValue(resp.UpdateTime)
	state.Name = types.StringPointerValue(resp.Name)

	if resp.AllocationType != nil {
		state.AllocationType = types.StringValue(string(*resp.AllocationType))
	}

	if resp.Rule != nil {
		var diags diag.Diagnostics
		state.Rule.Formula = types.StringPointerValue(resp.Rule.Formula)
		state.Rule.Components, diags = toAllocationRuleComponentsListValue(ctx, resp.Rule.Components)
		d.Append(diags...)
		if d.HasError() {
			return d
		}
	}

	return d
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

func toAllocationRuleComponentsListValue(ctx context.Context, components *[]models.AllocationComponent) (res basetypes.ListValue, diags diag.Diagnostics) {
	if components == nil || len(*components) == 0 {
		return
	}
	var d diag.Diagnostics
	stateComponents := make([]attr.Value, len(*components))
	for i, component := range *components {
		values := make([]attr.Value, len(component.Values))
		for j := range component.Values {
			values[j] = types.StringValue(component.Values[j])
		}
		v := resource_allocation.ComponentsValue{
			IncludeNull:      types.BoolPointerValue(component.IncludeNull),
			InverseSelection: types.BoolPointerValue(component.InverseSelection),
			Key:              types.StringValue(component.Key),
			Mode:             types.StringValue(string(component.Mode)),
			ComponentsType:   types.StringValue(string(component.Type)),
		}
		v.Values, d = types.ListValue(types.StringType, values)
		diags.Append(d...)
		if diags.HasError() {
			return
		}
		stateComponents[i], d = resource_allocation.NewComponentsValue(v.AttributeTypes(ctx), map[string]attr.Value{
			"include_null":      v.IncludeNull,
			"inverse_selection": v.InverseSelection,
			"key":               v.Key,
			"mode":              v.Mode,
			"type":              v.ComponentsType,
			"values":            v.Values,
		})
		diags.Append(d...)
		if diags.HasError() {
			return
		}
	}
	res, diags = types.ListValueFrom(ctx, stateComponents[0].Type(ctx), stateComponents)
	d.Append(diags...)
	return
}
