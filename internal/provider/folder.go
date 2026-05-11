// Package provider implements the DoiT Terraform provider.
package provider

import (
	"github.com/doitintl/terraform-provider-doit/internal/provider/models"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// mapFolderToModel maps the API response to the Terraform model.
// Used by Read and ImportState (full mapping from API response).
func mapFolderToModel(resp *models.Folder, state *folderResourceModel) {
	if resp.Id != nil {
		state.Id = types.StringValue(*resp.Id)
	} else {
		state.Id = types.StringNull()
	}

	if resp.Name != nil {
		state.Name = types.StringValue(*resp.Name)
	} else {
		state.Name = types.StringNull()
	}

	if resp.Description != nil && *resp.Description != "" {
		state.Description = types.StringValue(*resp.Description)
	} else {
		state.Description = types.StringNull()
	}

	if resp.ParentFolderId != nil {
		state.ParentFolderId = types.StringValue(*resp.ParentFolderId)
	} else {
		// API always returns parentFolderId; if somehow nil, default to "root"
		state.ParentFolderId = types.StringValue("root")
	}
}

// overlayFolderComputedFields uses the two-phase overlay pattern to reconcile
// the Terraform plan with the API response after Create/Update.
//
// Field classification:
//   - id:               Computed-only — always set from API response
//   - name:             Required — never touch
//   - description:      Optional+Computed — resolve when unknown
//   - parent_folder_id: Optional+Computed — resolve when unknown
func overlayFolderComputedFields(apiResp *models.Folder, plan *folderResourceModel) {
	// Phase 1: Build fully-resolved state from API response.
	resolved := *plan
	mapFolderToModel(apiResp, &resolved)

	// Phase 2: Overlay Computed-only fields.
	plan.Id = resolved.Id

	// Phase 3: Resolve Optional+Computed fields only when unknown.
	if plan.Description.IsUnknown() {
		plan.Description = resolved.Description
	}
	if plan.ParentFolderId.IsUnknown() {
		plan.ParentFolderId = resolved.ParentFolderId
	}

	// name: Required — never touch.
}
