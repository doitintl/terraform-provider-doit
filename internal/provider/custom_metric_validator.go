package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var _ validator.Object = customMetricDecimalsRangeValidator{}

// customMetricDecimalsRangeValidator rejects a decimals block whose min is
// greater than its max, which the API answers with issue out_of_range.
type customMetricDecimalsRangeValidator struct{}

func (v customMetricDecimalsRangeValidator) Description(_ context.Context) string {
	return "min must not be greater than max"
}

func (v customMetricDecimalsRangeValidator) MarkdownDescription(ctx context.Context) string {
	return v.Description(ctx)
}

func (v customMetricDecimalsRangeValidator) ValidateObject(ctx context.Context, req validator.ObjectRequest, resp *validator.ObjectResponse) {
	if req.ConfigValue.IsNull() || req.ConfigValue.IsUnknown() {
		return
	}

	var minValue, maxValue types.Int64
	diags := req.Config.GetAttribute(ctx, req.Path.AtName("min"), &minValue)
	resp.Diagnostics.Append(diags...)
	if diags.HasError() {
		return
	}
	diags = req.Config.GetAttribute(ctx, req.Path.AtName("max"), &maxValue)
	resp.Diagnostics.Append(diags...)
	if diags.HasError() {
		return
	}

	if minValue.IsUnknown() || maxValue.IsUnknown() {
		return
	}
	if minValue.IsNull() || maxValue.IsNull() {
		return
	}

	if minValue.ValueInt64() > maxValue.ValueInt64() {
		resp.Diagnostics.AddAttributeError(
			req.Path,
			"Invalid Decimals Range",
			fmt.Sprintf("decimals.min (%d) must not be greater than decimals.max (%d).", minValue.ValueInt64(), maxValue.ValueInt64()),
		)
	}
}
