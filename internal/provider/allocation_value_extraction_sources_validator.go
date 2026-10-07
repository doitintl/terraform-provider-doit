package provider

import (
	"context"
	"fmt"

	"github.com/doitintl/terraform-provider-doit/internal/provider/resource_allocation"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
)

// allocationValueExtractionSourcesValidator validates that value_extraction sources list does not contain null elements.
var _ validator.List = allocationValueExtractionSourcesValidator{}

type allocationValueExtractionSourcesValidator struct{}

func (v allocationValueExtractionSourcesValidator) Description(_ context.Context) string {
	return "validates that value_extraction sources list does not contain null elements"
}

func (v allocationValueExtractionSourcesValidator) MarkdownDescription(ctx context.Context) string {
	return v.Description(ctx)
}

func (v allocationValueExtractionSourcesValidator) ValidateList(ctx context.Context, req validator.ListRequest, resp *validator.ListResponse) {
	if req.ConfigValue.IsNull() || req.ConfigValue.IsUnknown() {
		return
	}

	elements := req.ConfigValue.Elements()
	for i, elem := range elements {
		sourceVal, ok := elem.(resource_allocation.SourcesValue)
		if !ok {
			continue
		}

		if sourceVal.IsUnknown() {
			continue
		}

		if sourceVal.IsNull() {
			resp.Diagnostics.AddAttributeError(
				req.Path.AtListIndex(i),
				"Invalid Attribute Value",
				fmt.Sprintf("sources[%d]: null element is not permitted", i),
			)
			continue
		}
	}
}
