package provider

import (
	"context"
	"fmt"
	"time"

	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
)

// requestTimeoutValidator validates the provider's request_timeout attribute:
// the value must be a Go duration string, and must satisfy the timeout ordering
// invariant documented in timeouts.go.
//
// This runs at terraform validate time, so a misconfigured timeout is reported
// before any API call. It only sees the HCL attribute — the equivalent check on
// the resolved value (which also covers DOIT_REQUEST_TIMEOUT) lives in the
// provider's Configure. Both delegate to validateRequestTimeout.
var _ validator.String = requestTimeoutValidator{}

type requestTimeoutValidator struct{}

func (v requestTimeoutValidator) Description(_ context.Context) string {
	return fmt.Sprintf("Validates that the value is a Go duration string greater than %s.", cloudflareEdgeTimeout)
}

func (v requestTimeoutValidator) MarkdownDescription(_ context.Context) string {
	return fmt.Sprintf("Validates that the value is a Go duration string greater than `%s`.", cloudflareEdgeTimeout)
}

func (v requestTimeoutValidator) ValidateString(_ context.Context, req validator.StringRequest, resp *validator.StringResponse) {
	if req.ConfigValue.IsNull() || req.ConfigValue.IsUnknown() {
		return
	}

	value := req.ConfigValue.ValueString()
	requestTimeout, err := time.ParseDuration(value)
	if err != nil {
		resp.Diagnostics.AddAttributeError(
			req.Path,
			"Invalid Request Timeout",
			fmt.Sprintf("Could not parse request_timeout %q as a duration: %s. Use Go duration format, e.g. \"30s\", \"2m\", \"1h\".", value, err),
		)
		return
	}

	resp.Diagnostics.Append(validateRequestTimeout(requestTimeout)...)
}
