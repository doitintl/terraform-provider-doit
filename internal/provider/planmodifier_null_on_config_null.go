package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
)

// useNullForUnknownWhenConfigNull returns a plan modifier that proposes null when
// the config value is null (either explicitly set or omitted).
//
// This is needed for the "public" attribute because the sharing API uses full-replacement
// PUT semantics: omitting the "public" field from the request body clears public access.
// Therefore, when the user omits "public" from their config, the planned value must be
// null so that the provider correctly omits it from the API request, which in turn clears
// any previously-set public access.
//
// Without this modifier, Terraform's default Optional+Computed behavior would preserve
// the prior state value when config is null, causing the provider to re-send the old
// public value and preventing the user from clearing public access.
//
// Note: Because Terraform does not distinguish between "attribute omitted" and
// "attribute explicitly set to null", both cases result in public access being cleared.
// This is consistent with the API's full-replacement semantics.
//
// See: https://github.com/hashicorp/terraform-plugin-framework/issues/603
func useNullForUnknownWhenConfigNull() planmodifier.String {
	return useNullForUnknownWhenConfigNullModifier{}
}

type useNullForUnknownWhenConfigNullModifier struct{}

func (m useNullForUnknownWhenConfigNullModifier) Description(_ context.Context) string {
	return "Proposes null when the config value is null (omitted or explicitly set), " +
		"allowing the attribute to be cleared. This matches the API's full-replacement PUT semantics."
}

func (m useNullForUnknownWhenConfigNullModifier) MarkdownDescription(ctx context.Context) string {
	return m.Description(ctx)
}

func (m useNullForUnknownWhenConfigNullModifier) PlanModifyString(_ context.Context, req planmodifier.StringRequest, resp *planmodifier.StringResponse) {
	// If the config value is null (user explicitly set null or omitted the field),
	// and there IS a prior state value, propose null to allow clearing.
	// This overrides the default Optional+Computed behavior of preserving prior state.
	//
	// This is correct because the API uses full-replacement PUT: omitting "public"
	// from the request body clears public access. So the plan must reflect that.
	if req.ConfigValue.IsNull() && !req.StateValue.IsNull() {
		resp.PlanValue = req.ConfigValue // null
	}
}
