package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var _ planmodifier.String = customerIDMatchesStateModifier{}

// customerIDMatchesState returns a plan modifier that validates at plan time that
// the customer_id in configuration matches the existing customer ID in state.
// If the configuration specifies a different customer ID, it rejects the plan with
// a diagnostic error rather than attempting resource replacement or mutation.
func customerIDMatchesState() planmodifier.String {
	return customerIDMatchesStateModifier{}
}

type customerIDMatchesStateModifier struct{}

func (m customerIDMatchesStateModifier) Description(_ context.Context) string {
	return "Validates that customer_id in configuration matches the existing customer ID in state."
}

func (m customerIDMatchesStateModifier) MarkdownDescription(ctx context.Context) string {
	return m.Description(ctx)
}

func (m customerIDMatchesStateModifier) PlanModifyString(ctx context.Context, req planmodifier.StringRequest, resp *planmodifier.StringResponse) {
	// If there is no prior state (e.g. create before import), nothing to compare against.
	if req.State.Raw.IsNull() {
		return
	}

	// Determine existing customer ID from state.
	stateVal := req.StateValue
	if stateVal.IsNull() || stateVal.IsUnknown() {
		var stateID types.String
		diags := req.State.GetAttribute(ctx, path.Root("id"), &stateID)
		resp.Diagnostics.Append(diags...)
		if resp.Diagnostics.HasError() {
			return
		}
		if !stateID.IsNull() && !stateID.IsUnknown() {
			stateVal = stateID
		}
	}

	if stateVal.IsNull() || stateVal.IsUnknown() {
		return
	}

	// If config is omitted/null or unknown, UseStateForUnknown preserves the state value.
	if req.ConfigValue.IsNull() || req.ConfigValue.IsUnknown() {
		return
	}

	// If config value is provided and does not match the existing customer ID in state:
	if !req.ConfigValue.Equal(stateVal) {
		resp.Diagnostics.AddAttributeError(
			req.Path,
			"Invalid Customer ID",
			fmt.Sprintf(
				"customer_id %q does not match the imported customer ID %q. "+
					"The doit_customer resource cannot be migrated to a different customer in-place. "+
					"To manage a different customer, import it into a separate resource or remove this resource from state.",
				req.ConfigValue.ValueString(),
				stateVal.ValueString(),
			),
		)
	}
}
