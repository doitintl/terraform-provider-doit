package provider

import (
	"context"
	"fmt"

	"github.com/doitintl/terraform-provider-doit/internal/provider/resource_resource_sharing"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// resourceSharingOwnerValidator validates that exactly one owner is defined in permissions.
type resourceSharingOwnerValidator struct{}

var _ resource.ConfigValidator = resourceSharingOwnerValidator{}

func (v resourceSharingOwnerValidator) Description(_ context.Context) string {
	return "Validates that the permissions list contains exactly one entry with role = 'owner'."
}

func (v resourceSharingOwnerValidator) MarkdownDescription(ctx context.Context) string {
	return v.Description(ctx)
}

func (v resourceSharingOwnerValidator) ValidateResource(ctx context.Context, req resource.ValidateConfigRequest, resp *resource.ValidateConfigResponse) {
	var permissions types.List
	resp.Diagnostics.Append(req.Config.GetAttribute(ctx, path.Root("permissions"), &permissions)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Skip validation when permissions is null or unknown (e.g., during import planning)
	if permissions.IsNull() || permissions.IsUnknown() {
		return
	}

	var permVals []resource_resource_sharing.PermissionsValue
	resp.Diagnostics.Append(permissions.ElementsAs(ctx, &permVals, false)...)
	if resp.Diagnostics.HasError() {
		return
	}

	ownerCount := 0
	unknownCount := 0
	for _, pv := range permVals {
		if pv.IsNull() || pv.IsUnknown() || pv.Role.IsNull() || pv.Role.IsUnknown() {
			unknownCount++
			continue
		}
		if pv.Role.ValueString() == "owner" {
			ownerCount++
		}
	}

	// If any roles are unknown (e.g. from variables), skip validation — one of them
	// might resolve to "owner" at apply time.
	if ownerCount == 0 && unknownCount == 0 {
		resp.Diagnostics.AddAttributeError(
			path.Root("permissions"),
			"Missing Owner",
			"The permissions list must contain exactly one entry with role = 'owner'. "+
				"Every shared resource must have an owner.",
		)
	} else if ownerCount > 1 {
		resp.Diagnostics.AddAttributeError(
			path.Root("permissions"),
			"Multiple Owners",
			fmt.Sprintf("The permissions list must contain exactly one entry with role = 'owner', but %d were found. "+
				"A shared resource can only have a single owner.", ownerCount),
		)
	}
}
