package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// allocationRulesValidator validates that rules with action="create" or "update" have a name.
//
// NOTE: This validator exists as a workaround for an API/OpenAPI spec issue.
// The DoiT API requires the "name" field when action is "create" or "update",
// but this conditional requirement is not reflected in the OpenAPI specification
// (which marks "name" as optional). Until the spec is updated to properly document
// this requirement, this validator provides early feedback at plan time rather
// than failing at apply time with a cryptic API error.
//
// See: https://github.com/doitintl/terraform-provider-doit/issues/70 for tracking.
var _ validator.List = allocationRulesValidator{}

type allocationRulesValidator struct{}

func (v allocationRulesValidator) Description(_ context.Context) string {
	return "validates that allocation rules with action 'create' or 'update' have a name"
}

func (v allocationRulesValidator) MarkdownDescription(ctx context.Context) string {
	return v.Description(ctx)
}

func (v allocationRulesValidator) ValidateList(ctx context.Context, req validator.ListRequest, resp *validator.ListResponse) {
	if req.ConfigValue.IsNull() || req.ConfigValue.IsUnknown() {
		return
	}

	elements := req.ConfigValue.Elements()
	for i, elem := range elements {
		objVal, ok := elem.(types.Object)
		if !ok || objVal.IsNull() || objVal.IsUnknown() {
			continue
		}

		attrs := objVal.Attributes()

		// Get the action value
		actionAttr, hasAction := attrs["action"]
		if !hasAction {
			continue
		}
		actionVal, ok := actionAttr.(types.String)
		if !ok || actionVal.IsNull() || actionVal.IsUnknown() {
			continue
		}

		action := actionVal.ValueString()

		// For "create" or "update" actions, name is required
		if action == "create" || action == "update" {
			nameAttr, hasName := attrs["name"]
			if !hasName {
				resp.Diagnostics.AddAttributeError(
					req.Path.AtListIndex(i).AtName("name"),
					"Missing Required Attribute",
					fmt.Sprintf("rules[%d]: 'name' is required when action is '%s'", i, action),
				)
				continue
			}

			nameVal, ok := nameAttr.(types.String)
			if !ok {
				continue
			}

			if nameVal.IsNull() || nameVal.IsUnknown() || nameVal.ValueString() == "" {
				resp.Diagnostics.AddAttributeError(
					req.Path.AtListIndex(i).AtName("name"),
					"Missing Required Attribute",
					fmt.Sprintf("rules[%d]: 'name' is required when action is '%s'", i, action),
				)
			}
		}
	}
}
