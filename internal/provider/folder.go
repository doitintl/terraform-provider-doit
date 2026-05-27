// Package provider implements the DoiT Terraform provider.
package provider

import (
	"context"
	"fmt"

	"github.com/doitintl/terraform-provider-doit/internal/provider/models"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// populateState fetches the folder from the API and populates the Terraform state.
// On 404, state.Id is set to null to signal Terraform to remove the resource from state.
func (r *folderResource) populateState(ctx context.Context, state *folderResourceModel) diag.Diagnostics {
	folderResp, err := r.client.GetFolderWithResponse(ctx, state.Id.ValueString())
	if err != nil {
		return diag.Diagnostics{
			diag.NewErrorDiagnostic("Error Reading Folder", "Could not read folder ID "+state.Id.ValueString()+": "+err.Error()),
		}
	}

	if folderResp.StatusCode() == 404 {
		state.Id = types.StringNull()
		return nil
	}

	if folderResp.StatusCode() != 200 {
		return diag.Diagnostics{
			diag.NewErrorDiagnostic("Error Reading Folder", fmt.Sprintf("Unexpected status code %d for folder ID %s: %s", folderResp.StatusCode(), state.Id.ValueString(), string(folderResp.Body))),
		}
	}

	if folderResp.JSON200 == nil {
		return diag.Diagnostics{
			diag.NewErrorDiagnostic("Error Reading Folder", "Received empty response body for folder ID "+state.Id.ValueString()),
		}
	}

	mapFolderToModel(folderResp.JSON200, state)
	return nil
}

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

	if resp.Description != nil {
		state.Description = types.StringValue(*resp.Description)
	} else {
		state.Description = types.StringNull()
	}

	// Defend against API returning nil: fall back to "root" to match the
	// schema default and prevent perpetual plan drift.
	if resp.ParentFolderId != nil {
		state.ParentFolderId = types.StringValue(*resp.ParentFolderId)
	} else {
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
//   - parent_folder_id: Optional+Computed — has schema default, never Unknown
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
	// parent_folder_id: has schema default "root" — never Unknown at plan time.

	// name: Required — never touch.
}

// toCreateRequest converts the TF model to a CreateFolderRequest.
func (plan *folderResourceModel) toCreateRequest() models.CreateFolderRequest {
	req := models.CreateFolderRequest{
		Name: plan.Name.ValueString(),
	}

	req.ParentFolderId = plan.ParentFolderId.ValueStringPointer()

	if !plan.Description.IsNull() && !plan.Description.IsUnknown() {
		req.Description = new(plan.Description.ValueString())
	}

	return req
}

// toUpdateRequest converts the TF model to an UpdateFolderRequest.
func (plan *folderResourceModel) toUpdateRequest() models.UpdateFolderRequest {
	req := models.UpdateFolderRequest{
		Name: new(plan.Name.ValueString()),
	}

	req.ParentFolderId = plan.ParentFolderId.ValueStringPointer()

	if !plan.Description.IsNull() && !plan.Description.IsUnknown() {
		req.Description = new(plan.Description.ValueString())
	}

	return req
}
