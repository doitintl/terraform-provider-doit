package provider

import (
	"context"
	"fmt"

	"github.com/doitintl/terraform-provider-doit/internal/provider/models"

	"github.com/doitintl/terraform-provider-doit/internal/provider/resource_allocation"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/hashicorp/terraform-plugin-framework-validators/resourcevalidator"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
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

// NewAllocationResource creates a new allocation resource instance.
func NewAllocationResource() resource.Resource {
	return &allocationResource{}
}

// Configure adds the provider configured client to the resource.
func (r *allocationResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}

	client, ok := req.ProviderData.(*models.ClientWithResponses)
	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Data Source Configure Type",
			fmt.Sprintf("Expected *models.ClientWithResponses, got: %T. Please report this issue to the provider developers.", req.ProviderData),
		)
		return
	}

	r.client = client
}

func (r *allocationResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

func (r *allocationResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_allocation"
}

func (r *allocationResource) Schema(ctx context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	s := resource_allocation.AllocationResourceSchema(ctx)

	// Inject validator for rules attribute to enforce 'name' is required for 'create'/'update' actions.
	// See allocationRulesValidator for context on why this workaround is needed.
	if rules, ok := s.Attributes["rules"]; ok {
		if listAttr, ok := rules.(schema.ListNestedAttribute); ok {
			listAttr.Validators = append(listAttr.Validators, allocationRulesValidator{})

			// Also inject components validator into rules[].components
			if components, ok := listAttr.NestedObject.Attributes["components"]; ok {
				if compListAttr, ok := components.(schema.ListNestedAttribute); ok {
					compListAttr.Validators = append(compListAttr.Validators, allocationComponentsValidator{})
					listAttr.NestedObject.Attributes["components"] = compListAttr
				}
			}

			s.Attributes["rules"] = listAttr
		}
	}

	// Inject components validator into rule.components
	if rule, ok := s.Attributes["rule"]; ok {
		if singleAttr, ok := rule.(schema.SingleNestedAttribute); ok {
			if components, ok := singleAttr.Attributes["components"]; ok {
				if compListAttr, ok := components.(schema.ListNestedAttribute); ok {
					compListAttr.Validators = append(compListAttr.Validators, allocationComponentsValidator{})
					singleAttr.Attributes["components"] = compListAttr
				}
			}
			s.Attributes["rule"] = singleAttr
		}
	}

	resp.Schema = s
}

func (r *allocationResource) ConfigValidators(_ context.Context) []resource.ConfigValidator {
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

	if allocationResp.StatusCode() != 200 {
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

	if allocationResp.JSON200.Id == nil {
		resp.Diagnostics.AddError(
			"Error creating allocation",
			"Could not create allocation, response missing ID",
		)
		return
	}

	// Plan-first state pattern: keep all user-configured values from the plan
	// exactly as-is, and only overlay Computed-only fields from the API response.
	// This prevents "Provider produced inconsistent result" errors caused by the
	// API normalizing user-provided values (stripping sentinels, renaming services).
	// Read and ImportState still use mapAllocationToModel for the full API response.
	resp.Diagnostics.Append(overlayComputedFields(allocationResp.JSON200, plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, plan)...)
}

func (r *allocationResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	state := new(allocationResourceModel)
	diags := req.State.Get(ctx, state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	// allowNotFound=true: 404 means resource was deleted externally, remove from state
	diags = r.populateState(ctx, state, true)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Handle externally deleted resource (populateState sets Id to null on 404)
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

	// We need the allocation ID from state for the API call.
	var stateId types.String
	diags = req.State.GetAttribute(ctx, path.Root("id"), &stateId)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Update the allocation
	updateResp, err := r.client.UpdateAllocationWithResponse(ctx, stateId.ValueString(), allocation)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error updating allocation",
			"Could not update allocation, unexpected error: "+err.Error(),
		)
		return
	}

	if updateResp.StatusCode() != 200 {
		resp.Diagnostics.AddError(
			"Error updating allocation",
			fmt.Sprintf("Could not update allocation, status: %d, body: %s", updateResp.StatusCode(), string(updateResp.Body)),
		)
		return
	}

	if updateResp.JSON200 == nil {
		resp.Diagnostics.AddError(
			"Error updating allocation",
			"Could not update allocation, empty response",
		)
		return
	}

	if updateResp.JSON200.Id == nil {
		resp.Diagnostics.AddError(
			"Error updating allocation",
			"Could not update allocation, response missing ID",
		)
		return
	}

	// Plan-first state pattern: keep all user-configured values from the plan
	// exactly as-is, and only overlay Computed-only fields from the API response.
	resp.Diagnostics.Append(overlayComputedFields(updateResp.JSON200, plan)...)
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

	deleteResp, err := r.client.DeleteAllocationWithResponse(ctx, state.Id.ValueString())
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Deleting DoiT Allocation",
			"Could not delete allocation, unexpected error: "+err.Error(),
		)
		return
	}

	// Treat 404 as success - resource is already gone (deleted outside Terraform)
	if deleteResp.StatusCode() != 200 && deleteResp.StatusCode() != 204 && deleteResp.StatusCode() != 404 {
		resp.Diagnostics.AddError(
			"Error Deleting DoiT Allocation",
			fmt.Sprintf("Could not delete allocation, status: %d, body: %s", deleteResp.StatusCode(), string(deleteResp.Body)),
		)
		return
	}
}
