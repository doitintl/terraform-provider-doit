package provider

import (
	"context"
	"fmt"
	"log"
	"strings"
	"terraform-provider-doit/internal/provider/models"
	"terraform-provider-doit/internal/provider/resource_allocation"
	"time"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
)

var _ resource.Resource = (*allocationResource)(nil)

func NewAllocationResource() resource.Resource {
	return &allocationResource{}
}

type allocationResource struct {
	client *ClientTest
}

type allocationResourceModel struct {
	resource_allocation.AllocationModel
}

// Configure adds the provider configured client to the resource.
func (r *allocationResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	log.Print(" allocation Configure")
	if req.ProviderData == nil {
		return
	}

	client, ok := req.ProviderData.(*ClientTest)

	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Data Source Configure Type",
			fmt.Sprintf("Expected *ClientTest, got: %T. Please report this issue to the provider developers.", req.ProviderData),
		)

		return
	}

	r.client = client
}

func (r *allocationResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_allocation"
}

func (r *allocationResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = resource_allocation.AllocationResourceSchema(ctx)
}

func (r *allocationResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	log.Println(" allocation Create")
	var plan allocationResourceModel

	// Read Terraform plan data into the model
	diags := req.Plan.Get(ctx, &plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	// Generate API request body from plan
	var allocation models.SingleAllocation
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
			resp.Diagnostics.Append(diags...)
			if resp.Diagnostics.HasError() {
				return
			}
			createComponents := make([]models.AllocationComponent, 0)
			allocation.Rule.Components = &[]models.AllocationComponent{}
			for _, component := range planComponents {
				createComponent := models.AllocationComponent{
					IncludeNull:      component.IncludeNull.ValueBoolPointer(),
					InverseSelection: component.InverseSelection.ValueBoolPointer(),
					Key:              component.Key.ValueString(),
					Mode:             models.AllocationComponentMode(component.Mode.ValueString()),
					Type:             models.DimensionsTypes(component.ComponentsType.ValueString()),
				}
				diags = component.Values.ElementsAs(ctx, &createComponent.Values, false)
				resp.Diagnostics.Append(diags...)
				if resp.Diagnostics.HasError() {
					return
				}
				createComponents = append(createComponents, createComponent)
			}
			allocation.Rule.Components = &createComponents
		}
	}

	// Save data into Terraform state
	allocationResponse, err := r.client.CreateAllocation(allocation)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error creating allocation",
			"Could not create allocation, unexpected error: "+err.Error(),
		)
		return
	}
	log.Println("allocation id---------------------------------------------------")
	log.Println(allocationResponse.Id)
	plan.Id = types.StringPointerValue(allocationResponse.Id)
	plan.UpdateTime = types.Int64Value(time.Now().Unix())

	// Set state to fully populated data
	diags = resp.State.Set(ctx, plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
}

func (r *allocationResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	// Get current state
	var state allocationResourceModel
	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	log.Print("state id:::::::::::::::::::::::::")
	log.Print(state.Id.ValueString())
	// Get refreshed allocation value from DoiT
	allocation, err := r.client.GetAllocation(state.Id.ValueString())
	if err != nil {
		if strings.Contains(err.Error(), "404") {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError(
			"Error Reading Doit Console Allocation",
			"Could not read Doit Console Allocation ID "+state.Id.ValueString()+": "+err.Error(),
		)
		return
	}
	state.Id = types.StringPointerValue(allocation.Id)
	state.Description = types.StringPointerValue(allocation.Description)
	state.Type = types.StringPointerValue(allocation.Type)
	state.Name = types.StringPointerValue(allocation.Name)
	if allocation.AllocationType != nil {
		allocationType := string(*allocation.AllocationType)
		state.AllocationType = types.StringValue(allocationType)
	}
	state.AnomalyDetection = types.BoolPointerValue(allocation.AnomalyDetection)
	state.UpdateTime = types.Int64Value(time.Now().Unix())
	// Overwrite components with refreshed state
	if allocation.Rule != nil {
		state.Rule = resource_allocation.RuleValue{
			Formula: types.StringPointerValue(allocation.Rule.Formula),
		}
		if allocation.Rule.Components != nil {
			stateComponents := []resource_allocation.ComponentsValue{}
			for _, component := range *allocation.Rule.Components {
				stateComponent := resource_allocation.ComponentsValue{
					IncludeNull:      types.BoolPointerValue(component.IncludeNull),
					InverseSelection: types.BoolPointerValue(component.InverseSelection),
					Key:              types.StringValue(component.Key),
					Mode:             types.StringValue(string(component.Mode)),
					ComponentsType:   types.StringValue(string(component.Type)),
				}
				stateComponent.Values, diags = basetypes.NewListValueFrom(ctx, types.StringType, stateComponents)
				resp.Diagnostics.Append(diags...)
				if resp.Diagnostics.HasError() {
					return
				}
				stateComponents = append(stateComponents, stateComponent)
			}
			state.Rule.Components, diags = basetypes.NewListValueFrom(ctx, resource_allocation.ComponentsType{}, stateComponents)
			resp.Diagnostics.Append(diags...)
			if resp.Diagnostics.HasError() {
				return
			}
		}
	}

	// Set refreshed state
	diags = resp.State.Set(ctx, &state)
	log.Print("state read")
	log.Print(state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
}

func (r *allocationResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	log.Print("allocation Update")
	// Retrieve values from plan
	var plan allocationResourceModel
	diags := req.Plan.Get(ctx, &plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	var state allocationResourceModel
	diags = req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Generate API request body from plan
	var allocation models.SingleAllocation
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
			resp.Diagnostics.Append(diags...)
			if resp.Diagnostics.HasError() {
				return
			}
			createComponents := make([]models.AllocationComponent, 0)
			allocation.Rule.Components = &[]models.AllocationComponent{}
			for _, component := range planComponents {
				createComponent := models.AllocationComponent{
					IncludeNull:      component.IncludeNull.ValueBoolPointer(),
					InverseSelection: component.InverseSelection.ValueBoolPointer(),
					Key:              component.Key.ValueString(),
					Mode:             models.AllocationComponentMode(component.Mode.ValueString()),
					Type:             models.DimensionsTypes(component.ComponentsType.ValueString()),
				}
				diags = component.Values.ElementsAs(ctx, &createComponent.Values, false)
				resp.Diagnostics.Append(diags...)
				if resp.Diagnostics.HasError() {
					return
				}
				createComponents = append(createComponents, createComponent)
			}
			allocation.Rule.Components = &createComponents
		}
	}

	// Update existing allocation
	_, err := r.client.UpdateAllocation(state.Id.ValueString(), allocation)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Updating DoiT Allocation",
			"Could not update allocation, unexpected error: "+err.Error(),
		)
		return
	}

	// Fetch updated items from GetAllocation as UpdateAllocation items are not
	// populated.
	allocationResponse, err := r.client.GetAllocation(state.Id.ValueString())
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Reading Doit Console Allocation",
			"Could not read Doit Console allocation ID "+plan.Id.ValueString()+": "+err.Error(),
		)
		return
	}

	// Update resource state with updated items and timestamp
	state.Id = types.StringPointerValue(allocationResponse.Id)
	state.Description = types.StringPointerValue(allocationResponse.Description)
	state.Type = types.StringPointerValue(allocationResponse.Type)
	state.Name = types.StringPointerValue(allocationResponse.Name)
	if allocationResponse.AllocationType != nil {
		allocationType := string(*allocationResponse.AllocationType)
		state.AllocationType = types.StringValue(allocationType)
	}
	state.AnomalyDetection = types.BoolPointerValue(allocationResponse.AnomalyDetection)
	state.UpdateTime = types.Int64Value(time.Now().Unix())
	// Overwrite components with refreshed state
	if allocationResponse.Rule != nil {
		state.Rule = resource_allocation.RuleValue{
			Formula: types.StringPointerValue(allocationResponse.Rule.Formula),
		}
		if allocationResponse.Rule.Components != nil {
			stateComponents := []resource_allocation.ComponentsValue{}
			for _, component := range *allocationResponse.Rule.Components {
				stateComponent := resource_allocation.ComponentsValue{
					IncludeNull:      types.BoolPointerValue(component.IncludeNull),
					InverseSelection: types.BoolPointerValue(component.InverseSelection),
					Key:              types.StringValue(component.Key),
					Mode:             types.StringValue(string(component.Mode)),
					ComponentsType:   types.StringValue(string(component.Type)),
				}
				stateComponent.Values, diags = basetypes.NewListValueFrom(ctx, types.StringType, stateComponents)
				resp.Diagnostics.Append(diags...)
				if resp.Diagnostics.HasError() {
					return
				}
				stateComponents = append(stateComponents, stateComponent)
			}
			state.Rule.Components, diags = basetypes.NewListValueFrom(ctx, resource_allocation.ComponentsType{}, stateComponents)
			resp.Diagnostics.Append(diags...)
			if resp.Diagnostics.HasError() {
				return
			}
		}
	}

	// Set refreshed state
	diags = resp.State.Set(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
}

func (r *allocationResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	log.Print(" allocation Delete")
	// Retrieve values from state
	var state allocationResourceModel
	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Delete existing allocation
	err := r.client.DeleteAllocation(state.Id.ValueString())
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Deleting DoiT Allocation",
			"Could not delete allocation, unexpected error: "+err.Error(),
		)
		return
	}
}
