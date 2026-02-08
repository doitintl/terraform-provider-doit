package provider

import (
	"context"
	"fmt"

	"github.com/doitintl/terraform-provider-doit/internal/provider/models"
	"github.com/doitintl/terraform-provider-doit/internal/provider/resource_alert"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// Ensure the implementation satisfies expected interfaces.
var (
	_ resource.Resource                     = &alertResource{}
	_ resource.ResourceWithConfigure        = &alertResource{}
	_ resource.ResourceWithImportState      = &alertResource{}
	_ resource.ResourceWithConfigValidators = &alertResource{}
)

type (
	alertResource struct {
		client *models.ClientWithResponses
	}
	alertResourceModel struct {
		resource_alert.AlertModel
	}
)

// NewAlertResource creates a new alert resource instance.
func NewAlertResource() resource.Resource {
	return &alertResource{}
}

// Configure adds the provider configured client to the resource.
func (r *alertResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *alertResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_alert"
}

func (r *alertResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

func (r *alertResource) Schema(ctx context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = resource_alert.AlertResourceSchema(ctx)
}

func (r *alertResource) ConfigValidators(_ context.Context) []resource.ConfigValidator {
	return []resource.ConfigValidator{
		alertRecipientsValidator{},
	}
}

func (r *alertResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan alertResourceModel

	// Read Terraform plan data into the model
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Convert model to API alert request
	alertReq, diags := plan.toAlertRequest(ctx)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Create new alert via API
	alertResp, err := r.client.CreateAlertWithResponse(ctx, alertReq)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Creating Alert",
			"Could not create alert, unexpected error: "+err.Error(),
		)
		return
	}

	if alertResp.StatusCode() != 201 {
		resp.Diagnostics.AddError(
			"Error Creating Alert",
			fmt.Sprintf("Could not create alert, status: %d, body: %s", alertResp.StatusCode(), string(alertResp.Body)),
		)
		return
	}

	if alertResp.JSON201 == nil {
		resp.Diagnostics.AddError(
			"Error Creating Alert",
			"Could not create alert, empty response",
		)
		return
	}

	// Map response directly to state (API returns complete object)
	plan.Id = types.StringPointerValue(alertResp.JSON201.Id)
	resp.Diagnostics.Append(mapAlertToModel(ctx, alertResp.JSON201, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Save data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *alertResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state alertResourceModel

	// Read Terraform prior state
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Populate state from API
	resp.Diagnostics.Append(r.populateState(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Handle externally deleted resource
	if state.Id.IsNull() {
		resp.State.RemoveResource(ctx)
		return
	}

	// Save updated data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *alertResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan alertResourceModel

	// Read Terraform plan data
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Convert model to API alert update request
	alertReq, diags := plan.toAlertUpdateRequest(ctx)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Get the ID from the state
	var state alertResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	alertID := state.Id.ValueString()

	// Update alert via API
	updateResp, err := r.client.UpdateAlertWithResponse(ctx, alertID, alertReq)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Updating Alert",
			"Could not update alert, unexpected error: "+err.Error(),
		)
		return
	}

	if updateResp.StatusCode() != 200 {
		resp.Diagnostics.AddError(
			"Error Updating Alert",
			fmt.Sprintf("Could not update alert, status: %d, body: %s", updateResp.StatusCode(), string(updateResp.Body)),
		)
		return
	}

	// Map response directly to state (API returns complete object)
	if updateResp.JSON200 == nil {
		resp.Diagnostics.AddError(
			"Error Updating Alert",
			"Received empty response body",
		)
		return
	}

	plan.Id = state.Id
	resp.Diagnostics.Append(mapAlertToModel(ctx, updateResp.JSON200, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Save updated data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *alertResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state alertResourceModel

	// Read Terraform prior state data into the model
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Delete alert via API
	deleteResp, err := r.client.DeleteAlertWithResponse(ctx, state.Id.ValueString())
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Deleting DoiT Alert",
			"Could not delete alert, unexpected error: "+err.Error(),
		)
		return
	}

	// Treat 404 as success - resource is already gone (deleted outside Terraform)
	if deleteResp.StatusCode() != 200 && deleteResp.StatusCode() != 204 && deleteResp.StatusCode() != 404 {
		resp.Diagnostics.AddError(
			"Error Deleting DoiT Alert",
			fmt.Sprintf("Could not delete alert, status: %d, body: %s", deleteResp.StatusCode(), string(deleteResp.Body)),
		)
		return
	}
}
