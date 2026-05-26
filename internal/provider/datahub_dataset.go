// Package provider implements the DoiT Terraform provider.
package provider

import (
	"context"
	"fmt"

	"github.com/doitintl/terraform-provider-doit/internal/provider/models"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// populateState fetches the dataset from the API and populates the Terraform state.
// On 404, state.Name is set to null to signal Terraform to remove the resource from state.
func (r *datahubDatasetResource) populateState(ctx context.Context, state *datahubDatasetResourceModel) diag.Diagnostics {
	datasetResp, err := r.client.GetDatahubDatasetWithResponse(ctx, state.Name.ValueString())
	if err != nil {
		return diag.Diagnostics{
			diag.NewErrorDiagnostic("Error Reading DataHub Dataset", "Could not read dataset "+state.Name.ValueString()+": "+err.Error()),
		}
	}

	if datasetResp.StatusCode() == 404 {
		state.Name = types.StringNull()
		return nil
	}

	if datasetResp.StatusCode() != 200 {
		return diag.Diagnostics{
			diag.NewErrorDiagnostic("Error Reading DataHub Dataset", fmt.Sprintf("Unexpected status code %d for dataset %s: %s", datasetResp.StatusCode(), state.Name.ValueString(), string(datasetResp.Body))),
		}
	}

	if datasetResp.JSON200 == nil {
		return diag.Diagnostics{
			diag.NewErrorDiagnostic("Error Reading DataHub Dataset", "Received empty response body for dataset "+state.Name.ValueString()),
		}
	}

	mapDatahubDatasetToModel(datasetResp.JSON200.Name, datasetResp.JSON200.Description, datasetResp.JSON200.Records, datasetResp.JSON200.UpdatedBy, datasetResp.JSON200.LastUpdated, state)
	return nil
}

// mapDatahubDatasetToModel maps the API response to the Terraform model.
func mapDatahubDatasetToModel(name, description *string, records *int64, updatedBy, lastUpdated *string, state *datahubDatasetResourceModel) {
	state.Name = types.StringPointerValue(name)
	state.Description = types.StringPointerValue(description)
	state.Records = types.Int64PointerValue(records)
	state.UpdatedBy = types.StringPointerValue(updatedBy)
	state.LastUpdated = types.StringPointerValue(lastUpdated)
}

// overlayDatahubDatasetComputedFields uses the two-phase overlay pattern to
// reconcile the Terraform plan with the API response after Create/Update.
func overlayDatahubDatasetComputedFields(name, description *string, records *int64, updatedBy, lastUpdated *string, plan *datahubDatasetResourceModel) {
	// Phase 1: Build fully-resolved state from API response.
	resolved := *plan
	mapDatahubDatasetToModel(name, description, records, updatedBy, lastUpdated, &resolved)

	// Phase 2: Overlay.
	// Name: Required — never touch.
	// Records, UpdatedBy, LastUpdated: Computed-only — always from resolved.
	plan.Records = resolved.Records
	plan.UpdatedBy = resolved.UpdatedBy
	plan.LastUpdated = resolved.LastUpdated

	// Description: Optional+Computed — resolve when Unknown.
	if plan.Description.IsUnknown() {
		plan.Description = resolved.Description
	}
}

// toCreateRequest converts the TF model to a CreateDatahubDatasetRequestBody.
func (plan *datahubDatasetResourceModel) toCreateRequest() models.CreateDatahubDatasetRequestBody {
	req := models.CreateDatahubDatasetRequestBody{
		Name: plan.Name.ValueString(),
	}
	if !plan.Description.IsNull() && !plan.Description.IsUnknown() {
		req.Description = new(plan.Description.ValueString())
	}
	return req
}

// toUpdateRequest converts the TF model to an UpdateDatahubDatasetRequestBody.
func (plan *datahubDatasetResourceModel) toUpdateRequest() models.UpdateDatahubDatasetRequestBody {
	req := models.UpdateDatahubDatasetRequestBody{}
	if !plan.Description.IsNull() && !plan.Description.IsUnknown() {
		req.Description = new(plan.Description.ValueString())
	}
	return req
}
