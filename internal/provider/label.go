// Package provider implements the DoiT Terraform provider.
package provider

import (
	"context"
	"fmt"
	"time"

	"github.com/doitintl/terraform-provider-doit/internal/provider/models"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// populateState fetches the label from the API and populates the Terraform state.
// On 404, state.Id is set to null to signal Terraform to remove the resource from state.
func (r *labelResource) populateState(ctx context.Context, state *labelResourceModel) diag.Diagnostics {
	labelResp, err := r.client.GetLabelWithResponse(ctx, state.Id.ValueString())
	if err != nil {
		return diag.Diagnostics{
			diag.NewErrorDiagnostic("Error Reading Label", "Could not read label ID "+state.Id.ValueString()+": "+err.Error()),
		}
	}

	if labelResp.StatusCode() == 404 {
		state.Id = types.StringNull()
		return nil
	}

	if labelResp.StatusCode() != 200 {
		return diag.Diagnostics{
			diag.NewErrorDiagnostic("Error Reading Label", fmt.Sprintf("Unexpected status code %d for label ID %s: %s", labelResp.StatusCode(), state.Id.ValueString(), string(labelResp.Body))),
		}
	}

	if labelResp.JSON200 == nil {
		return diag.Diagnostics{
			diag.NewErrorDiagnostic("Error Reading Label", "Received empty response body for label ID "+state.Id.ValueString()),
		}
	}

	mapLabelToModel(labelResp.JSON200, state)
	return nil
}

// mapLabelToModel maps the API response to the Terraform model.
func mapLabelToModel(resp *models.LabelListItem, state *labelResourceModel) {
	state.Id = types.StringValue(resp.Id)
	state.Name = types.StringValue(resp.Name)
	state.Color = types.StringValue(string(resp.Color))

	if resp.Type != nil {
		state.Type = types.StringValue(string(*resp.Type))
	} else {
		state.Type = types.StringNull()
	}

	if resp.CreateTime != nil {
		state.CreateTime = types.StringValue(resp.CreateTime.Format(time.RFC3339))
	} else {
		state.CreateTime = types.StringNull()
	}

	if resp.UpdateTime != nil {
		state.UpdateTime = types.StringValue(resp.UpdateTime.Format(time.RFC3339))
	} else {
		state.UpdateTime = types.StringNull()
	}
}

// overlayLabelComputedFields uses the two-phase overlay pattern to reconcile
// the Terraform plan with the API response after Create/Update.
//
// Label has no Optional+Computed fields — all non-Required fields are
// Computed-only — so the overlay simply sets Computed fields from the
// resolved model while preserving the plan's Required fields.
func overlayLabelComputedFields(apiResp *models.LabelListItem, plan *labelResourceModel) {
	// Phase 1: Build fully-resolved state from API response.
	resolved := *plan
	mapLabelToModel(apiResp, &resolved)

	// Phase 2: Overlay Computed-only fields.
	plan.Id = resolved.Id
	plan.Type = resolved.Type
	plan.CreateTime = resolved.CreateTime
	plan.UpdateTime = resolved.UpdateTime

	// Name, Color: Required — never touch.
}

// toCreateRequest converts the TF model to a CreateLabelRequest.
func (plan *labelResourceModel) toCreateRequest() models.CreateLabelRequest {
	return models.CreateLabelRequest{
		Color: models.CreateLabelRequestColor(plan.Color.ValueString()),
		Name:  plan.Name.ValueString(),
	}
}

// toUpdateRequest converts the TF model to an UpdateLabelRequest.
func (plan *labelResourceModel) toUpdateRequest() models.UpdateLabelRequest {
	return models.UpdateLabelRequest{
		Color: pointerToNullable(new(models.UpdateLabelRequestColor(plan.Color.ValueString()))),
		Name:  pointerToNullable(new(plan.Name.ValueString())),
	}
}
