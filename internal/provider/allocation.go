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
	"time"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func (plan *allocationResourceModel) toRequest(ctx context.Context) (allocation models.SingleAllocation, d diag.Diagnostics) {
	var diags diag.Diagnostics
	allocationType := models.SingleAllocationAllocationType(plan.AllocationType.ValueString())
	allocation.AllocationType = &allocationType
	allocation.AnomalyDetection = plan.AnomalyDetection.ValueBoolPointer()
	allocation.Description = plan.Description.ValueStringPointer()
	allocation.Name = plan.Name.ValueStringPointer()
	allocation.Type = plan.Type.ValueStringPointer()
	if !plan.Rule.IsNull() {
		allocation.Rule = &models.AllocationRule{
			Formula: plan.Rule.Formula.ValueStringPointer(),
		}
		if !plan.Rule.Components.IsNull() {
			planComponents := []resource_allocation.ComponentsValue{}
			diags = plan.Rule.Components.ElementsAs(ctx, &planComponents, false)
			d.Append(diags...)
			if d.HasError() {
				return
			}
			createComponents := make([]models.AllocationComponent, len(planComponents))
			allocation.Rule.Components = &createComponents
			for i := range planComponents {
				createComponent := models.AllocationComponent{
					IncludeNull:      planComponents[i].IncludeNull.ValueBoolPointer(),
					InverseSelection: planComponents[i].InverseSelection.ValueBoolPointer(),
					Key:              planComponents[i].Key.ValueString(),
					Mode:             models.AllocationComponentMode(planComponents[i].Mode.ValueString()),
					Type:             models.DimensionsTypes(planComponents[i].ComponentsType.ValueString()),
				}
				diags = planComponents[i].Values.ElementsAs(ctx, &createComponent.Values, false)
				d.Append(diags...)
				if d.HasError() {
					return
				}
				createComponents[i] = createComponent
			}
			allocation.Rule.Components = &createComponents
		}
	}
	return allocation, d
}

func (state *allocationResourceModel) populate(allocation *models.SingleAllocation, ctx context.Context) (d diag.Diagnostics) {
	var diags diag.Diagnostics
	state.Id = types.StringPointerValue(allocation.Id)
	state.Description = types.StringPointerValue(allocation.Description)
	state.Type = types.StringPointerValue(allocation.Type)
	if allocation.AllocationType != nil {
		allocationType := string(*allocation.AllocationType)
		state.AllocationType = types.StringValue(allocationType)
	}
	state.AnomalyDetection = types.BoolPointerValue(allocation.AnomalyDetection)
	state.UpdateTime = types.Int64Value(time.Now().Unix())
	// Overwrite components with refreshed state
	if allocation.Rule != nil {
		state.Rule.Formula = types.StringPointerValue(allocation.Rule.Formula)
		if allocation.Rule.Components != nil {
			stateComponents := make([]attr.Value, len(*allocation.Rule.Components))
			for i, component := range *allocation.Rule.Components {
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
				d.Append(diags...)
				if d.HasError() {
					return
				}
				stateComponents[i], diags = resource_allocation.NewComponentsValue(stateComponent.AttributeTypes(ctx), map[string]attr.Value{
					"include_null":      stateComponent.IncludeNull,
					"inverse_selection": stateComponent.InverseSelection,
					"key":               stateComponent.Key,
					"mode":              stateComponent.Mode,
					"type":              stateComponent.ComponentsType,
					"values":            stateComponent.Values,
				})
				d.Append(diags...)
				if d.HasError() {
					return
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
			d.Append(diags...)
			if d.HasError() {
				return
			}
		}
	}
	return diags
}

// CreateAllocation - Create new allocation
func (c *ClientTest) CreateAllocation(allocation models.SingleAllocation) (*models.SingleAllocation, error) {
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

	allocationResponse := models.SingleAllocation{}
	err = json.Unmarshal(body, &allocationResponse)
	if err != nil {
		log.Println("ERROR UNMARSHALL----------------")
		log.Println(err)
		return nil, err
	}
	log.Println("Allocation response----------------")
	log.Println(allocationResponse)
	return &allocationResponse, nil
}

// UpdateAllocation - Updates an allocation
func (c *ClientTest) UpdateAllocation(allocationID string, allocation models.SingleAllocation) (*models.SingleAllocation, error) {
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
		return nil, err
	}

	allocationResponse := models.SingleAllocation{}
	err = json.Unmarshal(body, &allocationResponse)
	if err != nil {
		return nil, err
	}
	log.Println("Allocation response----------------")
	log.Println(allocationResponse)
	return &allocationResponse, nil
}

func (c *ClientTest) DeleteAllocation(allocationID string) error {
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

// GetAllocation - Returns a specifc allocation
func (c *ClientTest) GetAllocation(orderID string) (*models.SingleAllocation, error) {
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

	allocation := models.SingleAllocation{}
	err = json.Unmarshal(body, &allocation)
	if err != nil {
		return nil, err
	}

	return &allocation, nil
}
