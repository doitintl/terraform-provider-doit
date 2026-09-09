package provider

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"testing/synctest"
	"time"

	"github.com/doitintl/terraform-provider-doit/internal/provider/models"
)

// These tests drive DCIRetryClient's timeout and retry behavior against a fake
// API.
//
// Every TestDCIRetryClient_* test runs inside a synctest bubble, where the time
// package uses a fake clock that advances only once every goroutine in the
// bubble is durably blocked. That is what makes it affordable to assert against
// the constants the provider actually ships — a 150s request timeout, a
// 5-minute operation timeout, a 2s-to-60s backoff — instead of the shrunken
// millisecond stand-ins these tests used while the waits were real. It also
// turns every timing assertion into exact equality: there is no timer
// granularity to absorb, so "waited exactly 30s" and "did not wait at all" are
// both statements a test can make.
//
// The rule for this file is that a test making a claim about time or retry
// sequencing belongs in a bubble — including the ones that expect no wait,
// since elapsed == 0 is the most direct possible proof that a status was not
// retried. The constructor tests at the bottom of the file make no such claim —
// they assert what NewClient wires up, and that construction issues no requests
// at all — so they stay on t.Parallel().
//
// Note the absence of t.Parallel() in the bubbled tests: a bubble owns its own
// clock and requires every goroutine in it to be durably blocked before time
// advances, so the bubble must be the test's whole world. httptest.NewTestServer
// is what makes this work — its in-memory network is durably blocking, where a
// real TCP listener would not be, and the clock would simply never advance.
// See internal/provider/async_report_test.go for the same pattern applied to
// the async report poll loop.

// newTimeoutTestServer creates an httptest.Server for the provided handler.
//
// Inside a synctest bubble this must be called from within the bubble, so the
// fake network's channels belong to it. A server built outside the bubble makes
// a blocked read non-durable, which stops the clock advancing entirely — timer
// driven tests then hang to the package timeout rather than failing.
func newTimeoutTestServer(t *testing.T, handler http.HandlerFunc) *httptest.Server {
	return httptest.NewTestServer(t, handler)
}

// countingServer returns a test server wrapping handler with a request counter.
// The counter is incremented before handler runs, so it reflects requests that
// reached the server even if the client later abandons them.
//
// The increment ordering also matters under a fake clock: the clock cannot
// advance until the handler goroutine is durably blocked, which is necessarily
// after the increment, so a count read from the root goroutine sees every
// request that has been answered. Where a handler is still parked when the
// client gives up, call synctest.Wait() before reading the counter.
func countingServer(t *testing.T, count *atomic.Int64, handler http.HandlerFunc) *httptest.Server {
	return newTimeoutTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		count.Add(1)
		handler(w, r)
	})
}

// respondAfter writes status once delay has elapsed, or returns immediately if
// the client disconnects first. Honoring the request context matters: without it
// httptest.Server.Close blocks draining handlers the client already abandoned,
// which would add the full delay to every timeout test — and inside a bubble
// that block fast-forwards the fake clock by the whole delay during cleanup.
//
// The context escape depends on the server having started its background read,
// which for a request carrying an unconsumed body is deferred until the body is
// read to EOF. It is reliable for the bodyless requests these tests issue; a
// future POST timeout test should reach for neverRespond instead.
func respondAfter(delay time.Duration, status int) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-time.After(delay):
			w.WriteHeader(status)
		case <-r.Context().Done():
		}
	}
}

// neverRespond blocks until the client goes away and answers nothing, standing
// in for an API that has stopped responding.
//
// Preferred over respondAfter with a large delay when the response is never
// expected to arrive: it plants no timer in the bubble, which keeps the set of
// scheduled wake-ups small enough to reason about by hand, and it exits the
// instant the connection closes. It also fails loudly rather than quietly — a
// test that forgets to bound both the request timeout and the context deadline
// deadlocks the bubble, which synctest reports immediately instead of hanging.
func neverRespond() http.HandlerFunc {
	return func(_ http.ResponseWriter, r *http.Request) {
		<-r.Context().Done()
	}
}

// doGet issues a GET through the retry client and closes any response body.
//
// Closing the body is what lets the server's own cleanup reap the connection:
// an open body keeps the connection out of the idle pool, so Close cannot
// collect it and the transport's read loop never exits — which inside a bubble
// surfaces as a leaked-goroutine failure. Registering it as a cleanup rather
// than a defer also keeps the ordering right against the cleanup
// httptest.NewTestServer registers for itself, which is why no test in this
// file adds a `defer server.Close()`.
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
// enforced: when the server stops responding, each attempt is cancelled locally
// and retried, and with MaxElapsedTime=0 the loop then defers to the context
// deadline.
//
// Driven at the shipped defaults, the outcome is exactly two attempts. The
// first is cut off at DefaultRequestTimeout (150s) and backs off 2s; the
// second's own deadline would land at 302s, past the 300s operation deadline,
// so net/http leaves the request timeout inert and the operation deadline ends
// the run at exactly 300s. That is what minRetryHeadroom buys: room for one
// retry of a request that hangs for its full timeout, and no more.
func TestDCIRetryClient_RequestTimeout(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		var requestCount atomic.Int64
		server := countingServer(t, &requestCount, neverRespond())

		client := newTestRetryClient(server, DefaultRequestTimeout, constantBackOff(retryInitialInterval))

		ctx, cancel := context.WithTimeout(t.Context(), DefaultReadTimeout)
		defer cancel()

		start := time.Now()
		_, err := doGet(ctx, t, client, server.URL)
		elapsed := time.Since(start)

		if err == nil {
			t.Fatal("expected timeout error, got nil")
		}
		if want := DefaultReadTimeout; elapsed != want {
			t.Errorf("elapsed = %v, want %v — the operation deadline must bound the run", elapsed, want)
		}
		if count := requestCount.Load(); count != 2 {
			t.Errorf("attempts = %d, want 2", count)
		}
	})
}

// TestDCIRetryClient_ContextCancellation verifies that a parent context
// cancellation propagates through the retry loop and stops retries immediately,
// even when the per-request timeout is far larger.
//
// The per-request timeout is deliberately inverted relative to this test's
// parent deadline. Such a value is legal — validateRequestTimeout errors only
// at or below the 120s edge timeout, and merely warns above the operation
// defaults, since raising the timeouts {} block to match is a legitimate thing
// to do — but it is not the shape the defaults describe, where the request
// timeout sits below the operation budget with headroom to retry. The inversion
// is the point: it leaves the parent context as the only thing that can end the
// run, which is what makes the assertion meaningful.
func TestDCIRetryClient_ContextCancellation(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		const (
			clientTimeout = 10 * time.Minute // deliberately never reached
			parentTimeout = 30 * time.Second // the real bound
		)

		var requestCount atomic.Int64
		server := countingServer(t, &requestCount, neverRespond())

		client := newTestRetryClient(server, clientTimeout, constantBackOff(retryInitialInterval))

		ctx, cancel := context.WithTimeout(t.Context(), parentTimeout)
		defer cancel()

		start := time.Now()
		_, err := doGet(ctx, t, client, server.URL)
		elapsed := time.Since(start)

		if err == nil {
			t.Fatal("expected context cancellation error, got nil")
		}
		if want := parentTimeout; elapsed != want {
			t.Errorf("elapsed = %v, want %v — the parent context, not the request timeout", elapsed, want)
		}

		// The handler is still parked on the cancelled request at this point, so
		// settle the bubble before reading what the server observed.
		synctest.Wait()
		if count := requestCount.Load(); count != 1 {
			t.Errorf("attempts = %d, want 1 — a cancelled context must not be retried", count)
		}
	})
}

// TestDCIRetryClient_RetryRespectsContextDeadline verifies that the retry loop
// defers to the context deadline (MaxElapsedTime = 0). Retries continue until
// the context is cancelled, not until a hardcoded elapsed time.
func TestDCIRetryClient_RetryRespectsContextDeadline(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		const (
			contextDeadline = DefaultReadTimeout

			// Deliberately not one of the production intervals: 45s does not
			// divide the deadline, so the last backoff timer and the deadline
			// never come due at the same virtual instant. If they did, the retry
			// loop's select over the two would pick either one and the attempt
			// count would vary between runs.
			retryInterval = 45 * time.Second
		)

		var requestCount atomic.Int64
		server := countingServer(t, &requestCount, func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusServiceUnavailable) // 503 triggers retry
		})

		client := newTestRetryClient(server, DefaultRequestTimeout, constantBackOff(retryInterval))

		ctx, cancel := context.WithTimeout(t.Context(), contextDeadline)
		defer cancel()

		start := time.Now()
		_, err := doGet(ctx, t, client, server.URL)
		elapsed := time.Since(start)

		if err == nil {
			t.Fatal("expected context deadline error, got nil")
		}
		if want := contextDeadline; elapsed != want {
			t.Errorf("elapsed = %v, want %v", elapsed, want)
		}
		// One attempt up front, then one per completed interval.
		if want := int64(contextDeadline/retryInterval) + 1; requestCount.Load() != want {
			t.Errorf("attempts = %d, want %d", requestCount.Load(), want)
		}
	})
}

// TestDCIRetryClient_CloudflareTimeoutIsPermanent is the regression test for the
// failure this timeout work addresses.
//
// A 524 means the API's edge already waited out the full origin timeout, so the
// response must fail fast rather than re-running an expensive query. Two things
// prove no retry happened: the request count, and an elapsed time of exactly
// zero — the injected backoff would have waited a full minute.
func TestDCIRetryClient_CloudflareTimeoutIsPermanent(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		var requestCount atomic.Int64
		server := countingServer(t, &requestCount, func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(httpStatusCloudflareTimeout)
			_, _ = w.Write([]byte("error code: 524"))
		})

		client := newTestRetryClient(server, DefaultRequestTimeout, constantBackOff(retryMaxInterval))

		// A deadline the expected path never reaches, so a regression that starts
		// retrying fails here instead of spinning virtual time indefinitely.
		ctx, cancel := context.WithTimeout(t.Context(), DefaultReadTimeout)
		defer cancel()

		start := time.Now()
		_, err := doGet(ctx, t, client, server.URL)
		elapsed := time.Since(start)

		if err == nil {
			t.Fatal("expected an error for 524, got nil")
		}
		if elapsed != 0 {
			t.Errorf("elapsed = %v, want 0 — a 524 must fail immediately", elapsed)
		}
		if count := requestCount.Load(); count != 1 {
			t.Errorf("attempts = %d, want 1 (524 must not be retried)", count)
		}
		// backoff.Retry unwraps PermanentError, so the caller sees the inner error
		// rather than a *backoff.PermanentError — assert on the message instead.
		if !strings.Contains(err.Error(), "524") {
			t.Errorf("error should mention the 524 status, got: %v", err)
		}
	})
}

// TestDCIRetryClient_429NoRetryAfter_Retries covers the DoiT API's actual
// observed behavior: 429 responses carry no Retry-After header, so the retry
// falls back to the exponential policy. This is therefore the production retry
// path, and it runs here at the production initial interval.
func TestDCIRetryClient_429NoRetryAfter_Retries(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		const failures = 2

		var requestCount atomic.Int64
		server := countingServer(t, &requestCount, func(w http.ResponseWriter, _ *http.Request) {
			if requestCount.Load() <= failures {
				w.WriteHeader(http.StatusTooManyRequests) // deliberately no Retry-After
				return
			}
			w.WriteHeader(http.StatusOK)
		})

		client := newTestRetryClient(server, DefaultRequestTimeout, constantBackOff(retryInitialInterval))

		ctx, cancel := context.WithTimeout(t.Context(), DefaultReadTimeout)
		defer cancel()

		start := time.Now()
		resp, err := doGet(ctx, t, client, server.URL)
		elapsed := time.Since(start)

		if err != nil {
			t.Fatalf("expected success after retries, got error: %v", err)
		}
		if resp.StatusCode != http.StatusOK {
			t.Errorf("StatusCode = %d, want %d", resp.StatusCode, http.StatusOK)
		}
		if count := requestCount.Load(); count != failures+1 {
			t.Errorf("attempts = %d, want %d", count, failures+1)
		}
		// One policy interval between each pair of attempts.
		if want := failures * retryInitialInterval; elapsed != want {
			t.Errorf("elapsed = %v, want %v", elapsed, want)
		}
	})
}

// TestDCIRetryClient_429WithRetryAfter_Honored verifies a server-supplied
// Retry-After takes precedence over the exponential policy. The injected policy
// would wait the entire operation budget, so completing in exactly the header's
// delay proves the header won.
func TestDCIRetryClient_429WithRetryAfter_Honored(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		const retryAfter = 30 * time.Second

		var requestCount atomic.Int64
		server := countingServer(t, &requestCount, func(w http.ResponseWriter, _ *http.Request) {
			if requestCount.Load() == 1 {
				w.Header().Set("Retry-After", strconv.Itoa(int(retryAfter.Seconds())))
				w.WriteHeader(http.StatusTooManyRequests)
				return
			}
			w.WriteHeader(http.StatusOK)
		})

		client := newTestRetryClient(server, DefaultRequestTimeout, constantBackOff(DefaultReadTimeout))

		ctx, cancel := context.WithTimeout(t.Context(), 2*DefaultReadTimeout)
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
			t.Errorf("attempts = %d, want 2", count)
		}
		if want := retryAfter; elapsed != want {
			t.Errorf("elapsed = %v, want %v — the header, not the injected policy", elapsed, want)
		}
	})
}

// TestDCIRetryClient_429RetryAfterBounds pins the clamping and fallback rules
// end to end, through the retry loop rather than against parseRetryAfter alone.
//
// The injected policy waits retryInitialInterval, so the expected elapsed time
// says which path ran: anything else came from the header, and a wait of exactly
// one policy interval means the header was rejected and the policy took over.
func TestDCIRetryClient_429RetryAfterBounds(t *testing.T) {
	for _, tc := range []struct {
		name   string
		header string
		want   time.Duration
	}{
		{"above the cap is clamped", "86400", maxRetryAfter},
		{"zero falls back to the policy", "0", retryInitialInterval},
		{"negative falls back to the policy", "-5", retryInitialInterval},
		{"unparseable falls back to the policy", "soon", retryInitialInterval},
	} {
		t.Run(tc.name, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				var requestCount atomic.Int64
				server := countingServer(t, &requestCount, func(w http.ResponseWriter, _ *http.Request) {
					if requestCount.Load() == 1 {
						w.Header().Set("Retry-After", tc.header)
						w.WriteHeader(http.StatusTooManyRequests)
						return
					}
					w.WriteHeader(http.StatusOK)
				})

				client := newTestRetryClient(server, DefaultRequestTimeout, constantBackOff(retryInitialInterval))

				ctx, cancel := context.WithTimeout(t.Context(), 2*DefaultReadTimeout)
				defer cancel()

				start := time.Now()
				resp, err := doGet(ctx, t, client, server.URL)
				elapsed := time.Since(start)

				if err != nil {
					t.Fatalf("expected success after one retry, got error: %v", err)
				}
				if resp.StatusCode != http.StatusOK {
					t.Errorf("StatusCode = %d, want %d", resp.StatusCode, http.StatusOK)
				}
				if count := requestCount.Load(); count != 2 {
					t.Errorf("attempts = %d, want 2", count)
				}
				if elapsed != tc.want {
					t.Errorf("elapsed = %v, want %v", elapsed, tc.want)
				}
			})
		})
	}
}

// TestDCIRetryClient_404PassesThrough guards the client's most load-bearing
// documented behavior: a 404 is returned as a normal response, not an error, so
// resource handlers can interpret it contextually (Read: externally deleted,
// Delete: already gone).
func TestDCIRetryClient_404PassesThrough(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		var requestCount atomic.Int64
		server := countingServer(t, &requestCount, func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusNotFound)
		})

		client := newTestRetryClient(server, DefaultRequestTimeout, constantBackOff(retryMaxInterval))

		ctx, cancel := context.WithTimeout(t.Context(), DefaultReadTimeout)
		defer cancel()

		start := time.Now()
		resp, err := doGet(ctx, t, client, server.URL)
		elapsed := time.Since(start)

		if err != nil {
			t.Fatalf("404 must not be an error, got: %v", err)
		}
		if resp.StatusCode != http.StatusNotFound {
			t.Errorf("StatusCode = %d, want %d", resp.StatusCode, http.StatusNotFound)
		}
		if elapsed != 0 {
			t.Errorf("elapsed = %v, want 0 — a 404 must not wait", elapsed)
		}
		if count := requestCount.Load(); count != 1 {
			t.Errorf("attempts = %d, want 1 (404 must not be retried)", count)
		}
	})
}

// TestDCIRetryClient_500NotRetried guards the documented decision not to retry
// 500, unlike 502/503/504.
func TestDCIRetryClient_500NotRetried(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		var requestCount atomic.Int64
		server := countingServer(t, &requestCount, func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
		})

		client := newTestRetryClient(server, DefaultRequestTimeout, constantBackOff(retryMaxInterval))

		ctx, cancel := context.WithTimeout(t.Context(), DefaultReadTimeout)
		defer cancel()

		start := time.Now()
		_, err := doGet(ctx, t, client, server.URL)
		elapsed := time.Since(start)

		if err == nil {
			t.Fatal("expected an error for 500, got nil")
		}
		if elapsed != 0 {
			t.Errorf("elapsed = %v, want 0 — a 500 must fail immediately", elapsed)
		}
		if count := requestCount.Load(); count != 1 {
			t.Errorf("attempts = %d, want 1 (500 must not be retried)", count)
		}
	})
}

// TestNewClient_CustomTimeout verifies that NewClient wires the requested
// per-request timeout through to the http.Client that actually enforces it.
//
// Asserting on the wiring rather than on observed traffic is deliberate:
// NewClient builds its own &http.Client{Timeout: ...} with the default
// transport, so a request it issued would leave for the real network and could
// never be seen by an in-memory test server. A request counter therefore cannot
// say anything about NewClient at all — see TestNewClientNoConstructorIO, which
// asserts the no-I/O property against the injectable constructor where the
// counter is genuinely observable.
func TestNewClient_CustomTimeout(t *testing.T) {
	t.Parallel()

	const (
		host           = "https://api.example.test"
		requestTimeout = 42 * time.Second
	)

	client, err := NewClient(host, "test-token", "", "1.0.0", "dev", requestTimeout)
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}
	if client == nil {
		t.Fatal("NewClient() returned nil client")
	}

	if got, want := requestTimeoutOf(t, client), requestTimeout; got != want {
		t.Errorf("request timeout = %v, want %v", got, want)
	}
	if got, want := serverOf(t, client), host+"/"; got != want {
		t.Errorf("server = %q, want %q", got, want)
	}
}

// TestNewClient_DefaultTimeout verifies that DefaultRequestTimeout is accepted
// and reaches the same place a custom value does.
func TestNewClient_DefaultTimeout(t *testing.T) {
	t.Parallel()

	client, err := NewClient(
		"https://api.example.test", "test-token", "", "1.0.0", "dev", DefaultRequestTimeout,
	)
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}
	if client == nil {
		t.Fatal("NewClient() returned nil client")
	}

	if got, want := requestTimeoutOf(t, client), DefaultRequestTimeout; got != want {
		t.Errorf("request timeout = %v, want %v", got, want)
	}
}

// TestNewClientNoConstructorIO verifies that building a client performs no
// network I/O — no token exchange, no discovery, no health check.
//
// This goes through newClientWithHTTPClient, the constructor NewClient
// delegates to, so the injected client is the test server's and any request the
// constructor made would reach the counter. Through NewClient itself the
// assertion would be unfalsifiable: its own http.Client would not route here.
func TestNewClientNoConstructorIO(t *testing.T) {
	t.Parallel()

	var reqCount atomic.Int64
	server := countingServer(t, &reqCount, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	// Start the server in its own statement rather than inline below. Go
	// specifies the order of calls relative to each other, but not relative to
	// an ordinary field read in the same argument list — the spec's own example
	// is `[]int{a, f()}` — so `server.Client()` is not guaranteed to run before
	// `server.URL` is read. It does today, but this assertion is one that passes
	// when the plumbing is broken as readily as when the property holds, so it
	// should not rest on that.
	httpClient := server.Client()

	client, err := newClientWithHTTPClient(
		server.URL, "test-token", "", "1.0.0", "dev", httpClient,
	)
	if err != nil {
		t.Fatalf("newClientWithHTTPClient() error = %v", err)
	}
	if client == nil {
		t.Fatal("newClientWithHTTPClient() returned nil client")
	}
	if got := reqCount.Load(); got != 0 {
		t.Errorf("construction made %d HTTP requests, want 0", got)
	}
}

// requestTimeoutOf digs out the timeout on the http.Client buried inside a
// generated client, so a test can assert the value survived the trip through
// DCIRetryClient and models.NewClientWithResponses.
func requestTimeoutOf(t *testing.T, client *models.ClientWithResponses) time.Duration {
	t.Helper()

	retryClient, ok := doerOf(t, client).(*DCIRetryClient)
	if !ok {
		t.Fatalf("request doer is %T, want *DCIRetryClient", doerOf(t, client))
	}
	return retryClient.client.Timeout
}

// serverOf returns the base URL the generated client will resolve paths against.
func serverOf(t *testing.T, client *models.ClientWithResponses) string {
	t.Helper()

	inner, ok := client.ClientInterface.(*models.Client)
	if !ok {
		t.Fatalf("ClientInterface is %T, want *models.Client", client.ClientInterface)
	}
	return inner.Server
}

func doerOf(t *testing.T, client *models.ClientWithResponses) models.HttpRequestDoer {
	t.Helper()

	inner, ok := client.ClientInterface.(*models.Client)
	if !ok {
		t.Fatalf("ClientInterface is %T, want *models.Client", client.ClientInterface)
	}
	return inner.Client
}
