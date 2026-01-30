package provider

import (
	"context"
	"fmt"
	"time"

	"github.com/doitintl/terraform-provider-doit/internal/provider/models"
	"github.com/doitintl/terraform-provider-doit/internal/provider/resource_annotation"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

type (
	annotationResource struct {
		client *models.ClientWithResponses
	}
	annotationResourceModel struct {
		resource_annotation.AnnotationModel
	}
)

// Ensure the implementation satisfies expected interfaces.
var (
	_ resource.Resource                = &annotationResource{}
	_ resource.ResourceWithConfigure   = &annotationResource{}
	_ resource.ResourceWithImportState = &annotationResource{}
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
			"Unexpected Data Source Configure Type",
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

	resp.Schema = s
}

func (r *annotationResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan annotationResourceModel

	// Read Terraform plan data into the model
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Parse timestamp
	timestamp, err := time.Parse(time.RFC3339, plan.Timestamp.ValueString())
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Parsing Timestamp",
			fmt.Sprintf("Could not parse timestamp '%s': %s", plan.Timestamp.ValueString(), err.Error()),
		)
		return
	}

	// Convert model to API request type
	apiReq := models.CreateAnnotationRequest{
		Content:   plan.Content.ValueString(),
		Timestamp: timestamp,
	}

	// Handle optional labels list
	if !plan.Labels.IsNull() && !plan.Labels.IsUnknown() {
		var labels []string
		diags := plan.Labels.ElementsAs(ctx, &labels, false)
		resp.Diagnostics.Append(diags...)
		if resp.Diagnostics.HasError() {
			return
		}
		apiReq.Labels = &labels
	}

	// Handle optional reports list
	if !plan.Reports.IsNull() && !plan.Reports.IsUnknown() {
		var reports []string
		diags := plan.Reports.ElementsAs(ctx, &reports, false)
		resp.Diagnostics.Append(diags...)
		if resp.Diagnostics.HasError() {
			return
		}
		apiReq.Reports = &reports
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

	// Map response to state
	diags := mapAnnotationToModel(ctx, annotationResp.JSON201, &plan)
	resp.Diagnostics.Append(diags...)
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

	// Get refreshed annotation value from API
	annotationResp, err := r.client.GetAnnotationWithResponse(ctx, state.Id.ValueString())
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Reading Annotation",
			"Could not read annotation ID "+state.Id.ValueString()+": "+err.Error(),
		)
		return
	}

	// Handle externally deleted resource - remove from state
	if annotationResp.StatusCode() == 404 {
		resp.State.RemoveResource(ctx)
		return
	}

	// Check for successful response
	if annotationResp.StatusCode() != 200 {
		resp.Diagnostics.AddError(
			"Error Reading Annotation",
			fmt.Sprintf("Unexpected status code %d for annotation ID %s: %s", annotationResp.StatusCode(), state.Id.ValueString(), string(annotationResp.Body)),
		)
		return
	}

	if annotationResp.JSON200 == nil {
		resp.Diagnostics.AddError(
			"Error Reading Annotation",
			"Received empty response body for annotation ID "+state.Id.ValueString(),
		)
		return
	}

	// Map response to state
	diags := mapAnnotationToModel(ctx, annotationResp.JSON200, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
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

	// Get the ID from the state
	var state annotationResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	annotationID := state.Id.ValueString()

	// Parse timestamp
	timestamp, err := time.Parse(time.RFC3339, plan.Timestamp.ValueString())
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Parsing Timestamp",
			fmt.Sprintf("Could not parse timestamp '%s': %s", plan.Timestamp.ValueString(), err.Error()),
		)
		return
	}

	// Convert model to API request type
	content := plan.Content.ValueString()
	apiReq := models.UpdateAnnotationRequest{
		Content:   &content,
		Timestamp: &timestamp,
	}

	// Handle optional labels list
	if !plan.Labels.IsNull() && !plan.Labels.IsUnknown() {
		var labels []string
		diags := plan.Labels.ElementsAs(ctx, &labels, false)
		resp.Diagnostics.Append(diags...)
		if resp.Diagnostics.HasError() {
			return
		}
		apiReq.Labels = &labels
	}

	// Handle optional reports list
	if !plan.Reports.IsNull() && !plan.Reports.IsUnknown() {
		var reports []string
		diags := plan.Reports.ElementsAs(ctx, &reports, false)
		resp.Diagnostics.Append(diags...)
		if resp.Diagnostics.HasError() {
			return
		}
		apiReq.Reports = &reports
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

	// Map response to state
	diags := mapAnnotationToModel(ctx, updateResp.JSON200, &plan)
	resp.Diagnostics.Append(diags...)
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

// mapAnnotationToModel maps the API response to the Terraform model.
func mapAnnotationToModel(ctx context.Context, resp *models.AnnotationListItem, state *annotationResourceModel) (diags diag.Diagnostics) {
	state.Id = types.StringValue(resp.Id)
	state.Content = types.StringValue(resp.Content)

	// Check if the timestamp has changed semantically before overwriting to preserve user formatting
	existingTime, err := time.Parse(time.RFC3339, state.Timestamp.ValueString())
	if err == nil && existingTime.Equal(resp.Timestamp) {
		// Keep the existing string to avoid diffs if the time is the same
	} else {
		state.Timestamp = types.StringValue(resp.Timestamp.UTC().Format(time.RFC3339))
	}

	if resp.CreateTime != nil {
		state.CreateTime = types.StringValue(resp.CreateTime.Format(time.RFC3339))
	} else {
		state.CreateTime = types.StringNull()
	}

	if resp.UpdateTime != nil {
		state.UpdateTime = types.StringValue(resp.UpdateTime.Format(time.RFC3339))
	} else {
		state.UpdateTime = types.StringNull()
	}

	// Map labels - API returns []LabelInfo, but we store just the IDs
	// Use resp.Labels != nil (not len check) to handle explicit empty lists correctly
	if resp.Labels != nil {
		labelIDs := make([]string, len(*resp.Labels))
		for i, label := range *resp.Labels {
			labelIDs[i] = label.Id
		}
		labelsList, d := types.ListValueFrom(ctx, types.StringType, labelIDs)
		diags.Append(d...)
		if diags.HasError() {
			return diags
		}
		state.Labels = labelsList
	} else {
		state.Labels = types.ListNull(types.StringType)
	}

	// Map reports
	// Use resp.Reports != nil (not len check) to handle explicit empty lists correctly
	if resp.Reports != nil {
		reportsList, d := types.ListValueFrom(ctx, types.StringType, *resp.Reports)
		diags.Append(d...)
		if diags.HasError() {
			return diags
		}
		state.Reports = reportsList
	} else {
		state.Reports = types.ListNull(types.StringType)
	}

	return diags
}
