// Package provider implements the DoiT Terraform provider.
package provider

import (
	"context"
	"fmt"
	"strings"

	"github.com/doitintl/terraform-provider-doit/internal/provider/models"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
	openapi_types "github.com/oapi-codegen/runtime/types"
)

// normalizePhone restores the "+" prefix that the API strips from phone
// country codes. The spec accepts "^\+[0-9]+$" but stores without the "+".
func normalizePhone(phone *string) types.String {
	if phone == nil || *phone == "" {
		return types.StringPointerValue(phone)
	}
	if !strings.HasPrefix(*phone, "+") {
		normalized := "+" + *phone
		return types.StringValue(normalized)
	}
	return types.StringValue(*phone)
}

// mapUserToModel maps the API UserListItem to the Terraform model.
// Used by Read and ImportState (full mapping from API response).
func mapUserToModel(user *models.UserListItem, state *userResourceModel) {
	// id = email (stable identifier)
	state.Id = types.StringPointerValue(user.Email)
	state.Email = types.StringPointerValue(user.Email)
	state.FirstName = types.StringPointerValue(user.FirstName)
	state.LastName = types.StringPointerValue(user.LastName)
	state.DisplayName = types.StringPointerValue(user.DisplayName)
	state.OrganizationId = types.StringPointerValue(user.OrganizationId)
	state.RoleId = types.StringPointerValue(user.RoleId)
	state.Phone = normalizePhone(user.Phone)
	state.PhoneExtension = types.StringPointerValue(user.PhoneExtension)

	if user.JobTitle != nil {
		state.JobTitle = types.StringValue(string(*user.JobTitle))
	} else {
		state.JobTitle = types.StringNull()
	}

	if user.Language != nil {
		state.Language = types.StringValue(string(*user.Language))
	} else {
		state.Language = types.StringNull()
	}

	if user.Status != nil {
		state.Status = types.StringValue(strings.ToLower(string(*user.Status)))
	} else {
		state.Status = types.StringNull()
	}
}

// overlayUserComputedFields implements the plan-first overlay pattern for
// Create and Update. It preserves user-configured values from the plan and
// only sets Computed-only fields from the API response.
func overlayUserComputedFields(user *models.UserListItem, plan *userResourceModel) {
	// Phase 1: Build fully-resolved state from API response.
	var resolved userResourceModel
	mapUserToModel(user, &resolved)

	// Phase 2: Overlay computed-only fields — always from resolved.
	plan.Id = resolved.Id
	plan.DisplayName = resolved.DisplayName
	plan.Status = resolved.Status

	// Optional+Computed fields: resolve ONLY when unknown (user omitted them).
	// When known, the user explicitly set them — preserve the plan value.
	if plan.FirstName.IsUnknown() {
		plan.FirstName = resolved.FirstName
	}
	if plan.LastName.IsUnknown() {
		plan.LastName = resolved.LastName
	}
	if plan.JobTitle.IsUnknown() {
		plan.JobTitle = resolved.JobTitle
	}
	if plan.RoleId.IsUnknown() {
		plan.RoleId = resolved.RoleId
	}
	if plan.OrganizationId.IsUnknown() {
		plan.OrganizationId = resolved.OrganizationId
	}

	// Note: phone, phone_extension, and language are Optional-only (not Computed).
	// They should always preserve the plan value regardless of Unknown/Known state.
}

// resolveInternalID looks up the current internal UUID for a user by their
// email address. The internal ID is needed for Update and Delete API calls.
// Returns the internal ID or diagnostics if the lookup fails.
func (r *userResource) resolveInternalID(ctx context.Context, email string) (string, diag.Diagnostics) {
	var diags diag.Diagnostics

	params := &models.ListUsersParams{
		Email: new(openapi_types.Email(email)),
	}

	listResp, err := r.client.ListUsersWithResponse(ctx, params)
	if err != nil {
		diags.AddError(
			"Error Resolving User ID",
			"Could not list users to resolve internal ID for "+email+": "+err.Error(),
		)
		return "", diags
	}

	if listResp.StatusCode() != 200 || listResp.JSON200 == nil {
		diags.AddError(
			"Error Resolving User ID",
			fmt.Sprintf("Could not list users, status: %d, body: %s", listResp.StatusCode(), string(listResp.Body)),
		)
		return "", diags
	}

	if listResp.JSON200.Users == nil || len(*listResp.JSON200.Users) == 0 {
		diags.AddError(
			"User Not Found",
			"No user found with email "+email+". The user may have been deleted outside of Terraform.",
		)
		return "", diags
	}

	users := *listResp.JSON200.Users
	if users[0].Id == nil {
		diags.AddError(
			"Error Resolving User ID",
			"User found but has no internal ID for email "+email,
		)
		return "", diags
	}

	return *users[0].Id, diags
}

// lookupUser fetches a single user by email from the ListUsers endpoint.
// Returns the user or nil if not found. Used by Read and Create overlay.
func (r *userResource) lookupUser(ctx context.Context, email string) (*models.UserListItem, diag.Diagnostics) {
	var diags diag.Diagnostics

	params := &models.ListUsersParams{
		Email: new(openapi_types.Email(email)),
	}

	listResp, err := r.client.ListUsersWithResponse(ctx, params)
	if err != nil {
		diags.AddError(
			"Error Reading User",
			"Could not list users for email "+email+": "+err.Error(),
		)
		return nil, diags
	}

	if listResp.StatusCode() != 200 || listResp.JSON200 == nil {
		diags.AddError(
			"Error Reading User",
			fmt.Sprintf("Could not list users, status: %d, body: %s", listResp.StatusCode(), string(listResp.Body)),
		)
		return nil, diags
	}

	if listResp.JSON200.Users == nil || len(*listResp.JSON200.Users) == 0 {
		// User not found — not an error, caller handles this.
		return nil, diags
	}

	users := *listResp.JSON200.Users
	return &users[0], diags
}
