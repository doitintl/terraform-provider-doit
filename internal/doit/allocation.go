package doit

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"terraform-provider-doit/internal/doit/models"
	"terraform-provider-doit/internal/doit/resource_allocation"
	"terraform-provider-doit/internal/doit/resource_allocation_group"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
)

func (plan *allocationResourceModel) toRequest(ctx context.Context) (models.SingleAllocation, diag.Diagnostics) {
	var (
		req   models.SingleAllocation
		diags diag.Diagnostics
	)

	allocationType := models.SingleAllocationAllocationType(plan.AllocationType.ValueString())
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
		state.Rule.Components, diags = toAllocationRuleComponentsListValue[resource_allocation.ComponentsType](ctx, resp.Rule.Components)
		d.Append(diags...)
		if d.HasError() {
			return d
		}
	}

	return d
}

func (c *Client) CreateAllocation(ctx context.Context, allocation models.SingleAllocation) (*models.SingleAllocation, error) {
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

	allocationResponse := models.SingleAllocation{}
	err = json.Unmarshal(body, &allocationResponse)
	if err != nil {
		return nil, err
	}
	return &allocationResponse, nil
}

func (c *Client) UpdateAllocation(ctx context.Context, allocationID string, allocation models.SingleAllocation) (*models.SingleAllocation, error) {
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

	allocationResponse := models.SingleAllocation{}
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

func (c *Client) GetAllocation(ctx context.Context, id string) (*models.SingleAllocation, error) {
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

	allocation := models.SingleAllocation{}
	err = json.Unmarshal(body, &allocation)
	if err != nil {
		return nil, err
	}

	return &allocation, nil
}

func toAllocationRuleComponentsListValue[T resource_allocation.ComponentsType | resource_allocation_group.ComponentsType](ctx context.Context, components *[]models.AllocationComponent) (basetypes.ListValue, diag.Diagnostics) {
	var (
		res      basetypes.ListValue
		d, diags diag.Diagnostics
	)

	if components == nil || len(*components) == 0 {
		return res, nil
	}

	stateComponents := make([]attr.Value, len(*components))
	for i, component := range *components {

		values := make([]attr.Value, len(component.Values))
		for j := range component.Values {
			values[j] = types.StringValue(component.Values[j])
		}

		switch any(T{}).(type) {
		case resource_allocation.ComponentsType:
			v := resource_allocation.ComponentsValue{
				IncludeNull:      types.BoolPointerValue(component.IncludeNull),
				InverseSelection: types.BoolPointerValue(component.InverseSelection),
				Key:              types.StringValue(component.Key),
				Mode:             types.StringValue(string(component.Mode)),
				ComponentsType:   types.StringValue(string(component.Type)),
			}
			v.Values, diags = types.ListValue(types.StringType, values)
			d.Append(diags...)
			if d.HasError() {
				return res, d
			}

			stateComponents[i], diags = resource_allocation.NewComponentsValue(v.AttributeTypes(ctx), map[string]attr.Value{
				"include_null":      v.IncludeNull,
				"inverse_selection": v.InverseSelection,
				"key":               v.Key,
				"mode":              v.Mode,
				"type":              v.ComponentsType,
				"values":            v.Values,
			})
			d.Append(diags...)
			if d.HasError() {
				return res, d
			}

		case resource_allocation_group.ComponentsType:
			v := resource_allocation_group.ComponentsValue{
				IncludeNull:      types.BoolPointerValue(component.IncludeNull),
				InverseSelection: types.BoolPointerValue(component.InverseSelection),
				Key:              types.StringValue(component.Key),
				Mode:             types.StringValue(string(component.Mode)),
				ComponentsType:   types.StringValue(string(component.Type)),
			}

			v.Values, diags = types.ListValue(types.StringType, values)
			d.Append(diags...)
			if d.HasError() {
				return res, d
			}

			stateComponents[i], diags = resource_allocation_group.NewComponentsValue(v.AttributeTypes(ctx), map[string]attr.Value{
				"include_null":      v.IncludeNull,
				"inverse_selection": v.InverseSelection,
				"key":               v.Key,
				"mode":              v.Mode,
				"type":              v.ComponentsType,
				"values":            v.Values,
			})
			d.Append(diags...)
			if d.HasError() {
				return res, d
			}
		}
	}

	res, diags = types.ListValueFrom(ctx, stateComponents[0].Type(ctx), stateComponents)
	d.Append(diags...)

	return res, d
}
