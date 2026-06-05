package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// statussheetComponentIDsValidator validates that at least one non-empty
// component ID list is provided. The statussheet endpoint's AccessGuard
// requires at least one component ID in the request body for non-employee
// users; sending an empty body or only empty arrays returns 403.
type statussheetComponentIDsValidator struct{}

var _ datasource.ConfigValidator = statussheetComponentIDsValidator{}

func (v statussheetComponentIDsValidator) Description(_ context.Context) string {
	return "Validates that at least one non-empty component ID list is provided"
}

func (v statussheetComponentIDsValidator) MarkdownDescription(_ context.Context) string {
	return "Validates that at least one of `node_ids`, `element_ids`, `group_ids`, `link_ids`, " +
		"`attachment_ids`, `combiner_ids`, or `note_ids` is provided and non-empty"
}

func (v statussheetComponentIDsValidator) ValidateDataSource(ctx context.Context, req datasource.ValidateConfigRequest, resp *datasource.ValidateConfigResponse) {
	idAttrs := []string{
		"node_ids",
		"element_ids",
		"group_ids",
		"link_ids",
		"attachment_ids",
		"combiner_ids",
		"note_ids",
	}

	for _, attrName := range idAttrs {
		var list types.List

		diags := req.Config.GetAttribute(ctx, path.Root(attrName), &list)
		resp.Diagnostics.Append(diags...)
		if resp.Diagnostics.HasError() {
			return
		}

		// If any list is unknown, skip validation — its value will be
		// resolved at apply time (e.g. from another data source).
		if list.IsUnknown() {
			return
		}

		// A non-null list with at least one element satisfies the requirement.
		if !list.IsNull() && len(list.Elements()) > 0 {
			return
		}
	}

	resp.Diagnostics.AddError(
		"Missing Component IDs",
		"At least one non-empty component ID list (node_ids, element_ids, group_ids, link_ids, "+
			"attachment_ids, combiner_ids, or note_ids) must be provided. "+
			"Use doit_cloud_diagrams_schemes with layer_ids to discover component IDs.",
	)
}
