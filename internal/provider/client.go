package provider

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/cenkalti/backoff/v5"
	"github.com/doitintl/terraform-provider-doit/internal/provider/models"
	"golang.org/x/oauth2"
)

// DCIRetryClient wraps an HTTP client with retry logic tailored for the DoiT Console API (DCI).
//
// # Why a Custom Client?
//
// This client is specifically designed for the DoiT Console API and has opinionated
// behavior that may not be appropriate for general-purpose HTTP clients. Key differences
// from standard retry libraries (like go-retryablehttp):
//
// # 404 Passthrough (Critical for Terraform)
//
// This client explicitly passes 404 responses through WITHOUT treating them as errors.
// This is essential for Terraform resource lifecycle:
//   - Read: 404 means "externally deleted" → remove from state
//   - Delete: 404 means "already gone" → success (idempotent)
//   - Create/Update: 404 after operation → handled by resource logic
//
// Standard retry libraries would block or error on 404, breaking Terraform semantics.
//
// # Retry Strategy
//
// Only specific transient errors trigger retries:
//   - 429 (Too Many Requests): Respects Retry-After header (seconds or HTTP-date format)
//   - 502 (Bad Gateway): Temporary upstream issue
//   - 503 (Service Unavailable): Temporary server overload
//   - 504 (Gateway Timeout): Temporary timeout
//
// All other 4xx/5xx errors are treated as permanent failures (no retry).
//
// # NOT Suitable For
//
// Do NOT use this client for:
//   - Non-DCI APIs that expect standard 404 handling
//   - APIs where 500 should be retried (we don't retry 500)
//   - APIs with different retry semantics
//
// If you need a general-purpose retry client, use go-retryablehttp instead.
type DCIRetryClient struct {
	client *http.Client
}

// Do executes an HTTP request with retry logic for transient errors.
//
// # Request Body Handling
//
// The request body is buffered on first read to allow re-sending on retries.
// This is necessary because http.Request.Body is a one-time stream.
//
// # Status Code Behavior
//
// | Status Code | Behavior |
// |-------------|----------|
// | 200, 201, 202, 204 | Success - return response |
// | 404 | Pass through - NOT an error (for Terraform resource semantics) |
// | 429 | Retry with Retry-After or exponential backoff |
// | 502, 503, 504 | Retry with exponential backoff |
// | Other 4xx/5xx | Permanent error - no retry |
//
// # Timeout
//
// Operations have a maximum elapsed time of 2 minutes, after which they fail.
func (c *DCIRetryClient) Do(req *http.Request) (*http.Response, error) {
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
			http.StatusNotFound: // 404 - INTENTIONALLY passed through for Terraform resource semantics
			// These codes don't need retry - return response for downstream handling
			// Note: 404 is NOT an error here. Resource handlers interpret it contextually:
			// - Read: externally deleted → remove from state
			// - Delete: already gone → success
			return resp, nil

		default:
			// All other status codes are considered permanent errors
			// This includes:
			// - 4xx client errors (400, 401, 403, etc.)
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
//
// The terraformVersion and providerVersion parameters are used to construct
// a User-Agent header following HashiCorp conventions:
//
//	Terraform/{tfVersion} terraform-provider-doit/{provVersion}
//
// The TF_APPEND_USER_AGENT environment variable is also respected, allowing
// users to append custom identifiers (e.g., CI system, org name).
func NewClient(ctx context.Context, host, apiToken, customerContext, terraformVersion, providerVersion string) (*models.ClientWithResponses, error) {
	retryClient := &DCIRetryClient{
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
	}

	ts := oauth2.StaticTokenSource(
		&oauth2.Token{AccessToken: apiToken},
	)

	userAgent := fmt.Sprintf("Terraform/%s terraform-provider-doit/%s", terraformVersion, providerVersion)
	if add := os.Getenv("TF_APPEND_USER_AGENT"); add != "" {
		userAgent += " " + strings.TrimSpace(add)
	}

	client, err := models.NewClientWithResponses(host,
		models.WithHTTPClient(retryClient),
		models.WithRequestEditorFn(func(_ context.Context, req *http.Request) error {
			token, err := ts.Token()
			if err != nil {
				return err
			}
			token.SetAuthHeader(req)
			req.Header.Set("User-Agent", userAgent)
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
