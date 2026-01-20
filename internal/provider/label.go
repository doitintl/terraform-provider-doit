// Package provider implements the DoiT Terraform provider.
package provider

import (
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
		state.CreateTime = types.StringValue(resp.CreateTime.Format("2006-01-02T15:04:05Z07:00"))
	} else {
		state.CreateTime = types.StringNull()
	}

	if resp.UpdateTime != nil {
		state.UpdateTime = types.StringValue(resp.UpdateTime.Format("2006-01-02T15:04:05Z07:00"))
	} else {
		state.UpdateTime = types.StringNull()
	}
}
