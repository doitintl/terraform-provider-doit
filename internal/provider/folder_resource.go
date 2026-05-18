package provider

import (
	"context"
	"fmt"
	"time"

	"github.com/doitintl/terraform-provider-doit/internal/provider/models"
	"github.com/doitintl/terraform-provider-doit/internal/provider/resource_folder"
	"github.com/hashicorp/terraform-plugin-framework-timeouts/resource/timeouts"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
)

type (
	folderResource struct {
		client *models.ClientWithResponses
	}
	folderResourceModel struct {
		resource_folder.FolderModel
		Timeouts timeouts.Value `tfsdk:"timeouts"`
	}
)

// Ensure the implementation satisfies expected interfaces.
var (
	_ resource.Resource                = (*folderResource)(nil)
	_ resource.ResourceWithConfigure   = (*folderResource)(nil)
	_ resource.ResourceWithImportState = (*folderResource)(nil)
)

// NewFolderResource creates a new folder resource instance.
func NewFolderResource() resource.Resource {
	return &folderResource{}
}

// Configure adds the provider configured client to the resource.
func (r *folderResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *folderResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_folder"
}

func (r *folderResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

func (r *folderResource) Schema(ctx context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	s := resource_folder.FolderResourceSchema(ctx)

	// Add UseStateForUnknown to the Computed-only id field so it doesn't
	// show as "(known after apply)" on every plan that modifies the resource.
	if attr, ok := s.Attributes["id"].(schema.StringAttribute); ok {
		attr.PlanModifiers = append(attr.PlanModifiers, stringplanmodifier.UseStateForUnknown())
		s.Attributes["id"] = attr
	}

	// Reject empty strings for parent_folder_id — the API normalizes "" to
	// "root", which would cause perpetual plan drift. Users who want the
	// default (root) should omit the attribute instead.
	if attr, ok := s.Attributes["parent_folder_id"].(schema.StringAttribute); ok {
		attr.Validators = append(attr.Validators, stringvalidator.LengthAtLeast(1))
		s.Attributes["parent_folder_id"] = attr
	}

	s.Attributes["timeouts"] = timeouts.Attributes(ctx, timeouts.Opts{
		Create: true,
		Read:   true,
		Update: true,
		Delete: true,
	})

	resp.Schema = s
}

func (r *folderResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan folderResourceModel

	// Read Terraform plan data into the model
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	createTimeout, diags := plan.Timeouts.Create(ctx, 5*time.Minute)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	ctx, cancel := context.WithTimeout(ctx, createTimeout)
	defer cancel()

	// Build API request. The API defaults to the root level when
	// parent_folder_id is omitted; we send "root" explicitly for clarity.
	apiReq := models.CreateFolderRequest{
		Name: plan.Name.ValueString(),
	}

	if !plan.ParentFolderId.IsNull() && !plan.ParentFolderId.IsUnknown() {
		apiReq.ParentFolderId = new(plan.ParentFolderId.ValueString())
	} else {
		apiReq.ParentFolderId = new("root")
	}

	if !plan.Description.IsNull() && !plan.Description.IsUnknown() {
		apiReq.Description = new(plan.Description.ValueString())
	}

	// Create new folder via API
	folderResp, err := r.client.CreateFolderWithResponse(ctx, apiReq)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Creating Folder",
			"Could not create folder, unexpected error: "+err.Error(),
		)
		return
	}

	if folderResp.StatusCode() != 201 {
		resp.Diagnostics.AddError(
			"Error Creating Folder",
			fmt.Sprintf("Could not create folder, status: %d, body: %s", folderResp.StatusCode(), string(folderResp.Body)),
		)
		return
	}

	if folderResp.JSON201 == nil {
		resp.Diagnostics.AddError(
			"Error Creating Folder",
			"Could not create folder, empty response",
		)
		return
	}

	// Plan-first state pattern: overlay Computed-only fields from API response.
	overlayFolderComputedFields(folderResp.JSON201, &plan)

	// Save data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *folderResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state folderResourceModel

	// Read Terraform prior state
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	readTimeout, diags := state.Timeouts.Read(ctx, 2*time.Minute)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	ctx, cancel := context.WithTimeout(ctx, readTimeout)
	defer cancel()

	// Get refreshed folder from API
	folderResp, err := r.client.GetFolderWithResponse(ctx, state.Id.ValueString())
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Reading Folder",
			"Could not read folder ID "+state.Id.ValueString()+": "+err.Error(),
		)
		return
	}

	// Handle externally deleted resource - remove from state
	if folderResp.StatusCode() == 404 {
		resp.State.RemoveResource(ctx)
		return
	}

	// Check for successful response
	if folderResp.StatusCode() != 200 {
		resp.Diagnostics.AddError(
			"Error Reading Folder",
			fmt.Sprintf("Unexpected status code %d for folder ID %s: %s", folderResp.StatusCode(), state.Id.ValueString(), string(folderResp.Body)),
		)
		return
	}

	if folderResp.JSON200 == nil {
		resp.Diagnostics.AddError(
			"Error Reading Folder",
			"Received empty response body for folder ID "+state.Id.ValueString(),
		)
		return
	}

	// Map response to state (full mapping for Read path)
	mapFolderToModel(folderResp.JSON200, &state)

	// Save updated data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *folderResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan folderResourceModel

	// Read Terraform plan data
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	updateTimeout, diags := plan.Timeouts.Update(ctx, 5*time.Minute)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	ctx, cancel := context.WithTimeout(ctx, updateTimeout)
	defer cancel()

	// Get the ID from the state
	var state folderResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	folderID := state.Id.ValueString()

	// Build API request. Default parent_folder_id to "root" when unset.
	apiReq := models.UpdateFolderRequest{
		Name: new(plan.Name.ValueString()),
	}

	if !plan.ParentFolderId.IsNull() && !plan.ParentFolderId.IsUnknown() {
		apiReq.ParentFolderId = new(plan.ParentFolderId.ValueString())
	} else {
		apiReq.ParentFolderId = new("root")
	}

	if !plan.Description.IsNull() && !plan.Description.IsUnknown() {
		apiReq.Description = new(plan.Description.ValueString())
	}

	// Update folder via API
	updateResp, err := r.client.UpdateFolderWithResponse(ctx, folderID, apiReq)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Updating Folder",
			"Could not update folder, unexpected error: "+err.Error(),
		)
		return
	}

	if updateResp.StatusCode() != 200 {
		resp.Diagnostics.AddError(
			"Error Updating Folder",
			fmt.Sprintf("Could not update folder, status: %d, body: %s", updateResp.StatusCode(), string(updateResp.Body)),
		)
		return
	}

	if updateResp.JSON200 == nil {
		resp.Diagnostics.AddError(
			"Error Updating Folder",
			"Received empty response body",
		)
		return
	}

	// Plan-first state pattern: overlay Computed-only fields from API response.
	overlayFolderComputedFields(updateResp.JSON200, &plan)

	// Save updated data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *folderResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state folderResourceModel

	// Read Terraform prior state data into the model
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	deleteTimeout, diags := state.Timeouts.Delete(ctx, 2*time.Minute)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	ctx, cancel := context.WithTimeout(ctx, deleteTimeout)
	defer cancel()

	// Delete folder via API
	deleteResp, err := r.client.DeleteFolderWithResponse(ctx, state.Id.ValueString())
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Deleting Folder",
			"Could not delete folder, unexpected error: "+err.Error(),
		)
		return
	}

	// Treat 404 as success - resource is already gone (deleted outside Terraform)
	if deleteResp.StatusCode() != 200 && deleteResp.StatusCode() != 204 && deleteResp.StatusCode() != 404 {
		resp.Diagnostics.AddError(
			"Error Deleting Folder",
			fmt.Sprintf("Could not delete folder, status: %d, body: %s", deleteResp.StatusCode(), string(deleteResp.Body)),
		)
		return
	}
}
