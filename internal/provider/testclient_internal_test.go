package provider

import (
	"net/http/httptest"
	"testing"
	"time"

	"github.com/cenkalti/backoff/v5"

	"github.com/doitintl/terraform-provider-doit/internal/provider/models"
)

// Shared constructors for the API clients unit tests drive against an
// httptest.NewTestServer.
//
// Two shapes, and the choice between them is what a test is about rather than a
// convenience: newTestAPIClient reaches the server directly, with no retry
// client in the path, and suits a test whose subject is a handler's response;
// newTestRetryAPIClient goes through the real DCIRetryClient, and is what a test
// needs to exercise retry, timeout, or status-passthrough behavior.
//
// Both call server.Client() before reading server.URL, which matters more than
// it looks: httptest.NewTestServer starts lazily, and Server.Client() is the
// call that starts it and assigns Server.URL. Go specifies the order of calls
// relative to each other but not relative to a plain field read in the same
// argument list, so the two must not share one. Centralizing that here is the
// point of these helpers.

// retryTestTimeout bounds a test that drives the real DCIRetryClient.
//
// DCIRetryClient runs with backoff.WithMaxElapsedTime(0), so the request context
// is the only thing that can end a retry loop. Without a deadline, the first
// retryable status a test meets retries forever and surfaces as a package-wide
// test-timeout panic rather than as a failure of the test at fault.
//
// Five seconds, because:
//
//   - the expected path is a single in-memory request, so this is orders of
//     magnitude of headroom and cannot flake on a loaded runner. A deadline that
//     can fire on the passing path is worse than none;
//   - it is far enough under the suite's own -timeout that a retry regression
//     fails as this test, in seconds, with a context-deadline error — instead of
//     panicking the package and taking every other test down with it;
//   - it clears retryInitialInterval, so a future case whose response carries a
//     Retry-After gets a few bounded attempts rather than exactly one.
//
// Deliberately not DefaultReadTimeout, which the testing skill's example uses:
// that example presumes a synctest bubble, where five virtual minutes are free.
// These tests run on the real clock, where they are five real minutes.
const retryTestTimeout = 5 * time.Second

// constantBackOff returns a factory for a fixed-delay, jitter-free retry policy.
//
// Tests inject this so retry timing is deterministic and cheap: the backoff
// library's timer hook is unexported, so replacing the policy is the only way to
// control how long the retry loop sleeps. A long delay is equally useful — it
// proves a response was NOT retried, or that a Retry-After header took
// precedence over the policy.
//
// It does not cap every wait. A 429 carrying a Retry-After header makes Do
// return a backoff.RetryAfterError, and the backoff library overrides the
// injected policy with the header's value, clamped to [retryInitialInterval,
// maxRetryAfter]. So this bounds the header-less path only; a context deadline
// is what bounds the rest.
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
//
// This is the call that lazily starts the test server: httptest.NewTestServer
// defers startup to the first Server.Client(), which is also what assigns
// Server.URL. Two consequences for callers — inside a synctest bubble this must
// run within the bubble, so the fake network's channels belong to it and a
// blocked read is durably blocking; and Server.URL reads as "" until it has run.
//
// The client is copied rather than used in place. Server.Client() hands back the
// one *http.Client the server owns — and reaches into its Transport during Close
// to reap idle connections — so mutating it would make Timeout shared state
// between every client built against the same server. Copying keeps the Transport
// pointer, so Close still works, while per-client timeouts stay independent.
func newTestRetryClient(server *httptest.Server, requestTimeout time.Duration, newBackOff func() backoff.BackOff) *DCIRetryClient {
	c := *server.Client()
	c.Timeout = requestTimeout
	return &DCIRetryClient{
		client:     &c,
		newBackOff: newBackOff,
	}
}

// newTestAPIClient builds a generated client that reaches server directly, with
// no DCIRetryClient in the path.
//
// Use it when the subject of the test is what a handler returns. Nothing here
// can retry, so no context deadline is needed to bound it.
//
// Unlike newTestRetryClient this passes server.Client() as-is rather than a
// copy: it sets no field on it, and Server.Close reaches into the client the
// server owns to reap idle connections. Do not "improve" this into a copy.
func newTestAPIClient(t *testing.T, server *httptest.Server) *models.ClientWithResponses {
	t.Helper()

	httpClient := server.Client()

	client, err := models.NewClientWithResponses(server.URL, models.WithHTTPClient(httpClient))
	if err != nil {
		t.Fatalf("models.NewClientWithResponses() error = %v", err)
	}
	return client
}

// newTestRetryAPIClient builds a generated client that reaches server through
// the real DCIRetryClient with an injected policy.
//
// Callers must bound their context — retryTestTimeout says why.
//
// Callers are split: async_report_test.go calls this from inside synctest
// bubbles, delete_notfound_test.go from ordinary tests. Because some callers are
// bubbled, this must keep allocating nothing outside the caller's goroutine — a
// timer or channel created elsewhere is not durably blocking, and a bubble whose
// clock cannot advance hangs to the suite timeout instead of failing.
func newTestRetryAPIClient(
	t *testing.T, server *httptest.Server,
	requestTimeout time.Duration, newBackOff func() backoff.BackOff,
) *models.ClientWithResponses {
	t.Helper()

	retryClient := newTestRetryClient(server, requestTimeout, newBackOff)

	client, err := models.NewClientWithResponses(server.URL, models.WithHTTPClient(retryClient))
	if err != nil {
		t.Fatalf("models.NewClientWithResponses() error = %v", err)
	}
	return client
}
