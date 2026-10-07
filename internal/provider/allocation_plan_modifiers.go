package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
)

// useNullForUnknownValueExtraction proposes null for rules[*].value_extraction
// when the attribute is omitted from configuration (config is null).
// This enables omitting the block to clear it in the API via rule.ValueExtraction.SetNull().
func useNullForUnknownValueExtraction() planmodifier.Object {
	return useNullForUnknownValueExtractionModifier{}
}

type useNullForUnknownValueExtractionModifier struct{}

func (m useNullForUnknownValueExtractionModifier) Description(_ context.Context) string {
	return "Proposes null when config.value_extraction is null so that omitting the block clears it."
}

func (m useNullForUnknownValueExtractionModifier) MarkdownDescription(ctx context.Context) string {
	return m.Description(ctx)
}

func (m useNullForUnknownValueExtractionModifier) PlanModifyObject(_ context.Context, req planmodifier.ObjectRequest, resp *planmodifier.ObjectResponse) {
	if req.ConfigValue.IsNull() {
		resp.PlanValue = req.ConfigValue
	}
}
