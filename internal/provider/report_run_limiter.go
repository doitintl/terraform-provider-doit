package provider

import (
	"context"
	"sync"

	"github.com/doitintl/terraform-provider-doit/internal/provider/models"
	"golang.org/x/sync/semaphore"
)

// maxConcurrentReportRuns is how many report runs one configured client has in
// flight at once. It mirrors the API's cap on pending or running report
// operations per user (3 at the time of writing).
//
// It is a hint, not the source of truth. The API still answers a run beyond its
// cap with a 429, which the retry client waits out: other Terraform runs, other
// provider aliases and other users share the cap and are invisible here.
const maxConcurrentReportRuns = 3

// reportRunLimiter queues the report runs of one configured client so that a
// single Terraform run, which reads independent data sources in parallel, never
// exceeds the API's cap by itself.
//
// Waiters are served in arrival order. That replaces waiting on the API's 429,
// where every waiter wakes on the same 30 second cycle and keeps losing races
// for a free slot.
type reportRunLimiter struct {
	sem *semaphore.Weighted
}

func newReportRunLimiter(runs int64) *reportRunLimiter {
	return &reportRunLimiter{sem: semaphore.NewWeighted(runs)}
}

// reportRunLimiters holds one limiter per client, which is one per Terraform
// run. A registry keeps the limiter out of ProviderData: every data source and
// resource asserts that to *models.ClientWithResponses, and only the two report
// data sources need a limiter.
//
// Entries are never removed. A provider process configures one client, and a
// test binary a few hundred, each entry a few words.
var reportRunLimiters sync.Map // *models.ClientWithResponses -> *reportRunLimiter

// reportRunLimiterFor returns the limiter shared by every report run made
// through client.
func reportRunLimiterFor(client *models.ClientWithResponses) *reportRunLimiter {
	if limiter, ok := reportRunLimiters.Load(client); ok {
		return limiter.(*reportRunLimiter) //nolint:forcetypeassert // only ever stores *reportRunLimiter
	}
	limiter, _ := reportRunLimiters.LoadOrStore(client, newReportRunLimiter(maxConcurrentReportRuns))
	return limiter.(*reportRunLimiter) //nolint:forcetypeassert // only ever stores *reportRunLimiter
}

// acquire waits for a free slot or for ctx to end. The returned release is safe
// to call more than once, so every exit path of a run can call it.
func (l *reportRunLimiter) acquire(ctx context.Context) (release func(), err error) {
	if err := l.sem.Acquire(ctx, 1); err != nil {
		return nil, err
	}
	var once sync.Once
	return func() { once.Do(func() { l.sem.Release(1) }) }, nil
}
