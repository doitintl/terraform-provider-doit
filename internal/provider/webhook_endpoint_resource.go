package provider

import (
	"context"
	"fmt"
	"time"

	"github.com/doitintl/terraform-provider-doit/internal/provider/models"
	"github.com/doitintl/terraform-provider-doit/internal/provider/resource_webhook_endpoint"
	"github.com/hashicorp/terraform-plugin-framework-timeouts/resource/timeouts"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
)

type (
	webhookEndpointResource struct {
		client *models.ClientWithResponses
	}
	webhookEndpointResourceModel struct {
		resource_webhook_endpoint.WebhookEndpointModel
		Timeouts timeouts.Value `tfsdk:"timeouts"`
	}
)

var (
	_ resource.Resource                = (*webhookEndpointResource)(nil)
	_ resource.ResourceWithConfigure   = (*webhookEndpointResource)(nil)
	_ resource.ResourceWithImportState = (*webhookEndpointResource)(nil)
)

func NewWebhookEndpointResource() resource.Resource {
	return &webhookEndpointResource{}
}

func (r *webhookEndpointResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *webhookEndpointResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_webhook_endpoint"
}

func (r *webhookEndpointResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

func (r *webhookEndpointResource) Schema(ctx context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	s := resource_webhook_endpoint.WebhookEndpointResourceSchema(ctx)

	// UseStateForUnknown for stable Computed-only fields
	if attr, ok := s.Attributes["id"].(schema.StringAttribute); ok {
		attr.PlanModifiers = append(attr.PlanModifiers, stringplanmodifier.UseStateForUnknown())
		s.Attributes["id"] = attr
	}
	if attr, ok := s.Attributes["create_time"].(schema.StringAttribute); ok {
		attr.PlanModifiers = append(attr.PlanModifiers, stringplanmodifier.UseStateForUnknown())
		s.Attributes["create_time"] = attr
	}

	// Category A (clearable): description — user-authored content
	if attr, ok := s.Attributes["description"].(schema.StringAttribute); ok {
		attr.PlanModifiers = append(attr.PlanModifiers, useEmptyForUnknownWhenConfigNull())
		s.Attributes["description"] = attr
	}

	// Category A (clearable): events — user can reset to "all events" by omitting
	if attr, ok := s.Attributes["events"].(schema.ListAttribute); ok {
		attr.PlanModifiers = append(attr.PlanModifiers, useNullForUnknownListWhenConfigNull())
		s.Attributes["events"] = attr
	}

	s.Attributes["timeouts"] = timeouts.Attributes(ctx, timeouts.Opts{
		Create: true,
		Read:   true,
		Update: true,
		Delete: true,
	})

	resp.Schema = s
}

func (r *webhookEndpointResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan webhookEndpointResourceModel

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

	apiReq, buildDiags := plan.toCreateRequest(ctx)
	resp.Diagnostics.Append(buildDiags...)
	if resp.Diagnostics.HasError() {
		return
	}

	createResp, err := r.client.CreateWebhookEndpointWithResponse(ctx, apiReq)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Creating Webhook Endpoint",
			"Could not create webhook endpoint, unexpected error: "+err.Error(),
		)
		return
	}

	if createResp.StatusCode() != 201 {
		resp.Diagnostics.AddError(
			"Error Creating Webhook Endpoint",
			fmt.Sprintf("Could not create webhook endpoint, status: %d, body: %s", createResp.StatusCode(), string(createResp.Body)),
		)
		return
	}

	if createResp.JSON201 == nil {
		resp.Diagnostics.AddError(
			"Error Creating Webhook Endpoint",
			"Could not create webhook endpoint, empty response",
		)
		return
	}

	resp.Diagnostics.Append(overlayWebhookEndpointComputedFields(ctx, createResp.JSON201, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *webhookEndpointResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state webhookEndpointResourceModel

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

	resp.Diagnostics.Append(r.populateState(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if state.Id.IsNull() {
		resp.State.RemoveResource(ctx)
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *webhookEndpointResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan webhookEndpointResourceModel

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

	var state webhookEndpointResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	endpointID := state.Id.ValueString()

	apiReq, buildDiags := plan.toUpdateRequest(ctx)
	resp.Diagnostics.Append(buildDiags...)
	if resp.Diagnostics.HasError() {
		return
	}

	updateResp, err := r.client.UpdateWebhookEndpointWithResponse(ctx, endpointID, apiReq)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Updating Webhook Endpoint",
			"Could not update webhook endpoint, unexpected error: "+err.Error(),
		)
		return
	}

	if updateResp.StatusCode() != 200 {
		resp.Diagnostics.AddError(
			"Error Updating Webhook Endpoint",
			fmt.Sprintf("Could not update webhook endpoint, status: %d, body: %s", updateResp.StatusCode(), string(updateResp.Body)),
		)
		return
	}

	if updateResp.JSON200 == nil {
		resp.Diagnostics.AddError(
			"Error Updating Webhook Endpoint",
			"Received empty response body",
		)
		return
	}

	resp.Diagnostics.Append(overlayWebhookEndpointComputedFields(ctx, updateResp.JSON200, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *webhookEndpointResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state webhookEndpointResourceModel

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

	deleteResp, err := r.client.DeleteWebhookEndpointWithResponse(ctx, state.Id.ValueString())
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Deleting Webhook Endpoint",
			"Could not delete webhook endpoint, unexpected error: "+err.Error(),
		)
		return
	}

	if deleteResp.StatusCode() != 200 && deleteResp.StatusCode() != 204 && deleteResp.StatusCode() != 404 {
		resp.Diagnostics.AddError(
			"Error Deleting Webhook Endpoint",
			fmt.Sprintf("Could not delete webhook endpoint, status: %d, body: %s", deleteResp.StatusCode(), string(deleteResp.Body)),
		)
		return
	}
}
