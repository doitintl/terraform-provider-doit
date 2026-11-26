package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"testing/synctest"
	"time"

	"github.com/doitintl/terraform-provider-doit/internal/provider/models"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/oapi-codegen/nullable"
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

// awaitAsyncReport polls operationID to a terminal state and returns its result:
// runAsyncReport without the submit step and the queue, for tests that start
// from an operation the fake API already has.
//
//nolint:unparam // the fake API serves one operation, "op-1", in most tests
func awaitAsyncReport(
	ctx context.Context,
	client *models.ClientWithResponses,
	what, operationID string,
	fileOutput *models.GetAsyncOperationResultsParamsFileOutput,
) (*models.GetAsyncOperationResults200Response, diag.Diagnostics) {
	if _, diags := pollAsyncOperation(ctx, client, what, operationID); diags.HasError() {
		return nil, diags
	}
	return fetchAsyncReportResults(ctx, client, what, operationID, fileOutput)
}

// activeOperationsLimitBody is the 429 body the API sends when a caller already
// has the maximum number of pending or running report operations (3).
const activeOperationsLimitBody = `{"type":"https://developer.doit.com/errors/too_many_active_operations_for_user",` +
	`"title":"Too Many Active Operations","status":429,` +
	`"detail":"You have reached the limit of 3 active report operations. Wait for one of the listed operations to finish, or cancel it, then retry.",` +
	`"code":"too_many_active_operations_for_user","scope":"actor","limit":3,` +
	`"operationIds":["op-a","op-b","op-c"]}`

// asyncTestServer is a scriptable stand-in for the async report API.
type asyncTestServer struct {
	*httptest.Server

	mu sync.Mutex

	// script
	pollStatuses   []string // consumed one per poll; last value repeats
	pollRetryHdr   string   // Retry-After sent on non-terminal polls
	failureError   *models.AsyncOperationError
	resultsStatus  []int  // consumed one per results call; last value repeats
	resultsBody    string // if set, returned on http.StatusOK from results endpoint
	resultsErrBody string // if set, returned (as problem+json) on a non-OK results status
	submitStatus   int
	submitEmpty    int           // number of initial submits answered with a 202 carrying no operation ID
	submitLimited  int           // number of initial submits answered with the active-operations 429
	submitDelay    time.Duration // how long the submit handler withholds its response
	cancelStatus   int           // status returned by the cancel endpoint
	cancelOpState  string        // operation status reported in the cancel response body
	pollNotFound   bool          // poll returns 404

	// observations
	submitKeys    []string
	cancelKeys    []string
	pollCount     int
	resultCount   int
	resultQueries []string
	pollDelays    []time.Duration
	lastPollAt    time.Time
}

func newAsyncTestServer(t *testing.T, s *asyncTestServer) *asyncTestServer {
	t.Helper()

	if s.submitStatus == 0 {
		s.submitStatus = http.StatusAccepted
	}
	if s.cancelStatus == 0 {
		s.cancelStatus = http.StatusOK
	}
	if s.cancelOpState == "" {
		s.cancelOpState = "canceled"
	}
	if len(s.resultsStatus) == 0 {
		s.resultsStatus = []int{http.StatusOK}
	}

	mux := http.NewServeMux()

	submit := func(w http.ResponseWriter, r *http.Request) {
		key := r.Header.Get("Idempotency-Key")

		s.mu.Lock()
		seenBefore := slices.Contains(s.submitKeys, key)
		s.submitKeys = append(s.submitKeys, key)
		delay := s.submitDelay
		s.mu.Unlock()

		// Only a first submission is slow. Replaying a key the server has
		// already accepted is a lookup, so it answers promptly — which is what
		// makes recovering an abandoned submission possible at all.
		//
		// The wait is held outside the lock so a slow submit does not block the
		// cancel the abandoning caller issues concurrently. The operation counts
		// as accepted the moment the handler is entered: the server has it, the
		// client never sees the response.
		if delay > 0 && !seenBefore {
			select {
			case <-time.After(delay):
			case <-r.Context().Done():
				return
			}
		}

		s.mu.Lock()
		defer s.mu.Unlock()

		if seenBefore {
			w.Header().Set("Idempotency-Replayed", "true")
		}

		// The per-caller cap on pending or running operations, as the API
		// answers it (captured from the CI tenant): a problem+json body naming
		// the caller's active operations, and a Retry-After of 30 seconds.
		if len(s.submitKeys) <= s.submitLimited {
			w.Header().Set("Content-Type", "application/problem+json")
			w.Header().Set("Retry-After", "30")
			w.WriteHeader(http.StatusTooManyRequests)
			_, _ = w.Write([]byte(activeOperationsLimitBody))
			return
		}

		if s.submitStatus != http.StatusAccepted {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(s.submitStatus)
			_, _ = w.Write([]byte(`{"error":"not found"}`))
			return
		}

		// A contract-violating 202: accepted, but with no operation ID for the
		// caller to track. No Content-Type, so the body is genuinely absent
		// rather than an empty document that fails to unmarshal — the latter is
		// a different failure mode than the one under test.
		if len(s.submitKeys) <= s.submitEmpty {
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

	mux.HandleFunc("GET /analytics/v1/reports/operations/{operationId}/results", func(w http.ResponseWriter, r *http.Request) {
		s.mu.Lock()
		defer s.mu.Unlock()

		s.resultCount++
		s.resultQueries = append(s.resultQueries, r.URL.RawQuery)
		status := s.resultsStatus[min(s.resultCount-1, len(s.resultsStatus)-1)]

		if status != http.StatusOK && s.resultsErrBody != "" {
			w.Header().Set("Content-Type", "application/problem+json")
			w.WriteHeader(status)
			_, _ = w.Write([]byte(s.resultsErrBody))
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		switch status {
		case http.StatusOK:
			if s.resultsBody != "" {
				_, _ = w.Write([]byte(s.resultsBody))
			} else {
				_, _ = w.Write([]byte(`{"id":"rpt-1","reportName":"Test Report","result":{` +
					`"schema":[{"name":"cost","type":"float"}],` +
					`"rows":[["a",1],["b",2]],"cacheHit":false}}`))
			}
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
		_, _ = fmt.Fprintf(w, `{"operationId":"op-1","status":%q}`, s.cancelOpState)
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

	return newTestRetryAPIClient(t, server, requestTimeout, constantBackOff(time.Millisecond))
}

// submitViaInline drives submitAsyncReport against the inline run endpoint.
func submitViaInline(ctx context.Context, client *models.ClientWithResponses) (string, error) {
	operationID, diags := submitAsyncReport(ctx, client, "query",
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
		results, diags := awaitAsyncReport(t.Context(), client, "query", "op-1", nil)
		elapsed := time.Since(start)

		if diags.HasError() {
			t.Fatalf("unexpected diagnostics: %v", diags.Errors())
		}
		rows := nullableToPointer(results.Result.Rows)
		if rows == nil {
			t.Fatal("expected rows in result")
		}
		if got := len(*rows); got != 2 {
			t.Errorf("rows = %d, want 2", got)
		}
		// Three non-terminal polls, each followed by a 5s Retry-After wait.
		if want := 15 * time.Second; elapsed != want {
			t.Errorf("virtual elapsed = %v, want %v", elapsed, want)
		}
	})
}

func TestAwaitAsyncReport_FileOutput(t *testing.T) {
	pdf := models.GetAsyncOperationResultsParamsFileOutputPdf
	png := models.GetAsyncOperationResultsParamsFileOutputPng
	csv := models.GetAsyncOperationResultsParamsFileOutputCsv
	jsonFmt := models.GetAsyncOperationResultsParamsFileOutputJson
	for _, tc := range []struct {
		name      string
		format    *models.GetAsyncOperationResultsParamsFileOutput
		body      string
		wantQuery string
		wantURL   string
	}{
		{
			name: "omitted",
			body: `{"result":{"rows":[],"cacheHit":false}}`,
		},
		{
			name:      "pdf",
			format:    &pdf,
			body:      `{"result":{"rows":[],"cacheHit":false},"fileOutput":"https://storage.example/op.pdf?sig=abc"}`,
			wantQuery: "fileOutput=pdf",
			wantURL:   "https://storage.example/op.pdf?sig=abc",
		},
		{
			name:      "png",
			format:    &png,
			body:      `{"result":{"rows":[],"cacheHit":false},"fileOutput":"https://storage.example/op.png?sig=def"}`,
			wantQuery: "fileOutput=png",
			wantURL:   "https://storage.example/op.png?sig=def",
		},
		{
			name:      "csv",
			format:    &csv,
			body:      `{"result":{"rows":[],"cacheHit":false},"fileOutput":"https://storage.example/op.csv?sig=ghi"}`,
			wantQuery: "fileOutput=csv",
			wantURL:   "https://storage.example/op.csv?sig=ghi",
		},
		{
			name:      "json",
			format:    &jsonFmt,
			body:      `{"result":{"cacheHit":false},"fileOutput":"https://storage.example/op.json?sig=jkl"}`,
			wantQuery: "fileOutput=json",
			wantURL:   "https://storage.example/op.json?sig=jkl",
		},
		{
			name:      "render failed but result remains",
			format:    &pdf,
			body:      `{"result":{"rows":[],"cacheHit":false}}`,
			wantQuery: "fileOutput=pdf",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				srv := newAsyncTestServer(t, &asyncTestServer{
					pollStatuses: []string{"succeeded"},
					resultsBody:  tc.body,
				})
				client := newAsyncTestClient(t, srv.Server)
				result, diags := awaitAsyncReport(t.Context(), client, "query", "op-1", tc.format)
				if diags.HasError() {
					t.Fatalf("unexpected diagnostics: %v", diags.Errors())
				}
				if result.Result == nil {
					t.Fatal("expected result even without file output")
				}
				if result.FileOutput == nil {
					if tc.wantURL != "" {
						t.Errorf("file output is nil, want %q", tc.wantURL)
					}
				} else if *result.FileOutput != tc.wantURL {
					t.Errorf("file output = %q, want %q", *result.FileOutput, tc.wantURL)
				}
				srv.mu.Lock()
				queries := append([]string(nil), srv.resultQueries...)
				srv.mu.Unlock()
				if len(queries) != 1 || queries[0] != tc.wantQuery {
					t.Errorf("result queries = %v, want [%q]", queries, tc.wantQuery)
				}
			})
		})
	}
}

// TestAwaitAsyncReport_DetailsValueAliases verifies that when the results
// endpoint returns details with valueAliases, awaitAsyncReport populates
// Details on the result, and json.Marshal serializes details into the JSON output.
// problem413Body is the problem+json the API sends when a result exceeds its
// inline size limit.
const problem413Body = `{"type":"https://developer.doit.com/errors/result_too_large",` +
	`"title":"Result Too Large","status":413,` +
	`"detail":"The result of this operation is too large to return inline",` +
	`"code":"result_too_large","retryable":false}`

// TestAwaitAsyncReport_Results413 pins that an oversized result fails fast with
// remediation advice and is never retried: the same request would 413 again.
func TestAwaitAsyncReport_Results413(t *testing.T) {
	csv := models.GetAsyncOperationResultsParamsFileOutputCsv
	for _, tc := range []struct {
		name       string
		fileOutput *models.GetAsyncOperationResultsParamsFileOutput
		wantHints  []string
	}{
		{
			name:       "without file output suggests it",
			fileOutput: nil,
			wantHints:  []string{"too large", "Set file_output", "json", "result_too_large"},
		},
		{
			name:       "with file output says the render failed",
			fileOutput: &csv,
			wantHints:  []string{"too large", "did not succeed", "result_too_large"},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				srv := newAsyncTestServer(t, &asyncTestServer{
					pollStatuses:   []string{"succeeded"},
					resultsStatus:  []int{http.StatusRequestEntityTooLarge},
					resultsErrBody: problem413Body,
				})
				client := newAsyncTestClient(t, srv.Server)

				// The deadline is a tripwire, not part of the assertion: the retry
				// client has no elapsed-time limit, so without it a regression that
				// retries 413 would spin until the package timeout.
				ctx, cancel := context.WithTimeout(t.Context(), DefaultReadTimeout)
				defer cancel()

				start := time.Now()
				_, diags := awaitAsyncReport(ctx, client, "query", "op-1", tc.fileOutput)
				elapsed := time.Since(start)

				if !diags.HasError() {
					t.Fatal("expected an error")
				}
				detail := diags.Errors()[0].Detail()
				for _, want := range tc.wantHints {
					if !strings.Contains(detail, want) {
						t.Errorf("detail = %q, want it to contain %q", detail, want)
					}
				}
				if _, _, _, results, _ := srv.snapshot(); results != 1 {
					t.Errorf("results calls = %d, want 1 (413 must not be retried)", results)
				}
				if elapsed != 0 {
					t.Errorf("virtual elapsed = %v, want 0", elapsed)
				}
			})
		})
	}
}

// TestAwaitAsyncReport_FileOutputClearsRows covers the response shapes of a
// request with fileOutput set: a rendered file clears result.rows, while a
// transient render failure returns the full inline result without a file.
func TestAwaitAsyncReport_FileOutputClearsRows(t *testing.T) {
	csv := models.GetAsyncOperationResultsParamsFileOutputCsv
	for _, tc := range []struct {
		name         string
		body         string
		wantRowsNil  bool
		wantURL      bool
		wantRowCount types.Int64
		wantJSONRows string
	}{
		{
			name:         "rendered file: rows null",
			body:         `{"result":{"schema":[{"name":"cost","type":"float"}],"rows":null,"cacheHit":false},"fileOutput":"https://storage.example/op.csv?sig=abc"}`,
			wantRowsNil:  true,
			wantURL:      true,
			wantRowCount: types.Int64Null(),
			wantJSONRows: `"rows":null`,
		},
		{
			name:         "rendered file: rows omitted",
			body:         `{"result":{"schema":[{"name":"cost","type":"float"}],"cacheHit":false},"fileOutput":"https://storage.example/op.csv?sig=abc"}`,
			wantRowsNil:  true,
			wantURL:      true,
			wantRowCount: types.Int64Null(),
		},
		{
			name:         "render failed: full inline result",
			body:         `{"result":{"schema":[{"name":"cost","type":"float"}],"rows":[["a",1],["b",2]],"cacheHit":false}}`,
			wantRowsNil:  false,
			wantURL:      false,
			wantRowCount: types.Int64Value(2),
			wantJSONRows: `"rows":[["a",1],["b",2]]`,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				srv := newAsyncTestServer(t, &asyncTestServer{
					pollStatuses: []string{"succeeded"},
					resultsBody:  tc.body,
				})
				client := newAsyncTestClient(t, srv.Server)

				results, diags := awaitAsyncReport(t.Context(), client, "query", "op-1", &csv)
				if diags.HasError() {
					t.Fatalf("unexpected diagnostics: %v", diags.Errors())
				}
				if got := nullableToPointer(results.Result.Rows) == nil; got != tc.wantRowsNil {
					t.Errorf("rows nil = %v, want %v", got, tc.wantRowsNil)
				}
				if got := results.FileOutput != nil; got != tc.wantURL {
					t.Errorf("file output present = %v, want %v", got, tc.wantURL)
				}
				if got := rowCountFromResult(results); !got.Equal(tc.wantRowCount) {
					t.Errorf("row_count = %v, want %v", got, tc.wantRowCount)
				}
				if tc.wantJSONRows != "" {
					b, err := json.Marshal(results.Result)
					if err != nil {
						t.Fatalf("marshal: %v", err)
					}
					if !strings.Contains(string(b), tc.wantJSONRows) {
						t.Errorf("result_json = %s, want it to contain %s", b, tc.wantJSONRows)
					}
				}
			})
		})
	}
}

func TestRowCountFromResult(t *testing.T) {
	url := "https://storage.example/op.csv"
	rows := [][]nullable.Nullable[models.Value]{{}, {}, {}}
	for _, tc := range []struct {
		name string
		in   *models.GetAsyncOperationResults200Response
		want types.Int64
	}{
		{"rows present", &models.GetAsyncOperationResults200Response{
			Result: &models.GetAsyncOperationResults200ResponseResult{Rows: valueToNullable(rows)}}, types.Int64Value(3)},
		{"rows empty", &models.GetAsyncOperationResults200Response{
			Result: &models.GetAsyncOperationResults200ResponseResult{Rows: valueToNullable([][]nullable.Nullable[models.Value]{})}}, types.Int64Value(0)},
		{"rows absent, no file", &models.GetAsyncOperationResults200Response{
			Result: &models.GetAsyncOperationResults200ResponseResult{}}, types.Int64Value(0)},
		{"rows absent, file rendered", &models.GetAsyncOperationResults200Response{
			Result: &models.GetAsyncOperationResults200ResponseResult{}, FileOutput: &url}, types.Int64Null()},
		{"explicit null rows, file rendered", &models.GetAsyncOperationResults200Response{
			Result: &models.GetAsyncOperationResults200ResponseResult{Rows: nullable.NewNullNullable[[][]nullable.Nullable[models.Value]]()}, FileOutput: &url}, types.Int64Null()},
		{"no result", &models.GetAsyncOperationResults200Response{}, types.Int64Value(0)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := rowCountFromResult(tc.in); !got.Equal(tc.want) {
				t.Errorf("row_count = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestAwaitAsyncReport_DetailsValueAliases(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		resultsPayload := `{"id":"rpt-1","reportName":"Test Report","result":{` +
			`"schema":[{"name":"cloud_provider","type":"string"},{"name":"cost","type":"float"}],` +
			`"rows":[["test-manual",10.5]],` +
			`"cacheHit":false,` +
			`"details":{"valueAliases":{"fixed:cloud_provider":{"test-manual":"Test Manual Display"}}}` +
			`}}`

		srv := newAsyncTestServer(t, &asyncTestServer{
			pollStatuses: []string{"succeeded"},
			resultsBody:  resultsPayload,
		})
		client := newAsyncTestClient(t, srv.Server)

		results, diags := awaitAsyncReport(t.Context(), client, "query", "op-1", nil)
		if diags.HasError() {
			t.Fatalf("unexpected diagnostics: %v", diags.Errors())
		}
		if results.Result == nil {
			t.Fatal("expected non-nil result")
		}
		if results.Result.Details == nil || results.Result.Details.ValueAliases == nil {
			t.Fatal("expected non-nil Details.ValueAliases")
		}
		dimAliases, ok := (*results.Result.Details.ValueAliases)["fixed:cloud_provider"]
		if !ok {
			t.Fatal("expected valueAliases to contain fixed:cloud_provider")
		}
		if got := dimAliases["test-manual"]; got != "Test Manual Display" {
			t.Errorf("alias for test-manual = %q, want %q", got, "Test Manual Display")
		}

		// Verify json.Marshal matches what is exposed in result_json
		b, err := json.Marshal(results.Result)
		if err != nil {
			t.Fatalf("json.Marshal failed: %v", err)
		}
		var parsed map[string]any
		if err := json.Unmarshal(b, &parsed); err != nil {
			t.Fatalf("json.Unmarshal failed: %v", err)
		}
		detailsRaw, ok := parsed["details"]
		if !ok {
			t.Fatalf("details missing from marshaled result_json: %s", string(b))
		}
		detailsMap, ok := detailsRaw.(map[string]any)
		if !ok {
			t.Fatalf("details is not a map: %T", detailsRaw)
		}
		valAliasesRaw, ok := detailsMap["valueAliases"].(map[string]any)
		if !ok {
			t.Fatalf("valueAliases missing or not a map: %v", detailsMap)
		}
		fixedCloud, ok := valAliasesRaw["fixed:cloud_provider"].(map[string]any)
		if !ok {
			t.Fatalf("fixed:cloud_provider missing or not a map: %v", valAliasesRaw)
		}
		if got := fixedCloud["test-manual"]; got != "Test Manual Display" {
			t.Errorf("marshaled alias for test-manual = %v, want %q", got, "Test Manual Display")
		}
	})
}

// TestAwaitAsyncReport_ForecastRows verifies that when the results
// endpoint returns forecastRows, awaitAsyncReport populates
// ForecastRows on the result, and json.Marshal serializes forecastRows into the JSON output.
func TestAwaitAsyncReport_ForecastRows(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		resultsPayload := `{"id":"rpt-1","reportName":"Forecast Report","result":{` +
			`"schema":[{"name":"year","type":"datetime"},{"name":"month","type":"datetime"},{"name":"cost","type":"float"}],` +
			`"rows":[["2026","01",100.0]],` +
			`"forecastRows":[["2026","02",110.0],["2026","03",115.0]],` +
			`"cacheHit":false` +
			`}}`

		srv := newAsyncTestServer(t, &asyncTestServer{
			pollStatuses: []string{"succeeded"},
			resultsBody:  resultsPayload,
		})
		client := newAsyncTestClient(t, srv.Server)

		results, diags := awaitAsyncReport(t.Context(), client, "report results", "op-1", nil)
		if diags.HasError() {
			t.Fatalf("unexpected diagnostics: %v", diags.Errors())
		}
		if results.Result == nil {
			t.Fatal("expected non-nil result")
		}
		if results.Result.ForecastRows == nil {
			t.Fatal("expected non-nil ForecastRows")
		}
		if len(*results.Result.ForecastRows) != 2 {
			t.Fatalf("expected 2 forecast rows, got %d", len(*results.Result.ForecastRows))
		}

		// Verify json.Marshal matches what is exposed in result_json
		b, err := json.Marshal(results.Result)
		if err != nil {
			t.Fatalf("json.Marshal failed: %v", err)
		}
		var parsed map[string]any
		if err := json.Unmarshal(b, &parsed); err != nil {
			t.Fatalf("json.Unmarshal failed: %v", err)
		}
		forecastRaw, ok := parsed["forecastRows"]
		if !ok {
			t.Fatalf("forecastRows missing from marshaled result_json: %s", string(b))
		}
		forecastList, ok := forecastRaw.([]any)
		if !ok {
			t.Fatalf("forecastRows is not a list: %T", forecastRaw)
		}
		if len(forecastList) != 2 {
			t.Fatalf("expected 2 marshaled forecast rows, got %d", len(forecastList))
		}
	})
}

// TestAwaitAsyncReport_OmittedDetails verifies that when the API returns no
// details property, Details is nil and marshaled JSON omits details.
func TestAwaitAsyncReport_OmittedDetails(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		srv := newAsyncTestServer(t, &asyncTestServer{
			pollStatuses: []string{"succeeded"},
		})
		client := newAsyncTestClient(t, srv.Server)

		results, diags := awaitAsyncReport(t.Context(), client, "query", "op-1", nil)
		if diags.HasError() {
			t.Fatalf("unexpected diagnostics: %v", diags.Errors())
		}
		if results.Result == nil {
			t.Fatal("expected non-nil result")
		}
		if results.Result.Details != nil {
			t.Errorf("expected nil Details when omitted, got %+v", results.Result.Details)
		}

		b, err := json.Marshal(results.Result)
		if err != nil {
			t.Fatalf("json.Marshal failed: %v", err)
		}
		var parsed map[string]any
		if err := json.Unmarshal(b, &parsed); err != nil {
			t.Fatalf("json.Unmarshal failed: %v", err)
		}
		if _, ok := parsed["details"]; ok {
			t.Errorf("details should be omitted from JSON when nil, got: %s", string(b))
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
				if _, diags := awaitAsyncReport(t.Context(), client, "query", "op-1", nil); diags.HasError() {
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
		results, diags := awaitAsyncReport(t.Context(), client, "query", "op-1", nil)
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

		_, diags := awaitAsyncReport(ctx, client, "query", "op-1", nil)
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

		_, diags := awaitAsyncReport(ctx, client, "query", "op-1", nil)

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

		_, diags := awaitAsyncReport(ctx, client, "query", "op-1", nil)
		if !diags.HasError() {
			t.Fatal("expected an error after cancellation")
		}

		_, cancels, _, _, _ := srv.snapshot()
		if len(cancels) != 1 {
			t.Errorf("cancel calls = %d, want exactly 1", len(cancels))
		}

		// An interrupt is not a timeout: telling the user to raise a timeout
		// they never hit sends them after the wrong thing.
		summary := diags.Errors()[0].Summary()
		detail := diags.Errors()[0].Detail()
		if strings.Contains(summary, "Timed Out") {
			t.Errorf("summary reports a timeout for an interrupted run: %q", summary)
		}
		if strings.Contains(detail, "read timeout") || strings.Contains(detail, `read = "30m"`) {
			t.Errorf("detail offers timeout guidance for an interrupted run:\n%s", detail)
		}
		if !strings.Contains(detail, "op-1") {
			t.Errorf("detail should name the operation:\n%s", detail)
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

		_, diags := awaitAsyncReport(ctx, client, "query", "op-1", nil)
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

// TestAwaitAsyncReport_TimeoutRaceWithCompletion covers the race where the
// operation finishes just as the deadline expires. Cancelling is idempotent, so
// the API returns the terminal state unchanged — the diagnostic must report what
// actually happened rather than claiming a cancellation.
func TestAwaitAsyncReport_TimeoutRaceWithCompletion(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		srv := newAsyncTestServer(t, &asyncTestServer{
			pollStatuses:  []string{"running"},
			pollRetryHdr:  "5",
			cancelOpState: "succeeded", // finished before the cancel landed
		})
		client := newAsyncTestClient(t, srv.Server)

		ctx, stop := context.WithTimeout(t.Context(), 20*time.Second)
		defer stop()

		_, diags := awaitAsyncReport(ctx, client, "query", "op-1", nil)
		if !diags.HasError() {
			t.Fatal("expected a timeout error")
		}

		detail := diags.Errors()[0].Detail()
		if strings.Contains(detail, "was canceled") {
			t.Errorf("claimed cancellation for an operation that had already finished:\n%s", detail)
		}
		if !strings.Contains(detail, "already finished (succeeded)") {
			t.Errorf("detail should report the state the API returned:\n%s", detail)
		}
		if diags.WarningsCount() != 0 {
			t.Errorf("warnings = %d, want 0 — the cancel request itself succeeded", diags.WarningsCount())
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

		_, diags := awaitAsyncReport(ctx, client, "query", "op-1", nil)

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

		// The error must not claim an outcome the warning just contradicted.
		detail := diags.Errors()[0].Detail()
		if strings.Contains(detail, "was canceled") {
			t.Errorf("error claims the operation was canceled while the warning says it could not be:\n%s", detail)
		}
		if !strings.Contains(detail, "may still be running") {
			t.Errorf("error should say the operation may still be running:\n%s", detail)
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

		_, diags := awaitAsyncReport(t.Context(), client, "query", "op-1", nil)
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

// interruptedError is the failure the API gives an operation whose server run
// went away. Every call builds a new one, so a test cannot change another's.
func interruptedError() *models.AsyncOperationError {
	detail := "Run the report again."
	return &models.AsyncOperationError{
		Title:  "Report run interrupted",
		Code:   "operation_interrupted",
		Status: http.StatusServiceUnavailable,
		Detail: &detail,
	}
}

// TestAwaitAsyncReport_OperationInterrupted pins what reaches the user when the
// API fails an operation as interrupted: the code, status and the API's hint,
// and no cancel, since the operation is already terminal.
func TestAwaitAsyncReport_OperationInterrupted(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		srv := newAsyncTestServer(t, &asyncTestServer{
			pollStatuses: []string{"failed"},
			failureError: interruptedError(),
		})
		client := newAsyncTestClient(t, srv.Server)

		_, diags := awaitAsyncReport(t.Context(), client, "query", "op-1", nil)
		if !diags.HasError() {
			t.Fatal("expected an error")
		}

		got := diags.Errors()[0].Detail()
		for _, want := range []string{"op-1", "Report run interrupted", "operation_interrupted", "503", "Run the report again."} {
			if !strings.Contains(got, want) {
				t.Errorf("detail missing %q:\n%s", want, got)
			}
		}
		if _, cancels, _, _, _ := srv.snapshot(); len(cancels) != 0 {
			t.Errorf("cancel calls = %d, want 0: the operation was already terminal", len(cancels))
		}
	})
}

// TestRunAsyncReport_RetriesAnInterruptedOperation covers the whole run: only an
// operation the API failed as interrupted is submitted again, once, and the
// second interruption is reported rather than retried.
func TestRunAsyncReport_RetriesAnInterruptedOperation(t *testing.T) {
	notRetried := func(code string) *models.AsyncOperationError {
		return &models.AsyncOperationError{Title: "Failed", Code: code, Status: 413}
	}
	for _, tc := range []struct {
		name         string
		statuses     []string
		failure      *models.AsyncOperationError
		wantSubmits  int
		wantCancels  int
		wantErr      []string // all must be in the error detail; none expected when empty
		wantNotInErr []string
	}{
		{
			name:        "interrupted once then succeeds",
			statuses:    []string{"failed", "succeeded"},
			failure:     interruptedError(),
			wantSubmits: 2,
		},
		{
			name:         "interrupted twice is reported, not retried again",
			statuses:     []string{"failed", "failed"},
			failure:      interruptedError(),
			wantSubmits:  2,
			wantErr:      []string{"operation_interrupted", "Run the report again.", "ran it again once"},
			wantNotInErr: nil,
		},
		{
			name:         "another failure is not retried",
			statuses:     []string{"failed", "succeeded"},
			failure:      notRetried("result_too_large"),
			wantSubmits:  1,
			wantErr:      []string{"result_too_large"},
			wantNotInErr: []string{"ran it again"},
		},
		{
			name:         "failure without detail is not retried",
			statuses:     []string{"failed", "succeeded"},
			failure:      nil,
			wantSubmits:  1,
			wantErr:      []string{"no error detail"},
			wantNotInErr: []string{"ran it again"},
		},
		{
			name:         "canceled is not retried",
			statuses:     []string{"canceled", "succeeded"},
			wantSubmits:  1,
			wantErr:      []string{"was canceled"},
			wantNotInErr: []string{"ran it again"},
		},
		{
			name:         "the retry is cancelled when the read timeout ends it",
			statuses:     []string{"failed", "running"},
			failure:      interruptedError(),
			wantSubmits:  2,
			wantCancels:  1,
			wantErr:      []string{"did not finish within the read timeout"},
			wantNotInErr: []string{"ran it again"},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				srv := newAsyncTestServer(t, &asyncTestServer{pollStatuses: tc.statuses, failureError: tc.failure})
				client := newAsyncTestClient(t, srv.Server)

				// Half a second past a whole number of seconds, so the deadline
				// does not fall on the same virtual instant as a 1s poll wait ends.
				result, diags := runAsyncReportWith(t.Context(), newReportRunLimiter(1), client, "query",
					10*time.Minute+500*time.Millisecond, inlineSubmit(client), nil)

				submits, cancels, _, _, _ := srv.snapshot()
				if len(submits) != tc.wantSubmits {
					t.Errorf("submits = %d, want %d", len(submits), tc.wantSubmits)
				}
				if len(cancels) != tc.wantCancels {
					t.Errorf("cancel calls = %d, want %d", len(cancels), tc.wantCancels)
				}
				if len(submits) == 2 && submits[0] == submits[1] {
					t.Errorf("the retry reused Idempotency-Key %q; a replay would hand back the failed operation", submits[0])
				}

				if len(tc.wantErr) == 0 {
					if diags.HasError() {
						t.Fatalf("unexpected error: %v", diags)
					}
					if result == nil {
						t.Fatal("no result after the retry succeeded")
					}
					return
				}
				if !diags.HasError() {
					t.Fatal("expected an error")
				}
				if got := len(diags.Errors()); got != 1 {
					t.Errorf("errors = %d, want 1: %v", got, diags)
				}
				got := diags.Errors()[0].Detail()
				for _, want := range tc.wantErr {
					if !strings.Contains(got, want) {
						t.Errorf("detail missing %q:\n%s", want, got)
					}
				}
				for _, notWant := range tc.wantNotInErr {
					if strings.Contains(got, notWant) {
						t.Errorf("detail should not contain %q:\n%s", notWant, got)
					}
				}
			})
		})
	}
}

// TestRunAsyncReport_RetryDoesNotRenewTheReadTimeout pins that the read timeout
// bounds the whole run, not each attempt. The first attempt runs for 4 minutes
// before the API interrupts it, so the retry has the rest of the budget: its
// operation is cancelled when the original deadline passes, not 10 minutes
// after the retry started.
func TestRunAsyncReport_RetryDoesNotRenewTheReadTimeout(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		// Not a multiple of the 30s poll interval, so the deadline does not fall
		// on the same virtual instant as a poll wait ends.
		const readTimeout = 10*time.Minute + 500*time.Millisecond

		// Polls 1-8 find the operation running and each waits 30s, so poll 9 at
		// 4 minutes is the interruption. The retry's operation then stays running.
		statuses := append(slices.Repeat([]string{"running"}, 8), "failed", "running")
		srv := newAsyncTestServer(t, &asyncTestServer{
			pollStatuses: statuses,
			pollRetryHdr: "30",
			failureError: interruptedError(),
		})
		client := newAsyncTestClient(t, srv.Server)

		start := time.Now()
		_, diags := runAsyncReportWith(t.Context(), newReportRunLimiter(1), client, "query",
			readTimeout, inlineSubmit(client), nil)
		elapsed := time.Since(start)

		if !diags.HasError() {
			t.Fatal("expected the read timeout to end the retry")
		}
		if got := diags.Errors()[0].Detail(); !strings.Contains(got, "did not finish within the read timeout") {
			t.Errorf("detail = %q, want the read timeout error", got)
		}
		submits, cancels, _, _, _ := srv.snapshot()
		if len(submits) != 2 {
			t.Errorf("submits = %d, want 2: the interruption came before the deadline", len(submits))
		}
		if len(cancels) != 1 {
			t.Errorf("cancel calls = %d, want 1: the retry's operation", len(cancels))
		}
		if elapsed != readTimeout {
			t.Errorf("elapsed = %v, want %v: both attempts share one read timeout", elapsed, readTimeout)
		}
	})
}

func TestAwaitAsyncReport_TerminalStatesAndErrors(t *testing.T) {
	pdf := models.GetAsyncOperationResultsParamsFileOutputPdf
	// Pointers, not values: asyncTestServer carries a mutex and must not be copied.
	for _, tc := range []struct {
		name       string
		server     *asyncTestServer
		wantMsg    string
		fileOutput *models.GetAsyncOperationResultsParamsFileOutput
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
			wantMsg:    "422",
			fileOutput: &pdf,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				srv := newAsyncTestServer(t, tc.server)
				client := newAsyncTestClient(t, srv.Server)

				_, diags := awaitAsyncReport(t.Context(), client, "query", "op-1", tc.fileOutput)
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

// TestAwaitAsyncReport_UnrecognizedStatus pins what happens to a status the
// provider does not know. Generated enums are plain strings with no unmarshal
// validation, so an unexpected value would otherwise be treated as non-terminal
// — polling until the read timeout and then cancelling, rather than failing
// immediately with something the user can act on.
//
// This also pins the inverse: pending and running are the only statuses that
// should keep the loop going, so a terminal state added upstream cannot be
// silently spun on.
func TestAwaitAsyncReport_UnrecognizedStatus(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		srv := newAsyncTestServer(t, &asyncTestServer{
			pollStatuses: []string{"disintegrating"},
			pollRetryHdr: "5",
		})
		client := newAsyncTestClient(t, srv.Server)

		ctx, stop := context.WithTimeout(t.Context(), DefaultReadTimeout)
		defer stop()

		start := time.Now()
		_, diags := awaitAsyncReport(ctx, client, "query", "op-1", nil)
		elapsed := time.Since(start)

		if !diags.HasError() {
			t.Fatal("expected an error")
		}
		if detail := diags.Errors()[0].Detail(); !strings.Contains(detail, "disintegrating") {
			t.Errorf("error should name the unrecognized status:\n%s", detail)
		}
		if elapsed != 0 {
			t.Errorf("elapsed = %v, want 0 — must fail on the first poll, not burn the read timeout", elapsed)
		}

		_, cancels, polls, _, _ := srv.snapshot()
		if polls != 1 {
			t.Errorf("polls = %d, want 1", polls)
		}
		// The operation is in a state the provider does not know, so it may
		// still be running and counting against the API's cap. Giving up on it
		// without cancelling would leave it there.
		if len(cancels) != 1 {
			t.Errorf("cancel calls = %d, want 1 — the abandoned operation must not keep running", len(cancels))
		}
		if detail := diags.Errors()[0].Detail(); !strings.Contains(detail, "was canceled") {
			t.Errorf("error should say what became of the operation:\n%s", detail)
		}
	})
}

// TestAwaitAsyncReport_Results425ThenSucceeds pins how the declared 425
// response is handled: retried on the standard interval, not failed on. The
// endpoint should not return it once the operation has succeeded, so this
// guards the declared contract rather than an observed behavior.
func TestAwaitAsyncReport_Results425ThenSucceeds(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		srv := newAsyncTestServer(t, &asyncTestServer{
			pollStatuses:  []string{"succeeded"},
			resultsStatus: []int{http.StatusTooEarly, http.StatusOK},
		})
		client := newAsyncTestClient(t, srv.Server)

		start := time.Now()
		results, diags := awaitAsyncReport(t.Context(), client, "query", "op-1", nil)
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

func TestSubmitAsyncReport_Succeeds(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		srv := newAsyncTestServer(t, &asyncTestServer{})
		client := newAsyncTestClient(t, srv.Server)

		operationID, err := submitViaInline(t.Context(), client)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if operationID != "op-1" {
			t.Errorf("operationID = %q, want %q", operationID, "op-1")
		}

		submits, _, _, _, _ := srv.snapshot()
		if len(submits) != 1 {
			t.Fatalf("submits = %d, want exactly 1", len(submits))
		}
		if submits[0] == "" {
			t.Error("submit carried no Idempotency-Key")
		}
	})
}

// TestSubmitAsyncReport_AbandonedSubmitIsCleanedUp covers the window where the
// server accepts a submission but the read context expires before the response
// arrives. The provider never saw an operation ID, yet the run exists — and the
// documented timeout/Ctrl-C behavior promises not to leave one behind.
//
// Recovery leans on the Idempotency-Key: replaying it returns the original
// operation, which can then be canceled.
func TestSubmitAsyncReport_AbandonedSubmitIsCleanedUp(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		srv := newAsyncTestServer(t, &asyncTestServer{submitDelay: 30 * time.Second})
		client := newAsyncTestClient(t, srv.Server)

		ctx, stop := context.WithTimeout(t.Context(), 5*time.Second)
		defer stop()

		_, err := submitViaInline(ctx, client)
		if err == nil {
			t.Fatal("expected the submit to fail once the context expired")
		}

		// Cleaning up silently is not enough: the user is told the submit
		// failed, so they also need to know a run was created and stopped.
		if !strings.Contains(err.Error(), "op-1") {
			t.Errorf("error should name the operation that was cleaned up: %v", err)
		}
		if !strings.Contains(err.Error(), "canceled") {
			t.Errorf("error should say the operation was canceled: %v", err)
		}

		submits, cancels, _, _, _ := srv.snapshot()
		if len(cancels) != 1 {
			t.Fatalf("cancel calls = %d, want 1 — the accepted submission was orphaned", len(cancels))
		}
		// The replay must reuse the original key; a fresh one would create a
		// second operation rather than recovering the abandoned one.
		if len(submits) != 2 {
			t.Fatalf("submits = %d, want 2 (original + replay to recover the ID)", len(submits))
		}
		if submits[0] != submits[1] {
			t.Errorf("recovery used a new Idempotency-Key (%q vs %q); it must replay the original",
				submits[0], submits[1])
		}
	})
}

// TestSubmitAsyncReport_AcceptedWithoutOperationID pins the contract: an
// accepted submission must carry an operation ID. Without one there is nothing
// to poll or cancel, so this surfaces as an error rather than being guessed
// around — a silent re-submit would hide a real API regression.
func TestSubmitAsyncReport_AcceptedWithoutOperationID(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		srv := newAsyncTestServer(t, &asyncTestServer{submitEmpty: 99})
		client := newAsyncTestClient(t, srv.Server)

		_, err := submitViaInline(t.Context(), client)
		if err == nil {
			t.Fatal("expected an error when the response carries no operation ID")
		}
		if !strings.Contains(err.Error(), "no operation ID") {
			t.Errorf("unexpected error: %v", err)
		}

		submits, _, _, _, _ := srv.snapshot()
		if len(submits) != 1 {
			t.Errorf("submits = %d, want exactly 1 — the run must not be re-submitted", len(submits))
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

// TestSubmitAsyncReport_ActiveOperationLimitWaitsForASlot covers the per-caller
// cap on pending or running operations. Terraform reads data sources in
// parallel (-parallelism defaults to 10), so a configuration with more than
// three report data sources trips the cap by itself. The API answers the
// surplus runs with a 429 carrying Retry-After: 30; the submit must ride that
// out and succeed once a slot frees, not fail the read.
func TestSubmitAsyncReport_ActiveOperationLimitWaitsForASlot(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		srv := newAsyncTestServer(t, &asyncTestServer{submitLimited: 2})
		client := newAsyncTestClient(t, srv.Server)

		ctx, stop := context.WithTimeout(t.Context(), DefaultReadTimeout)
		defer stop()

		start := time.Now()
		operationID, err := submitViaInline(ctx, client)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if operationID != "op-1" {
			t.Errorf("operationID = %q, want %q", operationID, "op-1")
		}
		// Two 429s, each honored at the API's own 30s Retry-After.
		if elapsed := time.Since(start); elapsed != 60*time.Second {
			t.Errorf("elapsed = %v, want 60s (two Retry-After waits)", elapsed)
		}

		submits, cancels, _, _, _ := srv.snapshot()
		if len(submits) != 3 {
			t.Fatalf("submits = %d, want 3 (two limited, one accepted)", len(submits))
		}
		// Every attempt is the same request, so one key covers all of them: a
		// rejected run was never created, and the accepted one is created once.
		if submits[0] != submits[1] || submits[1] != submits[2] {
			t.Errorf("attempts used different Idempotency-Keys: %q", submits)
		}
		if len(cancels) != 0 {
			t.Errorf("cancel calls = %d, want 0: nothing was created until the last attempt", len(cancels))
		}
	})
}

// TestSubmitAsyncReport_ActiveOperationLimitOutlastsReadTimeout covers a cap
// that never frees within the read timeout (other callers hold the customer
// cap, or operations orphaned by a killed run hold the caller's slots for up
// to 30 minutes). The submit has to fail at the deadline, create nothing, and
// tell the user why: a bare "context deadline exceeded" gives no hint that the
// concurrency cap was the cause, and that something other than this Terraform
// run holds the slots.
func TestSubmitAsyncReport_ActiveOperationLimitOutlastsReadTimeout(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		srv := newAsyncTestServer(t, &asyncTestServer{submitLimited: 1_000})
		client := newAsyncTestClient(t, srv.Server)

		// A second past the read timeout, so the deadline does not fall on the
		// same virtual instant as a 30s Retry-After wait ends (the two would race).
		ctx, stop := context.WithTimeout(t.Context(), DefaultReadTimeout+time.Second)
		defer stop()

		_, err := submitViaInline(ctx, client)
		if err == nil {
			t.Fatal("expected the submit to fail once the read timeout elapsed")
		}

		_, cancels, _, _, _ := srv.snapshot()
		if len(cancels) != 0 {
			t.Errorf("cancel calls = %d, want 0: no operation was ever created", len(cancels))
		}
		// The status and body the API sent (naming the caller's own active
		// operations), the deadline that ended the wait, and what to do about it.
		for _, want := range []string{
			"context deadline exceeded", "status: 429", "too_many_active_operations_for_user",
			"op-a", "other Terraform runs, other users",
		} {
			if !strings.Contains(err.Error(), want) {
				t.Errorf("error should explain the cap, missing %q: %v", want, err)
			}
		}
	})
}

// TestSubmitAsyncReport_InterruptedWhileWaitingOnTheLimit covers the user
// stopping Terraform (Ctrl-C) during the wait on the active-operation cap. The
// run was stopped on purpose, so the error must not say that the read timeout
// elapsed or tell the user to raise it.
func TestSubmitAsyncReport_InterruptedWhileWaitingOnTheLimit(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		srv := newAsyncTestServer(t, &asyncTestServer{submitLimited: 1_000})
		client := newAsyncTestClient(t, srv.Server)

		ctx, cancel := context.WithCancel(t.Context())
		defer cancel()
		time.AfterFunc(45*time.Second, cancel)

		_, err := submitViaInline(ctx, client)
		if err == nil {
			t.Fatal("expected the submit to fail once the context was cancelled")
		}
		for _, notWant := range []string{"read timeout", "raise the read timeout"} {
			if strings.Contains(err.Error(), notWant) {
				t.Errorf("an interrupted run was told about the timeout (%q): %v", notWant, err)
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
