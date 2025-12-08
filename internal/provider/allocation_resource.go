package provider

import (
	"context"
	"fmt"
	"log"
	"time"

	"terraform-provider-doit/internal/provider/models"
	"terraform-provider-doit/internal/provider/resource_allocation"

	"github.com/cenkalti/backoff/v4"
	"github.com/hashicorp/terraform-plugin-framework-validators/resourcevalidator"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

type (
	allocationResource struct {
		client *models.ClientWithResponses
	}
	allocationResourceModel struct {
		resource_allocation.AllocationModel
	}
)

// Ensure the implementation satisfies the expected interfaces.
var (
	_ resource.Resource                     = &allocationResource{}
	_ resource.ResourceWithConfigure        = &allocationResource{}
	_ resource.ResourceWithImportState      = &allocationResource{}
	_ resource.ResourceWithConfigValidators = &allocationResource{}
)

func NewAllocationResource() resource.Resource {
	return new(allocationResource)
}

// Configure adds the provider configured client to the resource.
func (r *allocationResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}

	client, ok := req.ProviderData.(*Clients)
	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Data Source Configure Type",
			fmt.Sprintf("Expected *Clients, got: %T. Please report this issue to the provider developers.", req.ProviderData),
		)
		return
	}

	r.client = client.NewClient
}

func (r *allocationResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

func (r *allocationResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_allocation"
}

func (r *allocationResource) Schema(ctx context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = resource_allocation.AllocationResourceSchema(ctx)
}

func (r *allocationResource) ConfigValidators(ctx context.Context) []resource.ConfigValidator {
	return []resource.ConfigValidator{
		resourcevalidator.ExactlyOneOf(
			path.MatchRoot("rule"),
			path.MatchRoot("rules"),
		),
		resourcevalidator.RequiredTogether(
			path.MatchRoot("rules"),
			path.MatchRoot("unallocated_costs"),
		),
		resourcevalidator.Conflicting(
			path.MatchRoot("rule"),
			path.MatchRoot("unallocated_costs"),
		),
	}
}

func (r *allocationResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	plan := new(allocationResourceModel)

	diags := req.Plan.Get(ctx, plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Generate API request body from state
	allocationReq, diags := plan.toCreateRequest(ctx)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Save state into Terraform state
	allocationResp, err := r.client.CreateAllocationWithResponse(ctx, allocationReq)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error creating allocation",
			"Could not create allocation, unexpected error: "+err.Error(),
		)
		return
	}

	if allocationResp.StatusCode() != 200 && allocationResp.StatusCode() != 201 {
		resp.Diagnostics.AddError(
			"Error creating allocation",
			fmt.Sprintf("Could not create allocation, status: %d, body: %s", allocationResp.StatusCode(), string(allocationResp.Body)),
		)
		return
	}

	if allocationResp.JSON200 == nil {
		resp.Diagnostics.AddError(
			"Error creating allocation",
			"Could not create allocation, empty response",
		)
		return
	}

	plan.Id = types.StringPointerValue(allocationResp.JSON200.Id)

	diags = r.populateState(ctx, plan, nil)
	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.Append(diags...)
	resp.Diagnostics.Append(resp.State.Set(ctx, plan)...)
}

func (r *allocationResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	state := new(allocationResourceModel)
	diags := req.State.Get(ctx, state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	diags = r.populateState(ctx, state, nil)
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

func (r *allocationResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	plan := new(allocationResourceModel)
	diags := req.Plan.Get(ctx, plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Generate API request body from plan
	allocation, diags := plan.toUpdateRequest(ctx)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	state := new(allocationResourceModel)
	diags = req.State.Get(ctx, state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Update the allocation
	allocationResponse, err := r.client.UpdateAllocationWithResponse(ctx, state.Id.ValueString(), allocation)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error updating allocation",
			"Could not update allocation, unexpected error: "+err.Error(),
		)
		return
	}

	if allocationResponse.StatusCode() != 200 {
		resp.Diagnostics.AddError(
			"Error updating allocation",
			fmt.Sprintf("Could not update allocation, status: %d, body: %s", allocationResponse.StatusCode(), string(allocationResponse.Body)),
		)
		return
	}

	// Wait for consistency using backoff.
	// The API is eventually consistent, so we poll until Name, Description, and Rule/Rules are present.
	// This prevents Terraform from seeing null values immediately after an update.
	var lastFetchedResp *models.Allocation
	consistencyOp := func() error {
		getResp, err := r.client.GetAllocationWithResponse(ctx, state.Id.ValueString())
		if err != nil {
			// Allow retry on transient network errors
			return err
		}
		if getResp.JSON200 == nil {
			return fmt.Errorf("empty response from API")
		}

		lastFetchedResp = getResp.JSON200

		// Check that essential fields are not nil (eventual consistency issue)
		if getResp.JSON200.Name == nil {
			return fmt.Errorf("name is nil, waiting for consistency")
		}

		// For single allocations, Rule must be present; for group allocations, Rules must be present
		if !plan.Rule.IsNull() && getResp.JSON200.Rule == nil {
			return fmt.Errorf("rule is nil, waiting for consistency")
		}
		if !plan.Rules.IsNull() && getResp.JSON200.Rules == nil {
			return fmt.Errorf("rules is nil, waiting for consistency")
		}

		// Also verify Description matches what we sent
		fetchedDesc := ""
		if getResp.JSON200.Description != nil {
			fetchedDesc = *getResp.JSON200.Description
		}
		expectedDesc := plan.Description.ValueString()
		if fetchedDesc != expectedDesc {
			return fmt.Errorf("description mismatch: expected %q, got %q", expectedDesc, fetchedDesc)
		}

		log.Printf("[DEBUG] Consistency check passed: Name=%s, Desc=%s", *getResp.JSON200.Name, fetchedDesc)
		return nil
	}

	b := backoff.NewExponentialBackOff()
	b.MaxElapsedTime = 2 * time.Minute
	b.InitialInterval = 2 * time.Second

	err = backoff.Retry(consistencyOp, b)
	if err != nil {
		log.Printf("[WARN] Allocation update consistency check failed or timed out: %v", err)
	}

	// Use the last fetched response from consistency check to populate state
	// This avoids an extra API call and uses the verified consistent data
	diags = r.populateState(ctx, plan, lastFetchedResp)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, plan)...)
}

func (r *allocationResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	state := new(allocationResourceModel)
	diags := req.State.Get(ctx, state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	_, err := r.client.DeleteAllocation(ctx, state.Id.ValueString())
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Deleting DoiT Allocation",
			"Could not delete allocation, unexpected error: "+err.Error(),
		)
		return
	}

	resp.State.RemoveResource(ctx)
}
