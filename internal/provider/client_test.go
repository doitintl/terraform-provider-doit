package provider

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
)

// captureUserAgent returns an httptest.Server that records the User-Agent
// header of the first request it receives, and a function to retrieve it.
func captureUserAgent(t *testing.T) (*httptest.Server, func() string) {
	t.Helper()
	var (
		mu sync.Mutex
		ua string
	)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		ua = r.Header.Get("User-Agent")
		mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"email":"test@example.com"}`))
	}))
	t.Cleanup(server.Close)
	return server, func() string {
		mu.Lock()
		defer mu.Unlock()
		return ua
	}
}

func TestNewClient_UserAgent(t *testing.T) {
	// Not parallel — uses t.Setenv
	t.Setenv("TF_APPEND_USER_AGENT", "") // ensure clean env

	server, getUA := captureUserAgent(t)

	client, err := NewClient(context.Background(), server.URL, "test-token", "", "1.9.0", "1.0.0")
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}
	if client == nil {
		t.Fatal("NewClient() returned nil client")
	}

	expected := "Terraform/1.9.0 terraform-provider-doit/1.0.0"
	if got := getUA(); got != expected {
		t.Errorf("User-Agent = %q, want %q", got, expected)
	}
}

func TestNewClient_UserAgentDev(t *testing.T) {
	// Not parallel — uses t.Setenv
	t.Setenv("TF_APPEND_USER_AGENT", "") // ensure clean env

	server, getUA := captureUserAgent(t)

	client, err := NewClient(context.Background(), server.URL, "test-token", "", "1.9.0", "dev")
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}
	if client == nil {
		t.Fatal("NewClient() returned nil client")
	}

	expected := "Terraform/1.9.0 terraform-provider-doit/dev"
	if got := getUA(); got != expected {
		t.Errorf("User-Agent = %q, want %q", got, expected)
	}
}

func TestNewClient_UserAgentAppend(t *testing.T) {
	// Not parallel — uses t.Setenv
	t.Setenv("TF_APPEND_USER_AGENT", "my-ci-system/2.0")

	server, getUA := captureUserAgent(t)

	client, err := NewClient(context.Background(), server.URL, "test-token", "", "1.9.0", "1.0.0")
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}
	if client == nil {
		t.Fatal("NewClient() returned nil client")
	}

	expected := "Terraform/1.9.0 terraform-provider-doit/1.0.0 my-ci-system/2.0"
	if got := getUA(); got != expected {
		t.Errorf("User-Agent = %q, want %q", got, expected)
	}
}
