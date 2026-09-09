package testserver

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func handler() http.Handler { return http.NotFoundHandler() }

func TestBad_DeferredClose(t *testing.T) {
	server := httptest.NewTestServer(t, handler())
	defer server.Close() // want `needs no explicit Close`
	_ = server
}

func TestBad_RegisteredClose(t *testing.T) {
	server := httptest.NewTestServer(t, handler())
	t.Cleanup(server.Close) // want `needs no explicit Close`
}

func TestBad_PromotedCloseThroughEmbedding(t *testing.T) {
	// Types as *wrapper, not *httptest.Server. Resolving the expression's type
	// would miss this; the declaring type is what matters.
	w := &wrapper{Server: httptest.NewTestServer(t, handler())}
	defer w.Close() // want `needs no explicit Close`
}

func TestBad_NewServerIsBanned(t *testing.T) {
	// Both checks fire here, deliberately. This Close is genuinely required for
	// a NewServer-built server, which is why the cleanup diagnostic states the
	// rule ("a server from NewTestServer needs no explicit Close") rather than
	// claiming this server's cleanup is already registered. Read together they
	// say: switch the constructor, then drop this line.
	server := httptest.NewServer(handler()) // want `use httptest.NewTestServer`
	defer server.Close()                    // want `needs no explicit Close`
}

func TestBad_NewTLSServerIsBanned(t *testing.T) {
	_ = httptest.NewTLSServer(handler()) // want `use httptest.NewTestServer`
}

func TestBad_NewUnstartedServerIsBanned(t *testing.T) {
	_ = httptest.NewUnstartedServer(handler()) // want `use httptest.NewTestServer`
}

func TestGood_NoExplicitClose(t *testing.T) {
	server := httptest.NewTestServer(t, handler())
	_ = server.URL
}

func TestGood_ImmediateCloseIsAllowed(t *testing.T) {
	// Closing early to assert client behavior against a refused connection is
	// legitimate; only a deferred or registered close is redundant.
	server := httptest.NewTestServer(t, handler())
	server.Close()
}

func TestGood_UnrelatedCloseIsIgnored(t *testing.T) {
	// Same method name, different type: must not be flagged, which is what
	// protects the response-body closes in the real tests.
	var other notAServer
	defer other.Close() //nolint:errcheck // testdata
}

func TestGood_CleanupOfSomethingElse(t *testing.T) {
	var other notAServer
	t.Cleanup(func() { _ = other.Close() })
}
