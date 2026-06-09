package provider

import (
	"context"
	"fmt"
	"time"

	"github.com/doitintl/terraform-provider-doit/internal/provider/models"
	"github.com/doitintl/terraform-provider-doit/internal/provider/resource_annotation"
	"github.com/hashicorp/terraform-plugin-framework-timeouts/resource/timeouts"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
)

type (
	annotationResource struct {
		client *models.ClientWithResponses
	}
	annotationResourceModel struct {
		resource_annotation.AnnotationModel
		Timeouts timeouts.Value `tfsdk:"timeouts"`
	}
)

// Ensure the implementation satisfies expected interfaces.
var (
	_ resource.Resource                = (*annotationResource)(nil)
	_ resource.ResourceWithConfigure   = (*annotationResource)(nil)
	_ resource.ResourceWithImportState = (*annotationResource)(nil)
)

// NewAnnotationResource creates a new annotation resource instance.
func NewAnnotationResource() resource.Resource {
	return &annotationResource{}
}

// Configure adds the provider configured client to the resource.
func (r *annotationResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}

	client, ok := req.ProviderData.(*models.ClientWithResponses)
	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Resource Configure Type",
			fmt.Sprintf("Expected *models.ClientWithResponses, got: %T. Please report this issue to the provider developers.", req.ProviderData),
		)
		return
	}

	r.client = client
}

func (r *annotationResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_annotation"
}

func (r *annotationResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

func (r *annotationResource) Schema(ctx context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	s := resource_annotation.AnnotationResourceSchema(ctx)

	// Add RFC3339 validator for timestamp
	if timestamp, ok := s.Attributes["timestamp"]; ok {
		if strAttr, ok := timestamp.(schema.StringAttribute); ok {
			strAttr.Validators = append(strAttr.Validators, rfc3339Validator{})
			s.Attributes["timestamp"] = strAttr
		}
	}

	// Add UseStateForUnknown to stable Computed-only fields so they don't
	// show as "(known after apply)" on every plan that modifies the resource.
	if attr, ok := s.Attributes["id"].(schema.StringAttribute); ok {
		attr.PlanModifiers = append(attr.PlanModifiers, stringplanmodifier.UseStateForUnknown())
		s.Attributes["id"] = attr
	}
	if attr, ok := s.Attributes["create_time"].(schema.StringAttribute); ok {
		attr.PlanModifiers = append(attr.PlanModifiers, stringplanmodifier.UseStateForUnknown())
		s.Attributes["create_time"] = attr
	}

	// Classify Optional+Computed attributes (clearableattr).
	// See: https://github.com/doitintl/terraform-provider-doit/issues/233
	// Category B: labels and reports can be managed externally via
	// doit_label_assignments, so a clearing modifier would conflict with
	// externally-assigned values. Users can still clear by setting labels = [].
	acknowledgeNotClearable(s,
		"labels",  // managed externally via doit_label_assignments
		"reports", // managed externally via doit_label_assignments
	)

	s.Attributes["timeouts"] = timeouts.Attributes(ctx, timeouts.Opts{
		Create: true,
		Read:   true,
		Update: true,
		Delete: true,
	})

	resp.Schema = s
}

func (r *annotationResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan annotationResourceModel

	// Read Terraform plan data into the model
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	createTimeout, createDiags := plan.Timeouts.Create(ctx, 5*time.Minute)
	resp.Diagnostics.Append(createDiags...)
	if resp.Diagnostics.HasError() {
		return
	}
	ctx, cancel := context.WithTimeout(ctx, createTimeout)
	defer cancel()

	// Build API request
	apiReq, buildDiags := plan.toCreateRequest(ctx)
	resp.Diagnostics.Append(buildDiags...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Create new annotation via API
	annotationResp, err := r.client.CreateAnnotationWithResponse(ctx, apiReq)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Creating Annotation",
			"Could not create annotation, unexpected error: "+err.Error(),
		)
		return
	}

	if annotationResp.StatusCode() != 201 {
		resp.Diagnostics.AddError(
			"Error Creating Annotation",
			fmt.Sprintf("Could not create annotation, status: %d, body: %s", annotationResp.StatusCode(), string(annotationResp.Body)),
		)
		return
	}

	if annotationResp.JSON201 == nil {
		resp.Diagnostics.AddError(
			"Error creating annotation",
			"Could not create annotation, empty response",
		)
		return
	}

	// Plan-first state pattern: keep user-configured values from the plan
	// as-is, and overlay API response values for Computed fields plus any
	// Optional+Computed fields (labels, reports) that are still unknown.
	resp.Diagnostics.Append(overlayAnnotationComputedFields(ctx, annotationResp.JSON201, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Save data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *annotationResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state annotationResourceModel

	// Read Terraform prior state
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	readTimeout, readDiags := state.Timeouts.Read(ctx, 2*time.Minute)
	resp.Diagnostics.Append(readDiags...)
	if resp.Diagnostics.HasError() {
		return
	}
	ctx, cancel := context.WithTimeout(ctx, readTimeout)
	defer cancel()

	diags := r.populateState(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Handle externally deleted resource (populateState sets Id to null on 404)
	if state.Id.IsNull() {
		resp.State.RemoveResource(ctx)
		return
	}

	// Save updated data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *annotationResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan annotationResourceModel

	// Read Terraform plan data
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	updateTimeout, updateDiags := plan.Timeouts.Update(ctx, 5*time.Minute)
	resp.Diagnostics.Append(updateDiags...)
	if resp.Diagnostics.HasError() {
		return
	}
	ctx, cancel := context.WithTimeout(ctx, updateTimeout)
	defer cancel()

	// Get the ID from the state
	var state annotationResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	annotationID := state.Id.ValueString()

	// Build API request
	apiReq, buildDiags := plan.toUpdateRequest(ctx)
	resp.Diagnostics.Append(buildDiags...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Update annotation via API
	updateResp, err := r.client.UpdateAnnotationWithResponse(ctx, annotationID, apiReq)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Updating Annotation",
			"Could not update annotation, unexpected error: "+err.Error(),
		)
		return
	}

	if updateResp.StatusCode() != 200 {
		resp.Diagnostics.AddError(
			"Error Updating Annotation",
			fmt.Sprintf("Could not update annotation, status: %d, body: %s", updateResp.StatusCode(), string(updateResp.Body)),
		)
		return
	}

	if updateResp.JSON200 == nil {
		resp.Diagnostics.AddError(
			"Error Updating Annotation",
			"Received empty response body",
		)
		return
	}

	// Plan-first state pattern: keep user-configured values from the plan
	// as-is, and overlay API response values for Computed fields plus any
	// Optional+Computed fields (labels, reports) that are still unknown.
	resp.Diagnostics.Append(overlayAnnotationComputedFields(ctx, updateResp.JSON200, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Save updated data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *annotationResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state annotationResourceModel

	// Read Terraform prior state data into the model
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	deleteTimeout, deleteDiags := state.Timeouts.Delete(ctx, 2*time.Minute)
	resp.Diagnostics.Append(deleteDiags...)
	if resp.Diagnostics.HasError() {
		return
	}
	ctx, cancel := context.WithTimeout(ctx, deleteTimeout)
	defer cancel()

	// Delete annotation via API
	deleteResp, err := r.client.DeleteAnnotationWithResponse(ctx, state.Id.ValueString())
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Deleting DoiT Annotation",
			"Could not delete annotation, unexpected error: "+err.Error(),
		)
		return
	}

	// Treat 404 as success - resource is already gone (deleted outside Terraform)
	if deleteResp.StatusCode() != 200 && deleteResp.StatusCode() != 204 && deleteResp.StatusCode() != 404 {
		resp.Diagnostics.AddError(
			"Error Deleting DoiT Annotation",
			fmt.Sprintf("Could not delete annotation, status: %d, body: %s", deleteResp.StatusCode(), string(deleteResp.Body)),
		)
		return
	}
}
