// Package provider implements the DoiT Terraform provider.
package provider

import (
	"time"

	"github.com/doitintl/terraform-provider-doit/internal/provider/models"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

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
