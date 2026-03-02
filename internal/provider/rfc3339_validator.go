package provider

import (
	"context"
	"fmt"
	"time"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
)

// rfc3339Validator validates that a string value is in RFC3339 format.
var _ validator.String = rfc3339Validator{}

type rfc3339Validator struct{}

// Description returns a description of the validator.
func (v rfc3339Validator) Description(_ context.Context) string {
	return "Validates that the value is a valid RFC3339 timestamp (e.g., '2024-06-15T12:00:00Z')."
}

// MarkdownDescription returns a markdown description of the validator.
func (v rfc3339Validator) MarkdownDescription(_ context.Context) string {
	return "Validates that the value is a valid RFC3339 timestamp (e.g., `2024-06-15T12:00:00Z`)."
}

// ValidateString validates the string value.
func (v rfc3339Validator) ValidateString(_ context.Context, req validator.StringRequest, resp *validator.StringResponse) {
	if req.ConfigValue.IsNull() || req.ConfigValue.IsUnknown() {
		return
	}

	validateRFC3339(req.ConfigValue.ValueString(), req.Path, &resp.Diagnostics)
}

// validateRFC3339 checks that value is a valid RFC3339 timestamp and adds an
// attribute error to diags if it is not. This is the single source of truth for
// RFC3339 validation logic, used by both the attribute-level rfc3339Validator
// and the resource-level reportTimestampValidator.
func validateRFC3339(value string, attrPath path.Path, diags *diag.Diagnostics) {
	if _, err := time.Parse(time.RFC3339, value); err != nil {
		diags.AddAttributeError(
			attrPath,
			"Invalid RFC3339 Timestamp",
			fmt.Sprintf("Value must be a valid RFC3339 timestamp (e.g., '2024-06-15T12:00:00Z'). Got: %s", value),
		)
	}
}
