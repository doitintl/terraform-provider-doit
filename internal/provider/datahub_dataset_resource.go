package provider

import (
	"context"
	"fmt"
	"time"

	"github.com/doitintl/terraform-provider-doit/internal/provider/models"
	"github.com/doitintl/terraform-provider-doit/internal/provider/resource_datahub_dataset"
	"github.com/hashicorp/terraform-plugin-framework-timeouts/resource/timeouts"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
)

type (
	datahubDatasetResource struct {
		client *models.ClientWithResponses
	}
	datahubDatasetResourceModel struct {
		resource_datahub_dataset.DatahubDatasetModel
		Timeouts timeouts.Value `tfsdk:"timeouts"`
	}
)

var (
	_ resource.Resource                = (*datahubDatasetResource)(nil)
	_ resource.ResourceWithConfigure   = (*datahubDatasetResource)(nil)
	_ resource.ResourceWithImportState = (*datahubDatasetResource)(nil)
)

func NewDatahubDatasetResource() resource.Resource {
	return &datahubDatasetResource{}
}

func (r *datahubDatasetResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}

	client, ok := req.ProviderData.(*models.ClientWithResponses)
	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Resource Configure Type",
			fmt.Sprintf("Expected *models.ClientWithResponses, got: %T.", req.ProviderData),
		)
		return
	}

	r.client = client
}

func (r *datahubDatasetResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_datahub_dataset"
}

func (r *datahubDatasetResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("name"), req, resp)
}

func (r *datahubDatasetResource) Schema(ctx context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	s := resource_datahub_dataset.DatahubDatasetResourceSchema(ctx)
	s.Description = "Manages a DataHub dataset."
	s.MarkdownDescription = s.Description

	nameAttr, ok := s.Attributes["name"].(schema.StringAttribute)
	if ok {
		nameAttr.PlanModifiers = append(nameAttr.PlanModifiers, stringplanmodifier.RequiresReplace())
		s.Attributes["name"] = nameAttr
	}

	// Classify Optional+Computed attributes (clearableattr).
	// See: https://github.com/doitintl/terraform-provider-doit/issues/233
	if attr, ok := s.Attributes["description"].(schema.StringAttribute); ok {
		attr.PlanModifiers = append(attr.PlanModifiers, useEmptyForUnknownWhenConfigNull())
		s.Attributes["description"] = attr
	}

	s.Attributes["timeouts"] = timeouts.Attributes(ctx, timeouts.Opts{
		Create: true,
		Read:   true,
		Update: true,
		Delete: true,
	})

	resp.Schema = s
}

func (r *datahubDatasetResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan datahubDatasetResourceModel

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

	apiReq := plan.toCreateRequest()

	createResp, err := r.client.CreateDatahubDatasetWithResponse(ctx, apiReq)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Creating DataHub Dataset",
			"Could not create dataset, unexpected error: "+err.Error(),
		)
		return
	}

	if createResp.StatusCode() != 201 {
		resp.Diagnostics.AddError(
			"Error Creating DataHub Dataset",
			fmt.Sprintf("Could not create dataset, status: %d, body: %s", createResp.StatusCode(), string(createResp.Body)),
		)
		return
	}

	if createResp.JSON201 == nil {
		resp.Diagnostics.AddError(
			"Error Creating DataHub Dataset",
			"Could not create dataset, empty response",
		)
		return
	}

	overlayDatahubDatasetComputedFields(createResp.JSON201.Name, createResp.JSON201.Description, nullableToPointer(createResp.JSON201.Records), createResp.JSON201.UpdatedBy, createResp.JSON201.LastUpdated, &plan)

	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *datahubDatasetResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state datahubDatasetResourceModel

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

	diags = r.populateState(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Handle externally deleted resource (populateState sets Name to null on 404)
	if state.Name.IsNull() {
		resp.State.RemoveResource(ctx)
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *datahubDatasetResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan datahubDatasetResourceModel

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

	var state datahubDatasetResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	apiReq := plan.toUpdateRequest()

	updateResp, err := r.client.UpdateDatahubDatasetWithResponse(ctx, state.Name.ValueString(), apiReq)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Updating DataHub Dataset",
			"Could not update dataset, unexpected error: "+err.Error(),
		)
		return
	}

	if updateResp.StatusCode() != 200 {
		resp.Diagnostics.AddError(
			"Error Updating DataHub Dataset",
			fmt.Sprintf("Could not update dataset, status: %d, body: %s", updateResp.StatusCode(), string(updateResp.Body)),
		)
		return
	}

	if updateResp.JSON200 == nil {
		resp.Diagnostics.AddError(
			"Error Updating DataHub Dataset",
			"Received empty response body",
		)
		return
	}

	overlayDatahubDatasetComputedFields(updateResp.JSON200.Name, updateResp.JSON200.Description, nullableToPointer(updateResp.JSON200.Records), updateResp.JSON200.UpdatedBy, updateResp.JSON200.LastUpdated, &plan)

	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *datahubDatasetResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state datahubDatasetResourceModel

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

	deleteResp, err := r.client.DeleteDatahubDatasetWithResponse(ctx, state.Name.ValueString())
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Deleting DataHub Dataset",
			"Could not delete dataset, unexpected error: "+err.Error(),
		)
		return
	}

	if deleteResp.StatusCode() != 200 && deleteResp.StatusCode() != 204 && deleteResp.StatusCode() != 404 {
		resp.Diagnostics.AddError(
			"Error Deleting DataHub Dataset",
			fmt.Sprintf("Could not delete dataset, status: %d, body: %s", deleteResp.StatusCode(), string(deleteResp.Body)),
		)
		return
	}
}
