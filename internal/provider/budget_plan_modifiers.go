package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
)

// useNullForOmittedBudgetRollover proposes null for rollover when the
// configuration omits it, so omitting the block disables rollover: the update
// request sends enabled = false for a null plan (toUpdateRequest).
//
// Terraform proposes null for a removed block already, because its leaves are
// Required, but the framework then marks every Computed attribute that the
// configuration leaves null as unknown (MarkComputedNilsAsUnknown, v1.19.0),
// and an unknown rollover is left out of the request, so the stored rollover
// would survive. Planning null also on create and when nothing is stored keeps
// the plan free of an unknown rollover for the budgets that do not use it; the
// read path keeps the disabled rollover the API returns for them out of state
// (mapBudgetRollover).
func useNullForOmittedBudgetRollover() planmodifier.Object {
	return useNullForOmittedBudgetRolloverModifier{}
}

type useNullForOmittedBudgetRolloverModifier struct{}

func (m useNullForOmittedBudgetRolloverModifier) Description(_ context.Context) string {
	return "Proposes null when rollover is omitted from configuration, so omitting it disables rollover."
}

func (m useNullForOmittedBudgetRolloverModifier) MarkdownDescription(ctx context.Context) string {
	return m.Description(ctx)
}

func (m useNullForOmittedBudgetRolloverModifier) PlanModifyObject(_ context.Context, req planmodifier.ObjectRequest, resp *planmodifier.ObjectResponse) {
	if req.ConfigValue.IsNull() {
		resp.PlanValue = req.ConfigValue
	}
}
