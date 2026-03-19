package provider

import (
	"context"
	"fmt"
	"time"

	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
)

// dateValidator validates that a string value is a date in yyyy-mm-dd format.
var _ validator.String = dateValidator{}

type dateValidator struct{}

func (v dateValidator) Description(_ context.Context) string {
	return "Validates that the value is a date in yyyy-mm-dd format (e.g., '2025-01-15')."
}

func (v dateValidator) MarkdownDescription(_ context.Context) string {
	return "Validates that the value is a date in `yyyy-mm-dd` format (e.g., `2025-01-15`)."
}

func (v dateValidator) ValidateString(_ context.Context, req validator.StringRequest, resp *validator.StringResponse) {
	if req.ConfigValue.IsNull() || req.ConfigValue.IsUnknown() {
		return
	}

	value := req.ConfigValue.ValueString()
	if _, err := time.Parse(time.DateOnly, value); err != nil {
		resp.Diagnostics.AddAttributeError(
			req.Path,
			"Invalid Date Format",
			fmt.Sprintf("Value must be a date in yyyy-mm-dd format (e.g., '2025-01-15'). Got: %s", value),
		)
	}
}
