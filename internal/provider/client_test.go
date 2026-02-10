package provider

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestNewClient_UserAgent(t *testing.T) {
	t.Parallel()

	// Capture the User-Agent header from actual requests
	var capturedUA string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedUA = r.Header.Get("User-Agent")
		// Return a valid JSON response for the Validate call
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"email":"test@example.com"}`))
	}))
	defer server.Close()

	client, err := NewClient(context.Background(), server.URL, "test-token", "", "1.9.0", "1.0.0")
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}
	if client == nil {
		t.Fatal("NewClient() returned nil client")
	}

	expected := "Terraform/1.9.0 terraform-provider-doit/1.0.0"
	if capturedUA != expected {
		t.Errorf("User-Agent = %q, want %q", capturedUA, expected)
	}
}

func TestNewClient_UserAgentDev(t *testing.T) {
	t.Parallel()

	var capturedUA string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedUA = r.Header.Get("User-Agent")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"email":"test@example.com"}`))
	}))
	defer server.Close()

	client, err := NewClient(context.Background(), server.URL, "test-token", "", "1.9.0", "dev")
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}
	if client == nil {
		t.Fatal("NewClient() returned nil client")
	}

	expected := "Terraform/1.9.0 terraform-provider-doit/dev"
	if capturedUA != expected {
		t.Errorf("User-Agent = %q, want %q", capturedUA, expected)
	}
}

func TestNewClient_UserAgentAppend(t *testing.T) {
	// Not parallel — modifies environment variable
	t.Setenv("TF_APPEND_USER_AGENT", "my-ci-system/2.0")

	var capturedUA string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedUA = r.Header.Get("User-Agent")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"email":"test@example.com"}`))
	}))
	defer server.Close()

	client, err := NewClient(context.Background(), server.URL, "test-token", "", "1.9.0", "1.0.0")
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}
	if client == nil {
		t.Fatal("NewClient() returned nil client")
	}

	expected := "Terraform/1.9.0 terraform-provider-doit/1.0.0 my-ci-system/2.0"
	if capturedUA != expected {
		t.Errorf("User-Agent = %q, want %q", capturedUA, expected)
	}
}
