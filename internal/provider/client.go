package provider

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/cenkalti/backoff/v5"
	"github.com/doitintl/terraform-provider-doit/internal/provider/models"
	"golang.org/x/oauth2"
)

// RetryClient wraps an HTTP client with retry logic.
type RetryClient struct {
	client *http.Client
}

// Do executes an HTTP request with retry logic for transient errors.
func (c *RetryClient) Do(req *http.Request) (*http.Response, error) {
	// Preserve the original body for retries.
	// If the request has a body, we need to be able to re-read it on retries.
	var bodyBytes []byte
	if req.Body != nil && req.Body != http.NoBody {
		var err error
		bodyBytes, err = io.ReadAll(req.Body)
		if err != nil {
			return nil, fmt.Errorf("failed to read request body: %w", err)
		}
		if closeErr := req.Body.Close(); closeErr != nil {
			log.Printf("[WARN] Error closing original request body: %v", closeErr)
		}
	}

	operation := func() (*http.Response, error) {
		// Reset the body for each retry attempt
		if bodyBytes != nil {
			req.Body = io.NopCloser(bytes.NewReader(bodyBytes))
			req.ContentLength = int64(len(bodyBytes))
		}

		resp, err := c.client.Do(req)
		if err != nil {
			return nil, err
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
				// Try parsing as seconds (most common)
				if seconds, parseErr := strconv.Atoi(retryAfter); parseErr == nil {
					// Use backoff v5's integrated RetryAfter to respect the header
					return nil, backoff.RetryAfter(seconds)
				}
				// Try parsing as HTTP-date (RFC 7231)
				if t, parseErr := time.Parse(time.RFC1123, retryAfter); parseErr == nil {
					waitDuration := time.Until(t)
					if waitDuration > 0 {
						// Round up to ensure we wait at least the requested time
						seconds := int(waitDuration.Seconds()) + 1
						return nil, backoff.RetryAfter(seconds)
					}
				}
				// If parsing fails, fall back to exponential backoff
			}
			return nil, fmt.Errorf("rate limit exceeded: %d", resp.StatusCode)

		case http.StatusBadGateway, http.StatusServiceUnavailable, http.StatusGatewayTimeout:
			// Temporary server errors - retry with backoff
			if closeErr := resp.Body.Close(); closeErr != nil {
				log.Printf("[WARN] Error closing response body: %v", closeErr)
			}
			return nil, fmt.Errorf("temporary server error: %d", resp.StatusCode)

		case http.StatusOK, http.StatusCreated, http.StatusAccepted, http.StatusNoContent,
			http.StatusNotFound: // 404 - let oapi-codegen parse it for proper structured error handling
			// These codes don't need retry - return response for downstream handling
			return resp, nil

		default:
			// All other status codes are considered permanent errors
			// This includes:
			// - 4xx client errors (400, 401, 403, 404, etc.)
			// - 5xx server errors that shouldn't be retried (500, 501, etc.)
			if resp.StatusCode >= 400 {
				respBodyBytes, readErr := io.ReadAll(resp.Body)
				closeErr := resp.Body.Close()
				if readErr != nil {
					return nil, backoff.Permanent(fmt.Errorf("non-retryable error: %d, failed to read body: %w", resp.StatusCode, readErr))
				}
				if closeErr != nil {
					return nil, backoff.Permanent(fmt.Errorf("non-retryable error: %d, body: %s, failed to close body: %w", resp.StatusCode, string(respBodyBytes), closeErr))
				}
				return nil, backoff.Permanent(fmt.Errorf("non-retryable error: %d, body: %s", resp.StatusCode, string(respBodyBytes)))
			}
			// 2xx and 3xx codes that aren't explicitly handled above
			return resp, nil
		}
	}

	// Retry with exponential backoff and a 2-minute timeout
	return backoff.Retry(req.Context(), operation,
		backoff.WithBackOff(backoff.NewExponentialBackOff()),
		backoff.WithMaxElapsedTime(2*time.Minute), // Reasonable timeout for Terraform operations
	)
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
