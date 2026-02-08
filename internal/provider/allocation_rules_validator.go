package provider

import (
	"context"
	"fmt"

	"github.com/doitintl/terraform-provider-doit/internal/provider/resource_allocation"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
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

	// Block empty rules lists - empty rules work but cause drift because API returns
	// rules as null. The provider handles this correctly (returns empty list), but
	// blocking this guides users toward the cleaner pattern of omitting the attribute.
	if len(elements) == 0 {
		resp.Diagnostics.AddAttributeError(
			req.Path,
			"Invalid Rules Configuration",
			"Allocation rules cannot be empty. Specify at least one rule or omit the attribute.",
		)
		return
	}

	for i, elem := range elements {
		// The generated schema uses resource_allocation.RulesValue
		ruleVal, ok := elem.(resource_allocation.RulesValue)
		if !ok {
			continue
		}

		// Skip if the element is null or unknown
		if ruleVal.IsNull() || ruleVal.IsUnknown() {
			continue
		}

		// Get the action value
		if ruleVal.Action.IsNull() || ruleVal.Action.IsUnknown() {
			continue
		}

		action := ruleVal.Action.ValueString()

		// For "create" or "update" actions, name is required
		if action == "create" || action == "update" {
			if ruleVal.Name.IsNull() || ruleVal.Name.IsUnknown() || ruleVal.Name.ValueString() == "" {
				resp.Diagnostics.AddAttributeError(
					req.Path.AtListIndex(i).AtName("name"),
					"Missing Required Attribute",
					fmt.Sprintf("rules[%d]: 'name' is required when action is '%s'", i, action),
				)
			}
		}
	}
}
