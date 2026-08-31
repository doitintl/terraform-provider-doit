package provider

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// newTimeoutTestServer creates an httptest.Server for the provided handler.
func newTimeoutTestServer(handler http.HandlerFunc) *httptest.Server {
	return httptest.NewServer(handler)
}

// countingServer returns a test server wrapping handler with a request counter.
// The counter is incremented before handler runs, so it reflects requests that
// reached the server even if the client later abandons them.
func countingServer(count *atomic.Int64, handler http.HandlerFunc) *httptest.Server {
	return newTimeoutTestServer(func(w http.ResponseWriter, r *http.Request) {
		count.Add(1)
		handler(w, r)
	})
}

// respondAfter writes status once delay has elapsed, or returns immediately if
// the client disconnects first. Honoring the request context matters: without it
// httptest.Server.Close blocks draining handlers the client already abandoned,
// which would add the full delay to every timeout test.
func respondAfter(delay time.Duration, status int) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-time.After(delay):
			w.WriteHeader(status)
		case <-r.Context().Done():
		}
	}
}

// doGet issues a GET through the retry client and closes any response body.
func doGet(ctx context.Context, t *testing.T, c *DCIRetryClient, url string) (*http.Response, error) {
	t.Helper()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		t.Fatalf("http.NewRequestWithContext() error = %v", err)
	}

	resp, err := c.Do(req)
	if resp != nil {
		t.Cleanup(func() { _ = resp.Body.Close() })
	}
	return resp, err
}

// TestDCIRetryClient_RequestTimeout verifies that a per-request timeout is
// enforced: when the server takes longer than the configured timeout, each
// attempt is cancelled locally. With MaxElapsedTime=0, retries continue until
// the context deadline.
func TestDCIRetryClient_RequestTimeout(t *testing.T) {
	t.Parallel()

	const (
		serverDelay   = 200 * time.Millisecond
		clientTimeout = 50 * time.Millisecond
	)

	var requestCount atomic.Int64
	server := countingServer(&requestCount, respondAfter(serverDelay, http.StatusOK))
	defer server.Close()

	client := newTestRetryClient(clientTimeout, constantBackOff(time.Millisecond))

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	start := time.Now()
	_, err := doGet(ctx, t, client, server.URL)
	elapsed := time.Since(start)

	if err == nil {
		t.Fatal("expected timeout error, got nil")
	}

	// Each attempt should be cut off at clientTimeout, leaving room for several
	// attempts inside the context deadline.
	if count := requestCount.Load(); count < 2 {
		t.Fatalf("expected at least 2 attempts, got %d", count)
	}

	t.Logf("Made %d attempts in %v (per-request timeout %v)", requestCount.Load(), elapsed, clientTimeout)
}

// TestDCIRetryClient_ContextCancellation verifies that a parent context
// cancellation propagates through the retry loop and stops retries early, even
// when the per-request timeout is far larger.
func TestDCIRetryClient_ContextCancellation(t *testing.T) {
	t.Parallel()

	const (
		clientTimeout = 10 * time.Second       // deliberately never reached
		parentTimeout = 300 * time.Millisecond // the real bound
	)

	server := newTimeoutTestServer(respondAfter(5*time.Second, http.StatusOK))
	defer server.Close()

	client := newTestRetryClient(clientTimeout, constantBackOff(time.Millisecond))

	ctx, cancel := context.WithTimeout(context.Background(), parentTimeout)
	defer cancel()

	start := time.Now()
	_, err := doGet(ctx, t, client, server.URL)
	elapsed := time.Since(start)

	if err == nil {
		t.Fatal("expected context cancellation error, got nil")
	}

	// Should complete around the parent timeout, not the per-request timeout.
	if maxExpected := 2 * parentTimeout; elapsed > maxExpected {
		t.Fatalf("expected to complete within %v, took %v", maxExpected, elapsed)
	}
	t.Logf("Cancelled after %v (parent timeout %v)", elapsed, parentTimeout)
}

// TestDCIRetryClient_RetryRespectsContextDeadline verifies that the retry loop
// defers to the context deadline (MaxElapsedTime = 0). Retries continue until
// the context is cancelled, not until a hardcoded elapsed time.
func TestDCIRetryClient_RetryRespectsContextDeadline(t *testing.T) {
	t.Parallel()

	const contextDeadline = 300 * time.Millisecond

	var requestCount atomic.Int64
	server := countingServer(&requestCount, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable) // 503 triggers retry
	})
	defer server.Close()

	client := newTestRetryClient(10*time.Second, constantBackOff(25*time.Millisecond))

	ctx, cancel := context.WithTimeout(context.Background(), contextDeadline)
	defer cancel()

	start := time.Now()
	_, err := doGet(ctx, t, client, server.URL)
	elapsed := time.Since(start)

	if err == nil {
		t.Fatal("expected context deadline error, got nil")
	}

	if maxExpected := 2 * contextDeadline; elapsed > maxExpected {
		t.Fatalf("expected to complete around %v, took %v", contextDeadline, elapsed)
	}
	if count := requestCount.Load(); count < 2 {
		t.Fatalf("expected at least 2 retry attempts, got %d", count)
	}

	t.Logf("Retried %d times in %v (context deadline %v)", requestCount.Load(), elapsed, contextDeadline)
}

// TestDCIRetryClient_CloudflareTimeoutIsPermanent is the regression test for the
// failure this timeout work addresses.
//
// A 524 means the API's edge already waited out the full origin timeout, so the
// response must fail fast rather than re-running an expensive query. The injected
// backoff is long enough that a single retry would blow the context deadline —
// so the request count is what proves no retry happened.
func TestDCIRetryClient_CloudflareTimeoutIsPermanent(t *testing.T) {
	t.Parallel()

	var requestCount atomic.Int64
	server := countingServer(&requestCount, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(httpStatusCloudflareTimeout)
		_, _ = w.Write([]byte("error code: 524"))
	})
	defer server.Close()

	client := newTestRetryClient(10*time.Second, constantBackOff(10*time.Second))

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	_, err := doGet(ctx, t, client, server.URL)

	if err == nil {
		t.Fatal("expected an error for 524, got nil")
	}
	if count := requestCount.Load(); count != 1 {
		t.Errorf("expected exactly 1 attempt (524 must not be retried), got %d", count)
	}
	// backoff.Retry unwraps PermanentError, so the caller sees the inner error
	// rather than a *backoff.PermanentError — assert on the message instead.
	if !strings.Contains(err.Error(), "524") {
		t.Errorf("error should mention the 524 status, got: %v", err)
	}
}

// TestDCIRetryClient_429NoRetryAfter_Retries covers the DoiT API's actual
// observed behavior: 429 responses carry no Retry-After header, so the retry
// falls back to the exponential policy.
func TestDCIRetryClient_429NoRetryAfter_Retries(t *testing.T) {
	t.Parallel()

	const failures = 2

	var requestCount atomic.Int64
	server := countingServer(&requestCount, func(w http.ResponseWriter, _ *http.Request) {
		if requestCount.Load() <= failures {
			w.WriteHeader(http.StatusTooManyRequests) // deliberately no Retry-After
			return
		}
		w.WriteHeader(http.StatusOK)
	})
	defer server.Close()

	client := newTestRetryClient(10*time.Second, constantBackOff(time.Millisecond))

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	resp, err := doGet(ctx, t, client, server.URL)

	if err != nil {
		t.Fatalf("expected success after retries, got error: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Errorf("StatusCode = %d, want %d", resp.StatusCode, http.StatusOK)
	}
	if count := requestCount.Load(); count != failures+1 {
		t.Errorf("expected %d attempts, got %d", failures+1, count)
	}
}

// TestDCIRetryClient_429WithRetryAfter_Honored verifies a server-supplied
// Retry-After takes precedence over the exponential policy. The injected policy
// would wait 10s, so completing in roughly the header's delay proves the header
// won.
func TestDCIRetryClient_429WithRetryAfter_Honored(t *testing.T) {
	t.Parallel()

	// retryInitialInterval is the floor parseRetryAfter clamps to, so asking for
	// exactly that keeps the test as short as the floor allows.
	retryAfter := retryInitialInterval

	var requestCount atomic.Int64
	server := countingServer(&requestCount, func(w http.ResponseWriter, _ *http.Request) {
		if requestCount.Load() == 1 {
			w.Header().Set("Retry-After", strconv.Itoa(int(retryAfter.Seconds())))
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}
		w.WriteHeader(http.StatusOK)
	})
	defer server.Close()

	client := newTestRetryClient(10*time.Second, constantBackOff(10*time.Second))

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	start := time.Now()
	resp, err := doGet(ctx, t, client, server.URL)
	elapsed := time.Since(start)

	if err != nil {
		t.Fatalf("expected success after honoring Retry-After, got error: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Errorf("StatusCode = %d, want %d", resp.StatusCode, http.StatusOK)
	}
	if count := requestCount.Load(); count != 2 {
		t.Errorf("expected 2 attempts, got %d", count)
	}
	// Generous lower bound for timer granularity; the upper bound is what proves
	// the 10s policy was not used.
	if elapsed < retryAfter-50*time.Millisecond || elapsed > 5*time.Second {
		t.Errorf("elapsed = %v, want approximately the Retry-After delay of %v", elapsed, retryAfter)
	}
}

// TestDCIRetryClient_404PassesThrough guards the client's most load-bearing
// documented behavior: a 404 is returned as a normal response, not an error, so
// resource handlers can interpret it contextually (Read: externally deleted,
// Delete: already gone).
func TestDCIRetryClient_404PassesThrough(t *testing.T) {
	t.Parallel()

	var requestCount atomic.Int64
	server := countingServer(&requestCount, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	})
	defer server.Close()

	client := newTestRetryClient(10*time.Second, constantBackOff(10*time.Second))

	resp, err := doGet(context.Background(), t, client, server.URL)

	if err != nil {
		t.Fatalf("404 must not be an error, got: %v", err)
	}
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("StatusCode = %d, want %d", resp.StatusCode, http.StatusNotFound)
	}
	if count := requestCount.Load(); count != 1 {
		t.Errorf("expected exactly 1 attempt (404 must not be retried), got %d", count)
	}
}

// TestDCIRetryClient_500NotRetried guards the documented decision not to retry
// 500, unlike 502/503/504.
func TestDCIRetryClient_500NotRetried(t *testing.T) {
	t.Parallel()

	var requestCount atomic.Int64
	server := countingServer(&requestCount, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	})
	defer server.Close()

	client := newTestRetryClient(10*time.Second, constantBackOff(10*time.Second))

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	_, err := doGet(ctx, t, client, server.URL)

	if err == nil {
		t.Fatal("expected an error for 500, got nil")
	}
	if count := requestCount.Load(); count != 1 {
		t.Errorf("expected exactly 1 attempt (500 must not be retried), got %d", count)
	}
}

// TestNewClient_CustomTimeout verifies that NewClient accepts a custom timeout
// and performs zero network I/O during initialization.
func TestNewClient_CustomTimeout(t *testing.T) {
	t.Parallel()

	var reqCount atomic.Int64
	server := countingServer(&reqCount, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	defer server.Close()

	client, err := NewClient(
		server.URL, "test-token", "", "1.0.0", "dev", 42*time.Second,
	)
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}
	if client == nil {
		t.Fatal("NewClient() returned nil client")
	}
	if got := reqCount.Load(); got != 0 {
		t.Errorf("NewClient() made %d HTTP requests, want 0", got)
	}
}

// TestNewClient_DefaultTimeout verifies that DefaultRequestTimeout is a valid
// configuration value and performs zero network I/O during initialization.
func TestNewClient_DefaultTimeout(t *testing.T) {
	t.Parallel()

	var reqCount atomic.Int64
	server := countingServer(&reqCount, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	defer server.Close()

	client, err := NewClient(
		server.URL, "test-token", "", "1.0.0", "dev", DefaultRequestTimeout,
	)
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}
	if client == nil {
		t.Fatal("NewClient() returned nil client")
	}
	if got := reqCount.Load(); got != 0 {
		t.Errorf("NewClient() made %d HTTP requests, want 0", got)
	}
}
