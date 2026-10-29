package provider

import (
	"context"
	"fmt"
	"strings"

	"github.com/doitintl/terraform-provider-doit/internal/provider/resource_cloudflow_connection"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// cloudflowConnectionCollaboratorsValidator requires exactly one owner and no
// repeated email addresses. The API adds the caller as owner when the list has
// none, which would make state differ from the plan, and does not validate the
// list itself: an owner who cannot be matched to a user locks everyone out.
type cloudflowConnectionCollaboratorsValidator struct{}

var _ validator.List = cloudflowConnectionCollaboratorsValidator{}

func (v cloudflowConnectionCollaboratorsValidator) Description(_ context.Context) string {
	return "Must contain exactly one collaborator with the role `owner` and no repeated email addresses."
}

func (v cloudflowConnectionCollaboratorsValidator) MarkdownDescription(ctx context.Context) string {
	return v.Description(ctx)
}

func (v cloudflowConnectionCollaboratorsValidator) ValidateList(ctx context.Context, req validator.ListRequest, resp *validator.ListResponse) {
	if req.ConfigValue.IsNull() || req.ConfigValue.IsUnknown() {
		return
	}

	var elements []resource_cloudflow_connection.CollaboratorsValue
	resp.Diagnostics.Append(req.ConfigValue.ElementsAs(ctx, &elements, true)...)
	if resp.Diagnostics.HasError() {
		return
	}

	owners := 0
	// undecided is set when an element or its role is not known yet, or the role is not
	// one the role attribute accepts: the owner count cannot be judged then, and the
	// attribute's own validators report an invalid or missing value (they only run
	// once this validator passes).
	undecided := false
	seen := make(map[string]struct{}, len(elements))
	for _, element := range elements {
		if element.IsUnknown() {
			undecided = true
			continue
		}

		// Known emails are checked whatever else is unknown: the owner count does not depend on them.
		if !element.Email.IsUnknown() && !element.Email.IsNull() {
			email := strings.ToLower(element.Email.ValueString())
			if _, dup := seen[email]; dup {
				resp.Diagnostics.AddAttributeError(
					req.Path,
					"Duplicate collaborator",
					fmt.Sprintf("The email address %q is listed more than once. Each collaborator must appear once.", element.Email.ValueString()),
				)
			}
			seen[email] = struct{}{}
		}

		if element.Role.IsUnknown() {
			undecided = true
			continue
		}
		switch element.Role.ValueString() {
		case "owner":
			owners++
		case "editor", "user":
		default:
			undecided = true
		}
	}

	if owners > 1 {
		resp.Diagnostics.AddAttributeError(
			req.Path,
			"Too many owners",
			fmt.Sprintf("A connection has exactly one owner, but %d collaborators have the role \"owner\".", owners),
		)
	}
	if owners == 0 && !undecided {
		resp.Diagnostics.AddAttributeError(
			req.Path,
			"Missing owner",
			"Exactly one collaborator must have the role \"owner\". List the user behind the API token as the owner, "+
				"otherwise that user cannot access the connection after it is created.",
		)
	}
}

// cloudflowConnectionGcpConfigValidator requires the ID that matches gcp_config.level.
type cloudflowConnectionGcpConfigValidator struct{}

var _ resource.ConfigValidator = cloudflowConnectionGcpConfigValidator{}

func (v cloudflowConnectionGcpConfigValidator) Description(_ context.Context) string {
	return "gcp_config requires organization_id, folder_id or project_id to match its level."
}

func (v cloudflowConnectionGcpConfigValidator) MarkdownDescription(ctx context.Context) string {
	return v.Description(ctx)
}

func (v cloudflowConnectionGcpConfigValidator) ValidateResource(ctx context.Context, req resource.ValidateConfigRequest, resp *resource.ValidateConfigResponse) {
	var gcp resource_cloudflow_connection.GcpConfigValue
	resp.Diagnostics.Append(req.Config.GetAttribute(ctx, path.Root("gcp_config"), &gcp)...)
	if resp.Diagnostics.HasError() {
		return
	}

	validateCloudflowConnectionGcpConfig(gcp, &resp.Diagnostics)
}

// validateCloudflowConnectionGcpConfig requires the ID that matches the level, and a
// role ID for a custom role. role_id cannot be declared Required in the schema, see Schema().
func validateCloudflowConnectionGcpConfig(gcp resource_cloudflow_connection.GcpConfigValue, diags *diag.Diagnostics) {
	if gcp.IsNull() || gcp.IsUnknown() {
		return
	}

	validateCloudflowConnectionCustomRole(gcp.CustomRole, diags)

	if gcp.Level.IsNull() || gcp.Level.IsUnknown() {
		return
	}

	switch level := gcp.Level.ValueString(); level {
	case "organization":
		cloudflowConnectionRequireGcpID(diags, level, "organization_id", gcp.OrganizationId)
	case "folder":
		cloudflowConnectionRequireGcpID(diags, level, "folder_id", gcp.FolderId)
	case "project":
		cloudflowConnectionRequireGcpID(diags, level, "project_id", gcp.ProjectId)
	}
}

// validateCloudflowConnectionCustomRole requires role_id when a custom role is configured.
func validateCloudflowConnectionCustomRole(customRole resource_cloudflow_connection.CustomRoleValue, diags *diag.Diagnostics) {
	if customRole.IsNull() || customRole.IsUnknown() {
		return
	}
	if customRole.RoleId.IsUnknown() {
		return
	}
	if customRole.RoleId.IsNull() || customRole.RoleId.ValueString() == "" {
		diags.AddAttributeError(
			path.Root("gcp_config").AtName("custom_role").AtName("role_id"),
			"Missing custom role ID",
			"role_id is required when custom_role is set.",
		)
	}
}

// cloudflowConnectionRequireGcpID reports a missing ID. It defers to apply time
// while the value is unknown.
func cloudflowConnectionRequireGcpID(diags *diag.Diagnostics, level, name string, value types.String) {
	if value.IsUnknown() {
		return
	}
	if value.IsNull() || value.ValueString() == "" {
		diags.AddAttributeError(
			path.Root("gcp_config").AtName(name),
			"Missing GCP ID",
			fmt.Sprintf("%s is required when gcp_config.level is %q.", name, level),
		)
	}
}
