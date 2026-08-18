package provider

import (
	"context"
	"fmt"
	"regexp"

	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
)

var (
	_ validator.String = customerURLSlugValidator{}

	// customerURLSlugRegex matches 3 to 12 lowercase alphanumeric characters and hyphens,
	// starting and ending with an alphanumeric character.
	customerURLSlugRegex = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{1,10}[a-z0-9]$`)
)

type customerURLSlugValidator struct{}

func customerURLSlug() validator.String {
	return customerURLSlugValidator{}
}

func (v customerURLSlugValidator) Description(_ context.Context) string {
	return "Validates that the value is a valid customer URL slug (3-12 characters, lowercase alphanumeric and hyphens, starting and ending with an alphanumeric character, or empty string to remove)."
}

func (v customerURLSlugValidator) MarkdownDescription(_ context.Context) string {
	return "Validates that the value is a valid customer URL slug (3-12 characters, lowercase alphanumeric and hyphens, starting and ending with an alphanumeric character, or empty string to remove)."
}

func (v customerURLSlugValidator) ValidateString(_ context.Context, req validator.StringRequest, resp *validator.StringResponse) {
	if req.ConfigValue.IsNull() || req.ConfigValue.IsUnknown() {
		return
	}

	val := req.ConfigValue.ValueString()
	if val == "" {
		// Empty string is valid and used to clear/remove the active URL slug.
		return
	}

	if !customerURLSlugRegex.MatchString(val) {
		resp.Diagnostics.AddAttributeError(
			req.Path,
			"Invalid Customer URL Slug",
			fmt.Sprintf("Customer URL slug must be 3 to 12 characters long, contain only lowercase letters, digits, and hyphens, and start and end with an alphanumeric character (or be an empty string to remove the URL slug). Got: %q", val),
		)
	}
}
