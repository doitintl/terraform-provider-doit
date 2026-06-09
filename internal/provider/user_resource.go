package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"time"

	"github.com/doitintl/terraform-provider-doit/internal/provider/models"
	"github.com/doitintl/terraform-provider-doit/internal/provider/resource_user"
	"github.com/hashicorp/terraform-plugin-framework-timeouts/resource/timeouts"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

type (
	userResource struct {
		client *models.ClientWithResponses
	}
	userResourceModel struct {
		Id             types.String   `tfsdk:"id"`
		Email          types.String   `tfsdk:"email"`
		FirstName      types.String   `tfsdk:"first_name"`
		LastName       types.String   `tfsdk:"last_name"`
		JobTitle       types.String   `tfsdk:"job_title"`
		RoleId         types.String   `tfsdk:"role_id"`
		OrganizationId types.String   `tfsdk:"organization_id"`
		Phone          types.String   `tfsdk:"phone"`
		PhoneExtension types.String   `tfsdk:"phone_extension"`
		Language       types.String   `tfsdk:"language"`
		DisplayName    types.String   `tfsdk:"display_name"`
		Status         types.String   `tfsdk:"status"`
		Timeouts       timeouts.Value `tfsdk:"timeouts"`
	}
)

// Ensure the implementation satisfies expected interfaces.
var (
	_ resource.Resource                = (*userResource)(nil)
	_ resource.ResourceWithConfigure   = (*userResource)(nil)
	_ resource.ResourceWithImportState = (*userResource)(nil)
)

// NewUserResource creates a new user resource instance.
func NewUserResource() resource.Resource {
	return &userResource{}
}

// Configure adds the provider configured client to the resource.
func (r *userResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *userResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_user"
}

func (r *userResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	// Import identifier is the email address (which is also the resource id).
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
	// Also set email = id so the Read path works correctly.
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("email"), req.ID)...)
}

func (r *userResource) Schema(ctx context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	// Start from the generated schema (PATCH-as-read gives us email,
	// first_name, last_name, job_title, organization_id, role_id, id
	// with correct Optional/Computed flags and validators).
	s := resource_user.UserResourceSchema(ctx)

	// --- Remove response-only artifacts that leaked from the PATCH response ---
	delete(s.Attributes, "message") // UpdateUserResponse.message
	delete(s.Attributes, "user")    // UpdateUserResponse.user (nested)

	// --- Fix `id`: should be Computed-only, not Optional+Computed ---
	s.Attributes["id"] = schema.StringAttribute{
		Computed:            true,
		Description:         "The resource identifier (email address of the user).",
		MarkdownDescription: "The resource identifier (email address of the user).",
		PlanModifiers: []planmodifier.String{
			stringplanmodifier.UseStateForUnknown(),
		},
	}

	// --- Add plan modifiers to generated attributes ---

	// email: RequiresReplace (changing email means re-invite)
	if attr, ok := s.Attributes["email"].(schema.StringAttribute); ok {
		attr.PlanModifiers = append(attr.PlanModifiers, stringplanmodifier.RequiresReplace())
		s.Attributes["email"] = attr
	}

	// Classify Optional+Computed attributes (clearableattr).
	// See: https://github.com/doitintl/terraform-provider-doit/issues/233

	// Category B: API-assigned, not clearable.
	acknowledgeNotClearable(s,
		"first_name",      // API ignores "" PATCH — not clearable once set
		"last_name",       // API ignores "" PATCH — not clearable once set
		"job_title",       // API ignores "" PATCH — not clearable once set
	)

	// organization_id: RequiresReplace (immutable after invite) + UseStateForUnknown
	if attr, ok := s.Attributes["organization_id"].(schema.StringAttribute); ok { //nolint:clearableattr // API-level org assignment
		attr.PlanModifiers = append(attr.PlanModifiers,
			stringplanmodifier.RequiresReplace(),
			stringplanmodifier.UseStateForUnknown(),
		)
		s.Attributes["organization_id"] = attr
	}
	// role_id: UseStateForUnknown (preserve API default when user doesn't specify)
	if attr, ok := s.Attributes["role_id"].(schema.StringAttribute); ok { //nolint:clearableattr // API assigns default role on creation
		attr.PlanModifiers = append(attr.PlanModifiers, stringplanmodifier.UseStateForUnknown())
		s.Attributes["role_id"] = attr
	}

	// --- Add attributes not present in the generated schema ---
	// These exist in UserListItem but not in InviteUserRequest/UpdateUserResponse.

	s.Attributes["display_name"] = schema.StringAttribute{
		Computed:            true,
		Description:         "The user's display name.",
		MarkdownDescription: "The user's display name.",
	}

	s.Attributes["status"] = schema.StringAttribute{
		Computed:            true,
		Description:         "The status of the user (`active` or `invited`).",
		MarkdownDescription: "The status of the user (`active` or `invited`).",
		PlanModifiers: []planmodifier.String{
			stringplanmodifier.UseStateForUnknown(),
		},
	}

	s.Attributes["phone"] = schema.StringAttribute{
		Optional:            true,
		Description:         "The user's country code (e.g., `+44`).",
		MarkdownDescription: "The user's country code (e.g., `+44`).",
		Validators: []validator.String{
			stringvalidator.RegexMatches(
				regexp.MustCompile(`^\+[0-9]+$`),
				"must start with '+' followed by digits (e.g., +1, +44)",
			),
		},
	}

	s.Attributes["phone_extension"] = schema.StringAttribute{
		Optional:            true,
		Description:         "The user's phone extension.",
		MarkdownDescription: "The user's phone extension.",
	}

	s.Attributes["language"] = schema.StringAttribute{
		Optional:            true,
		Description:         "The user's preferred language.\nPossible values: `en`, `ja`",
		MarkdownDescription: "The user's preferred language.\nPossible values: `en`, `ja`",
		Validators: []validator.String{
			stringvalidator.OneOf("en", "ja"),
		},
	}

	// --- Add timeouts ---
	s.Attributes["timeouts"] = timeouts.Attributes(ctx, timeouts.Opts{
		Create: true,
		Read:   true,
		Update: true,
		Delete: true,
	})

	resp.Schema = s
}

func (r *userResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan userResourceModel

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

	// Build invite request from plan.
	apiReq := plan.toInviteRequest()

	// Invite the user.
	inviteResp, err := r.client.InviteUserWithResponse(ctx, apiReq)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Inviting User",
			"Could not invite user, unexpected error: "+err.Error(),
		)
		return
	}

	if inviteResp.StatusCode() != 200 && inviteResp.StatusCode() != 201 {
		resp.Diagnostics.AddError(
			"Error Inviting User",
			fmt.Sprintf("Could not invite user, status: %d, body: %s", inviteResp.StatusCode(), string(inviteResp.Body)),
		)
		return
	}

	// The generated client only auto-parses into JSON201 for status 201,
	// but the API actually returns 200. Parse the body ourselves when needed.
	inviteBody := inviteResp.JSON201
	if inviteBody == nil {
		var parsed models.InviteResponse
		if err := json.Unmarshal(inviteResp.Body, &parsed); err != nil {
			resp.Diagnostics.AddError(
				"Error Inviting User",
				"Could not parse invite response: "+err.Error(),
			)
			return
		}
		inviteBody = &parsed
	}

	if inviteBody.User == nil {
		resp.Diagnostics.AddError(
			"Error Inviting User",
			"Could not invite user, empty response",
		)
		return
	}

	// Auto-PATCH: The Invite API does not support phone, phoneExtension, or
	// language. If any of these are set in the plan, immediately follow up
	// with a PATCH request using the internal ID from the invite response.
	needsPatch := (!plan.Phone.IsNull() && !plan.Phone.IsUnknown()) ||
		(!plan.PhoneExtension.IsNull() && !plan.PhoneExtension.IsUnknown()) ||
		(!plan.Language.IsNull() && !plan.Language.IsUnknown())

	if needsPatch {
		internalID := inviteBody.User.Id
		if internalID == nil || *internalID == "" {
			resp.Diagnostics.AddError(
				"Error Updating User After Invite",
				"Invite response did not contain an internal user ID, cannot set phone/language fields",
			)
			return
		}

		patchReq := models.UpdateUserRequest{}
		if !plan.Phone.IsNull() && !plan.Phone.IsUnknown() {
			patchReq.Phone = plan.Phone.ValueStringPointer()
		}
		if !plan.PhoneExtension.IsNull() && !plan.PhoneExtension.IsUnknown() {
			patchReq.PhoneExtension = plan.PhoneExtension.ValueStringPointer()
		}
		if !plan.Language.IsNull() && !plan.Language.IsUnknown() {
			patchReq.Language = new(models.UpdateUserRequestLanguage(plan.Language.ValueString()))
		}

		patchResp, err := r.client.UpdateUserWithResponse(ctx, *internalID, patchReq)
		if err != nil {
			resp.Diagnostics.AddError(
				"Error Updating User After Invite",
				"Could not set phone/language after invite: "+err.Error(),
			)
			return
		}

		if patchResp.StatusCode() != 200 {
			resp.Diagnostics.AddError(
				"Error Updating User After Invite",
				fmt.Sprintf("Could not set phone/language after invite, status: %d, body: %s", patchResp.StatusCode(), string(patchResp.Body)),
			)
			return
		}
	}

	// Look up the user via ListUsers to get the full state (including
	// display_name and status) for the overlay.
	user, lookupDiags := r.lookupUser(ctx, plan.Email.ValueString())
	resp.Diagnostics.Append(lookupDiags...)
	if resp.Diagnostics.HasError() {
		return
	}

	if user == nil {
		resp.Diagnostics.AddError(
			"Error Reading User After Create",
			"User was invited but could not be found via ListUsers for email "+plan.Email.ValueString(),
		)
		return
	}

	// Plan-first overlay: keep user-configured values, set Computed-only fields.
	overlayUserComputedFields(user, &plan)

	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *userResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state userResourceModel

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

	// Handle externally deleted resource (populateState sets Id to null when not found)
	if state.Id.IsNull() {
		resp.State.RemoveResource(ctx)
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *userResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan userResourceModel

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

	// Resolve the current internal UUID from the email.
	internalID, idDiags := r.resolveInternalID(ctx, plan.Email.ValueString())
	resp.Diagnostics.Append(idDiags...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Build the update request from plan values.
	patchReq := plan.toUpdateRequest()

	updateResp, err := r.client.UpdateUserWithResponse(ctx, internalID, patchReq)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Updating User",
			"Could not update user, unexpected error: "+err.Error(),
		)
		return
	}

	if updateResp.StatusCode() != 200 {
		resp.Diagnostics.AddError(
			"Error Updating User",
			fmt.Sprintf("Could not update user, status: %d, body: %s", updateResp.StatusCode(), string(updateResp.Body)),
		)
		return
	}

	// Look up the full user to get display_name/status for overlay.
	user, lookupDiags := r.lookupUser(ctx, plan.Email.ValueString())
	resp.Diagnostics.Append(lookupDiags...)
	if resp.Diagnostics.HasError() {
		return
	}

	if user == nil {
		resp.Diagnostics.AddError(
			"User Not Found After Update",
			"The user was updated but could not be found via ListUsers. This may indicate a transient API issue.",
		)
		return
	}

	// Plan-first overlay: keep user-configured values, set Computed-only fields.
	overlayUserComputedFields(user, &plan)

	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *userResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state userResourceModel

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

	// Resolve the current internal UUID from the email.
	email := state.Email.ValueString()
	internalID, idDiags := r.resolveInternalID(ctx, email)
	if idDiags.HasError() {
		// If we can't find the user, treat it as already deleted.
		for _, d := range idDiags {
			if d.Summary() == "User Not Found" {
				return
			}
		}
		resp.Diagnostics.Append(idDiags...)
		return
	}

	// Delete the user via API.
	deleteResp, err := r.client.DeleteUserWithResponse(ctx, internalID)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Deleting User",
			"Could not delete user, unexpected error: "+err.Error(),
		)
		return
	}

	// Treat 404 as success — resource is already gone.
	if deleteResp.StatusCode() != 200 && deleteResp.StatusCode() != 204 && deleteResp.StatusCode() != 404 {
		resp.Diagnostics.AddError(
			"Error Deleting User",
			fmt.Sprintf("Could not delete user, status: %d, body: %s", deleteResp.StatusCode(), string(deleteResp.Body)),
		)
		return
	}
}
