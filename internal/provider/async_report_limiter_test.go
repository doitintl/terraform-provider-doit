package provider

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"testing/synctest"
	"time"

	"github.com/doitintl/terraform-provider-doit/internal/provider/models"
	"github.com/hashicorp/terraform-plugin-framework/diag"
)

// These tests drive runAsyncReport against a fake API that enforces the cap on
// pending or running report operations the way the real one does: a run beyond
// the cap is refused with a 429 carrying Retry-After: 30. Unlike asyncTestServer,
// which answers every submit with the same canned operation, the fake keeps
// track of how many operations are active, so a test can tell a run the
// provider held back from one the API had to turn away.
//
// The point is the behavior of one Terraform run, which shares one client
// across its data sources and reads them in parallel. Nothing here is helped
// by another test's queue.

// capServer is a scriptable stand-in for the async report API with an
// active-operation cap.
type capServer struct {
	*httptest.Server

	mu sync.Mutex

	// script
	cap           int           // active operations the API allows this caller
	runFor        time.Duration // how long an operation stays pending or running
	finalStatus   string        // what it becomes afterwards
	resultsDelay  time.Duration // how long the results endpoint takes to answer
	resultsStatus int
	submitStatus  int           // non-zero: every submit is refused with it
	pollNotFound  bool          // the poll endpoint answers 404
	heldByOthers  int           // operations other callers hold against the cap...
	heldFor       time.Duration // ...for this long after the server starts

	// state
	started time.Time
	ops     map[string]*capOp

	// observations
	peakActive int
	limited    int // 429s answered
	cancels    int
	acceptedAt []time.Time
}

type capOp struct {
	createdAt time.Time
	canceled  bool
}

func newCapServer(t *testing.T, s *capServer) *capServer {
	t.Helper()

	if s.cap == 0 {
		s.cap = maxConcurrentReportRuns
	}
	if s.runFor == 0 {
		s.runFor = 10 * time.Second
	}
	if s.finalStatus == "" {
		s.finalStatus = "succeeded"
	}
	if s.resultsStatus == 0 {
		s.resultsStatus = http.StatusOK
	}
	s.started = time.Now()
	s.ops = map[string]*capOp{}

	mux := http.NewServeMux()

	submit := func(w http.ResponseWriter, _ *http.Request) {
		s.mu.Lock()
		defer s.mu.Unlock()

		now := time.Now()

		if s.submitStatus != 0 {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(s.submitStatus)
			_, _ = w.Write([]byte(`{"error":"not found"}`))
			return
		}

		active := s.activeLocked(now)
		if active >= s.cap {
			s.limited++
			w.Header().Set("Content-Type", "application/problem+json")
			w.Header().Set("Retry-After", "30")
			w.WriteHeader(http.StatusTooManyRequests)
			_, _ = w.Write([]byte(activeOperationsLimitBody))
			return
		}

		id := fmt.Sprintf("op-%d", len(s.ops)+1)
		s.ops[id] = &capOp{createdAt: now}
		s.acceptedAt = append(s.acceptedAt, now)
		s.peakActive = max(s.peakActive, active+1)

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusAccepted)
		_, _ = fmt.Fprintf(w, `{"operationId":%q,"status":"pending","createTime":"2000-01-01T00:00:00Z"}`, id)
	}
	mux.HandleFunc("POST /analytics/v1/reports/actions/run", submit)
	mux.HandleFunc("POST /analytics/v1/reports/{id}/actions/run", submit)

	mux.HandleFunc("GET /analytics/v1/reports/operations/{operationId}", func(w http.ResponseWriter, r *http.Request) {
		s.mu.Lock()
		defer s.mu.Unlock()

		w.Header().Set("Content-Type", "application/json")
		if s.pollNotFound {
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte(`{"error":"operation not found"}`))
			return
		}

		id := r.PathValue("operationId")
		op := s.ops[id]
		status := s.finalStatus
		switch {
		case op.canceled:
			status = "canceled"
		case time.Now().Before(op.createdAt.Add(s.runFor)):
			status = "running"
		}

		body := fmt.Sprintf(`{"operationId":%q,"status":%q,"createTime":"2000-01-01T00:00:00Z","expireBy":"2000-01-08T00:00:00Z"`, id, status)
		if status == "failed" {
			body += `,"error":{"title":"Query failed","code":"query_failed","status":500}`
		}
		_, _ = w.Write([]byte(body + "}"))
	})

	mux.HandleFunc("GET /analytics/v1/reports/operations/{operationId}/results", func(w http.ResponseWriter, r *http.Request) {
		s.mu.Lock()
		delay, status := s.resultsDelay, s.resultsStatus
		s.mu.Unlock()

		// Held outside the lock, so a slow results fetch does not stop the
		// other runs from submitting.
		if delay > 0 {
			select {
			case <-time.After(delay):
			case <-r.Context().Done():
				return
			}
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		if status != http.StatusOK {
			_, _ = w.Write([]byte(`{"error":"result unavailable"}`))
			return
		}
		_, _ = w.Write([]byte(`{"id":"rpt-1","reportName":"Test Report","result":{` +
			`"schema":[{"name":"cost","type":"float"}],"rows":[["a",1]],"cacheHit":false}}`))
	})

	mux.HandleFunc("POST /analytics/v1/reports/operations/{operationId}/actions/cancel", func(w http.ResponseWriter, r *http.Request) {
		s.mu.Lock()
		defer s.mu.Unlock()

		s.cancels++
		id := r.PathValue("operationId")
		if op := s.ops[id]; op != nil {
			op.canceled = true
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprintf(w, `{"operationId":%q,"status":"canceled"}`, id)
	})

	s.Server = httptest.NewTestServer(t, mux)
	return s
}

// activeLocked counts the operations the API would hold against the cap at now:
// this caller's that have not finished or been cancelled, plus the ones other
// callers hold for a while.
func (s *capServer) activeLocked(now time.Time) int {
	active := 0
	for _, op := range s.ops {
		if !op.canceled && now.Before(op.createdAt.Add(s.runFor)) {
			active++
		}
	}
	if now.Before(s.started.Add(s.heldFor)) {
		active += s.heldByOthers
	}
	return active
}

// observed is what the server saw, taken under its lock.
type capObservation struct {
	peakActive int
	limited    int
	cancels    int
	acceptedAt []time.Duration // since the server started
}

func (s *capServer) observe() capObservation {
	s.mu.Lock()
	defer s.mu.Unlock()

	obs := capObservation{peakActive: s.peakActive, limited: s.limited, cancels: s.cancels}
	for _, at := range s.acceptedAt {
		obs.acceptedAt = append(obs.acceptedAt, at.Sub(s.started))
	}
	return obs
}

// inlineSubmit is the submit step of doit_report_query.
func inlineSubmit(client *models.ClientWithResponses) asyncSubmitFunc {
	return func(ctx context.Context, idempotencyKey string) (asyncSubmission, error) {
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
	}
}

// runMany starts n runs at once, as Terraform does for n independent data
// sources, and returns each run's diagnostics.
func runMany(n int, run func(i int) diag.Diagnostics) []diag.Diagnostics {
	out := make([]diag.Diagnostics, n)
	var wg sync.WaitGroup
	for i := range n {
		wg.Go(func() { out[i] = run(i) })
	}
	wg.Wait()
	return out
}

// failedRuns lists the error of every run that failed.
func failedRuns(all []diag.Diagnostics) []string {
	var failed []string
	for i, diags := range all {
		if diags.HasError() {
			failed = append(failed, fmt.Sprintf("run %d: %s: %s", i, diags.Errors()[0].Summary(), diags.Errors()[0].Detail()))
		}
	}
	return failed
}

// idle reports whether every slot of the limiter is free.
func (l *reportRunLimiter) idle(runs int64) bool {
	if !l.sem.TryAcquire(runs) {
		return false
	}
	l.sem.Release(runs)
	return true
}

// TestRunAsyncReport_OneClientNeverExceedsTheCap is the behavior of one
// Terraform run with more report data sources than the API allows at once. The
// provider holds the surplus back, so the API never has to turn a run away.
func TestRunAsyncReport_OneClientNeverExceedsTheCap(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		const runs = 10

		srv := newCapServer(t, &capServer{runFor: 10 * time.Second})
		client := newAsyncTestClient(t, srv.Server)

		start := time.Now()
		results := runMany(runs, func(int) diag.Diagnostics {
			_, diags := runAsyncReport(t.Context(), client, "query", DefaultReadTimeout, inlineSubmit(client), nil)
			return diags
		})

		if failed := failedRuns(results); len(failed) != 0 {
			t.Fatalf("%d of %d runs failed:\n%s", len(failed), runs, strings.Join(failed, "\n"))
		}

		obs := srv.observe()
		if obs.limited != 0 {
			t.Errorf("the API turned away %d runs with a 429, want 0: the provider should have held them back", obs.limited)
		}
		if obs.peakActive > maxConcurrentReportRuns {
			t.Errorf("peak active operations = %d, want at most %d", obs.peakActive, maxConcurrentReportRuns)
		}
		if len(obs.acceptedAt) != runs {
			t.Errorf("accepted runs = %d, want %d", len(obs.acceptedAt), runs)
		}
		// Waves of 3, 3, 3 and 1, each one report long.
		if elapsed, want := time.Since(start), 4*10*time.Second; elapsed != want {
			t.Errorf("elapsed = %v, want %v (four waves of 10s)", elapsed, want)
		}
	})
}

// TestRunAsyncReport_WithoutALimiterTheAPITurnsRunsAway is the control for the
// test above: the same workload without the queue makes the API refuse runs, so
// that test fails when the limiter stops working. The runs still succeed, which
// is what the 429 wait is for.
func TestRunAsyncReport_WithoutALimiterTheAPITurnsRunsAway(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		const runs = 10

		srv := newCapServer(t, &capServer{runFor: 10 * time.Second})
		client := newAsyncTestClient(t, srv.Server)
		unlimited := newReportRunLimiter(runs)

		results := runMany(runs, func(int) diag.Diagnostics {
			_, diags := runAsyncReportWith(t.Context(), unlimited, client, "query", DefaultReadTimeout, inlineSubmit(client), nil)
			return diags
		})

		if failed := failedRuns(results); len(failed) != 0 {
			t.Fatalf("%d of %d runs failed:\n%s", len(failed), runs, strings.Join(failed, "\n"))
		}
		if obs := srv.observe(); obs.limited == 0 {
			t.Error("the API never refused a run: the workload does not exceed the cap, so the test above proves nothing")
		}
	})
}

// TestRunAsyncReport_StillWaitsWhenOthersHoldTheCap covers what the queue
// cannot know: other Terraform runs, aliases and users share the API's cap. Two
// of the three slots are held elsewhere for a while, so the provider's runs meet
// 429s and have to wait them out.
func TestRunAsyncReport_StillWaitsWhenOthersHoldTheCap(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		const runs = 3

		srv := newCapServer(t, &capServer{
			runFor:       10 * time.Second,
			heldByOthers: 2,
			heldFor:      45 * time.Second,
		})
		client := newAsyncTestClient(t, srv.Server)

		results := runMany(runs, func(int) diag.Diagnostics {
			_, diags := runAsyncReport(t.Context(), client, "query", DefaultReadTimeout, inlineSubmit(client), nil)
			return diags
		})

		if failed := failedRuns(results); len(failed) != 0 {
			t.Fatalf("%d of %d runs failed:\n%s", len(failed), runs, strings.Join(failed, "\n"))
		}
		if obs := srv.observe(); obs.limited == 0 {
			t.Error("no run was refused, so the 429 wait was never exercised")
		}
	})
}

// TestReportRunLimiterFor_IsPerClient pins the scope: one limiter per client,
// the same one every time. A Terraform run configures one client; separate runs
// (and so separate tests) get separate limiters, which is why one test's
// reports never queue behind another's.
func TestReportRunLimiterFor_IsPerClient(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		srv := newCapServer(t, &capServer{})
		a := newAsyncTestClient(t, srv.Server)
		b := newAsyncTestClient(t, srv.Server)

		first, second := reportRunLimiterFor(a), reportRunLimiterFor(a)
		if first != second {
			t.Error("the same client got two different limiters")
		}
		if first == reportRunLimiterFor(b) {
			t.Fatal("two clients share a limiter")
		}

		// Saturating one client's limiter leaves the other's free.
		for range maxConcurrentReportRuns {
			release, err := reportRunLimiterFor(a).acquire(t.Context())
			if err != nil {
				t.Fatalf("acquire: %v", err)
			}
			defer release()
		}
		ctx, cancel := context.WithTimeout(t.Context(), time.Second)
		defer cancel()
		release, err := reportRunLimiterFor(b).acquire(ctx)
		if err != nil {
			t.Fatalf("a full limiter on one client blocked another client: %v", err)
		}
		release()
	})
}

// TestRunAsyncReport_ReadTimeoutStartsAfterTheSlot pins what `read` bounds: a
// report's own run, from the moment it holds a slot. Waiting behind another
// report does not use it up.
func TestRunAsyncReport_ReadTimeoutStartsAfterTheSlot(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		const readTimeout = 5 * time.Minute

		// One slot, two reports of 4 minutes each: the second waits 4 minutes
		// and then needs 4 more, 8 in all, over the 5 minute read timeout.
		srv := newCapServer(t, &capServer{runFor: 4 * time.Minute})
		client := newAsyncTestClient(t, srv.Server)
		limiter := newReportRunLimiter(1)

		start := time.Now()
		results := runMany(2, func(int) diag.Diagnostics {
			_, diags := runAsyncReportWith(t.Context(), limiter, client, "query", readTimeout, inlineSubmit(client), nil)
			return diags
		})

		if failed := failedRuns(results); len(failed) != 0 {
			t.Fatalf("a run failed although each fits its read timeout:\n%s", strings.Join(failed, "\n"))
		}
		if elapsed, want := time.Since(start), 8*time.Minute; elapsed != want {
			t.Errorf("elapsed = %v, want %v", elapsed, want)
		}
	})
}

// TestRunAsyncReport_ReportOverTheReadTimeoutFreesItsSlot covers a report that
// does outlast its own read timeout: it is cancelled, and the next report gets
// the slot right then, not after some further wait.
func TestRunAsyncReport_ReportOverTheReadTimeoutFreesItsSlot(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		const readTimeout = 5 * time.Minute

		srv := newCapServer(t, &capServer{runFor: 10 * time.Minute})
		client := newAsyncTestClient(t, srv.Server)
		limiter := newReportRunLimiter(1)

		results := runMany(2, func(int) diag.Diagnostics {
			_, diags := runAsyncReportWith(t.Context(), limiter, client, "query", readTimeout, inlineSubmit(client), nil)
			return diags
		})

		if failed := failedRuns(results); len(failed) != 2 {
			t.Fatalf("failed runs = %d, want 2: both outlast the read timeout\n%s", len(failed), strings.Join(failed, "\n"))
		}
		obs := srv.observe()
		if obs.cancels != 2 {
			t.Errorf("cancel calls = %d, want 2: an abandoned report must not keep running", obs.cancels)
		}
		if len(obs.acceptedAt) != 2 {
			t.Fatalf("accepted runs = %d, want 2", len(obs.acceptedAt))
		}
		if got := obs.acceptedAt[1]; got != readTimeout {
			t.Errorf("second run submitted at %v, want %v: the slot should free when the first is cancelled", got, readTimeout)
		}
		if !limiter.idle(1) {
			t.Error("the slot was not released")
		}
	})
}

// TestRunAsyncReport_StoppedWhileQueuedSubmitsNothing covers a run that is still
// waiting for a slot when its context ends, by Ctrl-C or by a deadline above the
// provider. It never submitted, so there is nothing to cancel.
func TestRunAsyncReport_StoppedWhileQueuedSubmitsNothing(t *testing.T) {
	tests := []struct {
		name        string
		stop        func(ctx context.Context) (context.Context, context.CancelFunc)
		wantSummary string
	}{
		{
			name: "cancelled",
			stop: func(ctx context.Context) (context.Context, context.CancelFunc) {
				ctx, cancel := context.WithCancel(ctx)
				time.AfterFunc(time.Minute, cancel)
				return ctx, cancel
			},
			wantSummary: "Canceled Running query",
		},
		{
			name: "deadline",
			stop: func(ctx context.Context) (context.Context, context.CancelFunc) {
				return context.WithTimeout(ctx, time.Minute)
			},
			wantSummary: "Timed Out Running query",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				srv := newCapServer(t, &capServer{runFor: 10 * time.Minute})
				client := newAsyncTestClient(t, srv.Server)
				limiter := newReportRunLimiter(1)

				results := runMany(2, func(i int) diag.Diagnostics {
					ctx := t.Context()
					if i == 1 {
						// The second run queues behind the first and is stopped there.
						time.Sleep(time.Second)
						var cancel context.CancelFunc
						ctx, cancel = tt.stop(ctx)
						defer cancel()
					}
					_, diags := runAsyncReportWith(ctx, limiter, client, "query", 30*time.Minute, inlineSubmit(client), nil)
					return diags
				})

				if failed := failedRuns(results[:1]); len(failed) != 0 {
					t.Fatalf("the first run should not be affected:\n%s", strings.Join(failed, "\n"))
				}
				if !results[1].HasError() {
					t.Fatal("expected the queued run to fail")
				}
				summary, detail := results[1].Errors()[0].Summary(), results[1].Errors()[0].Detail()
				if summary != tt.wantSummary {
					t.Errorf("summary = %q, want %q", summary, tt.wantSummary)
				}
				if !strings.Contains(detail, "Nothing was submitted") {
					t.Errorf("detail should say nothing was submitted: %s", detail)
				}

				obs := srv.observe()
				if len(obs.acceptedAt) != 1 {
					t.Errorf("accepted runs = %d, want 1: the queued run must not submit", len(obs.acceptedAt))
				}
				if obs.cancels != 0 {
					t.Errorf("cancel calls = %d, want 0", obs.cancels)
				}
				if !limiter.idle(1) {
					t.Error("the slot was not released")
				}
			})
		})
	}
}

// TestRunAsyncReport_ReleasesItsSlotOnEveryPath checks that no way for a run to
// end leaves its slot held: a slot that leaks shrinks the provider's capacity
// for the rest of the Terraform run.
func TestRunAsyncReport_ReleasesItsSlotOnEveryPath(t *testing.T) {
	tests := []struct {
		name        string
		server      func() *capServer
		readTimeout time.Duration
		wantError   bool
	}{
		{name: "succeeds", server: func() *capServer { return &capServer{} }, readTimeout: DefaultReadTimeout},
		{name: "submit rejected", server: func() *capServer { return &capServer{submitStatus: http.StatusNotFound} }, readTimeout: DefaultReadTimeout, wantError: true},
		{name: "operation fails", server: func() *capServer { return &capServer{finalStatus: "failed"} }, readTimeout: DefaultReadTimeout, wantError: true},
		{name: "operation cancelled elsewhere", server: func() *capServer { return &capServer{finalStatus: "canceled"} }, readTimeout: DefaultReadTimeout, wantError: true},
		{name: "poll not found", server: func() *capServer { return &capServer{pollNotFound: true} }, readTimeout: DefaultReadTimeout, wantError: true},
		{name: "results too large", server: func() *capServer { return &capServer{resultsStatus: http.StatusRequestEntityTooLarge} }, readTimeout: DefaultReadTimeout, wantError: true},
		{name: "read timeout", server: func() *capServer { return &capServer{runFor: time.Hour} }, readTimeout: time.Minute, wantError: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				srv := newCapServer(t, tt.server())
				client := newAsyncTestClient(t, srv.Server)
				limiter := newReportRunLimiter(maxConcurrentReportRuns)

				_, diags := runAsyncReportWith(t.Context(), limiter, client, "query", tt.readTimeout, inlineSubmit(client), nil)

				if diags.HasError() != tt.wantError {
					t.Fatalf("HasError = %v, want %v: %v", diags.HasError(), tt.wantError, diags)
				}
				if !limiter.idle(maxConcurrentReportRuns) {
					t.Error("a slot is still held after the run ended")
				}
			})
		})
	}
}

// TestRunAsyncReport_ReleasesItsSlotBeforeFetchingResults pins the release
// point: the API stops counting an operation when it finishes, so a slow results
// fetch must not hold the next report back.
func TestRunAsyncReport_ReleasesItsSlotBeforeFetchingResults(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		const (
			runFor       = 10 * time.Second
			resultsDelay = 2 * time.Minute // under DefaultRequestTimeout, which bounds one request
		)

		srv := newCapServer(t, &capServer{runFor: runFor, resultsDelay: resultsDelay})
		client := newAsyncTestClient(t, srv.Server)
		limiter := newReportRunLimiter(1)

		results := runMany(2, func(int) diag.Diagnostics {
			_, diags := runAsyncReportWith(t.Context(), limiter, client, "query", 10*time.Minute, inlineSubmit(client), nil)
			return diags
		})

		if failed := failedRuns(results); len(failed) != 0 {
			t.Fatalf("a run failed:\n%s", strings.Join(failed, "\n"))
		}
		obs := srv.observe()
		if len(obs.acceptedAt) != 2 {
			t.Fatalf("accepted runs = %d, want 2", len(obs.acceptedAt))
		}
		// The second submits as soon as the first operation finishes, not after
		// the first has also fetched its slow result.
		if got := obs.acceptedAt[1]; got != runFor {
			t.Errorf("second run submitted at %v, want %v", got, runFor)
		}
	})
}
