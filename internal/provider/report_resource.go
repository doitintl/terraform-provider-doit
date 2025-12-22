package provider

import (
	"context"
	"fmt"

	"terraform-provider-doit/internal/provider/models"
	"terraform-provider-doit/internal/provider/resource_report"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var (
	_ resource.Resource                     = &reportResource{}
	_ resource.ResourceWithConfigure        = &reportResource{}
	_ resource.ResourceWithImportState      = &reportResource{}
	_ resource.ResourceWithConfigValidators = &reportResource{}
)

func NewReportResource() resource.Resource {
	return &reportResource{}
}

type reportResource struct {
	client *models.ClientWithResponses
}

type reportResourceModel struct {
	resource_report.ReportModel
}

func (r *reportResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_report"
}

func (r *reportResource) Schema(ctx context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = resource_report.ReportResourceSchema(ctx)
}

func (r *reportResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *reportResource) ConfigValidators(ctx context.Context) []resource.ConfigValidator {
	return []resource.ConfigValidator{}
}

func (r *reportResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

func (r *reportResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan reportResourceModel

	diags := req.Plan.Get(ctx, &plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	reportReq, diags := plan.toCreateRequest(ctx)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	reportResp, err := r.client.CreateReportWithResponse(ctx, reportReq)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error creating report",
			"Could not create report, unexpected error: "+err.Error(),
		)
		return
	}

	if reportResp.StatusCode() != 201 {
		resp.Diagnostics.AddError(
			"Error creating report",
			fmt.Sprintf("Could not create report, status: %d, body: %s", reportResp.StatusCode(), string(reportResp.Body)),
		)
		return
	}

	if reportResp.JSON201 == nil || reportResp.JSON201.Id == nil {
		resp.Diagnostics.AddError(
			"Error creating report",
			"Could not create report, empty response or missing ID",
		)
		return
	}

	plan.Id = types.StringPointerValue(reportResp.JSON201.Id)

	diags = r.populateStateFromAPI(ctx, plan.Id.ValueString(), &plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, plan)...)
}

func (r *reportResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state reportResourceModel
	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	diags = r.populateStateFromAPI(ctx, state.Id.ValueString(), &state)
	if diags.HasError() {
		// Check if it's a 404 error based on the logic in populateStateFromAPI
		for _, d := range diags {
			if d.Summary() == "Report not found" {
				resp.State.RemoveResource(ctx)
				return
			}
		}
		resp.Diagnostics.Append(diags...)
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, state)...)
}

func (r *reportResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan reportResourceModel
	diags := req.Plan.Get(ctx, &plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	reportReq, diags := plan.toUpdateRequest(ctx)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	reportResp, err := r.client.UpdateReportWithResponse(ctx, plan.Id.ValueString(), reportReq)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error updating report",
			"Could not update report, unexpected error: "+err.Error(),
		)
		return
	}

	if reportResp.StatusCode() != 200 {
		resp.Diagnostics.AddError(
			"Error updating report",
			fmt.Sprintf("Could not update report, status: %d, body: %s", reportResp.StatusCode(), string(reportResp.Body)),
		)
		return
	}

	diags = r.populateStateFromAPI(ctx, plan.Id.ValueString(), &plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, plan)...)
}

func (r *reportResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state reportResourceModel
	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	deleteResp, err := r.client.DeleteReportWithResponse(ctx, state.Id.ValueString())
	if err != nil {
		resp.Diagnostics.AddError(
			"Error deleting report",
			"Could not delete report, unexpected error: "+err.Error(),
		)
		return
	}

	if deleteResp.StatusCode() != 200 && deleteResp.StatusCode() != 404 {
		resp.Diagnostics.AddError(
			"Error deleting report",
			fmt.Sprintf("Could not delete report, status: %d, body: %s", deleteResp.StatusCode(), string(deleteResp.Body)),
		)
		return
	}

	resp.State.RemoveResource(ctx)
}
