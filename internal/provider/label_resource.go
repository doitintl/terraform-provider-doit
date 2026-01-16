package provider

import (
	"context"
	"fmt"

	"github.com/doitintl/terraform-provider-doit/internal/provider/models"
	"github.com/doitintl/terraform-provider-doit/internal/provider/resource_label"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

type (
	labelResource struct {
		client *models.ClientWithResponses
	}
	labelResourceModel struct {
		resource_label.LabelModel
	}
)

// Ensure the implementation satisfies expected interfaces.
var (
	_ resource.Resource                = &labelResource{}
	_ resource.ResourceWithConfigure   = &labelResource{}
	_ resource.ResourceWithImportState = &labelResource{}
)

// NewLabelResource creates a new label resource instance.
func NewLabelResource() resource.Resource {
	return &labelResource{}
}

// Configure adds the provider configured client to the resource.
func (r *labelResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *labelResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_label"
}

func (r *labelResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

func (r *labelResource) Schema(ctx context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = resource_label.LabelResourceSchema(ctx)
}

func (r *labelResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan labelResourceModel

	// Read Terraform plan data into the model
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Convert model to API request type
	color := models.CreateLabelRequestColor(plan.Color.ValueString())
	apiReq := models.CreateLabelRequest{
		Color: color,
		Name:  plan.Name.ValueString(),
	}

	// Create new label via API
	labelResp, err := r.client.CreateLabelWithResponse(ctx, apiReq)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Creating Label",
			"Could not create label, unexpected error: "+err.Error(),
		)
		return
	}

	if labelResp.StatusCode() != 201 {
		resp.Diagnostics.AddError(
			"Error Creating Label",
			fmt.Sprintf("Could not create label, status: %d, body: %s", labelResp.StatusCode(), string(labelResp.Body)),
		)
		return
	}

	if labelResp.JSON201 == nil {
		resp.Diagnostics.AddError(
			"Error creating label",
			"Could not create label, empty response",
		)
		return
	}

	// Map response to state
	mapLabelToModel(labelResp.JSON201, &plan)

	// Save data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *labelResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state labelResourceModel

	// Read Terraform prior state
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Get refreshed label value from API
	labelResp, err := r.client.GetLabelWithResponse(ctx, state.Id.ValueString())
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Reading Label",
			"Could not read label ID "+state.Id.ValueString()+": "+err.Error(),
		)
		return
	}

	// Handle externally deleted resource - remove from state
	if labelResp.StatusCode() == 404 {
		resp.State.RemoveResource(ctx)
		return
	}

	if labelResp.JSON200 == nil {
		resp.Diagnostics.AddError(
			"Error Reading Label",
			"Received empty response body for label ID "+state.Id.ValueString(),
		)
		return
	}

	// Map response to state
	mapLabelToModel(labelResp.JSON200, &state)

	// Save updated data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *labelResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan labelResourceModel

	// Read Terraform plan data
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Get the ID from the state
	var state labelResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	labelID := state.Id.ValueString()

	// Convert model to API request type
	color := models.UpdateLabelRequestColor(plan.Color.ValueString())
	name := plan.Name.ValueString()
	apiReq := models.UpdateLabelRequest{
		Color: &color,
		Name:  &name,
	}

	// Update label via API
	updateResp, err := r.client.UpdateLabelWithResponse(ctx, labelID, apiReq)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Updating Label",
			"Could not update label, unexpected error: "+err.Error(),
		)
		return
	}

	if updateResp.StatusCode() != 200 {
		resp.Diagnostics.AddError(
			"Error Updating Label",
			fmt.Sprintf("Could not update label, status: %d, body: %s", updateResp.StatusCode(), string(updateResp.Body)),
		)
		return
	}

	if updateResp.JSON200 == nil {
		resp.Diagnostics.AddError(
			"Error Updating Label",
			"Received empty response body",
		)
		return
	}

	// Map response to state
	mapLabelToModel(updateResp.JSON200, &plan)

	// Save updated data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *labelResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state labelResourceModel

	// Read Terraform prior state data into the model
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Delete label via API
	deleteResp, err := r.client.DeleteLabelWithResponse(ctx, state.Id.ValueString())
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Deleting DoiT Label",
			"Could not delete label, unexpected error: "+err.Error(),
		)
		return
	}

	// Treat 404 as success - resource is already gone (deleted outside Terraform)
	if deleteResp.StatusCode() != 200 && deleteResp.StatusCode() != 204 && deleteResp.StatusCode() != 404 {
		resp.Diagnostics.AddError(
			"Error Deleting DoiT Label",
			fmt.Sprintf("Could not delete label, status: %d, body: %s", deleteResp.StatusCode(), string(deleteResp.Body)),
		)
		return
	}
}

// mapLabelToModel maps the API response to the Terraform model.
func mapLabelToModel(resp *models.LabelListItem, state *labelResourceModel) {
	state.Id = types.StringValue(resp.Id)
	state.Name = types.StringValue(resp.Name)
	state.Color = types.StringValue(string(resp.Color))

	if resp.Type != nil {
		state.Type = types.StringValue(string(*resp.Type))
	} else {
		state.Type = types.StringNull()
	}

	if resp.CreateTime != nil {
		state.CreateTime = types.StringValue(resp.CreateTime.Format("2006-01-02T15:04:05Z07:00"))
	} else {
		state.CreateTime = types.StringNull()
	}

	if resp.UpdateTime != nil {
		state.UpdateTime = types.StringValue(resp.UpdateTime.Format("2006-01-02T15:04:05Z07:00"))
	} else {
		state.UpdateTime = types.StringNull()
	}
}
