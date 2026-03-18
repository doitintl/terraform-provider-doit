package provider

import (
	"context"
	"fmt"

	"github.com/doitintl/terraform-provider-doit/internal/provider/models"
	"github.com/doitintl/terraform-provider-doit/internal/provider/resource_datahub_dataset"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

type (
	datahubDatasetResource struct {
		client *models.ClientWithResponses
	}
	datahubDatasetResourceModel struct {
		resource_datahub_dataset.DatahubDatasetModel
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
			"Unexpected Data Source Configure Type",
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
	resp.Schema = resource_datahub_dataset.DatahubDatasetResourceSchema(ctx)
	resp.Schema.Description = "Manages a DataHub dataset."
	resp.Schema.MarkdownDescription = resp.Schema.Description

	nameAttr, ok := resp.Schema.Attributes["name"].(schema.StringAttribute)
	if ok {
		nameAttr.PlanModifiers = append(nameAttr.PlanModifiers, stringplanmodifier.RequiresReplace())
		resp.Schema.Attributes["name"] = nameAttr
	}
}

func (r *datahubDatasetResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan datahubDatasetResourceModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	apiReq := models.CreateDatahubDatasetRequestBody{
		Name: plan.Name.ValueString(),
	}
	if !plan.Description.IsNull() && !plan.Description.IsUnknown() {
		apiReq.Description = new(plan.Description.ValueString())
	}

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

	mapDatahubDatasetToModel(createResp.JSON201.Name, createResp.JSON201.Description, createResp.JSON201.Records, createResp.JSON201.UpdatedBy, createResp.JSON201.LastUpdated, &plan)

	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *datahubDatasetResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state datahubDatasetResourceModel

	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	datasetResp, err := r.client.GetDatahubDatasetWithResponse(ctx, state.Name.ValueString())
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Reading DataHub Dataset",
			"Could not read dataset "+state.Name.ValueString()+": "+err.Error(),
		)
		return
	}

	if datasetResp.StatusCode() == 404 {
		resp.State.RemoveResource(ctx)
		return
	}

	if datasetResp.StatusCode() != 200 {
		resp.Diagnostics.AddError(
			"Error Reading DataHub Dataset",
			fmt.Sprintf("Unexpected status code %d for dataset %s: %s", datasetResp.StatusCode(), state.Name.ValueString(), string(datasetResp.Body)),
		)
		return
	}

	if datasetResp.JSON200 == nil {
		resp.Diagnostics.AddError(
			"Error Reading DataHub Dataset",
			"Received empty response body for dataset "+state.Name.ValueString(),
		)
		return
	}

	mapDatahubDatasetToModel(datasetResp.JSON200.Name, datasetResp.JSON200.Description, datasetResp.JSON200.Records, datasetResp.JSON200.UpdatedBy, datasetResp.JSON200.LastUpdated, &state)

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *datahubDatasetResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan datahubDatasetResourceModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	var state datahubDatasetResourceModel

	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	apiReq := models.UpdateDatahubDatasetRequestBody{}
	if !plan.Description.IsNull() && !plan.Description.IsUnknown() {
		apiReq.Description = new(plan.Description.ValueString())
	}

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

	mapDatahubDatasetToModel(updateResp.JSON200.Name, updateResp.JSON200.Description, updateResp.JSON200.Records, updateResp.JSON200.UpdatedBy, updateResp.JSON200.LastUpdated, &plan)

	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *datahubDatasetResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state datahubDatasetResourceModel

	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

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

func mapDatahubDatasetToModel(name, description *string, records *int64, updatedBy, lastUpdated *string, state *datahubDatasetResourceModel) {
	state.Name = types.StringPointerValue(name)
	state.Description = types.StringPointerValue(description)
	state.Records = types.Int64PointerValue(records)
	state.UpdatedBy = types.StringPointerValue(updatedBy)
	state.LastUpdated = types.StringPointerValue(lastUpdated)
}
