package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// allocationValueExtractionValidator validates that value_extraction requires fallback
// when on_missing is 'useFallback' (the default) and forbids fallback when on_missing is 'nextRule'.
var _ validator.Object = allocationValueExtractionValidator{}

type allocationValueExtractionValidator struct{}

func (v allocationValueExtractionValidator) Description(_ context.Context) string {
	return "validates that value_extraction requires fallback when on_missing is 'useFallback' and forbids fallback when on_missing is 'nextRule'"
}

func (v allocationValueExtractionValidator) MarkdownDescription(ctx context.Context) string {
	return v.Description(ctx)
}

func (v allocationValueExtractionValidator) ValidateObject(ctx context.Context, req validator.ObjectRequest, resp *validator.ObjectResponse) {
	if req.ConfigValue.IsNull() || req.ConfigValue.IsUnknown() {
		return
	}

	attrs := req.ConfigValue.Attributes()
	onMissingAttr, hasOnMissing := attrs["on_missing"]
	fallbackAttr, hasFallback := attrs["fallback"]
	if !hasOnMissing || !hasFallback {
		return
	}

	onMissing, okOnMissing := onMissingAttr.(types.String)
	fallback, okFallback := fallbackAttr.(types.String)
	if !okOnMissing || !okFallback {
		return
	}

	if onMissing.IsUnknown() || fallback.IsUnknown() {
		return
	}

	onMissingMode := "useFallback"
	if !onMissing.IsNull() {
		onMissingMode = onMissing.ValueString()
	}

	switch onMissingMode {
	case "useFallback":
		if fallback.IsNull() || fallback.ValueString() == "" {
			resp.Diagnostics.AddAttributeError(
				req.Path.AtName("fallback"),
				"Invalid Attribute Configuration",
				"fallback is required when on_missing is 'useFallback'",
			)
		}
	case "nextRule":
		if !fallback.IsNull() {
			resp.Diagnostics.AddAttributeError(
				req.Path.AtName("fallback"),
				"Invalid Attribute Configuration",
				"fallback is not allowed when on_missing is 'nextRule'",
			)
		}
	}
}
