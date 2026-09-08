// Package provider — shared plumbing for the async report execution API.
//
// Cloud Analytics reports run as async operations (CMP-44423): the caller
// submits a run, receives 202 with an operation handle, polls until the
// operation reaches a terminal state, then fetches the result from a separate
// endpoint. This file holds the parts both report data sources share; each one
// supplies only its own submit call.
//
// Because no single HTTP request stays open while the report runs, the API's
// ~120s edge timeout does not bound how long a report may take, and neither
// does the provider's request_timeout. The only limit is the caller's Read
// timeout, which covers the whole submit-poll-fetch cycle; when it expires the
// operation is cancelled rather than left running.
package provider

import (
	"context"
	"fmt"
	"net/http"
	"time"
	"uuid"

	"github.com/doitintl/terraform-provider-doit/internal/provider/models"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-log/tflog"
)

// asyncSubmission is the part of a submit response the shared logic needs,
// normalized so run-by-id and inline runs can share one code path despite
// having different generated response types.
type asyncSubmission struct {
	StatusCode  int
	OperationID string
	Body        []byte
}

// asyncSubmitFunc issues one submit attempt with the given Idempotency-Key.
type asyncSubmitFunc func(ctx context.Context, idempotencyKey string) (asyncSubmission, error)

// asyncOperationID picks the operation ID out of a submit response. A real run
// answers 202, but the generated client models 200 (the dry-run shape) with the
// same envelope, so both are checked rather than assuming which one is set.
func asyncOperationID(envelopes ...*models.AsyncOperationResponse) string {
	for _, envelope := range envelopes {
		if envelope != nil && envelope.OperationId != nil {
			return *envelope.OperationId
		}
	}
	return ""
}

// newIdempotencyKey mints a key for a single submit attempt.
//
// Version 7 is time-ordered, so keys sort by when they were minted — which makes
// correlating one with a log line or an operation record straightforward.
func newIdempotencyKey() string {
	return uuid.NewV7().String()
}

// submitAsyncReport submits a report run and returns the operation ID.
//
// The retry here is narrow and deliberate. DCIRetryClient retries 502/503/504
// beneath this layer, reusing the same Idempotency-Key, and the API answers a
// replayed key with 202 carrying an empty body and no Location header. Left
// alone that strands a running BigQuery job we can neither poll nor cancel.
//
// Re-submitting with a fresh key recovers it: deduplication is content-based
// rather than key-based, so an identical config attaches to the operation
// already in flight instead of starting a second one. Exactly one extra attempt
// is made — a replay that repeats is a server-side problem, not something to
// hammer.
func submitAsyncReport(ctx context.Context, what string, submit asyncSubmitFunc) (string, diag.Diagnostics) {
	var diags diag.Diagnostics

	for attempt := range 2 {
		sub, err := submit(ctx, newIdempotencyKey())
		if err != nil {
			diags.AddError(
				"Error Running "+what,
				fmt.Sprintf("Could not submit the %s run: %s", what, err.Error()),
			)
			return "", diags
		}

		if sub.StatusCode != http.StatusAccepted && sub.StatusCode != http.StatusOK {
			diags.AddError(
				"Error Running "+what,
				asyncSubmitErrorDetail(what, sub),
			)
			return "", diags
		}

		if sub.OperationID != "" {
			return sub.OperationID, diags
		}

		// Empty accepted response: the idempotent-replay case described above.
		tflog.Warn(ctx, "Async submit returned no operation ID; re-submitting with a fresh Idempotency-Key", map[string]any{
			"what":    what,
			"attempt": attempt + 1,
			"status":  sub.StatusCode,
		})
	}

	diags.AddError(
		"Error Running "+what,
		fmt.Sprintf("The API accepted the %s run but returned no operation ID, twice. "+
			"This happens when a submission is replayed against an already-used Idempotency-Key. "+
			"The report may still be running server-side; retry shortly.", what),
	)
	return "", diags
}

// asyncSubmitErrorDetail builds the detail for a failed submit, always
// including status and body (required by the errformat linter, and the only
// reliable error content — the shared error schema is a bare {"error": string}
// that most of these endpoints do not actually use).
func asyncSubmitErrorDetail(what string, sub asyncSubmission) string {
	detail := fmt.Sprintf("Could not submit the %s run, status: %d, body: %s",
		what, sub.StatusCode, string(sub.Body))
	if sub.StatusCode == http.StatusNotFound {
		detail += "\n\nA 404 here means either the report does not exist, or this account " +
			"cannot reach the async report execution API."
	}
	return detail
}

// awaitAsyncReport polls operationID to a terminal state and returns its result.
//
// If ctx expires or is cancelled while the operation is still running, the
// operation is cancelled server-side before returning, so an abandoned run does
// not keep burning query capacity. This covers both the Read timeout elapsing
// and the user interrupting Terraform.
func awaitAsyncReport(
	ctx context.Context,
	client *models.ClientWithResponses,
	what, operationID string,
) (*models.GetAsyncOperationResults200Response, diag.Diagnostics) {
	var diags diag.Diagnostics

	for {
		pollResp, err := client.GetAsyncOperationWithResponse(ctx, operationID)
		if err != nil {
			if ctx.Err() != nil {
				return nil, asyncTimedOut(ctx, client, what, operationID)
			}
			diags.AddError(
				"Error Running "+what,
				fmt.Sprintf("Could not poll operation %s: %s", operationID, err.Error()),
			)
			return nil, diags
		}

		if pollResp.StatusCode() != http.StatusOK || pollResp.JSON200 == nil {
			// A 404 is the one status here with a non-obvious cause, so it gets
			// an explanation appended to the raw status and body.
			var hint string
			if pollResp.StatusCode() == http.StatusNotFound {
				hint = "\n\nThe operation no longer exists — it may have expired, " +
					"or it belongs to a different user or tenant."
			}
			diags.AddError(
				"Error Running "+what,
				fmt.Sprintf("Could not poll operation %s, status: %d, body: %s%s",
					operationID, pollResp.StatusCode(), string(pollResp.Body), hint),
			)
			return nil, diags
		}

		switch pollResp.JSON200.Status {
		case models.AsyncOperationPollResponseStatusSucceeded:
			return fetchAsyncReportResults(ctx, client, what, operationID)

		case models.AsyncOperationPollResponseStatusFailed:
			diags.AddError(
				"Error Running "+what,
				asyncFailureDetail(operationID, pollResp.JSON200.Error),
			)
			return nil, diags

		case models.AsyncOperationPollResponseStatusCanceled:
			diags.AddError(
				"Error Running "+what,
				fmt.Sprintf("Operation %s was canceled before it produced a result.", operationID),
			)
			return nil, diags
		}

		// Still pending or running — wait, honoring the API's own guidance.
		wait := asyncPollInitialInterval
		if pollResp.Headers200 != nil && pollResp.Headers200.RetryAfter != nil {
			if suggested, ok := parseRetryAfterBounded(
				*pollResp.Headers200.RetryAfter, time.Now(),
				asyncPollInitialInterval, asyncPollMaxInterval,
			); ok {
				wait = suggested
			}
		}

		timer := time.NewTimer(wait)
		select {
		case <-ctx.Done():
			timer.Stop()
			return nil, asyncTimedOut(ctx, client, what, operationID)
		case <-timer.C:
		}
	}
}

// fetchAsyncReportResults retrieves the result of a succeeded operation.
//
// A 425 is possible even after the poll reports success: the operation record
// and the stored result are updated separately, so the result can lag by a
// moment. The API documents a Retry-After on that response but does not
// actually send one, so fall back to the standard poll interval.
func fetchAsyncReportResults(
	ctx context.Context,
	client *models.ClientWithResponses,
	what, operationID string,
) (*models.GetAsyncOperationResults200Response, diag.Diagnostics) {
	var diags diag.Diagnostics

	for {
		resultsResp, err := client.GetAsyncOperationResultsWithResponse(ctx, operationID)
		if err != nil {
			diags.AddError(
				"Error Running "+what,
				fmt.Sprintf("Could not fetch results for operation %s: %s", operationID, err.Error()),
			)
			return nil, diags
		}

		if resultsResp.StatusCode() == http.StatusTooEarly {
			timer := time.NewTimer(asyncPollInitialInterval)
			select {
			case <-ctx.Done():
				// The operation already succeeded, so there is nothing running
				// to cancel — only the result fetch ran out of time.
				timer.Stop()
				diags.AddError(
					"Error Running "+what,
					fmt.Sprintf("Timed out waiting for the result of operation %s to become available: %s. "+
						"The operation succeeded, so retrying shortly should return it.",
						operationID, ctx.Err()),
				)
				return nil, diags
			case <-timer.C:
			}
			continue
		}

		if resultsResp.StatusCode() != http.StatusOK || resultsResp.JSON200 == nil {
			diags.AddError(
				"Error Running "+what,
				fmt.Sprintf("Could not fetch results for operation %s, status: %d, body: %s",
					operationID, resultsResp.StatusCode(), string(resultsResp.Body)),
			)
			return nil, diags
		}

		return resultsResp.JSON200, diags
	}
}

// asyncTimedOut cancels a still-running operation and reports the timeout.
//
// The cancel runs on a context detached from the expired one — reusing ctx
// would fail instantly, which is the whole reason the operation would otherwise
// be left running. A cancel that fails is reported as a warning rather than an
// error: the timeout is what the user needs to act on, and the warning names the
// operation so it can be cancelled from the console.
func asyncTimedOut(
	ctx context.Context,
	client *models.ClientWithResponses,
	what, operationID string,
) diag.Diagnostics {
	var diags diag.Diagnostics

	cancelCtx, stop := context.WithTimeout(context.WithoutCancel(ctx), asyncCancelGrace)
	defer stop()

	cancelResp, err := client.CancelAsyncOperationWithResponse(cancelCtx, operationID,
		&models.CancelAsyncOperationParams{IdempotencyKey: newIdempotencyKey()})

	// outcome describes what actually became of the operation, and is appended
	// to the timeout error. It stays empty when cancellation did not demonstrably
	// happen, so the error never claims an outcome the warning contradicts.
	var outcome string

	switch {
	case err != nil:
		diags.AddWarning(
			"Could Not Cancel Report Operation",
			fmt.Sprintf("Operation %s timed out but could not be canceled: %s. "+
				"It may still be running; cancel it from the DoiT console if needed.",
				operationID, err.Error()),
		)
	case cancelResp.StatusCode() != http.StatusOK:
		diags.AddWarning(
			"Could Not Cancel Report Operation",
			fmt.Sprintf("Operation %s timed out but could not be canceled, status: %d, body: %s. "+
				"It may still be running; cancel it from the DoiT console if needed.",
				operationID, cancelResp.StatusCode(), string(cancelResp.Body)),
		)
	default:
		// Cancelling is idempotent: an operation that already reached a terminal
		// state comes back unchanged rather than as "canceled". Report the state
		// the API returned instead of assuming the cancel took effect.
		outcome = fmt.Sprintf("A cancellation request was accepted for operation %s.", operationID)
		if cancelResp.JSON200 != nil && cancelResp.JSON200.Status != nil {
			status := *cancelResp.JSON200.Status
			if status == models.AsyncOperationResponseStatusCanceled {
				outcome = fmt.Sprintf("Operation %s was canceled.", operationID)
			} else {
				outcome = fmt.Sprintf("Operation %s had already finished (%s) and was left as-is.",
					operationID, status)
			}
		}
		tflog.Debug(ctx, "Requested cancellation of async report operation after timeout", map[string]any{
			"operation_id": operationID,
			"outcome":      outcome,
		})
	}

	if outcome == "" {
		outcome = fmt.Sprintf("Operation %s may still be running.", operationID)
	}

	diags.AddError(
		"Timed Out Running "+what,
		fmt.Sprintf("The %s did not finish within the read timeout: %s. %s\n\n"+
			"Increase the timeout for slow reports, for example:\n\n"+
			"  timeouts = {\n    read = \"30m\"\n  }",
			what, ctx.Err(), outcome),
	)
	return diags
}

// asyncFailureDetail renders a failed operation's error, which carries more
// detail than the HTTP layer does.
func asyncFailureDetail(operationID string, opErr *models.AsyncOperationError) string {
	if opErr == nil {
		return fmt.Sprintf("Operation %s failed, but the API reported no error detail.", operationID)
	}

	detail := fmt.Sprintf("Operation %s failed: %s (code: %s, status: %d)",
		operationID, opErr.Title, opErr.Code, opErr.Status)
	if opErr.Detail != nil && *opErr.Detail != "" {
		detail += "\n\n" + *opErr.Detail
	}
	return detail
}
