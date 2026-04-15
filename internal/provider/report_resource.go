package provider

import (
	"context"
	"fmt"

	"github.com/doitintl/terraform-provider-doit/internal/provider/models"
	"github.com/doitintl/terraform-provider-doit/internal/provider/resource_report"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
)

var (
	_ resource.Resource                     = &reportResource{}
	_ resource.ResourceWithConfigure        = &reportResource{}
	_ resource.ResourceWithImportState      = &reportResource{}
	_ resource.ResourceWithConfigValidators = &reportResource{}
)

// NewReportResource creates a new report resource instance.
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
	s := resource_report.ReportResourceSchema(ctx)

	// Add UseStateForUnknown to stable Computed-only fields so they don't
	// show as "(known after apply)" on every plan that modifies the resource.
	if attr, ok := s.Attributes["id"].(schema.StringAttribute); ok {
		attr.PlanModifiers = append(attr.PlanModifiers, stringplanmodifier.UseStateForUnknown())
		s.Attributes["id"] = attr
	}
	if attr, ok := s.Attributes["type"].(schema.StringAttribute); ok {
		attr.PlanModifiers = append(attr.PlanModifiers, stringplanmodifier.UseStateForUnknown())
		s.Attributes["type"] = attr
	}

	resp.Schema = s
}
func (r *reportResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *reportResource) ConfigValidators(_ context.Context) []resource.ConfigValidator {
	return []resource.ConfigValidator{
		// Must have 1-4 metrics if specified; empty list causes state inconsistency.
		reportMetricsLengthValidator{},
		// custom_time_range.from/to must be valid RFC3339 timestamps.
		reportTimestampValidator{},
		// Warn when legacy [... N/A] NullFallback sentinels are used in filter values.
		reportFilterNAValidator{},
	}
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

	if reportResp.JSON201 == nil {
		resp.Diagnostics.AddError(
			"Error creating report",
			"Could not create report, empty response",
		)
		return
	}

	if reportResp.JSON201.Id == nil {
		resp.Diagnostics.AddError(
			"Error creating report",
			"Could not create report, response missing ID",
		)
		return
	}

	// Plan-first: preserve the user's explicit plan values, while resolving Unknown
	// fields from the API response (id, type, labels, name, description, and nested
	// config fields). This avoids API normalization drift (sentinel stripping, alias
	// renaming, etc.) for all user-configured values.
	diags = r.overlayReportComputedFields(ctx, reportResp.JSON201, &plan)
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

	// allowNotFound=true: 404 means resource was deleted externally, remove from state
	diags = r.populateStateFromAPI(ctx, state.Id.ValueString(), &state, true)
	if diags.HasError() {
		resp.Diagnostics.Append(diags...)
		return
	}

	// Handle externally deleted resource (populateStateFromAPI sets Id to null on 404)
	if state.Id.IsNull() {
		resp.State.RemoveResource(ctx)
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

	var state reportResourceModel
	diags = req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)

	reportReq, diags := plan.toUpdateRequest(ctx)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	reportResp, err := r.client.UpdateReportWithResponse(ctx, state.Id.ValueString(), reportReq)
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

	if reportResp.JSON200 == nil {
		resp.Diagnostics.AddError(
			"Error updating report",
			"Could not update report, empty response",
		)
		return
	}

	if reportResp.JSON200.Id == nil {
		resp.Diagnostics.AddError(
			"Error updating report",
			"Could not update report, response missing ID",
		)
		return
	}

	// Plan-first: preserve the user's explicit plan values, while resolving Unknown
	// fields from the API response (id, type, labels, name, description, and nested
	// config fields). This avoids API normalization drift (sentinel stripping, alias
	// renaming, etc.) for all user-configured values.
	diags = r.overlayReportComputedFields(ctx, reportResp.JSON200, &plan)
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

	// Treat 404 as success - resource is already gone (deleted outside Terraform)
	if deleteResp.StatusCode() != 200 && deleteResp.StatusCode() != 204 && deleteResp.StatusCode() != 404 {
		resp.Diagnostics.AddError(
			"Error deleting report",
			fmt.Sprintf("Could not delete report, status: %d, body: %s", deleteResp.StatusCode(), string(deleteResp.Body)),
		)
		return
	}
}
