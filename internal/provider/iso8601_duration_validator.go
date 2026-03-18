package provider

import (
	"context"
	"fmt"
	"strings"
	"unicode"

	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
)

// iso8601DurationValidator validates that a string is an ISO 8601 date duration
// in the form P[n]Y[n]M[n]D (e.g., "P7D", "P1M", "P1Y2M3D").
// Time components (hours, minutes, seconds) are not supported.
var _ validator.String = iso8601DurationValidator{}

type iso8601DurationValidator struct{}

func (v iso8601DurationValidator) Description(_ context.Context) string {
	return "Validates that the value is an ISO 8601 date duration (e.g., 'P7D', 'P1M', 'P1Y2M3D')."
}

func (v iso8601DurationValidator) MarkdownDescription(_ context.Context) string {
	return "Validates that the value is an ISO 8601 date duration (e.g., `P7D`, `P1M`, `P1Y2M3D`)."
}

func (v iso8601DurationValidator) ValidateString(_ context.Context, req validator.StringRequest, resp *validator.StringResponse) {
	if req.ConfigValue.IsNull() || req.ConfigValue.IsUnknown() {
		return
	}

	value := req.ConfigValue.ValueString()
	if !isValidISO8601DateDuration(value) {
		resp.Diagnostics.AddAttributeError(
			req.Path,
			"Invalid ISO 8601 Duration",
			fmt.Sprintf(
				"Value must be an ISO 8601 date duration in the form P[n]Y[n]M[n]D "+
					"(e.g., 'P7D' for 7 days, 'P1M' for 1 month, 'P1Y2M3D' for 1 year, 2 months, 3 days). Got: %s",
				value,
			),
		)
	}
}

// isValidISO8601DateDuration parses a duration string like "P7D", "P1Y2M",
// "P1Y2M3D". Returns false for empty durations ("P"), time components ("PT1H"),
// or malformed strings.
func isValidISO8601DateDuration(s string) bool {
	if !strings.HasPrefix(s, "P") || len(s) < 2 {
		return false
	}

	// No time component allowed
	if strings.Contains(s, "T") {
		return false
	}

	rest := s[1:] // strip the "P"
	foundAny := false
	validDesignators := "YMD"
	lastDesignatorIdx := -1

	for len(rest) > 0 {
		// Must start with a digit
		if !unicode.IsDigit(rune(rest[0])) {
			return false
		}

		// Read digits
		i := 0
		for i < len(rest) && unicode.IsDigit(rune(rest[i])) {
			i++
		}

		// Must be followed by a designator
		if i >= len(rest) {
			return false
		}

		designator := rest[i]
		designatorIdx := strings.IndexByte(validDesignators, designator)
		if designatorIdx < 0 {
			return false
		}

		// Designators must appear in order (Y, M, D) and not repeat
		if designatorIdx <= lastDesignatorIdx {
			return false
		}
		lastDesignatorIdx = designatorIdx

		foundAny = true
		rest = rest[i+1:]
	}

	return foundAny
}
