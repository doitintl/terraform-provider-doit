package provider

import (
	"fmt"
	"strconv"
	"time"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
)

// This file is the single source of truth for the provider's timeout defaults.
// Changing a default means editing exactly one line here — every resource and
// data source references these constants rather than a literal duration, and
// the timeoutcheck linter rejects literal durations at Timeouts.* call sites.
//
// # The Two Layers
//
// The provider has two nested timeout layers:
//
//   - Request timeout (DefaultRequestTimeout, overridable via the provider's
//     request_timeout attribute): bounds a single HTTP attempt.
//   - Operation timeout (Default{Create,Read,Update,Delete}Timeout, overridable
//     via a resource's timeouts {} block): bounds the whole Terraform
//     operation, including every retry.
//
// # The Ordering Invariant
//
//	operation timeout > request timeout > cloudflareEdgeTimeout
//
// Both inequalities are load-bearing, and violating either produces a confusing
// failure rather than a clean one:
//
// Request timeout must exceed the Cloudflare edge timeout. A slow query is
// answered with a 524, which DCIRetryClient classifies as permanent — one
// attempt, then a clear error. But if the request timeout is at or below the
// edge timeout, http.Client aborts the socket first and we never receive that
// response. The result is a bare transport error, which is indistinguishable
// from a network blip and therefore retried, so an expensive query is re-issued
// until the operation deadline expires. Historically both values were 120s,
// making it a coin flip which one fired first.
//
// Operation timeout must exceed the request timeout so that a single slow
// attempt cannot consume the entire operation budget, leaving no room to retry
// a genuinely transient failure.
const (
	// cloudflareEdgeTimeout is the DoiT API's Cloudflare origin-response
	// timeout: requests still unanswered at this point receive a 524. This is
	// observed edge behavior we do not control, and exists here only as the
	// floor the request timeout must clear.
	cloudflareEdgeTimeout = 120 * time.Second

	// minRetryHeadroom is the margin by which each layer must clear the one
	// below it. Equal values are not sufficient — that is the coin flip
	// described above.
	minRetryHeadroom = 30 * time.Second
)

// Default timeouts. See the ordering invariant above before changing these.
const (
	// DefaultRequestTimeout bounds a single HTTP request to the DoiT API.
	// Overridable via the provider's request_timeout attribute or the
	// DOIT_REQUEST_TIMEOUT environment variable.
	DefaultRequestTimeout = 150 * time.Second

	// DefaultCreateTimeout bounds a whole Create operation, retries included.
	DefaultCreateTimeout = 5 * time.Minute

	// DefaultReadTimeout bounds a whole Read operation, retries included.
	// This applies to both resource reads and data source reads.
	DefaultReadTimeout = 5 * time.Minute

	// DefaultUpdateTimeout bounds a whole Update operation, retries included.
	DefaultUpdateTimeout = 5 * time.Minute

	// DefaultDeleteTimeout bounds a whole Delete operation, retries included.
	DefaultDeleteTimeout = 5 * time.Minute
)

// Compile-time enforcement of the ordering invariant. Converting a negative
// constant to an unsigned type is not representable, so any edit that breaks the
// ordering fails to compile with "constant -X overflows uint64" pointing at the
// offending line. TestTimeoutDefaults_Ordering asserts the same thing with a
// readable failure message.
//
// uint64, not uint: these margins are Durations in nanoseconds, so a valid
// 120-second margin is 120,000,000,000 — which does not fit in a 32-bit uint and
// would break the 386 and arm release targets in .goreleaser.yml. uint64 holds
// any time.Duration while still rejecting negative values.
const (
	_ = uint64(DefaultRequestTimeout - cloudflareEdgeTimeout - minRetryHeadroom)
	_ = uint64(DefaultCreateTimeout - DefaultRequestTimeout - minRetryHeadroom)
	_ = uint64(DefaultReadTimeout - DefaultRequestTimeout - minRetryHeadroom)
	_ = uint64(DefaultUpdateTimeout - DefaultRequestTimeout - minRetryHeadroom)
	_ = uint64(DefaultDeleteTimeout - DefaultRequestTimeout - minRetryHeadroom)
)

// requestTimeoutPath is the provider attribute both validation paths report against.
var requestTimeoutPath = path.Root("request_timeout")

// formatSeconds renders a duration the way a user writes it in configuration:
// whole seconds, e.g. "120s". Duration.String() would render that as "2m0s",
// which does not match anything the user typed.
func formatSeconds(d time.Duration) string {
	return strconv.FormatFloat(d.Seconds(), 'f', -1, 64) + "s"
}

// validateRequestTimeout checks a request timeout against the ordering
// invariant described above.
//
// It is shared by two callers so both agree on the rules: requestTimeoutValidator
// (which sees the HCL attribute at terraform validate time) and the provider's
// Configure (which sees the resolved value, and so is the only path that covers
// the DOIT_REQUEST_TIMEOUT environment variable).
//
// Too low is an error: at or below cloudflareEdgeTimeout the provider cannot
// observe the API's own 524 and degrades into retrying opaque local timeouts.
//
// Too high is only a warning. Exceeding the operation defaults is legitimate
// when the user raises their timeouts {} blocks to match, and the provider
// cannot see those — they are resolved later, inside each CRUD method.
func validateRequestTimeout(requestTimeout time.Duration) diag.Diagnostics {
	var diags diag.Diagnostics

	if requestTimeout <= cloudflareEdgeTimeout {
		diags.AddAttributeError(
			requestTimeoutPath,
			"Request Timeout Too Low",
			fmt.Sprintf("request_timeout must be greater than %s, the DoiT API's edge timeout, but is %s. "+
				"At or below that threshold the API's 524 response cannot arrive before the local timeout "+
				"cancels the request, so slow calls fail with an opaque deadline error and are retried "+
				"instead of failing fast. The default is %s.",
				formatSeconds(cloudflareEdgeTimeout), formatSeconds(requestTimeout), formatSeconds(DefaultRequestTimeout)),
		)
		return diags
	}

	// DefaultReadTimeout is the smallest of the operation defaults.
	if requestTimeout >= DefaultReadTimeout {
		diags.AddAttributeWarning(
			requestTimeoutPath,
			"Request Timeout Exceeds Operation Defaults",
			fmt.Sprintf("request_timeout is %s, which is not below the default operation timeout of %s. "+
				"A single slow request can consume the entire operation budget, leaving no room to retry a "+
				"transient failure. Raise the timeouts {} block on the affected resources and data sources "+
				"to at least %s so retries remain possible.",
				formatSeconds(requestTimeout), formatSeconds(DefaultReadTimeout), formatSeconds(requestTimeout+minRetryHeadroom)),
		)
	}

	return diags
}
