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

// newTestRetryAPIClient wraps the constructor and takes the policy at a
// different position, which is why the nil check keys off the parameter type
// rather than a name-and-index list.
func newTestRetryAPIClient(_ *http.Client, _ int, newBackOff func() BackOff) *DCIRetryClient {
	return newTestRetryClient(nil, 0, newBackOff)
}

// takesAnyPointer exists to prove the nil check is type-selective: a nil passed
// for a non-factory parameter must not be flagged.
func takesAnyPointer(_ *http.Client) {}

var _ = newTestRetryAPIClient
var _ = takesAnyPointer
var _ = newClientWithHTTPClient
var _ = newTestRetryClient
var _ = constantBackOff
