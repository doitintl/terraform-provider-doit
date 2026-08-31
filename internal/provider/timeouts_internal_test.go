package provider

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/cenkalti/backoff/v5"
	"github.com/hashicorp/terraform-plugin-framework/diag"
)

// TestTimeoutDefaults_Ordering verifies the timeout ordering invariant:
//
//	operation timeout > request timeout > cloudflareEdgeTimeout
//
// timeouts.go also enforces this at compile time, but that failure surfaces as
// "constant -X overflows uint". This test is the readable counterpart, and names
// the offending pair.
func TestTimeoutDefaults_Ordering(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		outer time.Duration
		inner time.Duration
	}{
		{"request timeout over Cloudflare edge timeout", DefaultRequestTimeout, cloudflareEdgeTimeout},
		{"create timeout over request timeout", DefaultCreateTimeout, DefaultRequestTimeout},
		{"read timeout over request timeout", DefaultReadTimeout, DefaultRequestTimeout},
		{"update timeout over request timeout", DefaultUpdateTimeout, DefaultRequestTimeout},
		{"delete timeout over request timeout", DefaultDeleteTimeout, DefaultRequestTimeout},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			if got := tc.outer - tc.inner; got < minRetryHeadroom {
				t.Errorf("%s: outer (%s) must exceed inner (%s) by at least %s, but the margin is %s",
					tc.name, tc.outer, tc.inner, minRetryHeadroom, got)
			}
		})
	}
}

// TestTimeoutDefaults_ReadIsSmallestOperation guards the assumption in
// validateRequestTimeout, which compares against DefaultReadTimeout as a proxy
// for "the smallest operation default".
func TestTimeoutDefaults_ReadIsSmallestOperation(t *testing.T) {
	t.Parallel()

	for _, other := range []time.Duration{DefaultCreateTimeout, DefaultUpdateTimeout, DefaultDeleteTimeout} {
		if DefaultReadTimeout > other {
			t.Errorf("DefaultReadTimeout (%s) is no longer the smallest operation default (found %s); "+
				"validateRequestTimeout compares against it and must be updated", DefaultReadTimeout, other)
		}
	}
}

// TestMaxRetryAfter_LeavesRetryBudget verifies a Retry-After honored at the cap
// still leaves room for the retry it is waiting for. A cap equal to the
// operation timeout would defeat its own purpose: the retry loop waits on the
// context, so the operation would expire mid-wait and never retry at all.
//
// timeouts.go-style compile-time assertion covers this too; this is the readable
// counterpart.
func TestMaxRetryAfter_LeavesRetryBudget(t *testing.T) {
	t.Parallel()

	smallest := min(DefaultCreateTimeout, DefaultReadTimeout, DefaultUpdateTimeout, DefaultDeleteTimeout)

	if got := smallest - maxRetryAfter; got < minRetryHeadroom {
		t.Errorf("maxRetryAfter (%s) must be below the smallest operation default (%s) by at least %s, "+
			"but the margin is %s — a wait at the cap would consume the whole operation budget",
			maxRetryAfter, smallest, minRetryHeadroom, got)
	}
}

// TestNewRetryBackOff_Config verifies the retry policy is configured from the
// retry* constants. The DoiT API returns 429 without a Retry-After header, so
// this policy governs the pace of nearly every retry.
func TestNewRetryBackOff_Config(t *testing.T) {
	t.Parallel()

	b := newRetryBackOff()

	if b.InitialInterval != retryInitialInterval {
		t.Errorf("InitialInterval = %s, want %s", b.InitialInterval, retryInitialInterval)
	}
	if b.Multiplier != retryMultiplier {
		t.Errorf("Multiplier = %v, want %v", b.Multiplier, retryMultiplier)
	}
	if b.MaxInterval != retryMaxInterval {
		t.Errorf("MaxInterval = %s, want %s", b.MaxInterval, retryMaxInterval)
	}
	// Jitter is deliberately left at the library default: concurrent operations
	// hitting the same rate limit should not retry in lockstep.
	if b.RandomizationFactor == 0 {
		t.Error("RandomizationFactor = 0, want jitter to be enabled")
	}
}

// TestNewRetryBackOff_Grows verifies the interval sequence grows geometrically
// and saturates at retryMaxInterval.
//
// Jitter is disabled for this check. With the production RandomizationFactor the
// windows of consecutive intervals overlap, so asserting that each interval
// exceeds its predecessor would be intermittently flaky —
// TestNewRetryBackOff_JitterBounds covers the randomized case instead.
func TestNewRetryBackOff_Grows(t *testing.T) {
	t.Parallel()

	b := newRetryBackOff()
	b.RandomizationFactor = 0
	b.Reset()

	// 2s, 4s, 8s, 16s, 32s, then capped at retryMaxInterval.
	want := []time.Duration{
		retryInitialInterval,
		2 * retryInitialInterval,
		4 * retryInitialInterval,
		8 * retryInitialInterval,
		16 * retryInitialInterval,
		retryMaxInterval,
		retryMaxInterval,
	}

	for i, w := range want {
		if got := b.NextBackOff(); got != w {
			t.Errorf("interval %d = %s, want %s", i+1, got, w)
		}
	}
}

// TestNewRetryBackOff_JitterBounds verifies every randomized interval stays
// within the jitter window around its base interval.
func TestNewRetryBackOff_JitterBounds(t *testing.T) {
	t.Parallel()

	b := newRetryBackOff()
	b.Reset()

	base := retryInitialInterval
	for i := range 7 {
		got := b.NextBackOff()

		lower := time.Duration(float64(base) * (1 - b.RandomizationFactor))
		// The library's randomization widens the range by 1ns, so the upper
		// bound is inclusive of that extra nanosecond.
		upper := time.Duration(float64(base)*(1+b.RandomizationFactor)) + time.Nanosecond

		if got < lower || got > upper {
			t.Errorf("interval %d = %s, want within [%s, %s] for base %s", i+1, got, lower, upper, base)
		}

		base = min(2*base, retryMaxInterval)
	}
}

// TestParseRetryAfter covers both Retry-After forms from RFC 7231 plus the
// degenerate values that must fall back to exponential backoff.
func TestParseRetryAfter(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 8, 14, 12, 0, 0, 0, time.UTC)
	// Comfortably inside [retryInitialInterval, maxRetryAfter] so these cases
	// exercise date parsing rather than the clamps.
	future := now.Add(30 * time.Second)
	past := now.Add(-90 * time.Second)
	farFuture := now.Add(10 * time.Minute)

	tests := []struct {
		name   string
		header string
		want   time.Duration
		wantOK bool
	}{
		// delay-seconds
		{"absent", "", 0, false},
		{"seconds", "30", 30 * time.Second, true},
		{"seconds with surrounding space", "  30  ", 30 * time.Second, true},
		{"seconds below floor is raised", "1", retryInitialInterval, true},
		{"seconds above cap is clamped", "86400", maxRetryAfter, true},
		{"zero seconds falls back", "0", 0, false},
		{"negative seconds falls back", "-5", 0, false},
		// A large negative value overflows seconds*time.Second and wraps to a
		// positive duration, which would otherwise pass the non-positive check
		// and be honored as a legitimate wait.
		{"overflowing negative seconds falls back", "-10000000000", 0, false},
		{"overflowing positive seconds is clamped", "10000000000", maxRetryAfter, true},

		// HTTP-date, all three formats RFC 7231 permits
		{"IMF-fixdate in future", future.Format(http.TimeFormat), 30 * time.Second, true},
		{"RFC 850 in future", future.Format(time.RFC850), 30 * time.Second, true},
		{"asctime in future", future.Format(time.ANSIC), 30 * time.Second, true},
		{"IMF-fixdate in past falls back", past.Format(http.TimeFormat), 0, false},
		{"IMF-fixdate beyond cap is clamped", farFuture.Format(http.TimeFormat), maxRetryAfter, true},

		// unusable
		{"garbage falls back", "not-a-date", 0, false},
		{"float notation falls back", "1e3", 0, false},
		{"fractional seconds falls back", "0.5", 0, false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got, ok := parseRetryAfter(tc.header, now)

			if ok != tc.wantOK {
				t.Fatalf("parseRetryAfter(%q) ok = %v, want %v", tc.header, ok, tc.wantOK)
			}
			if ok && got != tc.want {
				t.Errorf("parseRetryAfter(%q) = %s, want %s", tc.header, got, tc.want)
			}
		})
	}
}

// TestValidateRequestTimeout verifies the ordering invariant is enforced against
// user-supplied values: too low is an error, exceeding the operation defaults is
// only a warning.
func TestValidateRequestTimeout(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		timeout      time.Duration
		wantErrors   int
		wantWarnings int
	}{
		{"well below the edge timeout", 60 * time.Second, 1, 0},
		{"exactly the edge timeout", cloudflareEdgeTimeout, 1, 0},
		{"just above the edge timeout", cloudflareEdgeTimeout + time.Second, 0, 0},
		{"the default", DefaultRequestTimeout, 0, 0},
		{"just below the read default", DefaultReadTimeout - time.Second, 0, 0},
		{"exactly the read default", DefaultReadTimeout, 0, 1},
		{"well above the read default", 10 * time.Minute, 0, 1},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			diags := validateRequestTimeout(tc.timeout)

			if got := diags.ErrorsCount(); got != tc.wantErrors {
				t.Errorf("validateRequestTimeout(%s) errors = %d, want %d: %v",
					tc.timeout, got, tc.wantErrors, diags.Errors())
			}
			if got := diags.WarningsCount(); got != tc.wantWarnings {
				t.Errorf("validateRequestTimeout(%s) warnings = %d, want %d: %v",
					tc.timeout, got, tc.wantWarnings, diags.Warnings())
			}
		})
	}
}

// TestValidateRequestTimeout_AttributePath verifies diagnostics are attached to
// the request_timeout attribute so Terraform points at the right line.
func TestValidateRequestTimeout_AttributePath(t *testing.T) {
	t.Parallel()

	errDiags := validateRequestTimeout(time.Second)
	if len(errDiags.Errors()) != 1 {
		t.Fatalf("expected exactly 1 error, got %d", len(errDiags.Errors()))
	}

	warnDiags := validateRequestTimeout(10 * time.Minute)
	if len(warnDiags.Warnings()) != 1 {
		t.Fatalf("expected exactly 1 warning, got %d", len(warnDiags.Warnings()))
	}

	// Both must be attribute diagnostics rooted at request_timeout.
	for _, d := range append(errDiags.Errors(), warnDiags.Warnings()...) {
		withPath, ok := d.(diag.DiagnosticWithPath)
		if !ok {
			t.Errorf("diagnostic %q is not an attribute diagnostic", d.Summary())
			continue
		}
		if got := withPath.Path(); !got.Equal(requestTimeoutPath) {
			t.Errorf("diagnostic %q path = %s, want %s", d.Summary(), got, requestTimeoutPath)
		}
	}
}

// constantBackOff returns a factory for a fixed-delay, jitter-free retry policy.
//
// Tests inject this so retry timing is deterministic and cheap: the backoff
// library's timer hook is unexported, so replacing the policy is the only way to
// control how long the retry loop sleeps. A long delay is equally useful — it
// proves a response was NOT retried, or that a Retry-After header took
// precedence over the policy.
func constantBackOff(d time.Duration) func() backoff.BackOff {
	return func() backoff.BackOff {
		b := backoff.NewExponentialBackOff()
		b.InitialInterval = d
		b.RandomizationFactor = 0
		b.Multiplier = 1
		b.MaxInterval = d
		return b
	}
}

// newTestRetryClient builds a DCIRetryClient with an injected backoff policy
// for unit testing retry behavior.
func newTestRetryClient(server *httptest.Server, requestTimeout time.Duration, newBackOff func() backoff.BackOff) *DCIRetryClient {
	c := server.Client()
	c.Timeout = requestTimeout
	return &DCIRetryClient{
		client:     c,
		newBackOff: newBackOff,
	}
}
