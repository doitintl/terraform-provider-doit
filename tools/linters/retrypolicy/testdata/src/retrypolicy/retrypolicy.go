// Package retrypolicy is testdata: a stand-in for internal/provider, holding
// the shapes the analyzer matches on. The analyzer matches DCIRetryClient by
// type name, so a local stub exercises the real code path.
package retrypolicy

import "net/http"

// BackOff stands in for backoff.BackOff.
type BackOff interface {
	NextBackOff() int
}

// DCIRetryClient mirrors the real struct's relevant fields.
type DCIRetryClient struct {
	client     *http.Client
	newBackOff func() BackOff
}

func constantBackOff(_ int) func() BackOff {
	return func() BackOff { return nil }
}

func newTestRetryClient(_ *http.Client, _ int, newBackOff func() BackOff) *DCIRetryClient {
	// Sets the field, so this literal is not flagged — and it lives in a
	// non-test file here, which is a second reason.
	return &DCIRetryClient{newBackOff: newBackOff}
}

func newClientWithHTTPClient(httpClient *http.Client) *DCIRetryClient {
	// The production constructor: leaves newBackOff nil on purpose. Not flagged,
	// because this is not a test file.
	return &DCIRetryClient{client: httpClient}
}

var _ = newClientWithHTTPClient
var _ = newTestRetryClient
var _ = constantBackOff
