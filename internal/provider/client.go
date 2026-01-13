package provider

import (
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"

	"github.com/cenkalti/backoff/v4"
	"github.com/doitintl/terraform-provider-doit/internal/provider/models"
	"golang.org/x/oauth2"
)

// RetryClient wraps an HTTP client with retry logic.
type RetryClient struct {
	client *http.Client
}

// Do executes an HTTP request with retry logic for transient errors.
func (c *RetryClient) Do(req *http.Request) (*http.Response, error) {
	var resp *http.Response
	var err error

	operation := func() error {
		resp, err = c.client.Do(req)
		if err != nil {
			return err
		}

		// Retryable status codes:
		// - 429: Too Many Requests (rate limiting)
		// - 502: Bad Gateway (temporary upstream issue)
		// - 503: Service Unavailable (temporary server overload)
		// - 504: Gateway Timeout (temporary timeout)
		switch resp.StatusCode {
		case http.StatusTooManyRequests:
			// Respect Retry-After header if present
			retryAfter := resp.Header.Get("Retry-After")
			if closeErr := resp.Body.Close(); closeErr != nil {
				log.Printf("[WARN] Error closing response body: %v", closeErr)
			}

			if retryAfter != "" {
				// Parse Retry-After as duration in seconds
				if duration, parseErr := time.ParseDuration(retryAfter + "s"); parseErr == nil {
					// Sleep for the requested duration, respecting context cancellation
					// TODO: replace with backoff.RetryAfter after upgrade to backoff v5
					time.Sleep(duration)
				}
				// If parsing fails, fall back to exponential backoff
			}
			return fmt.Errorf("rate limit exceeded: %d", resp.StatusCode)

		case http.StatusBadGateway, http.StatusServiceUnavailable, http.StatusGatewayTimeout:
			// Temporary server errors - retry with backoff
			if closeErr := resp.Body.Close(); closeErr != nil {
				log.Printf("[WARN] Error closing response body: %v", closeErr)
			}
			return fmt.Errorf("temporary server error: %d", resp.StatusCode)

		case http.StatusOK, http.StatusCreated, http.StatusAccepted, http.StatusNoContent:
			// Success codes - no retry needed
			return nil

		default:
			// All other status codes are considered permanent errors
			// This includes:
			// - 4xx client errors (400, 401, 403, 404, etc.)
			// - 5xx server errors that shouldn't be retried (500, 501, etc.)
			if resp.StatusCode >= 400 {
				bodyBytes, readErr := io.ReadAll(resp.Body)
				closeErr := resp.Body.Close()
				if readErr != nil {
					return backoff.Permanent(fmt.Errorf("non-retryable error: %d, failed to read body: %w", resp.StatusCode, readErr))
				}
				if closeErr != nil {
					return backoff.Permanent(fmt.Errorf("non-retryable error: %d, body: %s, failed to close body: %w", resp.StatusCode, string(bodyBytes), closeErr))
				}
				return backoff.Permanent(fmt.Errorf("non-retryable error: %d, body: %s", resp.StatusCode, string(bodyBytes)))
			}
			// 2xx and 3xx codes that aren't explicitly handled above
			return nil
		}
	}

	// Use exponential backoff
	b := backoff.NewExponentialBackOff()
	b.MaxElapsedTime = 2 * time.Minute // Reasonable timeout for Terraform operations

	// Retry with exponential backoff
	err = backoff.Retry(operation, b)

	if err != nil {
		// If we exhausted retries or encountered a permanent error, return it
		// Note: resp.Body will be closed if we retried, so the caller shouldn't try to read it
		return resp, err
	}

	return resp, nil
}

// NewClient creates a new API client with retry logic.
func NewClient(ctx context.Context, host, apiToken, customerContext string) (*models.ClientWithResponses, error) {
	retryClient := &RetryClient{
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
	}

	ts := oauth2.StaticTokenSource(
		&oauth2.Token{AccessToken: apiToken},
	)

	client, err := models.NewClientWithResponses(host,
		models.WithHTTPClient(retryClient),
		models.WithRequestEditorFn(func(_ context.Context, req *http.Request) error {
			token, err := ts.Token()
			if err != nil {
				return err
			}
			token.SetAuthHeader(req)
			if customerContext != "" {
				url := req.URL.Query()
				url.Set("customerContext", customerContext)
				req.URL.RawQuery = url.Encode()
			}
			return nil
		}))
	if err != nil {
		return nil, err
	}
	_, err = client.Validate(ctx)
	if err != nil {
		return nil, err
	}
	return client, nil
}
