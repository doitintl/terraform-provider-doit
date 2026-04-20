package provider

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// newTimeoutTestServer creates an httptest.Server that returns 200 for the
// /auth/v1/validate endpoint (required by NewClient) and delegates all other
// requests to the provided handler.
func newTimeoutTestServer(handler http.HandlerFunc) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/auth/v1/validate") {
			w.WriteHeader(http.StatusOK)
			return
		}
		handler(w, r)
	}))
}

// TestDCIRetryClient_RequestTimeout verifies that a per-request timeout is
// enforced: when the server takes longer than the configured timeout, the
// client returns a timeout error for each attempt. With MaxElapsedTime=0,
// retries continue until the context deadline.
func TestDCIRetryClient_RequestTimeout(t *testing.T) {
	t.Parallel()

	serverDelay := 3 * time.Second
	clientTimeout := 1 * time.Second

	var requestCount int64
	server := newTimeoutTestServer(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt64(&requestCount, 1)
		time.Sleep(serverDelay)
		w.WriteHeader(http.StatusOK)
	})
	defer server.Close()

	client, err := NewClient(
		context.Background(),
		server.URL, "test-token", "", "1.0.0", "dev", clientTimeout,
	)
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}

	// Use a bounded context — with MaxElapsedTime=0, retries continue
	// until the context deadline.
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	start := time.Now()
	_, err = client.GetAlertWithResponse(ctx, "test-id")
	elapsed := time.Since(start)

	if err == nil {
		t.Fatal("expected timeout error, got nil")
	}

	count := atomic.LoadInt64(&requestCount)

	// Should have made multiple attempts (each timing out at clientTimeout)
	if count < 2 {
		t.Fatalf("expected at least 2 attempts, got %d", count)
	}

	t.Logf("Made %d attempts in %v (per-request timeout %v)", count, elapsed, clientTimeout)
}

// TestDCIRetryClient_ContextCancellation verifies that a parent context
// cancellation propagates through the retry loop and stops retries early.
func TestDCIRetryClient_ContextCancellation(t *testing.T) {
	t.Parallel()

	clientTimeout := 30 * time.Second // High per-request timeout
	parentTimeout := 2 * time.Second  // Short parent context

	server := newTimeoutTestServer(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(5 * time.Second) // Always slow
		w.WriteHeader(http.StatusOK)
	})
	defer server.Close()

	client, err := NewClient(
		context.Background(),
		server.URL, "test-token", "", "1.0.0", "dev", clientTimeout,
	)
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), parentTimeout)
	defer cancel()

	start := time.Now()
	_, err = client.GetAlertWithResponse(ctx, "test-id")
	elapsed := time.Since(start)

	if err == nil {
		t.Fatal("expected context cancellation error, got nil")
	}

	// Should complete around the parent timeout, not the per-request timeout
	maxExpected := 2 * parentTimeout
	if elapsed > maxExpected {
		t.Fatalf("expected to complete within %v, took %v", maxExpected, elapsed)
	}
	t.Logf("Cancelled after %v (parent timeout %v)", elapsed, parentTimeout)
}

// TestNewClient_CustomTimeout verifies that NewClient accepts a custom timeout.
func TestNewClient_CustomTimeout(t *testing.T) {
	t.Parallel()

	server := newTimeoutTestServer(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	defer server.Close()

	client, err := NewClient(
		context.Background(),
		server.URL, "test-token", "", "1.0.0", "dev", 42*time.Second,
	)
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}
	if client == nil {
		t.Fatal("NewClient() returned nil client")
	}
}

// TestNewClient_DefaultTimeout verifies that DefaultRequestTimeout (120s)
// is a valid configuration value.
func TestNewClient_DefaultTimeout(t *testing.T) {
	t.Parallel()

	server := newTimeoutTestServer(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	defer server.Close()

	client, err := NewClient(
		context.Background(),
		server.URL, "test-token", "", "1.0.0", "dev", DefaultRequestTimeout,
	)
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}
	if client == nil {
		t.Fatal("NewClient() returned nil client")
	}
}

// TestDCIRetryClient_RetryRespectsContextDeadline verifies that the retry
// loop defers to the context deadline (MaxElapsedTime = 0). Retries continue
// until the context is cancelled, not until a hardcoded elapsed time.
func TestDCIRetryClient_RetryRespectsContextDeadline(t *testing.T) {
	t.Parallel()

	requestTimeout := 30 * time.Second // High per-request timeout
	contextDeadline := 3 * time.Second // Short context deadline

	var requestCount int64
	server := newTimeoutTestServer(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt64(&requestCount, 1)
		w.WriteHeader(http.StatusServiceUnavailable) // 503 triggers retry
	})
	defer server.Close()

	client, err := NewClient(
		context.Background(),
		server.URL, "test-token", "", "1.0.0", "dev", requestTimeout,
	)
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), contextDeadline)
	defer cancel()

	start := time.Now()
	_, err = client.GetAlertWithResponse(ctx, "test-id")
	elapsed := time.Since(start)

	if err == nil {
		t.Fatal("expected context deadline error, got nil")
	}

	count := atomic.LoadInt64(&requestCount)

	// Should have retried until the context deadline, not a hardcoded max elapsed time
	if elapsed > 2*contextDeadline {
		t.Fatalf("expected to complete around %v, took %v", contextDeadline, elapsed)
	}

	// Should have made multiple attempts within the deadline
	if count < 2 {
		t.Fatalf("expected at least 2 retry attempts, got %d", count)
	}

	t.Logf("Retried %d times in %v (context deadline %v)", count, elapsed, contextDeadline)
}
