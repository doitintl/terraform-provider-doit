package provider

import (
	"context"
	"fmt"
	"uuid"

	"github.com/doitintl/terraform-provider-doit/internal/provider/models"
	"github.com/doitintl/terraform-provider-doit/internal/provider/resource_service_account"
	"github.com/hashicorp/terraform-plugin-framework-timeouts/resource/timeouts"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
)

type serviceAccountResource struct {
	client *models.ClientWithResponses
}

// Ensure the implementation satisfies expected interfaces.
var (
	_ resource.Resource                = (*serviceAccountResource)(nil)
	_ resource.ResourceWithConfigure   = (*serviceAccountResource)(nil)
	_ resource.ResourceWithImportState = (*serviceAccountResource)(nil)
)

// NewServiceAccountResource creates a new service account resource instance.
func NewServiceAccountResource() resource.Resource {
	return &serviceAccountResource{}
}

// Configure adds the provider configured client to the resource.
func (r *serviceAccountResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *serviceAccountResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_service_account"
}

func (r *serviceAccountResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

func (r *serviceAccountResource) Schema(ctx context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	s := resource_service_account.ServiceAccountResourceSchema(ctx)

	// Add UseStateForUnknown to stable Computed-only fields so they don't
	// show as "(known after apply)" on every plan that modifies the resource.
	if attr, ok := s.Attributes["id"].(schema.StringAttribute); ok {
		attr.PlanModifiers = append(attr.PlanModifiers, stringplanmodifier.UseStateForUnknown())
		s.Attributes["id"] = attr
	}
	if attr, ok := s.Attributes["customer_id"].(schema.StringAttribute); ok {
		attr.PlanModifiers = append(attr.PlanModifiers, stringplanmodifier.UseStateForUnknown())
		s.Attributes["customer_id"] = attr
	}
	if attr, ok := s.Attributes["created_by"].(schema.StringAttribute); ok {
		attr.PlanModifiers = append(attr.PlanModifiers, stringplanmodifier.UseStateForUnknown())
		s.Attributes["created_by"] = attr
	}
	if attr, ok := s.Attributes["created_by_email"].(schema.StringAttribute); ok {
		attr.PlanModifiers = append(attr.PlanModifiers, stringplanmodifier.UseStateForUnknown())
		s.Attributes["created_by_email"] = attr
	}
	if attr, ok := s.Attributes["created_by_user_id"].(schema.StringAttribute); ok {
		attr.PlanModifiers = append(attr.PlanModifiers, stringplanmodifier.UseStateForUnknown())
		s.Attributes["created_by_user_id"] = attr
	}
	if attr, ok := s.Attributes["create_time"].(schema.StringAttribute); ok {
		attr.PlanModifiers = append(attr.PlanModifiers, stringplanmodifier.UseStateForUnknown())
		s.Attributes["create_time"] = attr
	}

	// Add clearing plan modifiers to Optional+Computed attributes.
	if attr, ok := s.Attributes["description"].(schema.StringAttribute); ok {
		attr.PlanModifiers = append(attr.PlanModifiers, useEmptyForUnknownWhenConfigNull())
		s.Attributes["description"] = attr
	}
	if attr, ok := s.Attributes["permissions"].(schema.ListAttribute); ok {
		attr.PlanModifiers = append(attr.PlanModifiers, useNullForUnknownListWhenConfigNull())
		s.Attributes["permissions"] = attr
	}

	s.Attributes["timeouts"] = timeouts.Attributes(ctx, timeouts.Opts{
		Create: true,
		Read:   true,
		Update: true,
		Delete: true,
	})

	resp.Schema = s
}

func (r *serviceAccountResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan serviceAccountResourceModel

	// Read Terraform plan data into the model
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	createTimeout, diags := plan.Timeouts.Create(ctx, DefaultCreateTimeout)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	ctx, cancel := context.WithTimeout(ctx, createTimeout)
	defer cancel()

	apiReq, diags := plan.toCreateRequest(ctx)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	params := &models.CreateServiceAccountParams{
		IdempotencyKey: uuid.NewV4().String(),
	}

	createResp, err := r.client.CreateServiceAccountWithResponse(ctx, params, apiReq)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Creating Service Account",
			"Could not create service account, unexpected error: "+err.Error(),
		)
		return
	}

	if createResp.StatusCode() != 201 {
		resp.Diagnostics.AddError(
			"Error Creating Service Account",
			fmt.Sprintf("Could not create service account, status: %d, body: %s", createResp.StatusCode(), string(createResp.Body)),
		)
		return
	}

	if createResp.JSON201 == nil {
		resp.Diagnostics.AddError(
			"Error Creating Service Account",
			"Received empty response body for created service account",
		)
		return
	}

	// Plan-first state pattern: overlay Computed-only and resolved fields from API response.
	resp.Diagnostics.Append(overlayServiceAccountComputedFields(ctx, r.canonicalServiceAccount(ctx, createResp.JSON201), &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Save data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *serviceAccountResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state serviceAccountResourceModel

	// Read Terraform prior state
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	readTimeout, diags := state.Timeouts.Read(ctx, DefaultReadTimeout)
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

	// Handle externally deleted resource (populateState sets Id to null on 404)
	if state.Id.IsNull() {
		resp.State.RemoveResource(ctx)
		return
	}

	// Save updated data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *serviceAccountResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan serviceAccountResourceModel

	// Read Terraform plan data
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	updateTimeout, diags := plan.Timeouts.Update(ctx, DefaultUpdateTimeout)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	ctx, cancel := context.WithTimeout(ctx, updateTimeout)
	defer cancel()

	// Get the ID and ETag from the prior state
	var state serviceAccountResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	serviceAccountID := state.Id.ValueString()
	etag := state.Etag.ValueString()

	apiReq, diags := plan.toUpdateRequest(ctx)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	params := &models.UpdateServiceAccountParams{
		IfMatch:        etag,
		IdempotencyKey: uuid.NewV4().String(),
	}

	updateResp, err := r.client.UpdateServiceAccountWithApplicationMergePatchPlusJSONBodyWithResponse(ctx, serviceAccountID, params, apiReq)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Updating Service Account",
			"Could not update service account, unexpected error: "+err.Error(),
		)
		return
	}

	if updateResp.StatusCode() != 200 {
		resp.Diagnostics.AddError(
			"Error Updating Service Account",
			fmt.Sprintf("Could not update service account, status: %d, body: %s", updateResp.StatusCode(), string(updateResp.Body)),
		)
		return
	}

	if updateResp.JSON200 == nil {
		resp.Diagnostics.AddError(
			"Error Updating Service Account",
			"Received empty response body for updated service account",
		)
		return
	}

	// Plan-first state pattern: overlay Computed-only and resolved fields from API response.
	resp.Diagnostics.Append(overlayServiceAccountComputedFields(ctx, r.canonicalServiceAccount(ctx, updateResp.JSON200), &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Save updated data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *serviceAccountResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state serviceAccountResourceModel

	// Read Terraform prior state data into the model
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	deleteTimeout, diags := state.Timeouts.Delete(ctx, DefaultDeleteTimeout)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	ctx, cancel := context.WithTimeout(ctx, deleteTimeout)
	defer cancel()

	// Always send the current ETag instead of the one in state: it may predate an
	// out-of-band change, or come from a create/update response (see
	// canonicalServiceAccount), and a stale value fails the delete with 412.
	etag := ""
	getResp, err := r.client.GetServiceAccountWithResponse(ctx, state.Id.ValueString())
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Deleting Service Account",
			"Could not fetch service account to get ETag: "+err.Error(),
		)
		return
	}
	if getResp.StatusCode() == 404 {
		return
	}
	if getResp.StatusCode() != 200 || getResp.JSON200 == nil {
		resp.Diagnostics.AddError(
			"Error Deleting Service Account",
			fmt.Sprintf("Could not fetch service account to get ETag, status: %d, body: %s", getResp.StatusCode(), string(getResp.Body)),
		)
		return
	}
	if getResp.JSON200.Etag.IsSpecified() && !getResp.JSON200.Etag.IsNull() {
		etag = getResp.JSON200.Etag.MustGet()
	}

	params := &models.DeleteServiceAccountParams{
		IfMatch:        etag,
		IdempotencyKey: uuid.NewV4().String(),
	}

	deleteResp, err := r.client.DeleteServiceAccountWithResponse(ctx, state.Id.ValueString(), params)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Deleting Service Account",
			"Could not delete service account, unexpected error: "+err.Error(),
		)
		return
	}

	// If 412 (stale ETag), retry once with a fresh ETag
	if deleteResp.StatusCode() == 412 {
		getResp, err := r.client.GetServiceAccountWithResponse(ctx, state.Id.ValueString())
		if err != nil {
			resp.Diagnostics.AddError(
				"Error Deleting Service Account",
				"Could not refetch service account after 412: "+err.Error(),
			)
			return
		}
		if getResp.StatusCode() == 404 {
			return
		}
		if getResp.StatusCode() == 200 && getResp.JSON200 != nil && getResp.JSON200.Etag.IsSpecified() && !getResp.JSON200.Etag.IsNull() {
			params.IfMatch = getResp.JSON200.Etag.MustGet()
			params.IdempotencyKey = uuid.NewV4().String()
			deleteResp, err = r.client.DeleteServiceAccountWithResponse(ctx, state.Id.ValueString(), params)
			if err != nil {
				resp.Diagnostics.AddError(
					"Error Deleting Service Account",
					"Could not delete service account on retry: "+err.Error(),
				)
				return
			}
		}
	}

	// Treat 404 as success - resource is already gone (deleted outside Terraform)
	if deleteResp.StatusCode() != 200 && deleteResp.StatusCode() != 204 && deleteResp.StatusCode() != 404 {
		resp.Diagnostics.AddError(
			"Error Deleting Service Account",
			fmt.Sprintf("Could not delete service account, status: %d, body: %s", deleteResp.StatusCode(), string(deleteResp.Body)),
		)
		return
	}
}
