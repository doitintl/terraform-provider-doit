package provider

import (
	"context"
	"fmt"

	"github.com/doitintl/terraform-provider-doit/internal/provider/resource_allocation"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
)

// allocationComponentsValidator validates constraints on allocation rule components.
//
// When a component has type="allocation_rule", the following constraints apply:
//   - key must be set to "allocation_rule"
//   - mode must be either "is" or "contains"
//
// These constraints are documented in the upstream API but not enforced by the
// OpenAPI schema. This validator provides early plan-time feedback instead of
// deferring to apply-time API errors.
var _ validator.List = allocationComponentsValidator{}

type allocationComponentsValidator struct{}

func (v allocationComponentsValidator) Description(_ context.Context) string {
	return "validates that allocation_rule components have key='allocation_rule' and mode is 'is' or 'contains'"
}

func (v allocationComponentsValidator) MarkdownDescription(ctx context.Context) string {
	return v.Description(ctx)
}

func (v allocationComponentsValidator) ValidateList(ctx context.Context, req validator.ListRequest, resp *validator.ListResponse) {
	if req.ConfigValue.IsNull() || req.ConfigValue.IsUnknown() {
		return
	}

	for i, elem := range req.ConfigValue.Elements() {
		compVal, ok := elem.(resource_allocation.ComponentsValue)
		if !ok {
			continue
		}

		if compVal.IsNull() || compVal.IsUnknown() {
			continue
		}

		if compVal.ComponentsType.IsNull() || compVal.ComponentsType.IsUnknown() {
			continue
		}

		if compVal.ComponentsType.ValueString() != "allocation_rule" {
			continue
		}

		// Validate key must be "allocation_rule"
		if !compVal.Key.IsNull() && !compVal.Key.IsUnknown() {
			if compVal.Key.ValueString() != "allocation_rule" {
				resp.Diagnostics.AddAttributeError(
					req.Path.AtListIndex(i).AtName("key"),
					"Invalid Allocation Rule Component",
					fmt.Sprintf("components[%d]: when type is 'allocation_rule', key must be 'allocation_rule', got '%s'", i, compVal.Key.ValueString()),
				)
			}
		}

		// Validate mode must be "is" or "contains"
		if !compVal.Mode.IsNull() && !compVal.Mode.IsUnknown() {
			mode := compVal.Mode.ValueString()
			if mode != "is" && mode != "contains" {
				resp.Diagnostics.AddAttributeError(
					req.Path.AtListIndex(i).AtName("mode"),
					"Invalid Allocation Rule Component",
					fmt.Sprintf("components[%d]: when type is 'allocation_rule', mode must be 'is' or 'contains', got '%s'", i, mode),
				)
			}
		}
	}
}
