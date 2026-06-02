// Package provider implements the DoiT Terraform provider.
package provider

import (
	"context"
	"fmt"

	"github.com/doitintl/terraform-provider-doit/internal/provider/models"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// syntheticActiveThemeID is the fixed resource ID for this singleton resource.
const syntheticActiveThemeID = "active-theme"

// populateState fetches the active theme from the API and populates the Terraform state.
// On 404, state.Id is set to null to signal Terraform to remove the resource from state.
func (r *activeThemeResource) populateState(ctx context.Context, state *activeThemeResourceModel) diag.Diagnostics {
	themeResp, err := r.client.GetActiveThemeWithResponse(ctx)
	if err != nil {
		return diag.Diagnostics{
			diag.NewErrorDiagnostic("Error Reading Active Theme", "Could not read active theme: "+err.Error()),
		}
	}

	if themeResp.StatusCode() == 404 {
		state.Id = types.StringNull()
		return nil
	}

	if themeResp.StatusCode() != 200 {
		return diag.Diagnostics{
			diag.NewErrorDiagnostic("Error Reading Active Theme", fmt.Sprintf("Unexpected status code %d: %s", themeResp.StatusCode(), string(themeResp.Body))),
		}
	}

	if themeResp.JSON200 == nil {
		return diag.Diagnostics{
			diag.NewErrorDiagnostic("Error Reading Active Theme", "Received empty response body"),
		}
	}

	mapActiveThemeToModel(themeResp.JSON200, state)
	return nil
}

// mapActiveThemeToModel maps the API response to the Terraform model.
// Used by populateState (Read/ImportState) and as Phase 1 of overlay.
func mapActiveThemeToModel(resp *models.ActiveTheme, state *activeThemeResourceModel) {
	state.Id = types.StringValue(syntheticActiveThemeID)

	// Defend against API returning empty: fall back to "default" to match
	// the schema default and prevent perpetual plan drift.
	if resp.ThemeId != "" {
		state.ThemeId = types.StringValue(resp.ThemeId)
	} else {
		state.ThemeId = types.StringValue("default")
	}
}

// overlayActiveThemeComputedFields uses the two-phase overlay pattern to
// reconcile the Terraform plan with the API response after Create/Update.
//
// Field classification:
//   - id:        Computed-only — always set from synthetic value
//   - theme_id:  Optional+Computed with Default — never Unknown at plan time
func overlayActiveThemeComputedFields(apiResp *models.ActiveTheme, plan *activeThemeResourceModel) {
	// Phase 1: Build fully-resolved state from API response.
	resolved := *plan
	mapActiveThemeToModel(apiResp, &resolved)

	// Phase 2: Overlay Computed-only fields.
	plan.Id = resolved.Id

	// theme_id: Optional+Computed with Default "default" — never Unknown at plan time.
}

// toUpdateRequest converts the TF model to a SetActiveThemeRequest.
// Create and Update share the same PUT endpoint.
func (plan *activeThemeResourceModel) toUpdateRequest() models.SetActiveThemeRequest {
	req := models.SetActiveThemeRequest{
		ThemeId: plan.ThemeId.ValueString(),
	}

	return req
}
