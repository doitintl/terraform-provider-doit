package provider

import (
	"context"
	"fmt"
	"log"
	"strings"
	"terraform-provider-doit/internal/provider/resource_allocation_group"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var _ resource.Resource = (*allocationGroupResource)(nil)

func NewAllocationGroupResource() resource.Resource {
	return &allocationGroupResource{}
}

type allocationGroupResource struct {
	client *ClientTest
}

type allocationGroupResourceModel struct {
	resource_allocation_group.AllocationGroupModel
}

// Configure adds the provider configured client to the resource.
func (r *allocationGroupResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	log.Print(" allocation group Configure")
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
func (r *allocationGroupResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_allocation_group"
}

func (r *allocationGroupResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = resource_allocation_group.AllocationGroupResourceSchema(ctx)
}

func (r *allocationGroupResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	log.Println(" allocation group Create")
	var plan allocationGroupResourceModel

	// Read Terraform plan data into the model
	diags := req.Plan.Get(ctx, &plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Generate API request body from plan
	allocationGroup, diags := plan.toRequest(ctx)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Save data into Terraform state
	allocationgroupResponse, err := r.client.CreateAllocationGroup(allocationGroup)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error creating allocationgroup",
			"Could not create allocationgroup, unexpected error: "+err.Error(),
		)
		return
	}
	log.Println("allocationgroup id---------------------------------------------------")
	log.Println(allocationgroupResponse.Id)
	allocationCreated, err := r.client.GetAllocationGroup(*allocationgroupResponse.Id)
	if err != nil {
		if strings.Contains(err.Error(), "404") {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError(
			"Error Reading Doit Console Allocation Group",
			"Could not read Doit Console Allocation Group ID "+*allocationgroupResponse.Id+": "+err.Error(),
		)
		return
	}
	plan.Id = types.StringPointerValue(allocationgroupResponse.Id)
	plan.populate(allocationCreated, r.client, ctx)

	// Set state to fully populated data
	diags = resp.State.Set(ctx, plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
}

func (r *allocationGroupResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	// Get current state
	var state allocationGroupResourceModel
	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	log.Println("state read before")
	log.Println(state)
	log.Print("state id:::::::::::::::::::::::::")
	log.Print(state.Id.ValueString())

	// Get refreshed allocation value from DoiT
	allocation, err := r.client.GetAllocationGroup(state.Id.ValueString())
	if err != nil {
		if strings.Contains(err.Error(), "404") {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError(
			"Error Reading Doit Console Allocation Group",
			"Could not read Doit Console Allocation Group ID "+state.Id.ValueString()+": "+err.Error(),
		)
		return
	}
	diags = state.populate(allocation, r.client, ctx)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Set refreshed state
	diags = resp.State.Set(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
}

func (r *allocationGroupResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	log.Print("allocation group Update")
	// Retrieve values from plan
	var plan allocationGroupResourceModel
	diags := req.Plan.Get(ctx, &plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	var state allocationGroupResourceModel
	diags = req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Generate API request body from plan
	allocation, diags := plan.toRequest(ctx)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Update existing allocation
	_, err := r.client.UpdateAllocationGroup(state.Id.ValueString(), allocation)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Updating DoiT Allocation",
			"Could not update allocation, unexpected error: "+err.Error(),
		)
		return
	}

	// Fetch updated items from GetAllocation as UpdateAllocation items are not
	// populated.
	allocationResponse, err := r.client.GetAllocationGroup(state.Id.ValueString())
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Reading Doit Console Allocation",
			"Could not read Doit Console allocation ID "+plan.Id.ValueString()+": "+err.Error(),
		)
		return
	}
	fmt.Println("Rules length", len(*allocationResponse.Rules))
	diags = state.populate(allocationResponse, r.client, ctx)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Set refreshed state
	diags = resp.State.Set(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
}

func (r *allocationGroupResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	log.Print(" allocation group Delete")
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
