package provider

import (
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
)

// captureUserAgent returns an httptest.Server that records the User-Agent
// header of the first request it receives, the *http.Client that reaches it,
// and a function to retrieve the header.
//
// The client is returned rather than left to the caller because
// httptest.NewTestServer starts lazily: Server.Client() is the call that starts
// it and assigns Server.URL. Go does not specify a plain field read's order
// against a method call in the same argument list, so `f(server.URL,
// server.Client())` is not guaranteed to see a started server. Returning both
// from here keeps every caller on the safe ordering.
func captureUserAgent(t *testing.T) (*httptest.Server, *http.Client, func() string) {
	t.Helper()
	var (
		mu sync.Mutex
		ua string
	)
	server := httptest.NewTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		ua = r.Header.Get("User-Agent")
		mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"email":"test@example.com"}`))
	}))
	t.Cleanup(server.Close)
	return server, server.Client(), func() string {
		mu.Lock()
		defer mu.Unlock()
		return ua
	}
}

func TestNewClient_UserAgent(t *testing.T) {
	// Not parallel — uses t.Setenv
	t.Setenv("TF_APPEND_USER_AGENT", "") // ensure clean env

	server, httpClient, getUA := captureUserAgent(t)

	client, err := newClientWithHTTPClient(server.URL, "test-token", "", "1.9.0", "1.0.0", httpClient)
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}
	if client == nil {
		t.Fatal("NewClient() returned nil client")
	}

	resp, err := client.Validate(t.Context())
	if err != nil {
		t.Fatalf("client.Validate() error = %v", err)
	}
	if resp != nil && resp.Body != nil {
		_ = resp.Body.Close()
	}

	expected := "Terraform/1.9.0 terraform-provider-doit/1.0.0"
	if got := getUA(); got != expected {
		t.Errorf("User-Agent = %q, want %q", got, expected)
	}
}

func TestNewClient_UserAgentDev(t *testing.T) {
	// Not parallel — uses t.Setenv
	t.Setenv("TF_APPEND_USER_AGENT", "") // ensure clean env

	server, httpClient, getUA := captureUserAgent(t)

	client, err := newClientWithHTTPClient(server.URL, "test-token", "", "1.9.0", "dev", httpClient)
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}
	if client == nil {
		t.Fatal("NewClient() returned nil client")
	}

	resp, err := client.Validate(t.Context())
	if err != nil {
		t.Fatalf("client.Validate() error = %v", err)
	}
	if resp != nil && resp.Body != nil {
		_ = resp.Body.Close()
	}

	expected := "Terraform/1.9.0 terraform-provider-doit/dev"
	if got := getUA(); got != expected {
		t.Errorf("User-Agent = %q, want %q", got, expected)
	}
}

func TestNewClient_UserAgentAppend(t *testing.T) {
	// Not parallel — uses t.Setenv
	t.Setenv("TF_APPEND_USER_AGENT", "my-ci-system/2.0")

	server, httpClient, getUA := captureUserAgent(t)

	client, err := newClientWithHTTPClient(server.URL, "test-token", "", "1.9.0", "1.0.0", httpClient)
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}
	if client == nil {
		t.Fatal("NewClient() returned nil client")
	}

	resp, err := client.Validate(t.Context())
	if err != nil {
		t.Fatalf("client.Validate() error = %v", err)
	}
	if resp != nil && resp.Body != nil {
		_ = resp.Body.Close()
	}

	expected := "Terraform/1.9.0 terraform-provider-doit/1.0.0 my-ci-system/2.0"
	if got := getUA(); got != expected {
		t.Errorf("User-Agent = %q, want %q", got, expected)
	}
}
