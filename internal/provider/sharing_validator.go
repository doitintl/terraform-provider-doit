package provider

import (
	"context"
	"fmt"

	"github.com/doitintl/terraform-provider-doit/internal/provider/resource_sharing"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// sharingOwnerValidator validates that exactly one owner is defined in permissions.
type sharingOwnerValidator struct{}

var _ resource.ConfigValidator = sharingOwnerValidator{}

func (v sharingOwnerValidator) Description(_ context.Context) string {
	return "Validates that the permissions list contains exactly one entry with role = 'owner'."
}

func (v sharingOwnerValidator) MarkdownDescription(ctx context.Context) string {
	return v.Description(ctx)
}

func (v sharingOwnerValidator) ValidateResource(ctx context.Context, req resource.ValidateConfigRequest, resp *resource.ValidateConfigResponse) {
	var permissions types.List
	resp.Diagnostics.Append(req.Config.GetAttribute(ctx, path.Root("permissions"), &permissions)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Skip validation when permissions is null or unknown (e.g., during import planning)
	if permissions.IsNull() || permissions.IsUnknown() {
		return
	}

	var permVals []resource_sharing.PermissionsValue
	resp.Diagnostics.Append(permissions.ElementsAs(ctx, &permVals, false)...)
	if resp.Diagnostics.HasError() {
		return
	}

	ownerCount := 0
	unknownCount := 0
	for _, pv := range permVals {
		if pv.IsNull() || pv.IsUnknown() {
			unknownCount++
			continue
		}
		// Only truly unknown roles (e.g. from variables not yet resolved) should
		// defer validation. A null role is a config error, not an unknown.
		if pv.Role.IsUnknown() {
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

// sharingAllocationPublicValidator rejects setting "public" when
// resource_type is "allocations". The allocation API silently discards any
// public access role (even "viewer" is accepted but stored as ""), so allowing
// users to set it would always cause drift on the next plan.
type sharingAllocationPublicValidator struct{}

var _ resource.ConfigValidator = sharingAllocationPublicValidator{}

func (v sharingAllocationPublicValidator) Description(_ context.Context) string {
	return "Validates that 'public' is not set when resource_type is 'allocations'."
}

func (v sharingAllocationPublicValidator) MarkdownDescription(ctx context.Context) string {
	return v.Description(ctx)
}

func (v sharingAllocationPublicValidator) ValidateResource(ctx context.Context, req resource.ValidateConfigRequest, resp *resource.ValidateConfigResponse) {
	var resourceType types.String
	resp.Diagnostics.Append(req.Config.GetAttribute(ctx, path.Root("resource_type"), &resourceType)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Can't validate if resource_type is not yet known.
	if resourceType.IsNull() || resourceType.IsUnknown() {
		return
	}

	if resourceType.ValueString() != "allocations" {
		return
	}

	var public types.String
	resp.Diagnostics.Append(req.Config.GetAttribute(ctx, path.Root("public"), &public)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Null = not set in config → that's fine, we handle it internally.
	if public.IsNull() || public.IsUnknown() {
		return
	}

	resp.Diagnostics.AddAttributeError(
		path.Root("public"),
		"Public Access Not Supported for Allocations",
		"The 'public' attribute cannot be set when resource_type is \"allocations\". "+
			"The allocation API does not support public access roles — any value is silently discarded, "+
			"which would cause perpetual drift. Remove the 'public' attribute from your configuration.",
	)
}
