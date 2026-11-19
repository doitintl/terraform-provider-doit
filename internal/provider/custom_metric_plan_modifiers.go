package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// customMetricVariableLetters are the letters the API assigns to the variables,
// in list order.
const customMetricVariableLetters = "ABCDE"

// useNullForUnknownCustomMetricDecimalsWhenConfigNull proposes null for decimals
// when it is omitted from configuration (config is null). This makes omitting
// the block clear it in the API via req.Decimals.SetNull().
func useNullForUnknownCustomMetricDecimalsWhenConfigNull() planmodifier.Object {
	return useNullForUnknownCustomMetricDecimalsWhenConfigNullModifier{}
}

type useNullForUnknownCustomMetricDecimalsWhenConfigNullModifier struct{}

func (m useNullForUnknownCustomMetricDecimalsWhenConfigNullModifier) Description(_ context.Context) string {
	return "Proposes null when decimals is null so that omitting the block clears it."
}

func (m useNullForUnknownCustomMetricDecimalsWhenConfigNullModifier) MarkdownDescription(ctx context.Context) string {
	return m.Description(ctx)
}

func (m useNullForUnknownCustomMetricDecimalsWhenConfigNullModifier) PlanModifyObject(_ context.Context, req planmodifier.ObjectRequest, resp *planmodifier.ObjectResponse) {
	if req.ConfigValue.IsNull() {
		resp.PlanValue = req.ConfigValue
	}
}

// customMetricVariableLetterFromPosition plans variables[*].letter from the
// variable's position: the API assigns A to the first variable, B to the
// second, up to E. The letter is known at plan time, so changing the variables
// never shows it as "(known after apply)".
//
// The framework (v1.19.0, internal/fwserver/attribute_plan_modification.go)
// runs the modifiers of a list element's attributes with the element path
// req.AttributePath.AtListIndex(idx), so the index is the second-to-last step
// of req.Path.
func customMetricVariableLetterFromPosition() planmodifier.String {
	return customMetricVariableLetterFromPositionModifier{}
}

type customMetricVariableLetterFromPositionModifier struct{}

func (m customMetricVariableLetterFromPositionModifier) Description(_ context.Context) string {
	return "Plans the letter from the variable's position in the list: A for the first, up to E."
}

func (m customMetricVariableLetterFromPositionModifier) MarkdownDescription(ctx context.Context) string {
	return m.Description(ctx)
}

func (m customMetricVariableLetterFromPositionModifier) PlanModifyString(_ context.Context, req planmodifier.StringRequest, resp *planmodifier.StringResponse) {
	step, _ := req.Path.ParentPath().Steps().LastStep()
	index, ok := step.(path.PathStepElementKeyInt)
	if !ok || index < 0 || int(index) >= len(customMetricVariableLetters) {
		// More variables than letters: the list validator rejects the
		// configuration, so leave the plan to it.
		return
	}
	resp.PlanValue = types.StringValue(string(customMetricVariableLetters[index]))
}
