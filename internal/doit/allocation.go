package doit

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"terraform-provider-doit/internal/doit/models"
	"terraform-provider-doit/internal/doit/resource_allocation"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
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
	var diags diag.Diagnostics

	// Get refreshed allocation value from DoiT using the ID from the state.
	resp, err := r.client.GetAllocation(ctx, state.Id.ValueString())
	if err != nil {
		if strings.Contains(err.Error(), "404") {
			// The resource was deleted. This is an edge case for create,
			// but necessary for the read function.
			state.Id = types.StringNull()
			return diags
		}
		diags.AddError(
			"Error Reading Doit Console Allocation",
			"Could not read Doit Console Allocation ID "+state.Id.ValueString()+": "+err.Error(),
		)
		return diags
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
		state.Rule.Formula = types.StringPointerValue(resp.Rule.Formula)
		if resp.Rule.Components != nil {
			stateComponents := make([]attr.Value, len(*resp.Rule.Components))
			for i, component := range *resp.Rule.Components {
				stateComponent := resource_allocation.ComponentsValue{
					IncludeNull:      types.BoolPointerValue(component.IncludeNull),
					InverseSelection: types.BoolPointerValue(component.InverseSelection),
					Key:              types.StringValue(component.Key),
					Mode:             types.StringValue(string(component.Mode)),
					ComponentsType:   types.StringValue(string(component.Type)),
				}
				values := make([]attr.Value, len(component.Values))
				for j := range component.Values {
					values[j] = types.StringValue(component.Values[j])
				}
				stateComponent.Values, diags = types.ListValue(types.StringType, values)
				diags.Append(diags...)
				if diags.HasError() {
					return diags
				}
				stateComponents[i], diags = resource_allocation.NewComponentsValue(stateComponent.AttributeTypes(ctx), map[string]attr.Value{
					"include_null":      stateComponent.IncludeNull,
					"inverse_selection": stateComponent.InverseSelection,
					"key":               stateComponent.Key,
					"mode":              stateComponent.Mode,
					"type":              stateComponent.ComponentsType,
					"values":            stateComponent.Values,
				})
				diags.Append(diags...)
				if diags.HasError() {
					return diags
				}
			}
			// Using the first item's type is a bit of a hack and shouldn't be necessary as the list type should be resource_allocation.ComponentsType:
			// var elementType resource_allocation.ComponentsType
			// for idx, element := range stateComponents {
			// 	if !elementType.Equal(element.Type(ctx)) {
			// 		fmt.Printf("List Element Type: %s\n", elementType.String())
			// 		fmt.Printf("List Index (%d) Element Type: %s\n", idx, element.Type(ctx))
			// 		fmt.Printf("Equal to self 1: %v\n", element.Type(ctx).Equal(element.Type(ctx)))
			// 		fmt.Printf("Equal to self 2: %v\n", elementType.Equal(elementType))
			// 		fmt.Printf("Other way: %v\n", element.Type(ctx).Equal(elementType))
			// 	}
			// }
			// elements := state.Rule.Components.Elements()
			// fmt.Println("0 null", elements[0].IsNull())
			// fmt.Println("1 null", elements[1].IsNull())
			state.Rule.Components, diags = types.ListValue(stateComponents[0].Type(ctx), stateComponents)
			diags.Append(diags...)
			if diags.HasError() {
				return diags
			}
		}
	}
	return diags
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
