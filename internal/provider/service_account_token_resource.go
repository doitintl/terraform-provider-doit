package provider

import (
	"context"
	"fmt"
	"strings"
	"uuid"

	"github.com/doitintl/terraform-provider-doit/internal/provider/models"
	"github.com/doitintl/terraform-provider-doit/internal/provider/resource_service_account_token"
	"github.com/hashicorp/terraform-plugin-framework-timeouts/resource/timeouts"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

type serviceAccountTokenResource struct {
	client *models.ClientWithResponses
}

// Ensure the implementation satisfies expected interfaces.
var (
	_ resource.Resource                = (*serviceAccountTokenResource)(nil)
	_ resource.ResourceWithConfigure   = (*serviceAccountTokenResource)(nil)
	_ resource.ResourceWithImportState = (*serviceAccountTokenResource)(nil)
)

// NewServiceAccountTokenResource creates a new service account token resource instance.
func NewServiceAccountTokenResource() resource.Resource {
	return &serviceAccountTokenResource{}
}

// Configure adds the provider configured client to the resource.
func (r *serviceAccountTokenResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *serviceAccountTokenResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_service_account_token"
}

// ImportState imports a token by its composite ID `<service_account_id>/<id>`.
// The token ID alone cannot address the resource, because the API nests tokens
// under their service account.
func (r *serviceAccountTokenResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	parts := strings.Split(req.ID, "/")
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		resp.Diagnostics.AddError(
			"Invalid Import ID",
			fmt.Sprintf("Expected import ID in the format 'service_account_id/token_id', got: %s", req.ID),
		)
		return
	}

	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("service_account_id"), parts[0])...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), parts[1])...)
}

func (r *serviceAccountTokenResource) Schema(ctx context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	s := resource_service_account_token.ServiceAccountTokenResourceSchema(ctx)

	// The API cannot change these in place (PATCH accepts only state), so changing
	// them replaces the token — and mints a new credential.
	for _, name := range []string{"service_account_id", "name", "expires_time"} {
		if attr, ok := s.Attributes[name].(schema.StringAttribute); ok {
			attr.PlanModifiers = append(attr.PlanModifiers, stringplanmodifier.RequiresReplace())
			s.Attributes[name] = attr
		}
	}

	// Add UseStateForUnknown to stable Computed fields so they don't show as
	// "(known after apply)" on every plan that modifies the resource. last_used_time
	// is intentionally excluded: the API updates it whenever the token is used.
	if attr, ok := s.Attributes["id"].(schema.StringAttribute); ok {
		attr.PlanModifiers = append(attr.PlanModifiers, stringplanmodifier.UseStateForUnknown())
		s.Attributes["id"] = attr
	}
	if attr, ok := s.Attributes["customer_id"].(schema.StringAttribute); ok {
		attr.PlanModifiers = append(attr.PlanModifiers, stringplanmodifier.UseStateForUnknown())
		s.Attributes["customer_id"] = attr
	}
	if attr, ok := s.Attributes["create_time"].(schema.StringAttribute); ok {
		attr.PlanModifiers = append(attr.PlanModifiers, stringplanmodifier.UseStateForUnknown())
		s.Attributes["create_time"] = attr
	}
	if attr, ok := s.Attributes["access_token"].(schema.StringAttribute); ok {
		attr.PlanModifiers = append(attr.PlanModifiers, stringplanmodifier.UseStateForUnknown())
		s.Attributes["access_token"] = attr
	}
	// expires_time is Optional+Computed: this keeps the server-assigned default
	// stable when the attribute is omitted from config.
	if attr, ok := s.Attributes["expires_time"].(schema.StringAttribute); ok {
		attr.PlanModifiers = append(attr.PlanModifiers, stringplanmodifier.UseStateForUnknown())
		s.Attributes["expires_time"] = attr
	}

	if attr, ok := s.Attributes["expires_time"].(schema.StringAttribute); ok {
		// The regex pins the canonical spelling (UTC, whole seconds); it only checks
		// digit placement, so the RFC 3339 validator rejects impossible values such
		// as month 99 at plan time instead of at apply.
		attr.Validators = append(attr.Validators,
			rfc3339Validator{},
			stringvalidator.RegexMatches(serviceAccountTokenExpiresTimePattern, serviceAccountTokenExpiresTimeMessage),
		)
		attr.Description += " When omitted, the API sets it to one year after creation, so tokens always expire. Must be an RFC 3339 timestamp in UTC with whole seconds (for example `2027-01-01T00:00:00Z`). Changing it replaces the token."
		attr.MarkdownDescription = attr.Description
		s.Attributes["expires_time"] = attr
	}

	if attr, ok := s.Attributes["name"].(schema.StringAttribute); ok {
		attr.Description += " Changing it replaces the token."
		attr.MarkdownDescription = attr.Description
		s.Attributes["name"] = attr
	}

	if attr, ok := s.Attributes["access_token"].(schema.StringAttribute); ok {
		attr.Description += " It is only available after the token is created by Terraform and is `null` for imported tokens."
		attr.MarkdownDescription = attr.Description
		s.Attributes["access_token"] = attr
	}

	// Category B (not clearable): the API always assigns a value, so null is never a valid state.
	acknowledgeNotClearable(s,
		"state",        // API defaults to active
		"expires_time", // API assigns a default expiry when omitted; changing it replaces the token
	)

	s.Description = "Manages an API token of a service account. The token authenticates as the service account with the permissions it has at creation.\n\n" +
		"~> **Note:** The `access_token` is returned by the API only when the token is created. Terraform keeps it in state (marked sensitive); it is `null` for imported tokens.\n\n" +
		"~> **Note:** Changing `service_account_id`, `name` or `expires_time` replaces the token. The replacement is a **new credential**, so anything that uses `access_token` must pick up the new value.\n\n" +
		"~> **Note:** `state` is the stored value and never becomes `expired`. A token past its `expires_time` stops authenticating but keeps its stored state, so compare `expires_time` to tell whether a token still works. Re-enabling an expired token fails; create a new token instead.\n\n" +
		"A service account can have at most 10 tokens."
	s.MarkdownDescription = s.Description

	s.Attributes["timeouts"] = timeouts.Attributes(ctx, timeouts.Opts{
		Create: true,
		Read:   true,
		Update: true,
		Delete: true,
	})

	resp.Schema = s
}

func (r *serviceAccountTokenResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan serviceAccountTokenResourceModel

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

	apiReq, diags := plan.toCreateRequest()
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	params := &models.CreateServiceAccountTokenParams{
		IdempotencyKey: uuid.NewV4().String(),
	}

	createResp, err := r.client.CreateServiceAccountTokenWithResponse(ctx, plan.ServiceAccountId.ValueString(), params, apiReq)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Creating Service Account Token",
			"Could not create service account token, unexpected error: "+err.Error(),
		)
		return
	}

	if createResp.StatusCode() != 201 {
		resp.Diagnostics.AddError(
			"Error Creating Service Account Token",
			serviceAccountTokenStatusMessage("create", createResp.StatusCode(), createResp.Body),
		)
		return
	}

	if createResp.JSON201 == nil {
		resp.Diagnostics.AddError(
			"Error Creating Service Account Token",
			"Received empty response body for created service account token",
		)
		return
	}

	// The access token is only returned here, so it must be stored now.
	plan.AccessToken = types.StringPointerValue(createResp.JSON201.AccessToken)

	// Plan-first state pattern: overlay Computed-only and resolved fields from API response.
	created := serviceAccountTokenFromCreateResponse(createResp.JSON201)
	overlayServiceAccountTokenComputedFields(created, &plan)
	if plan.Id.IsNull() {
		resp.Diagnostics.AddError(
			"Error Creating Service Account Token",
			"The API did not return an ID for the created service account token",
		)
		return
	}

	// Save the created token before any follow-up call so a failure below cannot orphan it.
	desiredState := plan.State
	plan.State = types.StringValue(string(created.State))
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// The create endpoint has no state field: tokens always start active.
	// Apply a configured `disabled` with a follow-up PATCH.
	if !desiredState.IsUnknown() && !desiredState.IsNull() && desiredState.ValueString() != plan.State.ValueString() {
		plan.State = desiredState
		updated, diags := r.patchState(ctx, &plan)
		resp.Diagnostics.Append(diags...)
		if resp.Diagnostics.HasError() {
			return
		}
		plan.State = types.StringValue(string(updated.State))
		plan.LastUsedTime = nullableTimeToString(updated.LastUsedTime)
		resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
	}
}

func (r *serviceAccountTokenResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state serviceAccountTokenResourceModel

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

func (r *serviceAccountTokenResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan serviceAccountTokenResourceModel

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

	updated, diags := r.patchState(ctx, &plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Plan-first state pattern: overlay Computed-only and resolved fields from API response.
	overlayServiceAccountTokenComputedFields(updated, &plan)

	// Save updated data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *serviceAccountTokenResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state serviceAccountTokenResourceModel

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

	params := &models.DeleteServiceAccountTokenParams{
		IdempotencyKey: uuid.NewV4().String(),
	}

	deleteResp, err := r.client.DeleteServiceAccountTokenWithResponse(ctx, state.ServiceAccountId.ValueString(), state.Id.ValueString(), params)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Deleting Service Account Token",
			"Could not delete service account token, unexpected error: "+err.Error(),
		)
		return
	}

	// Treat 404 as success - the token is already gone (deleted outside Terraform,
	// or its service account was deleted).
	if deleteResp.StatusCode() != 200 && deleteResp.StatusCode() != 204 && deleteResp.StatusCode() != 404 {
		resp.Diagnostics.AddError(
			"Error Deleting Service Account Token",
			serviceAccountTokenStatusMessage("delete", deleteResp.StatusCode(), deleteResp.Body),
		)
		return
	}
}

// patchState moves the token to plan.State and returns the updated token.
func (r *serviceAccountTokenResource) patchState(ctx context.Context, plan *serviceAccountTokenResourceModel) (*models.ServiceAccountToken, diag.Diagnostics) {
	var diags diag.Diagnostics

	params := &models.UpdateServiceAccountTokenParams{
		IdempotencyKey: uuid.NewV4().String(),
	}

	updateResp, err := r.client.UpdateServiceAccountTokenWithResponse(ctx, plan.ServiceAccountId.ValueString(), plan.Id.ValueString(), params, plan.toUpdateRequest())
	if err != nil {
		diags.AddError(
			"Error Updating Service Account Token",
			"Could not update service account token, unexpected error: "+err.Error(),
		)
		return nil, diags
	}

	if updateResp.StatusCode() != 200 {
		diags.AddError(
			"Error Updating Service Account Token",
			serviceAccountTokenStatusMessage("update", updateResp.StatusCode(), updateResp.Body),
		)
		return nil, diags
	}

	if updateResp.JSON200 == nil {
		diags.AddError(
			"Error Updating Service Account Token",
			"Received empty response body for updated service account token",
		)
		return nil, diags
	}

	return updateResp.JSON200, diags
}

// serviceAccountTokenStatusMessage builds the error text for an unexpected status.
// The API body carries the specific reason (for example an expired token that
// cannot be re-enabled, a duplicate name, or the 10-token cap).
func serviceAccountTokenStatusMessage(action string, status int, body []byte) string {
	return fmt.Sprintf("Could not %s service account token, status: %d, body: %s", action, status, string(body))
}
