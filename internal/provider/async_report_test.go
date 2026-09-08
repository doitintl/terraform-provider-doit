package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"testing/synctest"
	"time"

	"github.com/doitintl/terraform-provider-doit/internal/provider/models"
)

// These tests drive the async polling helper against a fake operations API.
//
// They run inside synctest bubbles so the poll waits — which are seconds to
// minutes of API-directed Retry-After — cost no real time. That matters twice
// over: the whole unit suite runs under a 120s budget, and the timeout tests
// need a deadline to actually elapse, which is only practical with a fake clock.
//
// Note the absence of t.Parallel(): a synctest bubble owns its own clock and
// requires every goroutine in it to be durably blocked before time advances, so
// the bubble must be the test's whole world. httptest.NewTestServer is what
// makes this work at all — its in-memory network is durably blocking, where a
// real TCP listener would not be.

// asyncTestServer is a scriptable stand-in for the async report API.
type asyncTestServer struct {
	*httptest.Server

	mu sync.Mutex

	// script
	pollStatuses  []string // consumed one per poll; last value repeats
	pollRetryHdr  string   // Retry-After sent on non-terminal polls
	failureError  *models.AsyncOperationError
	resultsStatus []int // consumed one per results call; last value repeats
	submitStatus  int
	submitEmpty   int  // number of initial submits answered with an empty 202
	cancelStatus  int  // status returned by the cancel endpoint
	pollNotFound  bool // poll returns 404

	// observations
	submitKeys  []string
	cancelKeys  []string
	pollCount   int
	resultCount int
	pollDelays  []time.Duration
	lastPollAt  time.Time
}

func newAsyncTestServer(t *testing.T, s *asyncTestServer) *asyncTestServer {
	t.Helper()

	if s.submitStatus == 0 {
		s.submitStatus = http.StatusAccepted
	}
	if s.cancelStatus == 0 {
		s.cancelStatus = http.StatusOK
	}
	if len(s.resultsStatus) == 0 {
		s.resultsStatus = []int{http.StatusOK}
	}

	mux := http.NewServeMux()

	submit := func(w http.ResponseWriter, r *http.Request) {
		s.mu.Lock()
		defer s.mu.Unlock()

		s.submitKeys = append(s.submitKeys, r.Header.Get("Idempotency-Key"))

		if s.submitStatus != http.StatusAccepted {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(s.submitStatus)
			_, _ = w.Write([]byte(`{"error":"not found"}`))
			return
		}

		// The idempotent-replay case, reproduced as the live API sends it: 202
		// with no body, no Location, and no Content-Type. Deliberately not
		// JSON-typed — an empty body labelled application/json would fail to
		// unmarshal, which is a different failure mode than the one under test.
		if len(s.submitKeys) <= s.submitEmpty {
			w.Header().Set("Idempotency-Replayed", "true")
			w.WriteHeader(http.StatusAccepted)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Location", "/analytics/v1/reports/operations/op-1")
		w.WriteHeader(http.StatusAccepted)
		_, _ = w.Write([]byte(`{"operationId":"op-1","status":"pending","createTime":"2000-01-01T00:00:00Z"}`))
	}

	mux.HandleFunc("POST /analytics/v1/reports/actions/run", submit)
	mux.HandleFunc("POST /analytics/v1/reports/{id}/actions/run", submit)

	mux.HandleFunc("GET /analytics/v1/reports/operations/{operationId}", func(w http.ResponseWriter, _ *http.Request) {
		s.mu.Lock()
		defer s.mu.Unlock()

		now := time.Now()
		if !s.lastPollAt.IsZero() {
			s.pollDelays = append(s.pollDelays, now.Sub(s.lastPollAt))
		}
		s.lastPollAt = now
		s.pollCount++

		w.Header().Set("Content-Type", "application/json")
		if s.pollNotFound {
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte(`{"error":"operation not found"}`))
			return
		}

		status := s.pollStatuses[min(s.pollCount-1, len(s.pollStatuses)-1)]
		body := map[string]any{
			"operationId": "op-1",
			"status":      status,
			"createTime":  "2000-01-01T00:00:00Z",
			"expireBy":    "2000-01-08T00:00:00Z",
		}
		if status == "failed" && s.failureError != nil {
			body["error"] = s.failureError
		}
		if status == "pending" || status == "running" {
			if s.pollRetryHdr != "" {
				w.Header().Set("Retry-After", s.pollRetryHdr)
			}
		}
		_ = json.NewEncoder(w).Encode(body)
	})

	mux.HandleFunc("GET /analytics/v1/reports/operations/{operationId}/results", func(w http.ResponseWriter, _ *http.Request) {
		s.mu.Lock()
		defer s.mu.Unlock()

		s.resultCount++
		status := s.resultsStatus[min(s.resultCount-1, len(s.resultsStatus)-1)]

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		switch status {
		case http.StatusOK:
			_, _ = w.Write([]byte(`{"id":"rpt-1","reportName":"Test Report","result":{` +
				`"schema":[{"name":"cost","type":"float"}],` +
				`"rows":[["a",1],["b",2]],"cacheHit":false}}`))
		default:
			_, _ = w.Write([]byte(`{"error":"result unavailable"}`))
		}
	})

	mux.HandleFunc("POST /analytics/v1/reports/operations/{operationId}/actions/cancel", func(w http.ResponseWriter, r *http.Request) {
		s.mu.Lock()
		defer s.mu.Unlock()

		s.cancelKeys = append(s.cancelKeys, r.Header.Get("Idempotency-Key"))
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(s.cancelStatus)
		_, _ = w.Write([]byte(`{"operationId":"op-1","status":"canceled"}`))
	})

	s.Server = httptest.NewTestServer(t, mux)
	return s
}

func (s *asyncTestServer) snapshot() (submits, cancels []string, polls, results int, delays []time.Duration) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]string(nil), s.submitKeys...),
		append([]string(nil), s.cancelKeys...),
		s.pollCount, s.resultCount,
		append([]time.Duration(nil), s.pollDelays...)
}

// newAsyncTestClient builds a generated client that routes to the test server
// through the real DCIRetryClient, with a fast backoff so an incidental retry
// does not dominate a timing assertion.
func newAsyncTestClient(t *testing.T, server *httptest.Server) *models.ClientWithResponses {
	t.Helper()
	return newAsyncTestClientWithRequestTimeout(t, server, DefaultRequestTimeout)
}

// newAsyncTestClientWithRequestTimeout is newAsyncTestClient with control over
// the per-request timeout, for asserting what that layer does and does not bound.
func newAsyncTestClientWithRequestTimeout(
	t *testing.T, server *httptest.Server, requestTimeout time.Duration,
) *models.ClientWithResponses {
	t.Helper()

	retryClient := newTestRetryClient(server, requestTimeout, constantBackOff(time.Millisecond))
	client, err := models.NewClientWithResponses(server.URL, models.WithHTTPClient(retryClient))
	if err != nil {
		t.Fatalf("models.NewClientWithResponses() error = %v", err)
	}
	return client
}

// submitViaInline drives submitAsyncReport against the inline run endpoint.
func submitViaInline(ctx context.Context, client *models.ClientWithResponses) (string, error) {
	operationID, diags := submitAsyncReport(ctx, "query",
		func(ctx context.Context, idempotencyKey string) (asyncSubmission, error) {
			runResp, err := client.AsyncRunInlineWithResponse(ctx,
				&models.AsyncRunInlineParams{IdempotencyKey: idempotencyKey},
				models.AsyncRunInlineJSONRequestBody{})
			if err != nil {
				return asyncSubmission{}, err
			}
			return asyncSubmission{
				StatusCode:  runResp.StatusCode(),
				OperationID: asyncOperationID(runResp.JSON202, runResp.JSON200),
				Body:        runResp.Body,
			}, nil
		})
	if diags.HasError() {
		return "", fmt.Errorf("%s: %s", diags.Errors()[0].Summary(), diags.Errors()[0].Detail())
	}
	return operationID, nil
}

func TestAwaitAsyncReport_SucceedsAfterPolling(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		srv := newAsyncTestServer(t, &asyncTestServer{
			pollStatuses: []string{"pending", "running", "running", "succeeded"},
			pollRetryHdr: "5",
		})
		client := newAsyncTestClient(t, srv.Server)

		start := time.Now()
		results, diags := awaitAsyncReport(t.Context(), client, "query", "op-1")
		elapsed := time.Since(start)

		if diags.HasError() {
			t.Fatalf("unexpected diagnostics: %v", diags.Errors())
		}
		if results.Result == nil || results.Result.Rows == nil {
			t.Fatal("expected rows in result")
		}
		if got := len(*results.Result.Rows); got != 2 {
			t.Errorf("rows = %d, want 2", got)
		}
		// Three non-terminal polls, each followed by a 5s Retry-After wait.
		if want := 15 * time.Second; elapsed != want {
			t.Errorf("virtual elapsed = %v, want %v", elapsed, want)
		}
	})
}

func TestAwaitAsyncReport_HonorsRetryAfter(t *testing.T) {
	for _, tc := range []struct {
		name   string
		header string
		want   time.Duration
	}{
		{"api guidance", "5", 5 * time.Second},
		{"at ceiling", "30", asyncPollMaxInterval},
		{"above ceiling is clamped", "86400", asyncPollMaxInterval},
		{"zero falls back", "0", asyncPollInitialInterval},
		{"garbage falls back", "soon", asyncPollInitialInterval},
		{"absent falls back", "", asyncPollInitialInterval},
	} {
		t.Run(tc.name, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				srv := newAsyncTestServer(t, &asyncTestServer{
					pollStatuses: []string{"running", "succeeded"},
					pollRetryHdr: tc.header,
				})
				client := newAsyncTestClient(t, srv.Server)

				start := time.Now()
				if _, diags := awaitAsyncReport(t.Context(), client, "query", "op-1"); diags.HasError() {
					t.Fatalf("unexpected diagnostics: %v", diags.Errors())
				}

				if elapsed := time.Since(start); elapsed != tc.want {
					t.Errorf("poll wait = %v, want %v", elapsed, tc.want)
				}
			})
		})
	}
}

// TestAwaitAsyncReport_RequestTimeoutDoesNotBoundTheRun proves the property the
// documentation claims: the provider's request_timeout bounds one HTTP request,
// not the report. A run far longer than request_timeout still succeeds, because
// the waiting happens between requests rather than inside one.
//
// The request timeout here (1s) is an order of magnitude below the total run
// (55s of polling) and below a single Retry-After interval (5s), so if it were
// wrongly applied to the operation as a whole this could not pass.
func TestAwaitAsyncReport_RequestTimeoutDoesNotBoundTheRun(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		const requestTimeout = 1 * time.Second

		srv := newAsyncTestServer(t, &asyncTestServer{
			pollStatuses: []string{
				"running", "running", "running", "running", "running",
				"running", "running", "running", "running", "running",
				"running", "succeeded",
			},
			pollRetryHdr: "5",
		})
		client := newAsyncTestClientWithRequestTimeout(t, srv.Server, requestTimeout)

		start := time.Now()
		results, diags := awaitAsyncReport(t.Context(), client, "query", "op-1")
		elapsed := time.Since(start)

		if diags.HasError() {
			t.Fatalf("run failed under a short request_timeout: %v", diags.Errors())
		}
		if results.Result == nil {
			t.Fatal("expected a result")
		}
		// 11 non-terminal polls, each followed by a 5s wait.
		if want := 55 * time.Second; elapsed != want {
			t.Errorf("elapsed = %v, want %v", elapsed, want)
		}
		if elapsed <= requestTimeout {
			t.Errorf("run (%v) did not outlast request_timeout (%v), so this proves nothing",
				elapsed, requestTimeout)
		}
	})
}

// TestAwaitAsyncReport_RequestTimeoutStillBoundsOneRequest is the other half of
// the claim: request_timeout is inert for the run as a whole, but a single
// unresponsive request is still cut off by it rather than hanging forever.
func TestAwaitAsyncReport_RequestTimeoutStillBoundsOneRequest(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		const requestTimeout = 2 * time.Second

		// A poll that never answers on its own.
		var requests atomic.Int64
		srv := countingServer(t, &requests, respondAfter(time.Hour, http.StatusOK))
		client := newAsyncTestClientWithRequestTimeout(t, srv, requestTimeout)

		ctx, stop := context.WithTimeout(t.Context(), 30*time.Second)
		defer stop()

		_, diags := awaitAsyncReport(ctx, client, "query", "op-1")
		if !diags.HasError() {
			t.Fatal("expected an error when the request never completes")
		}

		// The request count is what discriminates here: both a live and a dead
		// per-request timeout end in an error, but only a live one cuts each
		// attempt short and retries. Without it there would be exactly one
		// attempt, hanging until the context expired.
		if got := requests.Load(); got < 2 {
			t.Errorf("server saw %d request(s), want several — request_timeout never fired", got)
		}
	})
}

// TestAwaitAsyncReport_TimeoutCancelsOperation is the headline case: when the
// caller's Read timeout expires mid-run, the operation must be cancelled rather
// than left burning query capacity.
func TestAwaitAsyncReport_TimeoutCancelsOperation(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		srv := newAsyncTestServer(t, &asyncTestServer{
			pollStatuses: []string{"running"}, // never completes
			pollRetryHdr: "5",
		})
		client := newAsyncTestClient(t, srv.Server)

		ctx, stop := context.WithTimeout(t.Context(), 30*time.Second)
		defer stop()

		_, diags := awaitAsyncReport(ctx, client, "query", "op-1")

		if !diags.HasError() {
			t.Fatal("expected a timeout error")
		}
		summary := diags.Errors()[0].Summary()
		detail := diags.Errors()[0].Detail()
		if !strings.Contains(summary, "Timed Out") {
			t.Errorf("summary = %q, want it to mention the timeout", summary)
		}
		for _, want := range []string{"op-1", "canceled", "read = \"30m\""} {
			if !strings.Contains(detail, want) {
				t.Errorf("detail missing %q:\n%s", want, detail)
			}
		}

		_, cancels, _, _, _ := srv.snapshot()
		if len(cancels) != 1 {
			t.Fatalf("cancel calls = %d, want exactly 1", len(cancels))
		}
		if cancels[0] == "" {
			t.Error("cancel request carried no Idempotency-Key")
		}
	})
}

// TestAwaitAsyncReport_ContextCancellationCancelsOperation covers the Ctrl-C
// path: Terraform cancels the context on interrupt, which should free the
// server-side job just as a deadline does.
func TestAwaitAsyncReport_ContextCancellationCancelsOperation(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		srv := newAsyncTestServer(t, &asyncTestServer{
			pollStatuses: []string{"running"},
			pollRetryHdr: "5",
		})
		client := newAsyncTestClient(t, srv.Server)

		ctx, cancel := context.WithCancel(t.Context())
		go func() {
			time.Sleep(12 * time.Second)
			cancel()
		}()

		_, diags := awaitAsyncReport(ctx, client, "query", "op-1")
		if !diags.HasError() {
			t.Fatal("expected an error after cancellation")
		}

		_, cancels, _, _, _ := srv.snapshot()
		if len(cancels) != 1 {
			t.Errorf("cancel calls = %d, want exactly 1", len(cancels))
		}
	})
}

// TestAwaitAsyncReport_TimeoutDuringResultsDoesNotCancel guards against
// cancelling an operation that already finished — there is nothing to cancel,
// and the API would reject it.
func TestAwaitAsyncReport_TimeoutDuringResultsDoesNotCancel(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		srv := newAsyncTestServer(t, &asyncTestServer{
			pollStatuses: []string{"succeeded"},
			// Never becomes readable, so the results loop runs until the deadline.
			resultsStatus: []int{http.StatusTooEarly},
		})
		client := newAsyncTestClient(t, srv.Server)

		ctx, stop := context.WithTimeout(t.Context(), 10*time.Second)
		defer stop()

		_, diags := awaitAsyncReport(ctx, client, "query", "op-1")
		if !diags.HasError() {
			t.Fatal("expected an error")
		}

		_, cancels, _, results, _ := srv.snapshot()
		if len(cancels) != 0 {
			t.Errorf("cancel calls = %d, want 0 — the operation already succeeded", len(cancels))
		}
		if results < 2 {
			t.Errorf("results attempts = %d, want the 425 to have been retried", results)
		}
	})
}

func TestAwaitAsyncReport_CancelFailureDoesNotMaskTimeout(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		srv := newAsyncTestServer(t, &asyncTestServer{
			pollStatuses: []string{"running"},
			pollRetryHdr: "5",
			cancelStatus: http.StatusInternalServerError,
		})
		client := newAsyncTestClient(t, srv.Server)

		ctx, stop := context.WithTimeout(t.Context(), 20*time.Second)
		defer stop()

		_, diags := awaitAsyncReport(ctx, client, "query", "op-1")

		if !diags.HasError() {
			t.Fatal("expected the timeout to still be reported as an error")
		}
		if !strings.Contains(diags.Errors()[0].Summary(), "Timed Out") {
			t.Errorf("timeout was masked by the cancel failure: %v", diags.Errors()[0])
		}
		if diags.WarningsCount() != 1 {
			t.Fatalf("warnings = %d, want 1 describing the failed cancel", diags.WarningsCount())
		}
		if !strings.Contains(diags.Warnings()[0].Detail(), "op-1") {
			t.Errorf("cancel warning should name the operation: %s", diags.Warnings()[0].Detail())
		}
	})
}

func TestAwaitAsyncReport_OperationFailed(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		detail := "The query scanned too much data."
		srv := newAsyncTestServer(t, &asyncTestServer{
			pollStatuses: []string{"failed"},
			failureError: &models.AsyncOperationError{
				Title:  "Query result set is too large",
				Code:   "result_too_large",
				Status: 413,
				Detail: &detail,
			},
		})
		client := newAsyncTestClient(t, srv.Server)

		_, diags := awaitAsyncReport(t.Context(), client, "query", "op-1")
		if !diags.HasError() {
			t.Fatal("expected an error")
		}

		got := diags.Errors()[0].Detail()
		for _, want := range []string{"op-1", "Query result set is too large", "result_too_large", "413", detail} {
			if !strings.Contains(got, want) {
				t.Errorf("detail missing %q:\n%s", want, got)
			}
		}
	})
}

func TestAwaitAsyncReport_TerminalStatesAndErrors(t *testing.T) {
	// Pointers, not values: asyncTestServer carries a mutex and must not be copied.
	for _, tc := range []struct {
		name    string
		server  *asyncTestServer
		wantMsg string
	}{
		{
			name:    "externally canceled",
			server:  &asyncTestServer{pollStatuses: []string{"canceled"}},
			wantMsg: "was canceled",
		},
		{
			name:    "poll 404",
			server:  &asyncTestServer{pollStatuses: []string{"running"}, pollNotFound: true},
			wantMsg: "no longer exists",
		},
		{
			name: "results 422",
			server: &asyncTestServer{
				pollStatuses:  []string{"succeeded"},
				resultsStatus: []int{http.StatusUnprocessableEntity},
			},
			// 422 stays a transport-level error from DCIRetryClient (unlike 425,
			// which is passed through so the poll loop can retry it), but the
			// status and body still reach the user.
			wantMsg: "422",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				srv := newAsyncTestServer(t, tc.server)
				client := newAsyncTestClient(t, srv.Server)

				_, diags := awaitAsyncReport(t.Context(), client, "query", "op-1")
				if !diags.HasError() {
					t.Fatal("expected an error")
				}
				if got := diags.Errors()[0].Detail(); !strings.Contains(got, tc.wantMsg) {
					t.Errorf("detail = %q, want it to contain %q", got, tc.wantMsg)
				}
			})
		})
	}
}

// TestAwaitAsyncReport_Results425ThenSucceeds covers the window where the
// operation reports success before its result is readable.
func TestAwaitAsyncReport_Results425ThenSucceeds(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		srv := newAsyncTestServer(t, &asyncTestServer{
			pollStatuses:  []string{"succeeded"},
			resultsStatus: []int{http.StatusTooEarly, http.StatusOK},
		})
		client := newAsyncTestClient(t, srv.Server)

		start := time.Now()
		results, diags := awaitAsyncReport(t.Context(), client, "query", "op-1")
		if diags.HasError() {
			t.Fatalf("unexpected diagnostics: %v", diags.Errors())
		}
		if results.Result == nil {
			t.Fatal("expected a result after the retry")
		}
		// The 425 carries no Retry-After, so the fallback interval applies.
		if elapsed := time.Since(start); elapsed != asyncPollInitialInterval {
			t.Errorf("elapsed = %v, want %v", elapsed, asyncPollInitialInterval)
		}
	})
}

// TestSubmitAsyncReport_EmptyIdempotentReplayResubmits covers the API returning
// 202 with no body when a key is replayed — recoverable only because dedup is
// content-based, so a fresh key attaches to the same in-flight operation.
func TestSubmitAsyncReport_EmptyIdempotentReplayResubmits(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		srv := newAsyncTestServer(t, &asyncTestServer{submitEmpty: 1})
		client := newAsyncTestClient(t, srv.Server)

		operationID, err := submitViaInline(t.Context(), client)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if operationID != "op-1" {
			t.Errorf("operationID = %q, want %q", operationID, "op-1")
		}

		submits, _, _, _, _ := srv.snapshot()
		if len(submits) != 2 {
			t.Fatalf("submits = %d, want exactly 2", len(submits))
		}
		if submits[0] == submits[1] {
			t.Error("re-submit reused the Idempotency-Key; it must be fresh")
		}
		for i, key := range submits {
			if key == "" {
				t.Errorf("submit %d carried no Idempotency-Key", i)
			}
		}
	})
}

func TestSubmitAsyncReport_EmptyReplayResubmitsOnlyOnce(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		srv := newAsyncTestServer(t, &asyncTestServer{submitEmpty: 99})
		client := newAsyncTestClient(t, srv.Server)

		if _, err := submitViaInline(t.Context(), client); err == nil {
			t.Fatal("expected an error when the replay repeats")
		}

		submits, _, _, _, _ := srv.snapshot()
		if len(submits) != 2 {
			t.Errorf("submits = %d, want exactly 2 — must not retry indefinitely", len(submits))
		}
	})
}

// TestSubmitAsyncReport_NotFound guards the acceptance test's error regex: the
// message must carry the raw status and body, not just a friendly hint.
func TestSubmitAsyncReport_NotFound(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		srv := newAsyncTestServer(t, &asyncTestServer{submitStatus: http.StatusNotFound})
		client := newAsyncTestClient(t, srv.Server)

		_, err := submitViaInline(t.Context(), client)
		if err == nil {
			t.Fatal("expected an error")
		}
		for _, want := range []string{"404", "not found"} {
			if !strings.Contains(err.Error(), want) {
				t.Errorf("error missing %q: %v", want, err)
			}
		}
	})
}

func TestNewIdempotencyKey_Unique(t *testing.T) {
	t.Parallel()

	seen := make(map[string]bool, 100)
	for range 100 {
		key := newIdempotencyKey()
		if key == "" {
			t.Fatal("newIdempotencyKey() returned an empty string")
		}
		if seen[key] {
			t.Fatalf("newIdempotencyKey() returned a duplicate: %s", key)
		}
		seen[key] = true
	}
}

func TestParseRetryAfterBounded(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)

	for _, tc := range []struct {
		name    string
		header  string
		minWait time.Duration
		maxWait time.Duration
		want    time.Duration
		wantOK  bool
	}{
		{"seconds within bounds", "5", time.Second, 30 * time.Second, 5 * time.Second, true},
		{"below floor is raised", "1", 2 * time.Second, 30 * time.Second, 2 * time.Second, true},
		{"above ceiling is capped", "86400", time.Second, 30 * time.Second, 30 * time.Second, true},
		{"zero rejected", "0", time.Second, 30 * time.Second, 0, false},
		{"negative rejected", "-5", time.Second, 30 * time.Second, 0, false},
		{"empty rejected", "", time.Second, 30 * time.Second, 0, false},
		{"garbage rejected", "soon", time.Second, 30 * time.Second, 0, false},
		{"http-date honored", now.Add(10 * time.Second).Format(http.TimeFormat), time.Second, 30 * time.Second, 10 * time.Second, true},
		{"past http-date rejected", now.Add(-time.Hour).Format(http.TimeFormat), time.Second, 30 * time.Second, 0, false},
		{"poll bounds differ from retry bounds", "1", asyncPollInitialInterval, asyncPollMaxInterval, time.Second, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got, ok := parseRetryAfterBounded(tc.header, now, tc.minWait, tc.maxWait)
			if ok != tc.wantOK {
				t.Fatalf("ok = %v, want %v", ok, tc.wantOK)
			}
			if ok && got != tc.want {
				t.Errorf("wait = %s, want %s", got, tc.want)
			}
		})
	}
}
