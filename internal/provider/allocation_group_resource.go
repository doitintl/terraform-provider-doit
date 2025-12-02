package provider

import (
	"context"
	"fmt"

	"terraform-provider-doit/internal/provider/resource_allocation_group"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

type (
	allocationGroupResource struct {
		client *Client
	}
	allocationGroupResourceModel struct {
		resource_allocation_group.AllocationGroupModel
	}
)

func NewAllocationGroupResource() resource.Resource {
	return new(allocationGroupResource)
}

// Configure adds the provider configured client to the resource.
func (r *allocationGroupResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}

	client, ok := req.ProviderData.(*Client)
	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Data Source Configure Type",
			fmt.Sprintf("Expected *Client, got: %T. Please report this issue to the provider developers.", req.ProviderData),
		)
		return
	}

	r.client = client
}

func (r *allocationGroupResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_allocation_group"
}

func (r *allocationGroupResource) Schema(ctx context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = resource_allocation_group.AllocationGroupResourceSchema(ctx)
}

func (r *allocationGroupResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	resp.Diagnostics.Append(
		diag.NewWarningDiagnostic("doit_allocation_group resource is deprecated", "use doit_allocation instead"),
	)
	plan := new(allocationGroupResourceModel)
	diags := req.Plan.Get(ctx, plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	allocationGroupReq, diags := plan.toRequest(ctx)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	allocationGroupResp, err := r.client.CreateAllocationGroup(ctx, allocationGroupReq)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error creating allocation group",
			"Could not create allocation group, unexpected error: "+err.Error(),
		)
		return
	}

	state := new(allocationGroupResourceModel)
	state.Id = types.StringPointerValue(allocationGroupResp.Id)

	diags = r.populateState(ctx, plan, state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, state)...)
}

func (r *allocationGroupResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	resp.Diagnostics.Append(
		diag.NewWarningDiagnostic("doit_allocation_group resource is deprecated", "use doit_allocation instead"),
	)
	state := new(allocationGroupResourceModel)
	diags := req.State.Get(ctx, state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	// as there's no plan in the read phase, we assimilate the plan to the current state of the resource
	diags = r.populateState(ctx, state, state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	if state.Id.IsNull() {
		resp.State.RemoveResource(ctx)
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, state)...)
}

func (r *allocationGroupResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	resp.Diagnostics.Append(
		diag.NewWarningDiagnostic("doit_allocation_group resource is deprecated", "use doit_allocation instead"),
	)
	plan := new(allocationGroupResourceModel)
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

	state := new(allocationGroupResourceModel)
	diags = req.State.Get(ctx, state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	respUpdateAlg, err := r.client.UpdateAllocationGroup(ctx, state.Id.ValueString(), allocationGroup)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Updating DoiT Allocation Group",
			"Could not update allocation group, unexpected error: "+err.Error(),
		)
		return
	}

	print(respUpdateAlg)

	diags = r.populateState(ctx, plan, state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, state)...)
}

func (r *allocationGroupResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	resp.Diagnostics.Append(
		diag.NewWarningDiagnostic("doit_allocation_group resource is deprecated", "use doit_allocation instead"),
	)
	// Retrieve values from state
	state := new(allocationGroupResourceModel)
	diags := req.State.Get(ctx, state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	err := r.client.DeleteAllocationGroup(ctx, state.Id.ValueString())
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Deleting DoiT Allocation Group",
			"Could not delete allocation, unexpected error: "+err.Error(),
		)
		return
	}

	resp.State.RemoveResource(ctx)
}
