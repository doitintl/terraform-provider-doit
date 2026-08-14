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
	"github.com/hashicorp/terraform-plugin-log/tflog"
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
// All other 4xx/5xx errors are treated as permanent failures (no retry). This
// deliberately includes 524 — see httpStatusCloudflareTimeout.
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

	// newBackOff builds the retry policy for a single Do call. When nil,
	// newRetryBackOff is used. Tests inject a fast policy so they do not sleep
	// for real — the backoff library's timer hook is unexported, so this is the
	// only way to control retry timing.
	//
	// This is deliberately a factory rather than a backoff.BackOff value:
	// ExponentialBackOff is not thread-safe and NextBackOff mutates its own
	// state, while one DCIRetryClient is shared by every resource for the whole
	// provider lifetime and Terraform runs operations concurrently. A shared
	// instance would have concurrent Do calls resetting and advancing each
	// other's intervals.
	newBackOff func() backoff.BackOff
}

// httpStatusCloudflareTimeout is Cloudflare's non-standard 524 "A Timeout
// Occurred" status, returned when the origin does not respond within the edge
// timeout (see cloudflareEdgeTimeout). Go's net/http has no constant for it.
//
// It is intentionally NOT retryable: the query already ran for the full edge
// timeout, so an immediate identical retry would only re-run an expensive query
// that is going to time out again. Falling through to the permanent-error branch
// turns it into one fast, self-explanatory failure.
const httpStatusCloudflareTimeout = 524

// Retry policy tuning. The DoiT API returns 429 without a Retry-After header,
// so the exponential policy — not the server's guidance — governs the pace of
// nearly every retry. It therefore starts conservatively: a 429 means the
// backend is already under pressure, and retrying a few hundred milliseconds
// later just sustains the condition.
const (
	retryInitialInterval = 2 * time.Second
	retryMultiplier      = 2.0
	retryMaxInterval     = 60 * time.Second

	// maxRetryAfter caps how long a server-supplied Retry-After is honored.
	// Without it, an outsized value (e.g. "86400") would sit and burn the whole
	// operation budget doing nothing: the retry loop waits on the context, so
	// once the wait reaches the operation timeout no retry can ever happen.
	//
	// It is deliberately tied to retryMaxInterval — we never wait longer than
	// our own policy's ceiling — which also keeps it well inside the smallest
	// operation default, so a capped wait still leaves budget for the retry it
	// was waiting for.
	maxRetryAfter = retryMaxInterval
)

// The Retry-After cap is only meaningful if a capped wait still leaves room for
// the retry itself. Enforced at compile time; see timeouts.go for the same
// technique applied to the timeout defaults.
const _ = uint(DefaultReadTimeout - maxRetryAfter - minRetryHeadroom)

// newRetryBackOff builds the default exponential retry policy.
//
// It returns the concrete type rather than backoff.BackOff so tests can assert
// on the configured fields directly.
func newRetryBackOff() *backoff.ExponentialBackOff {
	b := backoff.NewExponentialBackOff()
	b.InitialInterval = retryInitialInterval
	b.Multiplier = retryMultiplier
	b.MaxInterval = retryMaxInterval
	return b
}

// parseRetryAfter interprets an HTTP Retry-After header value, returning the
// duration to wait and whether the header was usable.
//
// Both forms from RFC 7231 are accepted: delay-seconds, and an HTTP-date (all
// three date formats, via http.ParseTime). An absent, malformed, or already-past
// value returns false, leaving the caller on exponential backoff.
//
// Non-positive values ("0", "-5", or a date already in the past) are rejected
// rather than honored as "retry immediately". Honoring them would be actively
// harmful twice over: it schedules a retry with no delay, and because the
// backoff library resets the exponential policy whenever a Retry-After is
// honored, a server repeatedly sending such a value would pin us to a flat
// cadence that never grows. Falling back to exponential backoff instead keeps
// the retries spreading out.
//
// An honored value is clamped to [retryInitialInterval, maxRetryAfter], so a
// date a few milliseconds out cannot produce a near-immediate retry either.
func parseRetryAfter(header string, now time.Time) (time.Duration, bool) {
	header = strings.TrimSpace(header)
	if header == "" {
		return 0, false
	}

	var wait time.Duration
	if seconds, err := strconv.Atoi(header); err == nil {
		// Reject and cap in units of seconds, before converting to a Duration:
		// seconds * time.Second overflows int64 for large magnitudes, and a
		// large negative value would wrap to a positive duration that then
		// looks like a legitimate wait.
		if seconds <= 0 {
			return 0, false
		}
		if seconds >= int(maxRetryAfter/time.Second) {
			return maxRetryAfter, true
		}
		wait = time.Duration(seconds) * time.Second
	} else {
		t, parseErr := http.ParseTime(header)
		if parseErr != nil {
			return 0, false
		}
		wait = t.Sub(now)
	}

	if wait <= 0 {
		return 0, false
	}
	return min(max(wait, retryInitialInterval), maxRetryAfter), true
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
// | 524 | Permanent error - no retry (Cloudflare edge timeout) |
// | Other 4xx/5xx | Permanent error - no retry |
//
// # Timeout
//
// The retry loop has no elapsed-time limit of its own (MaxElapsedTime is 0). It
// defers entirely to the deadline on the request's context, which is the
// Terraform operation timeout — see timeouts.go.
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

		resp, err := c.client.Do(req) //nolint:gosec // G704: host is operator-controlled provider config, paths are generated by oapi-codegen client
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

			if wait, ok := parseRetryAfter(retryAfter, time.Now()); ok {
				tflog.Debug(req.Context(), "Rate limited, honoring Retry-After", map[string]any{
					"url":         req.URL.String(),
					"retry_after": retryAfter,
					"wait":        wait.String(),
				})
				return nil, &backoff.RetryAfterError{Duration: wait}
			}
			tflog.Debug(req.Context(), "Rate limited, falling back to exponential backoff", map[string]any{
				"url":         req.URL.String(),
				"retry_after": retryAfter,
			})
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
			// - 524 (httpStatusCloudflareTimeout): the edge already waited out
			//   the full origin timeout, so retrying re-runs a query that will
			//   time out again. Failing fast here is what surfaces the 524 to
			//   the user instead of an opaque deadline error.
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

	// A fresh policy per call: ExponentialBackOff is not thread-safe, and this
	// client is shared across concurrent Terraform operations.
	newBackOff := c.newBackOff
	if newBackOff == nil {
		newBackOff = func() backoff.BackOff { return newRetryBackOff() }
	}

	// Retry with exponential backoff. MaxElapsedTime is disabled (0) so the
	// retry loop defers entirely to the provided context's deadline
	// (e.g., Terraform's timeouts {} block).
	return backoff.Retry(req.Context(), operation,
		backoff.WithBackOff(newBackOff()),
		backoff.WithMaxElapsedTime(0),
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
func NewClient(ctx context.Context, host, apiToken, customerContext, terraformVersion, providerVersion string, requestTimeout time.Duration) (*models.ClientWithResponses, error) {
	retryClient := &DCIRetryClient{
		client: &http.Client{
			Timeout: requestTimeout,
		},
	}

	ts := oauth2.StaticTokenSource(
		&oauth2.Token{AccessToken: apiToken},
	)

	userAgent := fmt.Sprintf("Terraform/%s terraform-provider-doit/%s", terraformVersion, providerVersion)
	if add := strings.TrimSpace(os.Getenv("TF_APPEND_USER_AGENT")); add != "" {
		userAgent += " " + add
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
				// DoiT employees scope requests to a customer via the
				// X-Tenant-Id header. This replaced the former
				// customerContext query parameter.
				req.Header.Set("X-Tenant-Id", customerContext)
			}
			return nil
		}))
	if err != nil {
		return nil, err
	}
	// Validate the API token with a bounded context derived from
	// requestTimeout so initialization does not hang indefinitely.
	validateCtx, validateCancel := context.WithTimeout(ctx, requestTimeout)
	defer validateCancel()
	validateResp, err := client.Validate(validateCtx)
	if validateResp != nil && validateResp.Body != nil {
		validateResp.Body.Close()
	}
	if err != nil {
		return nil, err
	}
	return client, nil
}
